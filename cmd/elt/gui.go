package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	neturl "net/url"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"

	"elt/internal/api"
	"elt/internal/apiclient"
	"elt/internal/localai"
	"elt/internal/setup"
	"elt/internal/store"
	"elt/internal/webui"
)

const guiHelp = `elt gui [--data-dir <dir>] [--listen host:port] [--no-browser] [--max-sync-mb=N] [--api-listen host:port]
elt gui --addr host:port [--listen host:port] [--no-browser] [--insecure]

The everyday way to use elt locally/solo: hosts the exact same API "elt
serve" does, and opens it in your default web browser automatically
(or just prints the URL with --no-browser). Runs until stopped
(Ctrl+C) — nothing to flush on exit, every mutation is already durably
saved to disk as it happens.

Zero flags is the normal case: with no --data-dir, the project
directory is picked from inside the browser's own first-run wizard
instead of the command line, and remembered (~/.config/elt/gui.json)
for next time — no need to pass --data-dir again on a later run in the
same directory. --data-dir remains a full escape hatch for development/
scripting: giving it skips the wizard entirely and opens/initializes
that directory directly, the same as "elt serve --data-dir" does.
--listen/--max-sync-mb, once set here or via the GUI's own Settings >
Server page, persist per-project and are reused on the next unflagged
run against the same directory — see "elt serve --help" for the exact
same persistence rule, shared code either way.

--addr host:port is a different mode entirely: a thin client. Instead of
embedding its own project (--data-dir/--project/--max-sync-mb are all
ignored when --addr is given), this process only serves the browser GUI's
static assets on --listen and transparently proxies every other request
(the API calls the page itself makes — sync, keys, glossary, locks, and
so on) through to the real "elt serve"/"elt api" at --addr, exactly as if
the browser were talking to that server directly — including whichever
bearer token the page itself already holds, forwarded unchanged. Meant
for each participant on a team server to run locally: the server itself
can run headless ("elt serve --no-ui"), and each translator points their
own "elt gui --addr=" at it instead of opening its address in a browser
directly. --addr also accepts a full "scheme://host[:port]" (not just a
bare host:port) for a server behind real TLS — see "elt project --help"'s
own --addr doc — and --insecure skips certificate verification for a
https:// --addr, needed for elt's own self-signed certificate (elt serve/
gui's own TLS story is deliberately just a self-signed cert with no real
CA behind it — see store.EnsureSelfSignedCert's doc): without --insecure,
a self-signed remote fails outright with "certificate signed by unknown
authority" for both the reverse-proxied API calls and the AI job's own
outgoing calls.

--api-listen host:port is the opposite case, only meaningful together
with an embedded project (--data-dir or the wizard — never with --addr,
which has no local project of its own to expose a second copy of):
--listen keeps serving the full UI+API combination as normal (e.g. on a
LAN-only address for people opening it in a browser here), while
--api-listen additionally exposes a second, UI-free copy of the exact
same project's API on its own address (e.g. opened more widely, for
remote "elt gui --addr="/CLI clients) — one process, two independently-
exposed addresses, instead of needing a second elt instance just to give
the API a different network exposure than the UI. Not persisted across
runs — pass it every launch, the same as --no-browser. Multi-user access
becomes available the moment either address is non-loopback-only, not
just --listen.
`

