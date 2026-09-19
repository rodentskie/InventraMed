package medicine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

const (
	uniqueViolation     = "23505"
	barcodeConstraint   = "medicines_barcode_key"
	nameBatchConstraint = "uq_medicines_name_batch"
)

// Repository reads and writes medicines.
type Repository interface {
	// ExistsByNameAndBatch reports whether an active medicine has the same
	// name and batch number, ignoring case. A nil batchNumber matches
	// medicines with no batch number.
	ExistsByNameAndBatch(ctx context.Context, name string, batchNumber *string) (bool, error)
	// ExistsByBarcode reports whether any medicine, soft-deleted or not, has the barcode.
	ExistsByBarcode(ctx context.Context, barcode string) (bool, error)
	Create(ctx context.Context, medicine *domain.Medicine) (*domain.Medicine, error)
	// Transaction runs fn with a Repository bound to a single transaction. It
	// commits when fn returns nil and rolls back otherwise.
	Transaction(ctx context.Context, fn func(tx Repository) error) error
}

type record struct {
	ID             string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name           string
	Barcode        string
	BatchNumber    *string
	ExpirationDate time.Time `gorm:"type:date"`
	Quantity       int
	CreatedBy      *string        `gorm:"type:uuid"`
	CreatedAt      time.Time      `gorm:"autoCreateTime:false;default:now()"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime:false;default:now()"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (record) TableName() string {
	return "medicines"
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ExistsByNameAndBatch(ctx context.Context, name string, batchNumber *string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&record{}).Where("lower(name) = lower(?)", name)
	if batchNumber == nil {
		query = query.Where("batch_number IS NULL")
	} else {
		query = query.Where("lower(batch_number) = lower(?)", *batchNumber)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check medicine name and batch: %w", err)
	}

	return count > 0, nil
}

func (r *repository) ExistsByBarcode(ctx context.Context, barcode string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Unscoped().
		Model(&record{}).
		Where("barcode = ?", barcode).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check medicine barcode: %w", err)
	}

	return count > 0, nil
}

func (r *repository) Create(ctx context.Context, medicine *domain.Medicine) (*domain.Medicine, error) {
	rec := toRecord(medicine)

	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, translateError(err)
	}

	return toDomain(rec), nil
}

func (r *repository) Transaction(ctx context.Context, fn func(tx Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&repository{db: tx})
	})
}

// translateError maps a unique violation to the matching apperror by
// constraint name. Anything else is wrapped and returned as is.
func translateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case barcodeConstraint:
			return apperror.ErrBarcodeExists
		case nameBatchConstraint:
			return apperror.ErrNameBatchExists
		}
	}

	return fmt.Errorf("create medicine: %w", err)
}

func toRecord(m *domain.Medicine) record {
	return record{
		Name:           m.Name,
		Barcode:        m.Barcode,
		BatchNumber:    m.BatchNumber,
		ExpirationDate: m.ExpirationDate,
		Quantity:       m.Quantity,
		CreatedBy:      m.CreatedBy,
	}
}

func toDomain(rec record) *domain.Medicine {
	return &domain.Medicine{
		ID:             rec.ID,
		Name:           rec.Name,
		Barcode:        rec.Barcode,
		BatchNumber:    rec.BatchNumber,
		ExpirationDate: rec.ExpirationDate,
		Quantity:       rec.Quantity,
		CreatedBy:      rec.CreatedBy,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}
}
