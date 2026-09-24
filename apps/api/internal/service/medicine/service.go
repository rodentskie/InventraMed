package medicine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/medicine"
	"apps/api/pkg/apperror"
)

// CreateInput is the validated data needed to create a medicine.
type CreateInput struct {
	Name           string
	Barcode        string
	BatchNumber    *string
	ExpirationDate time.Time
	Quantity       int
	// Location is the tray compartment (1–12), or nil when not placed.
	Location *int
	// CreatedBy is the ID of the user creating the medicine.
	CreatedBy string
}

// UpdateInput is the validated data needed to update a medicine. Quantity is
// deliberately absent: stock is not edited through an update.
type UpdateInput struct {
	Name           string
	Barcode        string
	BatchNumber    *string
	ExpirationDate time.Time
	// Location is the tray compartment (1–12), or nil to clear it.
	Location *int
}

// ListFilter selects one page of medicines, optionally filtered by name and barcode.
type ListFilter = medicine.ListFilter

// Page is one page of medicines and the total number matching the filter.
type Page struct {
	Medicines []*domain.Medicine
	Total     int64
}

// Service owns the medicine business logic.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*domain.Medicine, error)
	List(ctx context.Context, filter ListFilter) (*Page, error)
	GetByBarcode(ctx context.Context, barcode string) (*domain.Medicine, error)
	Update(ctx context.Context, id string, input UpdateInput) error
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo medicine.Repository
	log  *zap.Logger
}

func NewService(repo medicine.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

// Create registers a new medicine. It returns apperror.ErrNameBatchExists when
// the name and batch number are already taken, apperror.ErrBarcodeExists when
// the barcode is, or apperror.ErrLocationTaken when the location is. They are
// checked in that order.
func (s *service) Create(ctx context.Context, input CreateInput) (*domain.Medicine, error) {
	var created *domain.Medicine

	err := s.repo.Transaction(ctx, func(tx medicine.Repository) error {
		if err := checkUnique(ctx, tx, uniqueFields{
			name:        input.Name,
			batchNumber: input.BatchNumber,
			barcode:     input.Barcode,
			location:    input.Location,
		}, ""); err != nil {
			return err
		}

		createdBy := input.CreatedBy

		var err error
		created, err = tx.Create(ctx, &domain.Medicine{
			Name:           input.Name,
			Barcode:        input.Barcode,
			BatchNumber:    input.BatchNumber,
			ExpirationDate: input.ExpirationDate,
			Quantity:       input.Quantity,
			Location:       input.Location,
			CreatedBy:      &createdBy,
		})

		return err
	})
	if err != nil {
		return nil, s.fail("create", err)
	}

	return created, nil
}

// List returns one page of medicines and the total matching the filter.
func (s *service) List(ctx context.Context, filter ListFilter) (*Page, error) {
	medicines, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, s.fail("list", err)
	}

	return &Page{Medicines: medicines, Total: total}, nil
}

// GetByBarcode returns the medicine with the exact barcode, or
// apperror.ErrNotFound.
func (s *service) GetByBarcode(ctx context.Context, barcode string) (*domain.Medicine, error) {
	found, err := s.repo.FindByBarcode(ctx, barcode)
	if err != nil {
		return nil, s.fail("get", err)
	}

	return found, nil
}

// Update replaces the medicine's name, barcode, batch number, expiration date
// and location. It returns apperror.ErrNotFound for an unknown ID before it checks for
// duplicates, and the same conflict errors as Create otherwise. The
// medicine's own values never count as duplicates.
func (s *service) Update(ctx context.Context, id string, input UpdateInput) error {
	err := s.repo.Transaction(ctx, func(tx medicine.Repository) error {
		exists, err := tx.ExistsByID(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return apperror.ErrNotFound
		}

		if err := checkUnique(ctx, tx, uniqueFields{
			name:        input.Name,
			batchNumber: input.BatchNumber,
			barcode:     input.Barcode,
			location:    input.Location,
		}, id); err != nil {
			return err
		}

		return tx.Update(ctx, &domain.Medicine{
			ID:             id,
			Name:           input.Name,
			Barcode:        input.Barcode,
			BatchNumber:    input.BatchNumber,
			ExpirationDate: input.ExpirationDate,
			Location:       input.Location,
		})
	})
	if err != nil {
		return s.fail("update", err)
	}

	return nil
}

// Delete soft-deletes a medicine. It returns apperror.ErrNotFound for an
// unknown ID and apperror.ErrMedicineInPurchaseOrder while a purchase order
// uses the medicine.
func (s *service) Delete(ctx context.Context, id string) error {
	err := s.repo.Transaction(ctx, func(tx medicine.Repository) error {
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
			return apperror.ErrMedicineInPurchaseOrder
		}

		return tx.Delete(ctx, id)
	})
	if err != nil {
		return s.fail("delete", err)
	}

	return nil
}

// uniqueFields are the medicine values that no other medicine may share.
type uniqueFields struct {
	name        string
	batchNumber *string
	barcode     string
	location    *int
}

// checkUnique returns apperror.ErrNameBatchExists, apperror.ErrBarcodeExists or
// apperror.ErrLocationTaken when another medicine already has the name and
// batch number, the barcode, or the location. They are checked in that order,
// and a nil location is never checked. A non-empty excludeID is skipped.
func checkUnique(ctx context.Context, repo medicine.Repository, fields uniqueFields, excludeID string) error {
	taken, err := repo.ExistsByNameAndBatch(ctx, fields.name, fields.batchNumber, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return apperror.ErrNameBatchExists
	}

	taken, err = repo.ExistsByBarcode(ctx, fields.barcode, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return apperror.ErrBarcodeExists
	}

	return checkLocation(ctx, repo, fields.location, excludeID)
}

// checkLocation returns apperror.ErrLocationTaken when another medicine sits in
// the location. A nil location is never taken.
func checkLocation(ctx context.Context, repo medicine.Repository, location *int, excludeID string) error {
	if location == nil {
		return nil
	}

	taken, err := repo.ExistsByLocation(ctx, *location, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return apperror.ErrLocationTaken
	}

	return nil
}

// fail returns not found and conflict errors unchanged, so the handler can
// map them. Anything else is logged and wrapped.
func (s *service) fail(operation string, err error) error {
	if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
		return err
	}

	s.log.Error(operation+" medicine failed", zap.Error(err))

	return fmt.Errorf("%s medicine: %w", operation, err)
}
