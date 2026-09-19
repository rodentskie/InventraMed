package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func ok(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func status(r *Router, method, path string) int {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

	return rec.Code
}

func TestHandle_AppliesPrefix(t *testing.T) {
	r := New("/api")
	r.Handle("POST /login", ok)

	if got := status(r, http.MethodPost, "/api/login"); got != http.StatusOK {
		t.Errorf("POST /api/login: got %d, want %d", got, http.StatusOK)
	}
	if got := status(r, http.MethodPost, "/login"); got != http.StatusNotFound {
		t.Errorf("POST /login: got %d, want %d", got, http.StatusNotFound)
	}
}

func TestHandle_CustomPrefix(t *testing.T) {
	r := New("/api/v1")
	r.Handle("POST /login", ok)

	if got := status(r, http.MethodPost, "/api/v1/login"); got != http.StatusOK {
		t.Errorf("POST /api/v1/login: got %d, want %d", got, http.StatusOK)
	}
}

func TestHandle_EmptyPrefix(t *testing.T) {
	r := New("")
	r.Handle("POST /login", ok)

	if got := status(r, http.MethodPost, "/login"); got != http.StatusOK {
		t.Errorf("POST /login: got %d, want %d", got, http.StatusOK)
	}
}

func TestHandle_PatternWithoutMethod(t *testing.T) {
	r := New("/api")
	r.Handle("/login", ok)

	if got := status(r, http.MethodGet, "/api/login"); got != http.StatusOK {
		t.Errorf("GET /api/login: got %d, want %d", got, http.StatusOK)
	}
}

func TestHandle_MethodIsEnforced(t *testing.T) {
	r := New("/api")
	r.Handle("POST /login", ok)

	if got := status(r, http.MethodGet, "/api/login"); got != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/login: got %d, want %d", got, http.StatusMethodNotAllowed)
	}
}

func TestHandleExempt_IgnoresPrefix(t *testing.T) {
	r := New("/api")
	r.HandleExempt("GET /{$}", ok)
	r.HandleExempt("GET /ping", ok)

	if got := status(r, http.MethodGet, "/"); got != http.StatusOK {
		t.Errorf("GET /: got %d, want %d", got, http.StatusOK)
	}
	if got := status(r, http.MethodGet, "/ping"); got != http.StatusOK {
		t.Errorf("GET /ping: got %d, want %d", got, http.StatusOK)
	}
	if got := status(r, http.MethodGet, "/api/ping"); got != http.StatusNotFound {
		t.Errorf("GET /api/ping: got %d, want %d", got, http.StatusNotFound)
	}
}
