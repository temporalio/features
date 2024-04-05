package harness

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"

	"go.temporal.io/sdk/log"
)

// HTTPConnectProxyServer is a simple HTTP CONNECT proxy.
type HTTPConnectProxyServer struct {
	Address string
	// This is incremented on each successful CONNECT.
	ConnectionsTunneled atomic.Uint32

	server http.Server
	log    log.Logger
}

// HTTPConnectProxyServerOptions are options for HTTP connect proxies.
type HTTPConnectProxyServerOptions struct {
	Log log.Logger
}

// StartHTTPConnectProxyServer starts up an [http.Server] for HTTP CONNECT proxy
// handling on a random port localhost.
func StartHTTPConnectProxyServer(options HTTPConnectProxyServerOptions) (*HTTPConnectProxyServer, error) {
	if options.Log == nil {
		options.Log = DefaultLogger
	}

	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return nil, err
	}

	srv := &HTTPConnectProxyServer{Address: l.Addr().String(), log: options.Log}
	srv.server.Handler = http.HandlerFunc(srv.handler)
	go func() {
		if err := srv.server.Serve(l); err != http.ErrServerClosed {
			options.Log.Error("HTTP CONNECT proxy failed", "error", err)
		}
	}()
	return srv, nil
}

// Close immediately stops the proxy.
func (h *HTTPConnectProxyServer) Close() error { return h.server.Close() }

func (h *HTTPConnectProxyServer) handler(w http.ResponseWriter, r *http.Request) {
	// Much of this taken from TestTransportProxy in Go source
	if r.Method != "CONNECT" {
		http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
		return
	}
	h.log.Debug("Got HTTP proxy request", "host", r.Host)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		panic("no hijack iface")
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		panic("hijack failed")
	}
	res := &http.Response{
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}

	targetConn, err := net.Dial("tcp", r.URL.Host)
	if err != nil {
		panic(fmt.Sprintf("net.Dial(%q) failed: %v", r.URL.Host, err))
	}

	if err := res.Write(clientConn); err != nil {
		panic(fmt.Sprintf("Writing 200 OK failed: %v", err))
	}

	h.ConnectionsTunneled.Add(1)
	go io.Copy(targetConn, clientConn)
	go func() {
		io.Copy(clientConn, targetConn)
		targetConn.Close()
	}()
}
