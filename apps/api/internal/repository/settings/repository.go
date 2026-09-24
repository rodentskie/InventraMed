package settings

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

// Repository reads the settings.
type Repository interface {
	// Get returns the settings row, or apperror.ErrNotFound when the table is
	// empty. The table holds a single row; should there be more, the first by
	// ID wins.
	Get(ctx context.Context) (*domain.Settings, error)
}

type record struct {
	ID                   string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WarningThresholdDays int
}

func (record) TableName() string {
	return "settings"
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Get(ctx context.Context) (*domain.Settings, error) {
	var rec record

	err := r.db.WithContext(ctx).Order("id").Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	return &domain.Settings{WarningThresholdDays: rec.WarningThresholdDays}, nil
}
