package settings

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/settings"
	"apps/api/pkg/apperror"
)

// Service reads the inventory settings.
type Service interface {
	// Get returns the settings, or apperror.ErrNotFound when none are stored.
	Get(ctx context.Context) (*domain.Settings, error)
}

type service struct {
	repo settings.Repository
	log  *zap.Logger
}

func NewService(repo settings.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

func (s *service) Get(ctx context.Context) (*domain.Settings, error) {
	found, err := s.repo.Get(ctx)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		s.log.Error("get settings failed", zap.Error(err))
		return nil, fmt.Errorf("get settings: %w", err)
	}

	return found, nil
}
