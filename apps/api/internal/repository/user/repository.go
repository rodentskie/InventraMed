package user

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

// Repository reads users and their roles.
type Repository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

type userRecord struct {
	ID           string `gorm:"type:uuid;primaryKey"`
	Email        string
	Name         string
	PasswordHash string
}

func (userRecord) TableName() string {
	return "users"
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// FindByEmail returns the active user with the given email and their role
// names, or apperror.ErrNotFound. Soft-deleted users and roles are excluded.
func (r *repository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var record userRecord

	err := r.db.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", email).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	roles, err := r.findRoleNames(ctx, record.ID)
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:           record.ID,
		Email:        record.Email,
		Name:         record.Name,
		PasswordHash: record.PasswordHash,
		Roles:        roles,
	}, nil
}

func (r *repository) findRoleNames(ctx context.Context, userID string) ([]string, error) {
	roles := []string{}

	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND roles.deleted_at IS NULL", userID).
		Pluck("roles.name", &roles).Error
	if err != nil {
		return nil, fmt.Errorf("find user roles: %w", err)
	}

	return roles, nil
}
