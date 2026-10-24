package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"elt/internal/api"
	"elt/internal/store"
)

func wrapListenerForTLS(ln net.Listener, cfg *store.TLSConfig, st *store.Store) (net.Listener, string, error) {
	if cfg == nil || !cfg.Enabled {
		return ln, "http", nil
	}
	certPath, keyPath := cfg.Cert, cfg.Key
	if certPath == "" || keyPath == "" {
		var err error
		certPath, keyPath, err = st.EnsureSelfSignedCert()
		if err != nil {
			return nil, "", err
		}
		fmt.Println("elt: no TLS cert/key configured — using a generated self-signed certificate")
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, "", fmt.Errorf("loading TLS certificate/key: %w", err)
	}
	return tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}}), "https", nil
}

func quietErrorLog() *log.Logger {
	return log.New(handshakeErrorFilterWriter{out: os.Stderr}, "", log.LstdFlags)
}

type handshakeErrorFilterWriter struct {
	out io.Writer
}

func (f handshakeErrorFilterWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("TLS handshake error")) {
		return len(p), nil
	}
	return f.out.Write(p)
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ErrorLog: quietErrorLog()}
}

func startAPIOnlyListener(srv *api.Server, addr string, tlsCfg *store.TLSConfig, st *store.Store) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("binding --api-listen %s: %w", addr, err)
	}
	ln, scheme, err := wrapListenerForTLS(ln, tlsCfg, st)
	if err != nil {
		return nil, err
	}
	fmt.Printf("elt: API also listening on %s://%s (no UI)\n", scheme, ln.Addr())
	httpSrv := newHTTPServer(srv.Handler(nil))
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "elt: --api-listen listener on %s stopped: %v\n", addr, err)
		}
	}()
	return ln, nil
}
