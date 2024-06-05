package harness

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const HelpText = `The test proxy exposes the following control endpoints:

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

// GRPCProxyServer
type GRPCProxyServer struct {
	proxyServer   *ProxyServer
	controlServer *ControlServer
	log           log.Logger
}

// GRPCProxyServerOptions are options for GRPC proxy.
type GRPCProxyServerOptions struct {
	DialAddress    string
	ClientCertPath string
	ClientKeyPath  string
	Log            log.Logger
}

// StartGRPCProxyServer starts up a GRPC proxy
func StartGRPCProxyServer(options GRPCProxyServerOptions) (*GRPCProxyServer, error) {
	if options.Log == nil {
		options.Log = DefaultLogger
	}
	options.Log.Info("Starting GRPC proxy server")
	proxyServer, err := newProxyServer("127.0.0.1:", options.DialAddress, options.ClientCertPath, options.ClientKeyPath, options.Log)
	if err != nil {
		return nil, err
	}
	err = proxyServer.Run()
	if err != nil {
		options.Log.Error("failed to start gRPC proxy server", "error", err)
		return nil, err
	}

	controlServer, err := startControlServer(proxyServer, "127.0.0.1:", options.Log)
	if err != nil {
		options.Log.Error("failed to start HTTP control server", "error", err)
		return nil, err
	}
	err = controlServer.Run()
	if err != nil {
		return nil, err
	}

	srv := &GRPCProxyServer{
		proxyServer:   proxyServer,
		controlServer: controlServer,
		log:           options.Log,
	}
	return srv, nil
}

// Close immediately stops the proxy.
func (g *GRPCProxyServer) Close() error {
	err := g.controlServer.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		g.log.Warn("failed to gracefully shut down HTTP control server", "error", err)
	}

	err = g.proxyServer.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		g.log.Warn("failed to gracefully shut down HTTP proxy server", "error", err)
	}
	return nil
}

// ProxyAddress returns the address of the proxy server.
func (g *GRPCProxyServer) ProxyAddress() string {
	return g.proxyServer.listen
}

// ControlAddress returns the address of the control server.
func (g *GRPCProxyServer) ControlAddress() string {
	return g.controlServer.listen
}

var ErrUnknownCommand = errors.New("unknown command")

type queryKeyType struct{}

var queryKey queryKeyType

type ActionFunc = func(context.Context) error

func (c *ControlServer) actionRestart(ctx context.Context) error {
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

	c.Log.Info("/restart: restarting proxy", "forceful", forceful)

	mode := "gracefully"
	fn := c.ps.Shutdown
	if forceful {
		mode = "forcefully"
		fn = func(context.Context) error {
			return c.ps.Close()
		}
	}

	err := fn(ctx)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		c.Log.Warn("failed to shut down gRPC proxy server", "mode", mode, "error", err)
	}

	c.ps.ForceClose()

	if sleep > 0 {
		c.Log.Info("/restart: sleeping", "duration", sleep)
		time.Sleep(sleep)
	}

	err = c.ps.Run()
	if err != nil {
		return err
	}

	c.Log.Info("/restart: proxy has been restarted")
	return nil
}

func (cs *ControlServer) actionReject(ctx context.Context) error {
	cs.ps.mu.Lock()
	defer cs.ps.mu.Unlock()

	if cs.ps.stateRejecting {
		return nil
	}
	cs.ps.stateRejecting = true
	cs.Log.Info("/reject: proxy is rejecting requests")
	return nil
}

func (cs *ControlServer) actionAccept(ctx context.Context) error {
	cs.ps.mu.Lock()
	defer cs.ps.mu.Unlock()

	if !cs.ps.stateRejecting {
		return nil
	}
	cs.ps.stateRejecting = false
	cs.Log.Info("/accept: proxy is NOT rejecting requests")
	return nil
}

func (cs *ControlServer) actionFreeze(ctx context.Context) error {
	cs.ps.mu.Lock()
	defer cs.ps.mu.Unlock()

	if cs.ps.stateFrozen {
		return nil
	}
	cs.ps.stateFrozen = true
	cs.Log.Info("/freeze: proxy is stalling requests")
	return nil
}

func (cs *ControlServer) actionThaw(ctx context.Context) error {
	cs.ps.mu.Lock()
	defer cs.ps.mu.Unlock()

	if !cs.ps.stateFrozen {
		return nil
	}
	cs.ps.stateFrozen = false
	cs.ps.stateCond.Broadcast()
	cs.Log.Info("/thaw: proxy is NOT stalling requests")
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
	Log    log.Logger
	ps     *ProxyServer
}

func startControlServer(proxyServer *ProxyServer, listen string, log log.Logger) (*ControlServer, error) {
	var s ControlServer
	s.ps = proxyServer
	s.Log = log
	s.listen = listen
	s.cv = sync.NewCond(&s.mu)
	s.l = nil
	s.quitCh = nil
	s.server.Handler = &s.mux
	s.server.ReadTimeout = 30 * time.Second
	s.server.WriteTimeout = 30 * time.Second
	s.server.IdleTimeout = 60 * time.Second
	s.mux.HandleFunc("/", HandleHelp)
	s.mux.HandleFunc("/restart", HandleAction(s.actionRestart))
	s.mux.HandleFunc("/reject", HandleAction(s.actionReject))
	s.mux.HandleFunc("/accept", HandleAction(s.actionAccept))
	s.mux.HandleFunc("/freeze", HandleAction(s.actionFreeze))
	s.mux.HandleFunc("/thaw", HandleAction(s.actionThaw))
	return &s, nil
}

func (s *ControlServer) Run() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh != nil {
		panic("BUG! ControlServer is already running")
	}

	l, err := net.Listen("tcp", s.listen)
	s.listen = l.Addr().String()
	if err != nil {
		return fmt.Errorf("failed to listen on %q: %w", s.listen, err)
	}

	s.l = l
	s.quitCh = make(chan struct{})

	go s.serveThread()
	return nil
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
		s.Log.Warn("failed to stop HTTP control server", "error", err)
	}
}

func (s *ControlServer) serveThread() {
	defer s.finish()

	err := s.server.Serve(s.l)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		s.Log.Error("failed to serve HTTP control server", "error", err)
	}

	err = s.l.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		s.Log.Error("failed to close listener for HTTP control server", "error", err)
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
	listen         string
	dial           string
	tlsConfig      *tls.Config
	mu             sync.Mutex
	cv             *sync.Cond
	gc             *grpc.ClientConn
	gs             *grpc.Server
	l              net.Listener
	wc             workflowservice.WorkflowServiceClient
	ws             workflowservice.WorkflowServiceServer
	quitCh         chan struct{}
	log            log.Logger
	stateCond      *sync.Cond
	stateRejecting bool
	stateFrozen    bool
}

func newProxyServer(listen, dial, clientCertPath, clientKeyPath string, log log.Logger) (*ProxyServer, error) {
	p := &ProxyServer{
		listen: listen,
		dial:   dial,
		log:    log,
	}
	if clientCertPath != "" {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load certs: %s", err)
		}
		p.tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	p.cv = sync.NewCond(&p.mu)
	p.stateCond = sync.NewCond(&p.mu)
	return p, nil
}

func (s *ProxyServer) Run() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.quitCh != nil {
		panic("BUG! gRPC proxy server is already running")
	}

	l, err := net.Listen("tcp", s.listen)
	// keep a stable address across restarts
	s.listen = l.Addr().String()
	if err != nil {
		return fmt.Errorf("failed to listen on %q: %w", s.listen, err)
	}
	s.log.Info("gRPC proxy server is running on", "address", l.Addr().String())

	needListenerClose := true
	defer func() {
		if needListenerClose {
			err := l.Close()
			if IsErrClosed(err) {
				err = nil
			}
			if err != nil {
				s.log.Warn("failed to close listener for gRPC proxy server", "error", err)
			}
		}
	}()

	opts := []grpc.DialOption{grpc.WithBlock()}
	if s.tlsConfig != nil {
		creds := credentials.NewTLS(s.tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	gc, err := grpc.Dial(
		s.dial,
		opts...,
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
				s.log.Warn("failed to close gRPC client connection", "error", err)
			}
		}
	}()

	wc := workflowservice.NewWorkflowServiceClient(gc)
	ws, err := client.NewWorkflowServiceProxyServer(client.WorkflowServiceProxyOptions{Client: wc})
	if err != nil {
		return fmt.Errorf("failed to create WorkflowService proxy server: %w", err)
	}

	gs := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
			s.log.Debug("incoming gRPC request", "method", info.FullMethod)
			if err := s.awaitPermitted(); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if err := s.awaitPermitted(); err != nil {
				return err
			}
			return handler(srv, ss)
		}),
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
		s.log.Warn("failed to stop gRPC proxy server", "error", err)
	}
}

type Stopper interface {
	Stop()
}

func (s *ProxyServer) serveThread() {
	defer s.finish()

	err := s.gs.Serve(s.l)
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		s.log.Error("failed to serve gRPC proxy server", "error", err)
	}

	err = s.l.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		s.log.Warn("failed to close listener for gRPC proxy server", "error", err)
	}

	err = s.gc.Close()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		s.log.Warn("failed to close gRPC client connection", "error", err)
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

func (s *ProxyServer) awaitPermitted() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateRejecting {
		return status.Error(codes.Unavailable, "proxy unavailable")
	}
	for s.stateFrozen {
		s.stateCond.Wait()
	}
	return nil
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
