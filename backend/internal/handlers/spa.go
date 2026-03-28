package handlers

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// NewSpaHandler serves a static directory and falls back to index.html for unknown paths.
// This supports client-side routing in the frontend (Vue Router history mode).
func NewSpaHandler(distDir string) http.Handler {
	fs := http.Dir(distDir)
	fileServer := http.FileServer(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalize and prevent path traversal (http.Dir already protects, but keep it tidy).
		p := r.URL.Path
		if p == "" {
			p = "/"
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		clean := path.Clean(p)

		// If the file exists, serve it directly.
		// http.Dir and http.FileServer require slash-separated paths; on Windows
		// filepath.FromSlash would use '\' and fs.Open rejects that (see net/http Dir.Open).
		tryPath := strings.TrimPrefix(clean, "/")
		if tryPath == "" || tryPath == "." {
			tryPath = "index.html"
		}
		if f, err := fs.Open(tryPath); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Otherwise serve index.html (SPA fallback).
		if _, err := os.Stat(filepath.Join(distDir, "index.html")); err == nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}

		http.NotFound(w, r)
	})
}

