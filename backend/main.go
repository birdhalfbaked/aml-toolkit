package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/handlers"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"

	"github.com/julienschmidt/httprouter"
)

func main() {
	dataDir := db.DefaultDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	sqldb, err := db.Open(db.DefaultDBPath())
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	layout, err := store.NewLayout(dataDir)
	if err != nil {
		log.Fatal(err)
	}

	rp := &repo.Repo{DB: sqldb}
	srv := &handlers.Server{Repo: rp, Layout: layout}

	router := httprouter.New()
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, err interface{}) {
		log.Printf("panic: %v", err)
		http.Error(w, "internal error", 500)
	}
	srv.Register(router)

	// Optional: serve the built frontend (SPA) from Go.
	// This enables deep links like /project/123 without a 404 from the backend server.
	frontendDir := os.Getenv("AUDIO_TAGGER_FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = filepath.FromSlash("../frontend/dist")
	}
	absFrontendDir, _ := filepath.Abs(frontendDir)
	var spaHandler http.Handler
	if st, err := os.Stat(absFrontendDir); err == nil && st.IsDir() {
		spaHandler = handlers.NewSpaHandler(absFrontendDir)
		log.Printf("serving frontend from %s", absFrontendDir)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/favicon") {
			router.ServeHTTP(w, r)
			return
		}
		if spaHandler != nil {
			spaHandler.ServeHTTP(w, r)
			return
		}
		router.ServeHTTP(w, r)
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}
