package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const HelpText = `The test proxy exposes the following control endpoints:

- POST /quit
  Shut down the proxy and exit.

- POST /restart
  Gracefully shut down the gRPC server, then start it again.

  - Query param: sleep=<duration>
    Forces the restart to block for the given duration; default: 0s.

  - Query param: forceful=<bool>
    If true, forces a non-graceful shutdown; default: false.

- POST /reject
  Immediately reject incoming gRPC requests with UNAVAILABLE.

- POST /accept
  Accept incoming gRPC requests; this is the default.

- POST /freeze
  Block on incoming accepted gRPC requests.

- POST /thaw
  Process incoming accepted gRPC requests immediately; this is the default.
`

var ErrUnknownCommand = errors.New("unknown command")

var (
	flagTrace   bool
	flagControl string
	flagListen  string
	flagDial    string

	gListenConfig  net.ListenConfig
	gExitCh        chan struct{}
	gRootContext   context.Context
	gServerMutex   sync.Mutex
	gControlServer ControlServer
	gProxyServer   ProxyServer

	gStateMutex     sync.Mutex
	gStateCond      *sync.Cond
	gStateRejecting bool
	gStateFrozen    bool
)

func init() {
	flag.BoolVar(&flagTrace, "trace", false, "enable tracing logs")
	flag.StringVar(&flagControl, "control", "", "TCP host:port to listen on for HTTP control commands")
	flag.StringVar(&flagListen, "listen", "", "TCP host:port to listen on for proxying to -dial")
	flag.StringVar(&flagDial, "dial", "", "TCP host:port to connect to")
}

func main() {
	flag.Parse()

	if flagControl == "" {
		Fatal(1, "must specify -control")
		panic(nil)
	}
	if flagListen == "" {
		Fatal(1, "must specify -listen")
		panic(nil)
	}
	if flagDial == "" {
		Fatal(1, "must specify -dial")
		panic(nil)
	}

	gExitCh = make(chan struct{})
	gStateCond = sync.NewCond(&gStateMutex)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	gRootContext = ctx

	gControlServer.Init(flagControl)
	err := gControlServer.Run(ctx)
	if err != nil {
		Fatal(2, "%v", err)
		panic(nil)
	}

	needControlClose := true
	defer func() {
		if needControlClose {
			gControlServer.ForceClose()
		}
	}()

	gProxyServer.Init(flagListen, flagDial)
	err = gProxyServer.Run(ctx)
	if err != nil {
		Fatal(2, "%v", err)
		panic(nil)
	}

	needProxyClose := true
	defer func() {
		if needProxyClose {
			gProxyServer.ForceClose()
		}
	}()

	Info("HTTP control server is running on: %s", flagControl)
	Info("gRPC proxy server is running on: %s", flagListen)
	Info("gRPC proxy server is connected to: %s", flagDial)

	select {
	case <-gExitCh:
	case <-ctx.Done():
	}

	Info("terminating")

	err = gControlServer.Shutdown(ctx)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to gracefully shut down HTTP control server: %v", err)
	}

	needControlClose = false
	gControlServer.ForceClose()

	gServerMutex.Lock()
	defer gServerMutex.Unlock()

	err = gProxyServer.Shutdown(ctx)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to gracefully shut down gRPC proxy server: %v", err)
	}

	needProxyClose = false
	gProxyServer.ForceClose()

	Info("done")
}

type queryKeyType struct{}

var queryKey queryKeyType

type ActionFunc = func(context.Context) error

func ActionQuit(ctx context.Context) error {
	close(gExitCh)
	Info("/quit: proxy is shutting down")
	return nil
}

func ActionRestart(ctx context.Context) error {
	q, _ := ctx.Value(queryKey).(url.Values)

	var sleep time.Duration
	if q.Has("sleep") {
		d, err := time.ParseDuration(q.Get("sleep"))
		if err != nil {
			return err
		}
		if d < 0 {
			d = 0
		}
		sleep = d
	}

	var forceful bool
	if q.Has("forceful") {
		b, err := strconv.ParseBool(q.Get("forceful"))
		if err != nil {
			return err
		}
		forceful = b
	}

	gServerMutex.Lock()
	defer gServerMutex.Unlock()

	Info("/restart: restarting proxy, forceful=%t", forceful)

	mode := "gracefully"
	fn := gProxyServer.Shutdown
	if forceful {
		mode = "forcefully"
		fn = func(context.Context) error {
			return gProxyServer.Close()
		}
	}

	err := fn(ctx)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to %s shut down gRPC proxy server: %v", mode, err)
	}

	gProxyServer.ForceClose()

	if sleep > 0 {
		Info("/restart: sleeping for %v", sleep)
		time.Sleep(sleep)
	}

	err = gProxyServer.Run(gRootContext)
	if err != nil {
		close(gExitCh)
		return err
	}

	Info("/restart: proxy has been restarted")
	return nil
}

