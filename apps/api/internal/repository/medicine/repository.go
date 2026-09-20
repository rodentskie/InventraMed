package medicine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

const (
	uniqueViolation     = "23505"
	barcodeConstraint   = "medicines_barcode_key"
	nameBatchConstraint = "uq_medicines_name_batch"
)

// ListFilter selects one page of medicines. Name and Barcode, when not empty,
// match anywhere in the value, ignoring case; both given means both must match.
type ListFilter struct {
	Limit   int
	Offset  int
	Name    string
	Barcode string
}

// Repository reads and writes medicines.
type Repository interface {
	// ExistsByNameAndBatch reports whether an active medicine has the same
	// name and batch number, ignoring case. A nil batchNumber matches
	// medicines with no batch number. A non-empty excludeID skips that medicine.
	ExistsByNameAndBatch(ctx context.Context, name string, batchNumber *string, excludeID string) (bool, error)
	// ExistsByBarcode reports whether any medicine, soft-deleted or not, has
	// the barcode. A non-empty excludeID skips that medicine.
	ExistsByBarcode(ctx context.Context, barcode, excludeID string) (bool, error)
	// ExistsByID reports whether an active medicine has the ID.
	ExistsByID(ctx context.Context, id string) (bool, error)
	// LockByID is ExistsByID that also locks the row until the transaction ends.
	LockByID(ctx context.Context, id string) (bool, error)
	// LockQuantityByID is LockByID that also returns the medicine's current
	// quantity, for callers that need to check it before writing a dependent
	// row in the same transaction (e.g. an inventory entry).
	LockQuantityByID(ctx context.Context, id string) (quantity int, found bool, err error)
	// ExistsInPurchaseOrder reports whether an item of a purchase order that
	// is not soft-deleted references the medicine, whatever the order's status.
	ExistsInPurchaseOrder(ctx context.Context, id string) (bool, error)
	// List returns one page of active medicines, newest first, and the total
	// number of medicines matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*domain.Medicine, int64, error)
	// FindByBarcode returns the active medicine with the exact barcode, or
	// apperror.ErrNotFound.
	FindByBarcode(ctx context.Context, barcode string) (*domain.Medicine, error)
	Create(ctx context.Context, medicine *domain.Medicine) (*domain.Medicine, error)
	// Update writes the name, barcode, batch number and expiration date, but
	// never the quantity. It returns apperror.ErrNotFound when no active
	// medicine has the ID.
	Update(ctx context.Context, medicine *domain.Medicine) error
	// Delete soft-deletes the medicine. It returns apperror.ErrNotFound when no
	// active medicine has the ID.
	Delete(ctx context.Context, id string) error
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

func (r *repository) ExistsByNameAndBatch(ctx context.Context, name string, batchNumber *string, excludeID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&record{}).Where("lower(name) = lower(?)", name)
	if batchNumber == nil {
		query = query.Where("batch_number IS NULL")
	} else {
		query = query.Where("lower(batch_number) = lower(?)", *batchNumber)
	}
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check medicine name and batch: %w", err)
	}

	return count > 0, nil
}

func (r *repository) ExistsByBarcode(ctx context.Context, barcode, excludeID string) (bool, error) {
	query := r.db.WithContext(ctx).Unscoped().Model(&record{}).Where("barcode = ?", barcode)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check medicine barcode: %w", err)
	}

	return count > 0, nil
}

func (r *repository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&record{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check medicine id: %w", err)
	}

	return count > 0, nil
}

func (r *repository) LockByID(ctx context.Context, id string) (bool, error) {
	var rec record

	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ?", id).
		Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock medicine: %w", err)
	}

	return true, nil
}

func (r *repository) LockQuantityByID(ctx context.Context, id string) (int, bool, error) {
	var rec record

	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "quantity").
		Where("id = ?", id).
		Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("lock medicine quantity: %w", err)
	}

	return rec.Quantity, true, nil
}

func (r *repository) ExistsInPurchaseOrder(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Table("purchase_order_items").
		Joins("JOIN purchase_orders ON purchase_orders.id = purchase_order_items.purchase_order_id").
		Where("purchase_order_items.medicine_id = ? AND purchase_orders.deleted_at IS NULL", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check medicine purchase orders: %w", err)
	}

	return count > 0, nil
}

func (r *repository) List(ctx context.Context, filter ListFilter) ([]*domain.Medicine, int64, error) {
	var total int64

	err := r.db.WithContext(ctx).Model(&record{}).Scopes(matching(filter)).Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("count medicines: %w", err)
	}

	var records []record

	err = r.db.WithContext(ctx).
		Model(&record{}).
		Scopes(matching(filter)).
		Order("created_at DESC, id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list medicines: %w", err)
	}

	medicines := make([]*domain.Medicine, 0, len(records))
	for _, rec := range records {
		medicines = append(medicines, toDomain(rec))
	}

	return medicines, total, nil
}

// matching applies the name and barcode filters of filter.
func matching(filter ListFilter) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if filter.Name != "" {
			db = db.Where(`name ILIKE ? ESCAPE '\'`, "%"+escapeLike(filter.Name)+"%")
		}
		if filter.Barcode != "" {
			db = db.Where(`barcode ILIKE ? ESCAPE '\'`, "%"+escapeLike(filter.Barcode)+"%")
		}

		return db
	}
}

// escapeLike escapes the LIKE wildcards in term so they match literally.
func escapeLike(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}

func (r *repository) FindByBarcode(ctx context.Context, barcode string) (*domain.Medicine, error) {
	var rec record

	err := r.db.WithContext(ctx).Where("barcode = ?", barcode).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find medicine by barcode: %w", err)
	}

	return toDomain(rec), nil
}

func (r *repository) Create(ctx context.Context, medicine *domain.Medicine) (*domain.Medicine, error) {
	rec := toRecord(medicine)

	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, translateError(err)
	}

	return toDomain(rec), nil
}

func (r *repository) Update(ctx context.Context, medicine *domain.Medicine) error {
	// Select makes GORM write these columns even when they are zero, so a
	// cleared batch number is stored as NULL. Quantity is deliberately left out.
	result := r.db.WithContext(ctx).
		Model(&record{}).
		Where("id = ?", medicine.ID).
		Select("name", "barcode", "batch_number", "expiration_date").
		Updates(record{
			Name:           medicine.Name,
			Barcode:        medicine.Barcode,
			BatchNumber:    medicine.BatchNumber,
			ExpirationDate: medicine.ExpirationDate,
		})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&record{})
	if result.Error != nil {
		return fmt.Errorf("delete medicine: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
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

	return fmt.Errorf("save medicine: %w", err)
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
