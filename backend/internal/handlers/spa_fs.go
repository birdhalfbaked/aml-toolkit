package handlers

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewSpaHandlerFS serves a static tree from fsys (e.g. embed.FS) with SPA fallback to index.html.
func NewSpaHandlerFS(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "" {
			p = "/"
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		clean := path.Clean(p)

		tryPath := strings.TrimPrefix(clean, "/")
		if tryPath == "" || tryPath == "." {
			tryPath = "index.html"
		}

		// Root and /index.html: serve bytes directly. http.FileServer(http.FS(embed.FS)) can 301 for
		// "/index.html", and Wails replaces non-200 document responses with its default "index not found" page.
		if tryPath == "index.html" {
			if data, err := fs.ReadFile(fsys, "index.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
				return
			}
			http.NotFound(w, r)
			return
		}

		if f, err := fsys.Open(tryPath); err == nil {
			_ = f.Close()
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + tryPath
			fileServer.ServeHTTP(w, r2)
			return
		}

		if data, err := fs.ReadFile(fsys, "index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}

		http.NotFound(w, r)
	})
}
