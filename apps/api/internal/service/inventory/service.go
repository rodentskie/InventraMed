package inventory

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/inventory"
	medicinerepo "apps/api/internal/repository/medicine"
	"apps/api/pkg/apperror"
)

// CreateInput is the validated data needed to record an inventory entry.
type CreateInput struct {
	MedicineID string
	Direction  string
	Quantity   int
	Reason     string
	Notes      *string
	// CountedBy is the ID of the user recording the entry.
	CountedBy string
}

// ListFilter selects one page of inventory entries.
type ListFilter = inventory.ListFilter

// Page is one page of inventory entries and the total number matching the
// filter.
type Page struct {
	Entries []*domain.InventoryEntry
	Total   int64
}

// Service owns the inventory entry business logic.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*domain.InventoryEntry, error)
	List(ctx context.Context, filter ListFilter) (*Page, error)
	GetByID(ctx context.Context, id string) (*domain.InventoryEntry, error)
}

type service struct {
	repo inventory.Repository
	log  *zap.Logger
}

func NewService(repo inventory.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

// Create records a stock adjustment. It returns apperror.ErrNotFound when
// MedicineID does not reference an active medicine, and
// apperror.ErrInsufficientQuantity when a subtraction would take the
// medicine's quantity below zero.
func (s *service) Create(ctx context.Context, input CreateInput) (*domain.InventoryEntry, error) {
	var created *domain.InventoryEntry

	err := s.repo.Transaction(ctx, func(tx inventory.Repository, medicines medicinerepo.Repository) error {
		quantity, found, err := medicines.LockQuantityByID(ctx, input.MedicineID)
		if err != nil {
			return err
		}
		if !found {
			return apperror.ErrNotFound
		}

		if input.Direction == domain.DirectionSubtraction && quantity < input.Quantity {
			return apperror.ErrInsufficientQuantity
		}

		created, err = tx.Create(ctx, &domain.InventoryEntry{
			MedicineID: input.MedicineID,
			Direction:  input.Direction,
			Quantity:   input.Quantity,
			Reason:     input.Reason,
			CountedBy:  domain.InventoryEntryUser{ID: input.CountedBy},
			Notes:      input.Notes,
		})

		return err
	})
	if err != nil {
		return nil, s.fail("create", err)
	}

	return created, nil
}

// List returns one page of inventory entries and the total.
func (s *service) List(ctx context.Context, filter ListFilter) (*Page, error) {
	entries, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, s.fail("list", err)
	}

	return &Page{Entries: entries, Total: total}, nil
}

// GetByID returns the entry with the ID, or apperror.ErrNotFound.
func (s *service) GetByID(ctx context.Context, id string) (*domain.InventoryEntry, error) {
	found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.fail("get", err)
	}

	return found, nil
}

// fail returns not found and conflict errors unchanged, so the handler can
// map them. Anything else is logged and wrapped.
func (s *service) fail(operation string, err error) error {
	if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
		return err
	}

	s.log.Error(operation+" inventory entry failed", zap.Error(err))

	return fmt.Errorf("%s inventory entry: %w", operation, err)
}