func runGUI(args []string) error {
	fs := flag.NewFlagSet("gui", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), guiHelp)
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	listen := fs.String("listen", "", "address to listen on (port 0 = pick any free port; default 127.0.0.1:0, or the project's own persisted value)")
	addr := fs.String("addr", "", "host:port (or scheme://host[:port], e.g. https://... for a TLS-terminated server) of a remote elt api/serve to connect to as a thin client, instead of embedding a local project (--data-dir/--project/--max-sync-mb are ignored when this is given)")
	dataDir := fs.String("data-dir", "", "project directory (omit to pick one from the browser wizard)")
	projectName := fs.String("project", "New Project", "project name; only used the first time --data-dir is initialized")
	noBrowser := fs.Bool("no-browser", false, "print the URL instead of opening a browser automatically (also true if the config file sets it)")
	maxSyncMB := fs.Int("max-sync-mb", 0, fmt.Sprintf("maximum upload size in MB for POST .../sync (default %d, or the project's own persisted value)", api.DefaultMaxSyncBodyBytes/(1024*1024)))
	apiListen := fs.String("api-listen", "", "additionally expose a UI-free copy of the API on this address, alongside --listen's own UI+API address (not persisted; only meaningful with an embedded project, never with --addr)")
	insecure := fs.Bool("insecure", false, "skip TLS certificate verification when --addr is a https:// server — needed for elt's own self-signed certificate; ignored without --addr or with a plain http:// --addr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *addr != "" && *apiListen != "" {
		return fmt.Errorf("--api-listen is meaningless with --addr — a thin client has no local project of its own to expose a second copy of")
	}

	guiConfigPath, err := defaultConfigPath("gui")
	if err != nil {
		return fmt.Errorf("resolving gui config path: %w", err)
	}
	cfg, err := loadOrInitConfig(guiConfigPath, AppConfig{})
	if err != nil {
		return fmt.Errorf("loading gui config: %w", err)
	}

	resolvedNoBrowser := *noBrowser || cfg.NoBrowser

	if *addr != "" {
		return runGUIThinClient(*addr, *listen, resolvedNoBrowser, *insecure)
	}

	resolvedDir, warning, err := resolveDataDir(*dataDir, cfg.DataDir)
	if err != nil {
		return fmt.Errorf("resolving project directory: %w", err)
	}

	var st *store.Store
	if resolvedDir != "" {
		var openErr error
		st, openErr = store.Open(resolvedDir, *projectName)
		if openErr != nil {
			if *dataDir != "" {

				return openErr
			}

			fmt.Printf("elt gui: could not open remembered project directory %s: %v\n", resolvedDir, openErr)
			warning = fmt.Sprintf("The project directory %s could not be opened (%v). Its files were left untouched — fix the problem and reopen it, or pick a different directory below.", resolvedDir, openErr)
			st = nil
		}
	}

	var deployment store.DeploymentConfig
	if st != nil {
		deployment, _, err = st.Deployment()
		if err != nil {
			return err
		}
	}

	ln, err := bindGUIListener(*listen, deployment.Listen, st)
	if err != nil {
		return err
	}

	loopbackOnly := isLoopbackListenAddr(ln.Addr().String()) && (*apiListen == "" || isLoopbackListenAddr(*apiListen))

	if st != nil {
		srv, err := api.NewServer(st)
		if err != nil {
			return err
		}
		srv.SetLoopbackOnly(loopbackOnly)
		applyMaxSyncMBFlag(srv, resolveSettingInt(*maxSyncMB, deployment.MaxSyncMB, 0))
		srv.SetListenConfigHooks(guiListenConfigHooks(st))

		tlsLn, scheme, err := wrapListenerForTLS(ln, deployment.TLS, st)
		if err != nil {
			return err
		}
		if *apiListen != "" {
			if _, err := startAPIOnlyListener(srv, *apiListen, deployment.TLS, st); err != nil {
				return err
			}
		}
		url := scheme + "://" + ln.Addr().String()
		fmt.Printf("elt gui: %s (project directory %s)\n", url, st.Dir())
		fmt.Println("Press Ctrl+C to stop.")
		announceURL(url, resolvedNoBrowser)

		httpSrv := newHTTPServer(srv.Handler(webui.Handler()))
		return httpSrv.Serve(tlsLn)
	}

	url := "http://" + ln.Addr().String()

	session := setup.NewSession()
	if warning != "" {
		session.SetWarning(warning)
	}
	handler := newSwappableHandler(session.Handler(webui.Handler()))
	httpSrv := newHTTPServer(handler)

	fmt.Printf("elt gui: %s (no project yet — pick one in the browser)\n", url)
	fmt.Println("Press Ctrl+C to stop.")
	announceURL(url, resolvedNoBrowser)

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	select {
	case newSt := <-session.Done():
		srv, err := api.NewServer(newSt)
		if err != nil {
			return err
		}
		srv.SetLoopbackOnly(loopbackOnly)
		newDeployment, _, err := newSt.Deployment()
		if err != nil {
			return err
		}
		applyMaxSyncMBFlag(srv, resolveSettingInt(*maxSyncMB, newDeployment.MaxSyncMB, 0))
		srv.SetListenConfigHooks(guiListenConfigHooks(newSt))
		if *apiListen != "" {
			if _, err := startAPIOnlyListener(srv, *apiListen, newDeployment.TLS, newSt); err != nil {
				return err
			}
		}

		if *listen == "" {
			persistResolvedListenAddr(newSt, ln.Addr().String(), "")
		}
		handler.Set(srv.Handler(webui.Handler()))
		if err := updateGUIConfig(guiConfigPath, func(c *AppConfig) { c.DataDir = newSt.Dir() }); err != nil {

			fmt.Printf("elt gui: could not remember %s for next time: %v\n", newSt.Dir(), err)
		}

		session.MarkHandedOff()
		fmt.Printf("elt gui: project ready at %s\n", newSt.Dir())
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

func runGUIThinClient(remoteAddr, listenFlag string, noBrowser, insecure bool) error {
	resolvedListen := listenFlag
	if resolvedListen == "" {
		resolvedListen = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", resolvedListen)
	if err != nil {
		return fmt.Errorf("starting thin client: %w", err)
	}
	handler, err := thinClientHandler(remoteAddr, insecure)
	if err != nil {
		return err
	}

	url := "http://" + ln.Addr().String()
	remoteBaseURL, err := apiclient.NormalizeBaseURL(remoteAddr)
	if err != nil {
		return err
	}
	fmt.Printf("elt gui: thin client at %s, proxying to %s\n", url, remoteBaseURL)
	fmt.Println("Press Ctrl+C to stop.")
	announceURL(url, noBrowser)

	httpSrv := newHTTPServer(handler)
	return httpSrv.Serve(ln)
}

func thinClientHandler(remoteAddr string, insecure bool) (http.Handler, error) {

	baseURL, err := apiclient.NormalizeBaseURL(remoteAddr)
	if err != nil {
		return nil, err
	}
	remoteURL, err := neturl.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid --addr %q: %w", remoteAddr, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(remoteURL)

	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = remoteURL.Host
	}
	if insecure {

		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // explicit, opt-in --insecure for a self-signed --addr
		}
	}

	proxy.Transport = newLimitedTransport(proxy.Transport, maxConcurrentUpstreamRequests)
	ui := webui.Handler()
	localAI := localai.NewManager().Handler(remoteAddr, insecure)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" || webui.HasFile(r.URL.Path):
			ui.ServeHTTP(w, r)
		case localai.IsLocalAIPath(r.URL.Path):
			localAI.ServeHTTP(w, r)
		default:
			proxy.ServeHTTP(w, r)
		}
	}), nil
}

