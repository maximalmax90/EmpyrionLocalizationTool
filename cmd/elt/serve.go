package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"

	"elt/internal/api"
	"elt/internal/store"
	"elt/internal/webui"
)

const defaultServeListen = "127.0.0.1:8080"

const serveHelp = `elt serve --data-dir <dir> [--listen host:port] [--max-sync-mb=N] [--no-ui] [--api-listen host:port]

Host a project's HTTP+JSON API — the same API every other client
(the browser GUI "elt gui" serves, the terminal client, every
"elt project <verb>" command) talks to — for other clients to connect to
over the network.
Opens <dir> if it already holds a project, or initializes a brand-new
one there on first run. Runs until stopped (Ctrl+C) — every mutation is
already durably saved to disk as it happens, there's nothing to flush
on exit. --listen/--max-sync-mb, once set here or via the GUI's own
Settings > Server page, are persisted per-project (in <dir>/.elt-local/
deployment.json) and reused on the next unflagged "elt serve" against
the same directory — an explicit flag on a given run always wins and is
never itself written back as the new remembered default.

--no-ui skips mounting the browser GUI's static assets entirely — a
plain JSON API with a normal 404 for anything else, meant for a headless
server deployment where every participant connects with their own
"elt gui --addr=..." (a thin client) or the CLI instead of opening this
process's own address in a browser directly.

--api-listen host:port additionally exposes a second, UI-free copy of
the API on its own address, sharing the exact same project as --listen —
one process, two independently-exposed addresses, instead of needing to
run a second elt instance just to give the API a different network
exposure than the UI. Meant for the opposite case from --no-ui: here
--listen keeps serving the full UI+API combination (e.g. on a LAN-only
address for people opening it in a browser), while --api-listen opens
just the API more widely (e.g. to the internet, for remote "elt gui
--addr="/CLI clients). Not persisted across runs — pass it every launch,
the same as --no-ui. Meaningless combined with --no-ui itself (nothing
left for it to be "the UI-free copy" of); combining them is an error.
Multi-user access becomes available the moment either address is
non-loopback-only, not just --listen — see SetLoopbackOnly's own doc.
`

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), serveHelp)
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}

	listen := fs.String("listen", "", fmt.Sprintf("address to listen on (default %s, or the project's own persisted value)", defaultServeListen))
	dataDir := fs.String("data-dir", "", "project directory (required)")
	projectName := fs.String("project", "New Project", "project name; only used the first time --data-dir is initialized")
	maxSyncMB := fs.Int("max-sync-mb", 0, fmt.Sprintf("maximum upload size in MB for POST .../sync (default %d, or the project's own persisted value)", api.DefaultMaxSyncBodyBytes/(1024*1024)))
	noUI := fs.Bool("no-ui", false, "don't serve the browser GUI's static assets — a plain JSON API only")
	apiListen := fs.String("api-listen", "", "additionally expose a UI-free copy of the API on this address, alongside --listen's own UI+API address (not persisted; incompatible with --no-ui)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dataDir == "" {
		return fmt.Errorf("usage: elt serve --data-dir <dir> [--listen host:port] [--max-sync-mb=N] [--no-ui] [--api-listen host:port]")
	}
	if *noUI && *apiListen != "" {
		return fmt.Errorf("--api-listen is meaningless with --no-ui — --listen is already API-only; pass --listen directly instead")
	}

	st, err := store.Open(*dataDir, *projectName)
	if err != nil {
		return err
	}
	deployment, _, err := st.Deployment()
	if err != nil {
		return err
	}

	srv, err := api.NewServer(st)
	if err != nil {
		return err
	}
	applyMaxSyncMBFlag(srv, resolveSettingInt(*maxSyncMB, deployment.MaxSyncMB, 0))
	srv.SetListenConfigHooks(serveListenConfigHooks(st))

	resolvedListen := resolveSetting(*listen, deployment.Listen, defaultServeListen)

	loopbackOnly := isLoopbackListenAddr(resolvedListen) && (*apiListen == "" || isLoopbackListenAddr(*apiListen))
	srv.SetLoopbackOnly(loopbackOnly)
	var fallback http.Handler
	if !*noUI {
		fallback = webui.Handler()
	}
	handler := srv.Handler(fallback)

	ln, err := net.Listen("tcp", resolvedListen)
	if err != nil {
		return err
	}
	ln, scheme, err := wrapListenerForTLS(ln, deployment.TLS, st)
	if err != nil {
		return err
	}

	if *apiListen != "" {
		if _, err := startAPIOnlyListener(srv, *apiListen, deployment.TLS, st); err != nil {
			return err
		}
	}

	fmt.Printf("elt serve: listening on %s://%s, project directory %s\n", scheme, ln.Addr(), st.Dir())
	httpSrv := newHTTPServer(handler)
	return httpSrv.Serve(ln)
}

func applyMaxSyncMBFlag(srv *api.Server, mb int) {
	if mb <= 0 {
		return
	}
	srv.SetMaxSyncBodyBytes(int64(mb) * 1024 * 1024)
}

func updateDeployment(st *store.Store, mutate func(*store.DeploymentConfig)) error {
	cfg, _, err := st.Deployment()
	if err != nil {
		return err
	}
	mutate(&cfg)
	return st.SetDeployment(cfg)
}

func serveListenConfigHooks(st *store.Store) api.ListenConfigHooks {
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

func isLoopbackListenAddr(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
