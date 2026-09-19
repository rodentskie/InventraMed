package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/service/login"
	"apps/api/pkg/apperror"
)

type stubService struct {
	tokens *login.Tokens
	err    error
	email  string
	called bool
}

func (s *stubService) Login(_ context.Context, email, _ string) (*login.Tokens, error) {
	s.called = true
	s.email = email

	return s.tokens, s.err
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

func post(h *Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return body
}

func TestLogin_Success(t *testing.T) {
	svc := &stubService{tokens: &login.Tokens{AccessToken: "a", RefreshToken: "r"}}
	h := NewHandler(svc, zap.NewNop())

	rec := post(h, `{"email":"  Admin@Local.com ","password":"secret"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.email != "admin@local.com" {
		t.Errorf("email passed to service: got %q, want trimmed and lowercased", svc.email)
	}

	body := decode(t, rec)
	if body["access_token"] != "a" || body["refresh_token"] != "r" {
		t.Errorf("body: got %v", body)
	}
}

func TestLogin_BadRequest(t *testing.T) {
	tests := map[string]struct {
		body    string
		message string
	}{
		"malformed json":   {`{"email":`, "invalid request body"},
		"empty body":       {``, "invalid request body"},
		"missing email":    {`{"password":"secret"}`, "email is required"},
		"blank email":      {`{"email":"   ","password":"secret"}`, "email is required"},
		"invalid email":    {`{"email":"not-an-email","password":"secret"}`, "email is invalid"},
		"display name":     {`{"email":"Admin <admin@local.com>","password":"secret"}`, "email is invalid"},
		"missing password": {`{"email":"admin@local.com"}`, "password is required"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := post(h, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := decode(t, rec)["error"]; got != tc.message {
				t.Errorf("error: got %q, want %q", got, tc.message)
			}
			if svc.called {
				t.Error("service must not be called for an invalid request")
			}
		})
	}
}

func TestLogin_Unauthorized(t *testing.T) {
	h := NewHandler(&stubService{err: apperror.ErrUnauthorized}, zap.NewNop())

	rec := post(h, `{"email":"admin@local.com","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := decode(t, rec)["error"]; got != "invalid credentials" {
		t.Errorf("error: got %q, want invalid credentials", got)
	}
}

func TestLogin_InternalError(t *testing.T) {
	h := NewHandler(&stubService{err: errors.New("db down")}, zap.NewNop())

	rec := post(h, `{"email":"admin@local.com","password":"secret"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("error: got %q, want internal server error", got)
	}
}

func TestLogin_LogsWriteFailure(t *testing.T) {
	svc := &stubService{tokens: &login.Tokens{AccessToken: "a", RefreshToken: "r"}}
	h := NewHandler(svc, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"admin@local.com","password":"secret"}`))

	h.Login(&brokenWriter{}, req)
}
