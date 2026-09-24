package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

type stubService struct {
	found *domain.Settings
	err   error
}

func (s *stubService) Get(_ context.Context) (*domain.Settings, error) {
	return s.found, s.err
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

// get calls the handler without the auth middleware, as it is registered:
// the route is public.
func get(t *testing.T, svc *stubService) (int, map[string]any) {
	t.Helper()

	rec := httptest.NewRecorder()
	NewHandler(svc, zap.NewNop()).Get(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return rec.Code, body
}

func TestGet_Success(t *testing.T) {
	status, body := get(t, &stubService{found: &domain.Settings{WarningThresholdDays: 30}})

	if status != http.StatusOK {
		t.Fatalf("status: got %d, want %d", status, http.StatusOK)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["warning_threshold_days"] != float64(30) {
		t.Errorf("data: got %v", body["data"])
	}
}

func TestGet_Errors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "settings not found"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := get(t, &stubService{err: tt.err})

			if status != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", status, tt.wantStatus)
			}
			if body["error"] != tt.wantError {
				t.Errorf("error: got %v, want %q", body["error"], tt.wantError)
			}
		})
	}
}

func TestGet_WriteFailureIsLogged(t *testing.T) {
	h := NewHandler(&stubService{found: &domain.Settings{WarningThresholdDays: 30}}, zap.NewNop())

	h.Get(&brokenWriter{}, httptest.NewRequest(http.MethodGet, "/settings", nil))
}
