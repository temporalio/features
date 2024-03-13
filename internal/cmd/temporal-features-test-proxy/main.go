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
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const HelpText = `The test proxy exposes the following control endpoints:

- POST /start
  Listen for new connections; this is the default

- POST /stop
  Do not listen for new connections

- POST /kill-all
  Close all existing connections

- POST /freeze
  Do not accept new connections or copy bytes on existing connections

- POST /thaw
  Accept new connections and copy bytes on existing connections; this is the default

- POST /quit
  Shut down the proxy and exit
`

var (
	ErrUnknownCommand = errors.New("unknown command")
	ErrAlreadyStarted = errors.New("already started")
	ErrAlreadyStopped = errors.New("already stopped")
	ErrAlreadyFrozen  = errors.New("already frozen")
	ErrAlreadyThawed  = errors.New("already thawed")
)

var (
	flagTrace   bool
	flagControl string
	flagListen  string
	flagDial    string

	gAliveChan chan struct{}

	gLastID uint32

	gFrozenMutex sync.Mutex
	gFrozenCond  *sync.Cond
	gFrozen      bool

	gListenerWG sync.WaitGroup
	gListener   net.Listener

	gPairWG       sync.WaitGroup
	gPairMapMutex sync.Mutex
	gPairMap      map[uint32]*Pair
)

func init() {
	flag.BoolVar(&flagTrace, "trace", false, "enable tracing logs")
	flag.StringVar(&flagControl, "control", "", "TCP host:port to listen on for HTTP control commands")
	flag.StringVar(&flagListen, "listen", "", "TCP host:port to listen on for proxying to -dial")
	flag.StringVar(&flagDial, "dial", "", "TCP host:port to connect to")
}

