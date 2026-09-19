package login

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/jwt"
	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
	"libs/go/hash"
)

var testTokens = TokenConfig{
	Secret:        []byte("test-secret"),
	AccessExpiry:  15 * time.Minute,
	RefreshExpiry: 168 * time.Hour,
}

type stubRepository struct {
	user *domain.User
	err  error
}

func (s *stubRepository) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return s.user, s.err
}

func newTestService(t *testing.T, repo *stubRepository) Service {
	t.Helper()

	return NewService(repo, testTokens, zap.NewNop())
}

func newUser(t *testing.T, password string) *domain.User {
	t.Helper()

	hashed, err := hash.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	return &domain.User{
		ID:           "u-1",
		Email:        "admin@local.com",
		Name:         "Admin",
		PasswordHash: hashed,
		Roles:        []string{"admin"},
	}
}

func TestLogin_Success(t *testing.T) {
	svc := newTestService(t, &stubRepository{user: newUser(t, "secret")})

	tokens, err := svc.Login(context.Background(), "admin@local.com", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	access, err := jwt.ParseAccessToken[domain.AccountPayload](tokens.AccessToken, testTokens.Secret)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if access.Subject != "u-1" {
		t.Errorf("Subject: got %q, want u-1", access.Subject)
	}
	if access.Payload.UserID != "u-1" || access.Payload.Email != "admin@local.com" {
		t.Errorf("Payload: got %+v", access.Payload)
	}
	if len(access.Payload.Roles) != 1 || access.Payload.Roles[0] != "admin" {
		t.Errorf("Roles: got %v, want [admin]", access.Payload.Roles)
	}

	refresh, err := jwt.ParseToken[domain.AccountPayload](tokens.RefreshToken, testTokens.Secret)
	if err != nil {
		t.Fatalf("ParseToken refresh: %v", err)
	}
	if refresh.TokenType != jwt.TokenTypeRefresh {
		t.Errorf("refresh TokenType: got %q, want %q", refresh.TokenType, jwt.TokenTypeRefresh)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newTestService(t, &stubRepository{err: apperror.ErrNotFound})

	tokens, err := svc.Login(context.Background(), "nobody@local.com", "secret")
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("Login: got %v, want ErrUnauthorized", err)
	}
	if tokens != nil {
		t.Error("expected nil tokens")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newTestService(t, &stubRepository{user: newUser(t, "secret")})

	tokens, err := svc.Login(context.Background(), "admin@local.com", "wrong")
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("Login: got %v, want ErrUnauthorized", err)
	}
	if tokens != nil {
		t.Error("expected nil tokens")
	}
}

func TestLogin_RepositoryError(t *testing.T) {
	repoErr := errors.New("db down")
	svc := newTestService(t, &stubRepository{err: repoErr})

	_, err := svc.Login(context.Background(), "admin@local.com", "secret")
	if !errors.Is(err, repoErr) {
		t.Fatalf("Login: got %v, want wrapped repository error", err)
	}
	if errors.Is(err, apperror.ErrUnauthorized) {
		t.Error("repository failure must not be reported as unauthorized")
	}
}

func TestLogin_MalformedStoredHash(t *testing.T) {
	u := newUser(t, "secret")
	u.PasswordHash = "not-a-bcrypt-hash"
	svc := newTestService(t, &stubRepository{user: u})

	_, err := svc.Login(context.Background(), "admin@local.com", "secret")
	if err == nil {
		t.Fatal("expected error for malformed stored hash, got nil")
	}
	if errors.Is(err, apperror.ErrUnauthorized) {
		t.Error("malformed stored hash must not be reported as unauthorized")
	}
}

func TestDummyHash_IsValid(t *testing.T) {
	if err := hash.Compare(dummyHash, "dummy-password"); err != nil {
		t.Fatalf("dummyHash must be a valid bcrypt hash: %v", err)
	}
}
