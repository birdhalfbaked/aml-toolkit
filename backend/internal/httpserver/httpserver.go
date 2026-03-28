package httpserver

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net"
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

// Config overrides default paths from env ([db.DefaultDataDir], [db.DefaultDBPath], AUDIO_TAGGER_LIBRARY).
// FrontendDir, if empty, is resolved in [NewHandler] from AUDIO_TAGGER_FRONTEND_DIR or ../frontend/dist.
// LibraryDir is the root for projects/collections on disk; if empty, [db.DefaultDataDir] is used (CLI default).
type Config struct {
	DataDir     string // deprecated for library: use LibraryDir; still used if LibraryDir empty and no env
	DBPath      string
	LibraryDir  string
	FrontendDir string
}

// Stack holds open DB and HTTP router wiring for [NewHandler].
type Stack struct {
	DB     *sql.DB
	Repo   *repo.Repo
	Router *httprouter.Router
	Server *handlers.Server
}

func resolvedDataDir(c Config) string {
	if c.DataDir != "" {
		return c.DataDir
	}
	return db.DefaultDataDir()
}

func resolvedDBPath(c Config) string {
	if c.DBPath != "" {
		return c.DBPath
	}
	return db.DefaultDBPath()
}

func resolvedLibraryDir(c Config) string {
	if c.LibraryDir != "" {
		return c.LibraryDir
	}
	if v := os.Getenv("AUDIO_TAGGER_LIBRARY"); v != "" {
		return v
	}
	return resolvedDataDir(c)
}

// OpenStack creates dirs, opens SQLite, layout, and registers API routes.
func OpenStack(cfg Config) (*Stack, error) {
	dbPath := resolvedDBPath(cfg)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	libraryDir := resolvedLibraryDir(cfg)
	if libraryDir == "" {
		return nil, fmt.Errorf("library directory is empty")
	}

	sqldb, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}

	layout, err := store.NewLayout(libraryDir)
	if err != nil {
		_ = sqldb.Close()
		return nil, err
	}

	rp := &repo.Repo{DB: sqldb}
	srv := handlers.NewServer(rp, layout)

	router := httprouter.New()
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, err interface{}) {
		log.Printf("panic: %v", err)
		http.Error(w, "internal error", 500)
	}
	srv.Register(router)

	return &Stack{DB: sqldb, Repo: rp, Router: router, Server: srv}, nil
}

// NewHandler builds the top-level mux: CORS preflight for /api, API + favicon, optional SPA fallback.
// If frontendDir resolves to a directory containing index.html, it is used; otherwise uiEmbedded (e.g. Wails
// release dist embedded next to main) is used when non-nil and contains index.html. Under Wails, the same
// fs is usually also passed as assetserver.Options.Assets so static GETs are served by Wails before this handler.
func NewHandler(stack *Stack, frontendDir string, uiEmbedded fs.FS) http.Handler {
	dir := frontendDir
	if dir == "" {
		dir = os.Getenv("AUDIO_TAGGER_FRONTEND_DIR")
		if dir == "" {
			dir = filepath.FromSlash("../frontend/dist")
		}
	}
	absFrontendDir, _ := filepath.Abs(dir)
	var spaHandler http.Handler
	if st, err := os.Stat(absFrontendDir); err == nil && st.IsDir() {
		if _, err := os.Stat(filepath.Join(absFrontendDir, "index.html")); err == nil {
			spaHandler = handlers.NewSpaHandler(absFrontendDir)
			log.Printf("serving frontend from %s", absFrontendDir)
		}
	}
	if spaHandler == nil && uiEmbedded != nil {
		if _, err := fs.Stat(uiEmbedded, "index.html"); err == nil {
			spaHandler = handlers.NewSpaHandlerFS(uiEmbedded)
			log.Printf("serving frontend from embedded static assets")
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/favicon") {
			stack.Router.ServeHTTP(w, r)
			return
		}
		if spaHandler != nil {
			spaHandler.ServeHTTP(w, r)
			return
		}
		stack.Router.ServeHTTP(w, r)
	})
}

// ListenAddr returns ":8080" or ":"+PORT when PORT is set.
func ListenAddr() string {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	return addr
}

// Serve runs the handler on an existing listener (e.g. TCP on 127.0.0.1:0 for Wails).
func Serve(l net.Listener, h http.Handler) error {
	return http.Serve(l, h)
}