func init() {
	gFrozenCond = sync.NewCond(&gFrozenMutex)
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

	l, err := net.Listen("tcp", flagControl)
	if err != nil {
		Fatal(2, "failed to listen on %q: %v", flagControl, err)
		panic(nil)
	}

	if err := ActionStart(); err != nil {
		Fatal(2, "failed to listen on %q: %v", flagListen, err)
		panic(nil)
	}
	defer func() { _ = ActionStop() }()

	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleHelp)
	mux.HandleFunc("/quit", HandleExit)
	mux.HandleFunc("/start", HandleAction(ActionStart))
	mux.HandleFunc("/stop", HandleAction(ActionStop))
	mux.HandleFunc("/kill-all", HandleAction(ActionKillAll))
	mux.HandleFunc("/freeze", HandleAction(ActionFreeze))
	mux.HandleFunc("/thaw", HandleAction(ActionThaw))

	s := &http.Server{
		Handler:      mux,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	gAliveChan = make(chan struct{})
	go func() {
		<-gAliveChan
		_ = s.Shutdown(context.Background())
	}()

	err = s.Serve(l)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if err != nil {
		Fatal(2, "failed to serve HTTP on %q: %v", flagControl, err)
		panic(nil)
	}
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

func HandleExit(w http.ResponseWriter, r *http.Request) {
	if !CheckMethod(w, r, http.MethodPost) {
		return
	}
	close(gAliveChan)
	w.WriteHeader(http.StatusNoContent)
}

func HandleAction(action func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckMethod(w, r, http.MethodPost) {
			return
		}
		if err := action(); err != nil {
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

func ActionStart() error {
	if gListener != nil {
		return ErrAlreadyStarted
	}

	l, err := net.Listen("tcp", flagListen)
	if err != nil {
		return err
	}

	gListener = l
	gListenerWG.Add(1)
	go AcceptLoop(l)
	return nil
}

func ActionStop() error {
	if gListener == nil {
		return ErrAlreadyStopped
	}

	err := gListener.Close()
	gListener = nil
	gListenerWG.Wait()
	if IsErrClosed(err) {
		err = nil
	}
	return err
}

func ActionKillAll() error {
	var pairMap map[uint32]*Pair
	gPairMapMutex.Lock()
	pairMap, gPairMap = gPairMap, nil
	gPairMapMutex.Unlock()

	for _, pair := range pairMap {
		pair.Close()
	}
	gPairWG.Wait()
	return nil
}

func ActionFreeze() error {
	gFrozenMutex.Lock()
	defer gFrozenMutex.Unlock()

	if gFrozen {
		return ErrAlreadyFrozen
	}
	gFrozen = true
	return nil
}

func ActionThaw() error {
	gFrozenMutex.Lock()
	defer gFrozenMutex.Unlock()

	if !gFrozen {
		return ErrAlreadyThawed
	}
	gFrozen = false
	gFrozenCond.Broadcast()
	return nil
}

func AcceptLoop(l net.Listener) {
	defer gListenerWG.Done()
	defer func() {
		err := l.Close()
		if IsErrClosed(err) {
			err = nil
		}
		if err != nil {
			Warn("error while closing listener: %v", err)
		}
	}()

	for {
		AwaitThaw()

		Trace("accepting")
		c, err := l.Accept()
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("accept failed")
			return
		}

		id := atomic.AddUint32(&gLastID, 1)
		Trace("accept complete [#%d]", id)

		pair := &Pair{ID: id, In: c}
		gPairMapMutex.Lock()
		if gPairMap == nil {
			gPairMap = make(map[uint32]*Pair, 4)
		}
		gPairMap[id] = pair
		gPairMapMutex.Unlock()

		gPairWG.Add(1)
		go pair.Thread(c)
	}
}

func AwaitThaw() {
	gFrozenMutex.Lock()
	for gFrozen {
		gFrozenCond.Wait()
	}
	gFrozenMutex.Unlock()
}

type Pair struct {
	ID    uint32
	Mutex sync.Mutex
	In    net.Conn
	Out   net.Conn
}

func (pair *Pair) Thread(in net.Conn) {
	defer gPairWG.Done()
	defer pair.Close()

	inLA := in.LocalAddr().String()
	inRA := in.RemoteAddr().String()
	badge := fmt.Sprintf("#%d; %s->%s", pair.ID, inRA, inLA)

	Trace("dialing [%s]", badge)
	out, err := net.Dial("tcp", flagDial)
	if err != nil {
		Error("failed to dial %q: %v [%s]", flagDial, err, badge)
		return
	}

	outLA := out.LocalAddr().String()
	outRA := out.RemoteAddr().String()
	badge = badge + "; " + outLA + "->" + outRA
	Trace("dial complete [%s]", badge)

	if !pair.Activate(out) {
		_ = out.Close()
		return
	}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	gPairWG.Add(2)
	go pair.CopyLoop(ch1, in, out, badge+"; server->client")
	go pair.CopyLoop(ch2, out, in, badge+"; client->server")
	select {
	case <-ch1:
	case <-ch2:
	}
}

func (pair *Pair) Activate(out net.Conn) bool {
	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()

	if pair.In == nil {
		return false
	}
	pair.Out = out
	return true
}

func (pair *Pair) CopyLoop(ch chan struct{}, dst net.Conn, src net.Conn, badge string) {
	defer gPairWG.Done()
	defer close(ch)
	for {
		const BufferSize = 1 << 12 // 4 KiB
		var buffer [BufferSize]byte

		AwaitThaw()

		n, err := src.Read(buffer[:])
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("read I/O error: %v [%s]", err, badge)
			return
		}

		if n <= 0 {
			continue
		}

		AwaitThaw()

		_, err = dst.Write(buffer[:n])
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("write I/O error: %v [%s]", err, badge)
			return
		}
	}
}

func (pair *Pair) Close() {
	gPairMapMutex.Lock()
	delete(gPairMap, pair.ID)
	gPairMapMutex.Unlock()

	var (
		in  net.Conn
		out net.Conn
	)
	pair.Mutex.Lock()
	in, pair.In = pair.In, nil
	out, pair.Out = pair.Out, nil
	pair.Mutex.Unlock()

	if in == nil {
		return
	}

	inLA := in.LocalAddr().String()
	inRA := in.RemoteAddr().String()
	badge := fmt.Sprintf("#%d; %s->%s", pair.ID, inRA, inLA)
	inErr := in.Close()

	var outErr error
	if out != nil {
		outLA := out.LocalAddr().String()
		outRA := out.RemoteAddr().String()
		badge = badge + "; " + outLA + "->" + outRA
		outErr = out.Close()
	}

	if IsErrClosed(inErr) {
		inErr = nil
	}
	if IsErrClosed(outErr) {
		outErr = nil
	}
	if inErr != nil {
		Warn("error while closing incoming connection: %v [%s]", inErr, badge)
	}
	if outErr != nil {
		Warn("error while closing outgoing connection: %v [%s]", outErr, badge)
	}
	Trace("connection closed [%s]", badge)
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
