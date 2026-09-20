package supplier

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
	uniqueViolation = "23505"
	nameConstraint  = "uq_suppliers_name"
)

// ListFilter selects one page of suppliers. Name, when not empty, matches
// anywhere in the supplier's name, ignoring case.
type ListFilter struct {
	Limit  int
	Offset int
	Name   string
}

// Repository reads and writes suppliers.
type Repository interface {
	// ExistsByName reports whether an active supplier has the name, ignoring
	// case. A non-empty excludeID skips that supplier.
	ExistsByName(ctx context.Context, name, excludeID string) (bool, error)
	// Create inserts the supplier. It returns apperror.ErrSupplierNameExists
	// when an active supplier already has the name.
	Create(ctx context.Context, supplier *domain.Supplier) (*domain.Supplier, error)
	// List returns one page of active suppliers, newest first, and the total
	// number of suppliers matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*domain.Supplier, int64, error)
	// FindByID returns the active supplier with the ID, or apperror.ErrNotFound.
	FindByID(ctx context.Context, id string) (*domain.Supplier, error)
	// Update writes the name, contact name, email, phone and address. It
	// returns apperror.ErrNotFound when no active supplier has the ID, and
	// apperror.ErrSupplierNameExists when another active supplier has the name.
	Update(ctx context.Context, supplier *domain.Supplier) error
	// LockByID reports whether an active supplier has the ID and locks the row
	// until the transaction ends.
	LockByID(ctx context.Context, id string) (bool, error)
	// ExistsInPurchaseOrder reports whether a purchase order that is not
	// soft-deleted references the supplier, whatever the order's status.
	ExistsInPurchaseOrder(ctx context.Context, id string) (bool, error)
	// Delete soft-deletes the supplier. It returns apperror.ErrNotFound when no
	// active supplier has the ID.
	Delete(ctx context.Context, id string) error
	// Transaction runs fn with a Repository bound to a single transaction. It
	// commits when fn returns nil and rolls back otherwise.
	Transaction(ctx context.Context, fn func(tx Repository) error) error
}

type record struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string
	ContactName *string
	Email       *string
	Phone       *string
	Address     *string
	CreatedAt   time.Time      `gorm:"autoCreateTime:false;default:now()"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime:false;default:now()"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (record) TableName() string {
	return "suppliers"
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ExistsByName(ctx context.Context, name, excludeID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&record{}).Where("lower(name) = lower(?)", name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check supplier name: %w", err)
	}

	return count > 0, nil
}

func (r *repository) Create(ctx context.Context, supplier *domain.Supplier) (*domain.Supplier, error) {
	rec := toRecord(supplier)

	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, translateError(err)
	}

	return toDomain(rec), nil
}

func (r *repository) List(ctx context.Context, filter ListFilter) ([]*domain.Supplier, int64, error) {
	var total int64

	err := r.db.WithContext(ctx).Model(&record{}).Scopes(matching(filter)).Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("count suppliers: %w", err)
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
		return nil, 0, fmt.Errorf("list suppliers: %w", err)
	}

	suppliers := make([]*domain.Supplier, 0, len(records))
	for _, rec := range records {
		suppliers = append(suppliers, toDomain(rec))
	}

	return suppliers, total, nil
}

// matching applies the name filter of filter.
func matching(filter ListFilter) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if filter.Name != "" {
			db = db.Where(`name ILIKE ? ESCAPE '\'`, "%"+escapeLike(filter.Name)+"%")
		}

		return db
	}
}

// escapeLike escapes the LIKE wildcards in term so they match literally.
func escapeLike(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}

func (r *repository) FindByID(ctx context.Context, id string) (*domain.Supplier, error) {
	var rec record

	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find supplier: %w", err)
	}

	return toDomain(rec), nil
}

func (r *repository) Update(ctx context.Context, supplier *domain.Supplier) error {
	// Select makes GORM write these columns even when they are zero, so a
	// cleared field is stored as NULL.
	result := r.db.WithContext(ctx).
		Model(&record{}).
		Where("id = ?", supplier.ID).
		Select("name", "contact_name", "email", "phone", "address").
		Updates(record{
			Name:        supplier.Name,
			ContactName: supplier.ContactName,
			Email:       supplier.Email,
			Phone:       supplier.Phone,
			Address:     supplier.Address,
		})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
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
		return false, fmt.Errorf("lock supplier: %w", err)
	}

	return true, nil
}

func (r *repository) ExistsInPurchaseOrder(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Table("purchase_orders").
		Where("supplier_id = ? AND deleted_at IS NULL", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check supplier purchase orders: %w", err)
	}

	return count > 0, nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&record{})
	if result.Error != nil {
		return fmt.Errorf("delete supplier: %w", result.Error)
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

// translateError maps a unique violation on the name index to
// apperror.ErrSupplierNameExists. This is what catches two requests that both
// passed the ExistsByName check. Anything else is wrapped and returned.
func translateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameConstraint {
		return apperror.ErrSupplierNameExists
	}

	return fmt.Errorf("save supplier: %w", err)
}

func toRecord(s *domain.Supplier) record {
	return record{
		Name:        s.Name,
		ContactName: s.ContactName,
		Email:       s.Email,
		Phone:       s.Phone,
		Address:     s.Address,
	}
}

func toDomain(rec record) *domain.Supplier {
	return &domain.Supplier{
		ID:          rec.ID,
		Name:        rec.Name,
		ContactName: rec.ContactName,
		Email:       rec.Email,
		Phone:       rec.Phone,
		Address:     rec.Address,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}
