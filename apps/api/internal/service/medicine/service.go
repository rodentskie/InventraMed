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
	// CreatedBy is the ID of the user creating the medicine.
	CreatedBy string
}

// Service owns the medicine business logic.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*domain.Medicine, error)
}

type service struct {
	repo medicine.Repository
	log  *zap.Logger
}

func NewService(repo medicine.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

// Create registers a new medicine. It returns apperror.ErrNameBatchExists when
// the name and batch number are already taken, or apperror.ErrBarcodeExists
// when the barcode is. Name and batch number are checked first.
func (s *service) Create(ctx context.Context, input CreateInput) (*domain.Medicine, error) {
	var created *domain.Medicine

	err := s.repo.Transaction(ctx, func(tx medicine.Repository) error {
		var err error
		created, err = createIn(ctx, tx, input)

		return err
	})
	if errors.Is(err, apperror.ErrConflict) {
		return nil, err
	}
	if err != nil {
		s.log.Error("create medicine failed", zap.Error(err))
		return nil, fmt.Errorf("create medicine: %w", err)
	}

	return created, nil
}

func createIn(ctx context.Context, repo medicine.Repository, input CreateInput) (*domain.Medicine, error) {
	taken, err := repo.ExistsByNameAndBatch(ctx, input.Name, input.BatchNumber)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, apperror.ErrNameBatchExists
	}

	taken, err = repo.ExistsByBarcode(ctx, input.Barcode)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, apperror.ErrBarcodeExists
	}

	createdBy := input.CreatedBy

	return repo.Create(ctx, &domain.Medicine{
		Name:           input.Name,
		Barcode:        input.Barcode,
		BatchNumber:    input.BatchNumber,
		ExpirationDate: input.ExpirationDate,
		Quantity:       input.Quantity,
		CreatedBy:      &createdBy,
	})
}
