package main

import (
	"bytes"
	"net/http"
	"testing"

	"com.birdhalfbaked.aml-toolkit/internal/httpserver"
)

func TestApiDispatch_JSON(t *testing.T) {
	stack, err := httpserver.OpenStack(httpserver.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stack.DB.Close() })

	h := httpserver.NewHandler(stack, "", nil)
	app := NewApp(stack, nil, h)

	out, err := app.ApiDispatch(http.MethodGet, "/api/projects", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", out.Status)
	}
	if out.ContentType == "" {
		t.Fatal("expected Content-Type")
	}
	if !bytes.Contains(out.Body, []byte("[")) {
		t.Fatalf("body = %q", out.Body)
	}
}

func TestApiDispatch_CreateProject_201Body(t *testing.T) {
	stack, err := httpserver.OpenStack(httpserver.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stack.DB.Close() })

	h := httpserver.NewHandler(stack, "", nil)
	app := NewApp(stack, nil, h)

	payload := []byte(`{"name":"ApiDispatchProj"}`)
	out, err := app.ApiDispatch(http.MethodPost, "/api/projects", "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != http.StatusCreated {
		t.Fatalf("status = %d, body=%q", out.Status, out.Body)
	}
	if len(out.Body) < 10 {
		t.Fatalf("unexpected empty or short body: %q", out.Body)
	}
	if !bytes.Contains(out.Body, []byte(`"id"`)) {
		t.Fatalf("body missing id: %q", out.Body)
	}
}

func TestApiDispatch_404(t *testing.T) {
	stack, err := httpserver.OpenStack(httpserver.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stack.DB.Close() })

	h := httpserver.NewHandler(stack, "", nil)
	app := NewApp(stack, nil, h)

	out, err := app.ApiDispatch(http.MethodGet, "/api/projects/999999", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", out.Status)
	}
}
