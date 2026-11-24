package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
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
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "." {
			p = "index.html"
		}

		// Never fall back to index.html for asset/file requests. A missing
		// hashed asset must return a real 404, otherwise the browser receives
		// HTML with a JS MIME type and renders a white page.
		if strings.HasPrefix(p, "assets/") || hasFileExtension(p) {
			if f, err := fsys.Open(p); err == nil {
				st, statErr := f.Stat()
				f.Close()
				if statErr == nil && !st.IsDir() {
					w.Header().Set("X-Content-Type-Options", "nosniff")
					switch {
					case p == "index.html":
						w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					case strings.HasPrefix(p, "assets/"):
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					default:
						w.Header().Set("Cache-Control", "no-cache")
					}
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			http.NotFound(w, r)
			return
		}

		// SPA fallback. index.html must always be revalidated so a reload
		// picks up a new deployment's hashed assets.
		if len(indexHTML) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexHTML)
	})
}

func hasFileExtension(p string) bool {
	return strings.Contains(path.Base(p), ".")
}