func ActionReject(ctx context.Context) error {
	gStateMutex.Lock()
	defer gStateMutex.Unlock()

	if gStateRejecting {
		return nil
	}
	gStateRejecting = true
	Info("/reject: proxy is rejecting requests")
	return nil
}

func ActionAccept(ctx context.Context) error {
	gStateMutex.Lock()
	defer gStateMutex.Unlock()

	if !gStateRejecting {
		return nil
	}
	gStateRejecting = false
	Info("/accept: proxy is NOT rejecting requests")
	return nil
}

func ActionFreeze(ctx context.Context) error {
	gStateMutex.Lock()
	defer gStateMutex.Unlock()

	if gStateFrozen {
		return nil
	}
	gStateFrozen = true
	Info("/freeze: proxy is stalling requests")
	return nil
}

func ActionThaw(ctx context.Context) error {
	gStateMutex.Lock()
	defer gStateMutex.Unlock()

	if !gStateFrozen {
		return nil
	}
	gStateFrozen = false
	gStateCond.Broadcast()
	Info("/thaw: proxy is NOT stalling requests")
	return nil
}

func HandleHelp(w http.ResponseWriter, r *http.Request) {
	if path.Clean(r.URL.Path) != "/" {
		http.NotFound(w, r)
		return
	}
	if !CheckMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}
	body := []byte(strings.ReplaceAll(HelpText, "\n", "\r\n"))
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func HandleAction(action ActionFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckMethod(w, r, http.MethodPost) {
			return
		}

		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, queryKey, q)
		if err := action(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func CheckMethod(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, item := range allowed {
		if r.Method == item {
			return true
		}
	}
	h := w.Header()
	h.Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

type ControlServer struct {
	listen string
	mu     sync.Mutex
	cv     *sync.Cond
	l      net.Listener
	quitCh chan struct{}
	server http.Server
	mux    http.ServeMux
}

func (s *ControlServer) Init(listen string) {
	s.listen = listen
	s.cv = sync.NewCond(&s.mu)
	s.l = nil
	s.quitCh = nil
	s.server.Handler = &s.mux
	s.server.ReadTimeout = 30 * time.Second
	s.server.WriteTimeout = 30 * time.Second
	s.server.IdleTimeout = 60 * time.Second
	s.mux.HandleFunc("/", HandleHelp)
	s.mux.HandleFunc("/quit", HandleAction(ActionQuit))
	s.mux.HandleFunc("/restart", HandleAction(ActionRestart))
	s.mux.HandleFunc("/reject", HandleAction(ActionReject))
	s.mux.HandleFunc("/accept", HandleAction(ActionAccept))
	s.mux.HandleFunc("/freeze", HandleAction(ActionFreeze))
	s.mux.HandleFunc("/thaw", HandleAction(ActionThaw))
}

func (s *ControlServer) Run(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh != nil {
		panic("BUG! ControlServer is already running")
	}

	l, err := gListenConfig.Listen(ctx, "tcp", s.listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %q: %w", s.listen, err)
	}

	s.l = l
	s.quitCh = make(chan struct{})

	s.server.BaseContext = func(l net.Listener) context.Context {
		return gRootContext
	}

	go s.serveThread()
	go s.closeThread(ctx, s.quitCh, &s.server)
	return nil
}

func (s *ControlServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh == nil {
		return nil
	}

	close(s.quitCh)
	err := s.server.Shutdown(ctx)
	for s.quitCh != nil {
		s.cv.Wait()
	}
	return err
}

func (s *ControlServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh == nil {
		return nil
	}

	close(s.quitCh)
	err := s.server.Close()
	for s.quitCh != nil {
		s.cv.Wait()
	}
	return err
}

func (s *ControlServer) ForceClose() {
	err := s.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to stop HTTP control server: %v", err)
	}
}

func (s *ControlServer) closeThread(ctx context.Context, quitCh <-chan struct{}, closer io.Closer) {
	select {
	case <-ctx.Done():
		err := closer.Close()
		if IsErrClosed(err) {
			err = nil
		}
		if err != nil {
			Error("failed to stop HTTP control server: %v", err)
		}
	case <-quitCh:
	}
}

func (s *ControlServer) serveThread() {
	defer s.finish()

	err := s.server.Serve(s.l)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Error("failed to serve HTTP control server: %v", err)
	}

	err = s.l.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Error("failed to close listener for HTTP control server: %v", err)
	}
}

func (s *ControlServer) finish() {
	s.mu.Lock()
	s.l = nil
	s.quitCh = nil
	s.cv.Broadcast()
	s.mu.Unlock()
}