const maxConcurrentUpstreamRequests = 4

type limitedTransport struct {
	rt  http.RoundTripper
	sem chan struct{}
}

func newLimitedTransport(rt http.RoundTripper, limit int) *limitedTransport {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &limitedTransport{rt: rt, sem: make(chan struct{}, limit)}
}

func (l *limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case l.sem <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	defer func() { <-l.sem }()
	return l.rt.RoundTrip(req)
}

func announceURL(url string, noBrowser bool) {
	if noBrowser {
		fmt.Printf("Open %s in your browser.\n", url)
		return
	}
	if err := openBrowser(url); err != nil {
		fmt.Printf("Could not open a browser automatically (%v) — open %s manually.\n", err, url)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	return cmd.Start()
}

type swappableHandler struct {
	v atomic.Value
}

func newSwappableHandler(initial http.Handler) *swappableHandler {
	h := &swappableHandler{}
	h.Set(initial)
	return h
}

func (h *swappableHandler) Set(next http.Handler) { h.v.Store(next) }

func (h *swappableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.v.Load().(http.Handler).ServeHTTP(w, r)
}

func updateGUIConfig(path string, mutate func(*AppConfig)) error {
	return updateConfig(path, AppConfig{}, mutate)
}

func guiListenConfigHooks(st *store.Store) api.ListenConfigHooks {
	return api.ListenConfigHooks{
		Get: func() (string, error) {
			c, _, err := st.Deployment()
			return c.Listen, err
		},
		Set: func(addr string) error {
			return updateDeployment(st, func(c *store.DeploymentConfig) { c.Listen = addr })
		},
	}
}

func resolveDataDir(explicit, remembered string) (dir string, warning string, err error) {
	if explicit != "" {
		return explicit, "", nil
	}
	if remembered == "" {
		return "", "", nil
	}
	if _, statErr := os.Stat(remembered); statErr != nil {
		if os.IsNotExist(statErr) {
			fmt.Printf("elt gui: remembered project directory %s no longer exists — starting setup again\n", remembered)
			return "", fmt.Sprintf("The project directory %s no longer exists. Pick or create a new one below.", remembered), nil
		}
		return "", "", statErr
	}
	return remembered, "", nil
}

func bindGUIListener(explicit, remembered string, st *store.Store) (net.Listener, error) {
	resolvedListen := resolveSetting(explicit, remembered, "127.0.0.1:0")
	listenIsExplicit := explicit != ""

	ln, err := net.Listen("tcp", resolvedListen)
	if err != nil {
		if listenIsExplicit {
			return nil, fmt.Errorf("starting server: %w", err)
		}
		fmt.Printf("elt gui: could not bind %s (%v) — picking a new port\n", resolvedListen, err)
		resolvedListen = "127.0.0.1:0"
		if ln, err = net.Listen("tcp", resolvedListen); err != nil {
			return nil, fmt.Errorf("starting server: %w", err)
		}
	}

	if !listenIsExplicit {
		persistResolvedListenAddr(st, ln.Addr().String(), remembered)
	}
	return ln, nil
}

func persistResolvedListenAddr(st *store.Store, actual, remembered string) {
	if st == nil || actual == remembered {
		return
	}
	if err := updateDeployment(st, func(c *store.DeploymentConfig) { c.Listen = actual }); err != nil {
		fmt.Printf("elt gui: could not remember listen address %s for next time: %v\n", actual, err)
	}
}
