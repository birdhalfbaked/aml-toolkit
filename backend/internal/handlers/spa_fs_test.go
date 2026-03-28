package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNewSpaHandlerFS_servesFileAndSpaFallback(t *testing.T) {
	t.Parallel()
	fs := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>idx</html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	h := NewSpaHandlerFS(fs)

	rr0 := httptest.NewRecorder()
	h.ServeHTTP(rr0, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr0.Code != http.StatusOK {
		t.Fatalf("GET / status %d body=%q", rr0.Code, rr0.Body.String())
	}
	if !strings.Contains(rr0.Body.String(), "idx") {
		t.Fatalf("GET / body %q", rr0.Body.String())
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /app.js status %d", rr.Code)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/missing-route", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("SPA fallback status %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "idx") {
		t.Fatalf("fallback body %q", rr2.Body.String())
	}
}