type ProxyServer struct {
	listen string
	dial   string
	mu     sync.Mutex
	cv     *sync.Cond
	gc     *grpc.ClientConn
	gs     *grpc.Server
	l      net.Listener
	wc     workflowservice.WorkflowServiceClient
	ws     workflowservice.WorkflowServiceServer
	quitCh chan struct{}
}

func (s *ProxyServer) Init(listen, dial string) {
	s.listen = listen
	s.dial = dial
	s.cv = sync.NewCond(&s.mu)
}

func (s *ProxyServer) Run(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh != nil {
		panic("BUG! gRPC proxy server is already running")
	}

	l, err := gListenConfig.Listen(ctx, "tcp", s.listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %q: %w", s.listen, err)
	}

	needListenerClose := true
	defer func() {
		if needListenerClose {
			err := l.Close()
			if IsErrClosed(err) {
				err = nil
			}
			if err != nil {
				Warn("failed to close listener for gRPC proxy server: %v", err)
			}
		}
	}()

	gc, err := grpc.DialContext(
		ctx,
		s.dial,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial %q: %w", s.dial, err)
	}

	needClientClose := true
	defer func() {
		if needClientClose {
			err := gc.Close()
			if IsErrClosed(err) {
				err = nil
			}
			if err != nil {
				Warn("failed to close gRPC client connection: %v", err)
			}
		}
	}()

	wc := workflowservice.NewWorkflowServiceClient(gc)
	ws, err := client.NewWorkflowServiceProxyServer(client.WorkflowServiceProxyOptions{Client: wc})
	if err != nil {
		return fmt.Errorf("failed to create WorkflowService proxy server: %w", err)
	}

	gs := grpc.NewServer(
		grpc.UnaryInterceptor(ProxyUnaryInterceptor),
		grpc.StreamInterceptor(ProxyStreamInterceptor),
	)
	grpc_health_v1.RegisterHealthServer(gs, &TrivialHealthServer{})
	workflowservice.RegisterWorkflowServiceServer(gs, ws)

	needClientClose = false
	needListenerClose = false
	s.gc = gc
	s.gs = gs
	s.l = l
	s.wc = wc
	s.ws = ws
	s.quitCh = make(chan struct{})

	go s.serveThread()
	go s.closeThread(ctx, s.quitCh, s.gs)
	return nil
}

func (s *ProxyServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh == nil {
		return nil
	}

	close(s.quitCh)
	s.gs.GracefulStop()
	for s.quitCh != nil {
		s.cv.Wait()
	}
	return nil
}

func (s *ProxyServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh == nil {
		return nil
	}

	close(s.quitCh)
	s.gs.Stop()
	for s.quitCh != nil {
		s.cv.Wait()
	}
	return nil
}

func (s *ProxyServer) ForceClose() {
	err := s.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to stop gRPC proxy server: %v", err)
	}
}

type Stopper interface {
	Stop()
}

func (s *ProxyServer) closeThread(ctx context.Context, quitCh <-chan struct{}, stopper Stopper) {
	select {
	case <-ctx.Done():
		stopper.Stop()
	case <-quitCh:
	}
}

func (s *ProxyServer) serveThread() {
	defer s.finish()

	err := s.gs.Serve(s.l)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Error("failed to serve gRPC proxy server: %v", err)
	}

	err = s.l.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to close listener for gRPC proxy server: %v", err)
	}

	err = s.gc.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Warn("failed to close gRPC client connection: %v", err)
	}
}

func (s *ProxyServer) finish() {
	s.mu.Lock()
	s.gc = nil
	s.gs = nil
	s.l = nil
	s.wc = nil
	s.ws = nil
	s.quitCh = nil
	s.cv.Broadcast()
	s.mu.Unlock()
}

func AwaitPermitted() error {
	gStateMutex.Lock()
	defer gStateMutex.Unlock()

	if gStateRejecting {
		return status.Error(codes.Unavailable, "proxy unavailable")
	}
	for gStateFrozen {
		gStateCond.Wait()
	}
	return nil
}

func ProxyUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	if err := AwaitPermitted(); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func ProxyStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := AwaitPermitted(); err != nil {
		return err
	}
	return handler(srv, ss)
}

type TrivialHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (*TrivialHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (resp *grpc_health_v1.HealthCheckResponse, err error) {
	return &grpc_health_v1.HealthCheckResponse{}, nil
}

func IsErrClosed(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, io.EOF):
		return true
	case errors.Is(err, fs.ErrClosed):
		return true
	case errors.Is(err, net.ErrClosed):
		return true
	case errors.Is(err, http.ErrServerClosed):
		return true
	default:
		return false
	}
}

func Trace(format string, args ...any) {
	if !flagTrace {
		return
	}
	fmt.Fprintf(os.Stderr, "trace: "+format+"\n", args...)
}

func Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "info: "+format+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warn: "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

func Fatal(rc int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(rc)
}
