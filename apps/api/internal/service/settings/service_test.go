package settings

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

type stubRepo struct {
	found *domain.Settings
	err   error
}

func (s *stubRepo) Get(_ context.Context) (*domain.Settings, error) {
	return s.found, s.err
}

func TestGet(t *testing.T) {
	svc := NewService(&stubRepo{found: &domain.Settings{WarningThresholdDays: 14}}, zap.NewNop())

	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WarningThresholdDays != 14 {
		t.Errorf("threshold: got %d, want 14", got.WarningThresholdDays)
	}
}

func TestGet_NotFoundIsReturnedUnchanged(t *testing.T) {
	svc := NewService(&stubRepo{err: apperror.ErrNotFound}, zap.NewNop())

	if _, err := svc.Get(context.Background()); err != apperror.ErrNotFound {
		t.Errorf("got %v, want %v", err, apperror.ErrNotFound)
	}
}

func TestGet_RepositoryErrorIsWrapped(t *testing.T) {
	boom := errors.New("connection reset")
	svc := NewService(&stubRepo{err: boom}, zap.NewNop())

	_, err := svc.Get(context.Background())
	if !errors.Is(err, boom) || err == boom {
		t.Errorf("got %v, want it to wrap %v", err, boom)
	}
}
