package apiclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"elt/internal/api"
	"elt/internal/setup"
	"elt/internal/store"
)

type LocalHandoff struct {
	Client  *Client
	Session *setup.Session

	cleanup func()
	handler *swappableHandler
}

func (h *LocalHandoff) Cleanup() {
	if h.cleanup != nil {
		h.cleanup()
	}
}

func ConnectLocalWithSetup(dataDir, projectName string) (*LocalHandoff, error) {
	kind, err := store.ClassifyDir(dataDir)
	if err != nil {
		return nil, err
	}
	if kind != store.DirEmpty {
		client, cleanup, err := Connect("", dataDir, projectName, "", false)
		if err != nil {
			return nil, err
		}
		return &LocalHandoff{Client: client, cleanup: cleanup}, nil
	}

	session := setup.NewSession()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("apiclient: starting embedded setup server: %w", err)
	}
	handler := newSwappableHandler(session.Handler(nil))
	httpSrv := &http.Server{Handler: handler}
	go httpSrv.Serve(ln) //nolint:errcheck // Shutdown below always returns http.ErrServerClosed

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}
	client := &Client{BaseURL: "http://" + ln.Addr().String()}

	body := map[string]any{"path": dataDir, "create": true}
	if err := client.CallJSON("POST", "/setup/directory", body, nil); err != nil {
		cleanup()
		return nil, fmt.Errorf("apiclient: choosing setup directory: %w", err)
	}

	return &LocalHandoff{
		Client:  client,
		Session: session,
		cleanup: cleanup,
		handler: handler,
	}, nil
}

func (h *LocalHandoff) Await() error {
	if h.Session == nil {
		return nil
	}
	st := <-h.Session.Done()
	srv, err := api.NewServer(st)
	if err != nil {
		return err
	}
	h.handler.Set(srv.Handler(nil))
	h.Session.MarkHandedOff()
	return nil
}

type swappableHandler struct {
	v atomic.Value
}

func newSwappableHandler(initial http.Handler) *swappableHandler {
	h := &swappableHandler{}
	h.v.Store(initial)
	return h
}

func (h *swappableHandler) Set(next http.Handler) { h.v.Store(next) }

func (h *swappableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.v.Load().(http.Handler).ServeHTTP(w, r)
}
