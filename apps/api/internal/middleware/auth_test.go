package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/jwt"
	"go.uber.org/zap"

	"apps/api/internal/domain"
)

var secret = []byte("test-secret")

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

func tokens(t *testing.T, payload domain.AccountPayload, signWith []byte, accessExpiry time.Duration) (string, string) {
	t.Helper()

	access, refresh, err := jwt.BuildTokenPair(payload, payload.UserID, signWith, accessExpiry, time.Hour)
	if err != nil {
		t.Fatalf("build token pair: %v", err)
	}

	return access, refresh
}

// serve runs a request through the middleware and reports whether the wrapped
// handler was called along with the account it saw.
func serve(header string) (*httptest.ResponseRecorder, bool, domain.AccountPayload) {
	var (
		called  bool
		account domain.AccountPayload
	)

	next := func(_ http.ResponseWriter, r *http.Request) {
		called = true
		account, _ = AccountFromContext(r.Context())
	}

	req := httptest.NewRequest(http.MethodPost, "/medicines", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()

	Auth(secret, zap.NewNop())(next)(rec, req)

	return rec, called, account
}

func TestAuth_ValidToken(t *testing.T) {
	payload := domain.AccountPayload{UserID: "user-1", Email: "a@b.com", Roles: []string{"admin"}}
	access, _ := tokens(t, payload, secret, time.Hour)

	tests := map[string]string{
		"Bearer":           "Bearer " + access,
		"lowercase scheme": "bearer " + access,
		"padded token":     "Bearer   " + access + "  ",
	}

	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			rec, called, account := serve(header)

			if !called {
				t.Fatalf("next not called, status %d", rec.Code)
			}
			if account.UserID != payload.UserID || account.Email != payload.Email {
				t.Errorf("account in context: got %+v, want %+v", account, payload)
			}
			if len(account.Roles) != 1 || account.Roles[0] != "admin" {
				t.Errorf("roles in context: got %v", account.Roles)
			}
		})
	}
}

func TestAuth_Rejected(t *testing.T) {
	valid := domain.AccountPayload{UserID: "user-1", Email: "a@b.com"}
	access, refresh := tokens(t, valid, secret, time.Hour)
	expired, _ := tokens(t, valid, secret, -time.Hour)
	wrongSecret, _ := tokens(t, valid, []byte("other-secret"), time.Hour)
	noUser, _ := tokens(t, domain.AccountPayload{Email: "a@b.com"}, secret, time.Hour)

	tests := map[string]string{
		"missing header":  "",
		"wrong scheme":    "Basic " + access,
		"no space":        "Bearer" + access,
		"scheme only":     "Bearer",
		"empty token":     "Bearer ",
		"blank token":     "Bearer    ",
		"malformed token": "Bearer not-a-jwt",
		"wrong secret":    "Bearer " + wrongSecret,
		"expired token":   "Bearer " + expired,
		"refresh token":   "Bearer " + refresh,
		"empty user id":   "Bearer " + noUser,
	}

	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			rec, called, _ := serve(header)

			if called {
				t.Fatal("next was called for a rejected request")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate: got %q, want Bearer", got)
			}
			if got := rec.Body.String(); got != "{\"error\":\"unauthorized\"}\n" {
				t.Errorf("body: got %q", got)
			}
		})
	}
}

func TestAuth_WriteFailure(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/medicines", nil)

	// Must not panic when the response can't be written.
	Auth(secret, zap.NewNop())(func(http.ResponseWriter, *http.Request) {})(&brokenWriter{}, req)
}

func TestAccountFromContext_Empty(t *testing.T) {
	if _, ok := AccountFromContext(context.Background()); ok {
		t.Error("expected no account in an empty context")
	}
}
