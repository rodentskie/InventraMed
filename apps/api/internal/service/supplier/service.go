package supplier

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/supplier"
	"apps/api/pkg/apperror"
)

// CreateInput is the validated data needed to create a supplier.
type CreateInput struct {
	Name        string
	ContactName *string
	Email       *string
	Phone       *string
	Address     *string
}

// UpdateInput is the validated data needed to update a supplier. It has the
// same fields as CreateInput; an update replaces all of them.
type UpdateInput = CreateInput

// ListFilter selects one page of suppliers, optionally filtered by name.
type ListFilter = supplier.ListFilter

// Page is one page of suppliers and the total number matching the filter.
type Page struct {
	Suppliers []*domain.Supplier
	Total     int64
}

// Service owns the supplier business logic.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*domain.Supplier, error)
	List(ctx context.Context, filter ListFilter) (*Page, error)
	GetByID(ctx context.Context, id string) (*domain.Supplier, error)
	Update(ctx context.Context, id string, input UpdateInput) error
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo supplier.Repository
	log  *zap.Logger
}

func NewService(repo supplier.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

// Create registers a new supplier. It returns apperror.ErrSupplierNameExists
// when an active supplier already has the name, ignoring case. Only the name
// must be unique; email and phone may repeat.
func (s *service) Create(ctx context.Context, input CreateInput) (*domain.Supplier, error) {
	taken, err := s.repo.ExistsByName(ctx, input.Name, "")
	if err != nil {
		return nil, s.fail("create", err)
	}
	if taken {
		return nil, apperror.ErrSupplierNameExists
	}

	created, err := s.repo.Create(ctx, &domain.Supplier{
		Name:        input.Name,
		ContactName: input.ContactName,
		Email:       input.Email,
		Phone:       input.Phone,
		Address:     input.Address,
	})
	if err != nil {
		return nil, s.fail("create", err)
	}

	return created, nil
}

// List returns one page of suppliers and the total matching the filter.
func (s *service) List(ctx context.Context, filter ListFilter) (*Page, error) {
	suppliers, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, s.fail("list", err)
	}

	return &Page{Suppliers: suppliers, Total: total}, nil
}

// GetByID returns the supplier with the ID, or apperror.ErrNotFound.
func (s *service) GetByID(ctx context.Context, id string) (*domain.Supplier, error) {
	found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.fail("get", err)
	}

	return found, nil
}

// Update replaces the supplier's details. It returns apperror.ErrNotFound for
// an unknown ID before it checks the name, so an unknown ID is never masked by
// a duplicate, and apperror.ErrSupplierNameExists when another supplier has
// the name. The supplier's own name never counts as a duplicate.
func (s *service) Update(ctx context.Context, id string, input UpdateInput) error {
	err := s.repo.Transaction(ctx, func(tx supplier.Repository) error {
		found, err := tx.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return apperror.ErrNotFound
		}

		taken, err := tx.ExistsByName(ctx, input.Name, id)
		if err != nil {
			return err
		}
		if taken {
			return apperror.ErrSupplierNameExists
		}

		return tx.Update(ctx, &domain.Supplier{
			ID:          id,
			Name:        input.Name,
			ContactName: input.ContactName,
			Email:       input.Email,
			Phone:       input.Phone,
			Address:     input.Address,
		})
	})
	if err != nil {
		return s.fail("update", err)
	}

	return nil
}

// Delete soft-deletes a supplier. It returns apperror.ErrNotFound for an
// unknown ID and apperror.ErrSupplierInPurchaseOrder while a purchase order
// uses the supplier.
func (s *service) Delete(ctx context.Context, id string) error {
	err := s.repo.Transaction(ctx, func(tx supplier.Repository) error {
		found, err := tx.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return apperror.ErrNotFound
		}

		used, err := tx.ExistsInPurchaseOrder(ctx, id)
		if err != nil {
			return err
		}
		if used {
			return apperror.ErrSupplierInPurchaseOrder
		}

		return tx.Delete(ctx, id)
	})
	if err != nil {
		return s.fail("delete", err)
	}

	return nil
}

// fail returns not found and conflict errors unchanged, so the handler can
// map them. Anything else is logged and wrapped.
func (s *service) fail(operation string, err error) error {
	if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
		return err
	}

	s.log.Error(operation+" supplier failed", zap.Error(err))

	return fmt.Errorf("%s supplier: %w", operation, err)
}
