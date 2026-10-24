package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

func openFS() http.FileSystem {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {

		panic("webui: dist/ was not embedded correctly: " + err.Error())
	}
	return http.FS(sub)
}

func HasFile(path string) bool {
	f, err := openFS().Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()

	return err == nil && !info.IsDir()
}

func Handler() http.Handler {
	httpFS := openFS()
	fileServer := http.FileServer(httpFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		isIndex := path == "/" || path == "/index.html"

		if f, err := httpFS.Open(path); err == nil {
			f.Close()
		} else {

			r = r.Clone(r.Context())
			r.URL.Path = "/"
			isIndex = true
		}

		if isIndex {

			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}
