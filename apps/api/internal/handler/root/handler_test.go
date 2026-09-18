package root

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

type stubService struct {
	greeting string
}

func (s *stubService) Greeting() string {
	return s.greeting
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

func TestHandlerGet(t *testing.T) {
	handler := NewHandler(&stubService{greeting: "inventramed REST API"}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["message"] != "inventramed REST API" {
		t.Fatalf("expected greeting message, got %q", body["message"])
	}
}

func TestHandlerGetLogsWriteFailure(t *testing.T) {
	handler := NewHandler(&stubService{greeting: "inventramed REST API"}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Get(&brokenWriter{}, req)
}
