package apiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"elt/internal/api"
	"elt/internal/store"
	"elt/internal/webui"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func InsecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // explicit, opt-in --insecure for a self-signed --addr
		},
	}
}

func NormalizeBaseURL(addr string) (string, error) {
	if !strings.Contains(addr, "://") {
		return "http://" + addr, nil
	}
	u, err := neturl.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("invalid --addr %q: %w", addr, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid --addr %q: unsupported scheme %q (must be http or https)", addr, u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid --addr %q: missing host", addr)
	}
	return strings.TrimSuffix(addr, "/"), nil
}

func Connect(addr, dataDir, projectName, token string, insecure bool) (c *Client, cleanup func(), err error) {
	if addr != "" {
		baseURL, err := NormalizeBaseURL(addr)
		if err != nil {
			return nil, nil, err
		}
		client := &Client{BaseURL: baseURL, Token: token}
		if insecure {
			client.HTTPClient = InsecureHTTPClient()
		}
		return client, func() {}, nil
	}
	if dataDir == "" {
		return nil, nil, fmt.Errorf("apiclient: either addr or dataDir is required")
	}

	st, err := store.Open(dataDir, projectName)
	if err != nil {
		return nil, nil, err
	}
	srv, err := api.NewServer(st)
	if err != nil {
		return nil, nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("apiclient: starting embedded server: %w", err)
	}
	httpSrv := &http.Server{Handler: srv.Handler(webui.Handler())}
	go httpSrv.Serve(ln) //nolint:errcheck // Shutdown below always returns http.ErrServerClosed

	cleanup = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}

	return &Client{BaseURL: "http://" + ln.Addr().String()}, cleanup, nil
}

type errorResponse struct {
	Error string `json:"error"`
}

type CallError struct {
	Status  int
	Body    []byte
	message string
}

func (e *CallError) Error() string { return e.message }

func (c *Client) Call(method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		message := fmt.Sprintf("server returned status %d: %s", resp.StatusCode, string(data))
		var e errorResponse
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			message = e.Error
		}
		return nil, &CallError{Status: resp.StatusCode, Body: data, message: message}
	}
	return data, nil
}

func (c *Client) CallJSON(method, path string, reqBody, out any) error {
	var body []byte
	if reqBody != nil {
		var err error
		body, err = json.Marshal(reqBody)
		if err != nil {
			return err
		}
	}
	data, err := c.Call(method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}
