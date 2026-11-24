package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var distFS embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

func Handler() http.Handler {
	fsys, err := FS()
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(fsys))
	indexHTML, _ := fs.ReadFile(fsys, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := fsys.Open(p); err == nil {
			st, statErr := f.Stat()
			f.Close()
			if statErr == nil && !st.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback
		if len(indexHTML) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
	})
}
