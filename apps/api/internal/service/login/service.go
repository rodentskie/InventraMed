package login

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/jwt"
	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/user"
	"apps/api/pkg/apperror"
	"libs/go/hash"
)

// Tokens is the access/refresh token pair issued on a successful login.
type Tokens struct {
	AccessToken  string
	RefreshToken string
}

// Service owns the login business logic.
type Service interface {
	Login(ctx context.Context, email, password string) (*Tokens, error)
}

// TokenConfig holds the JWT signing settings.
type TokenConfig struct {
	Secret        []byte
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// dummyHash is a valid bcrypt hash (cost 10, like real users) compared against
// when the email is unknown, so response time doesn't reveal which emails exist.
const dummyHash = "$2a$10$hk.Sa/Apz5dzwOPg0SESJu2pJTJHXmrnEKqzEJYfmfJ3gOclTgnB."

type service struct {
	repo   user.Repository
	tokens TokenConfig
	log    *zap.Logger
}

func NewService(repo user.Repository, tokens TokenConfig, log *zap.Logger) Service {
	return &service{repo: repo, tokens: tokens, log: log}
}

// Login verifies the credentials and returns a signed token pair. An unknown
// email and a wrong password both return apperror.ErrUnauthorized.
func (s *service) Login(ctx context.Context, email, password string) (*Tokens, error) {
	u, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, apperror.ErrNotFound) {
		// Result is irrelevant: this only spends the same time as a real compare.
		_ = hash.Compare(dummyHash, password)
		return nil, apperror.ErrUnauthorized
	}
	if err != nil {
		s.log.Error("find user failed", zap.Error(err))
		return nil, fmt.Errorf("login: %w", err)
	}

	if err := hash.Compare(u.PasswordHash, password); err != nil {
		if errors.Is(err, hash.ErrMismatch) {
			return nil, apperror.ErrUnauthorized
		}
		s.log.Error("compare password failed", zap.Error(err))
		return nil, fmt.Errorf("login: %w", err)
	}

	return s.issueTokens(u)
}

func (s *service) issueTokens(u *domain.User) (*Tokens, error) {
	payload := domain.AccountPayload{UserID: u.ID, Email: u.Email, Roles: u.Roles}

	access, refresh, err := jwt.BuildTokenPair(
		payload,
		u.ID,
		s.tokens.Secret,
		s.tokens.AccessExpiry,
		s.tokens.RefreshExpiry,
	)
	if err != nil {
		s.log.Error("build token pair failed", zap.Error(err))
		return nil, fmt.Errorf("login: %w", err)
	}

	return &Tokens{AccessToken: access, RefreshToken: refresh}, nil
}
