package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWritesPayload(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := JSON(rec, http.StatusOK, map[string]string{"message": "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	if got := rec.Body.String(); got != "{\"message\":\"hello\"}\n" {
		t.Fatalf("unexpected body: %q", got)
	}
}

type brokenWriter struct {
	header http.Header
}

func (w *brokenWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *brokenWriter) WriteHeader(int) {}

func TestJSONReturnsErrorOnWriteFailure(t *testing.T) {
	if err := JSON(&brokenWriter{}, http.StatusOK, map[string]string{"message": "hello"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestErrorWritesMessage(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := Error(rec, http.StatusUnauthorized, "invalid credentials"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if got := rec.Body.String(); got != "{\"error\":\"invalid credentials\"}\n" {
		t.Fatalf("unexpected body: %q", got)
	}
}
