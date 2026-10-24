# elt web UI

Svelte + Vite frontend for `elt`. This is a client of `internal/api` —
exactly like the CLI — nothing here talks to `internal/project` directly.

## Dev workflow

```bash
npm install        # once
npm run dev        # starts a dev server on :5173 with HMR
```

`npm run dev` proxies API calls (see `vite.config.js`) to a real `elt
serve`/`elt gui` instance, so run one of those separately (default
`127.0.0.1:8080`) while developing:

```bash
# in the go/ directory, in another terminal:
go run ./cmd/elt serve --data-dir /tmp/dev-project --listen 127.0.0.1:8080
```

## Building for the Go binary

```bash
npm run build
```

This writes straight into `../internal/webui/dist/`, which that package
embeds via `go:embed` and serves from the `elt` binary itself — there is no
separate frontend server in production, and Node is never a runtime
dependency of the distributed binary.

**Run this before `go build`/`go vet`/`go test`** whenever you change
anything here — `go:embed` reads `dist/` at Go build time, so a stale
build means the Go binary serves stale frontend code, not a build error.
The built `dist/` output is committed to git (see the root `.gitignore`'s
exception for this specific path) precisely so a plain `go build` works
for anyone who hasn't touched the frontend, without needing Node installed
at all.
