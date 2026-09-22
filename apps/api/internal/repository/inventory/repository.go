package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"apps/api/internal/domain"
	medicinerepo "apps/api/internal/repository/medicine"
	"apps/api/pkg/apperror"
)

// ListFilter selects one page of inventory entries.
type ListFilter struct {
	Limit  int
	Offset int
}

// Repository reads and writes inventory entries.
type Repository interface {
	Create(ctx context.Context, entry *domain.InventoryEntry) (*domain.InventoryEntry, error)
	// FindByID returns the entry with the ID, or apperror.ErrNotFound.
	FindByID(ctx context.Context, id string) (*domain.InventoryEntry, error)
	// List returns one page of entries, newest first, and the total number of
	// entries.
	List(ctx context.Context, filter ListFilter) ([]*domain.InventoryEntry, int64, error)
	// Transaction runs fn with an inventory Repository and a medicine
	// Repository, both bound to the same transaction, so a caller can guard a
	// write against the medicine's quantity atomically.
	Transaction(ctx context.Context, fn func(tx Repository, medicines medicinerepo.Repository) error) error
}

type record struct {
	ID         string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MedicineID string `gorm:"type:uuid"`
	Direction  string
	Quantity   int
	Reason     string
	CountedBy  string `gorm:"type:uuid"`
	Notes      *string
	CreatedAt  time.Time `gorm:"autoCreateTime:false;default:now()"`
}

func (record) TableName() string {
	return "inventory_entries"
}

// recordWithCounter is record plus the counting user's name, read with a
// join. It's select-only: Create still writes a plain record, since
// inventory_entries has no counted_by_name column of its own.
type recordWithCounter struct {
	ID            string
	MedicineID    string
	Direction     string
	Quantity      int
	Reason        string
	CountedBy     string
	CountedByName string
	Notes         *string
	CreatedAt     time.Time
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, entry *domain.InventoryEntry) (*domain.InventoryEntry, error) {
	rec := toRecord(entry)

	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, fmt.Errorf("create inventory entry: %w", err)
	}

	// Reload with the counting user's name joined in, so the response never
	// has to carry a bare ID. Runs in the same transaction as the insert.
	created, err := r.FindByID(ctx, rec.ID)
	if err != nil {
		return nil, fmt.Errorf("reload created inventory entry: %w", err)
	}

	return created, nil
}

func (r *repository) FindByID(ctx context.Context, id string) (*domain.InventoryEntry, error) {
	var rec recordWithCounter

	err := r.selectWithCounter(ctx).Where("inventory_entries.id = ?", id).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find inventory entry: %w", err)
	}

	return toDomain(rec), nil
}

func (r *repository) List(ctx context.Context, filter ListFilter) ([]*domain.InventoryEntry, int64, error) {
	var total int64

	if err := r.db.WithContext(ctx).Model(&record{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count inventory entries: %w", err)
	}

	var records []recordWithCounter

	err := r.selectWithCounter(ctx).
		Order("inventory_entries.created_at DESC, inventory_entries.id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory entries: %w", err)
	}

	entries := make([]*domain.InventoryEntry, 0, len(records))
	for _, rec := range records {
		entries = append(entries, toDomain(rec))
	}

	return entries, total, nil
}

// selectWithCounter is the base query for reading entries with their
// counting user's name joined in. A LEFT JOIN is used, not Unscoped, so a
// soft-deleted user's name still resolves for this historical record;
// users.deleted_at is never filtered on here regardless.
func (r *repository) selectWithCounter(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("inventory_entries").
		Select(`inventory_entries.id, inventory_entries.medicine_id, inventory_entries.direction,
			inventory_entries.quantity, inventory_entries.reason, inventory_entries.counted_by,
			COALESCE(users.name, '') AS counted_by_name, inventory_entries.notes, inventory_entries.created_at`).
		Joins("LEFT JOIN users ON users.id = inventory_entries.counted_by")
}

func (r *repository) Transaction(ctx context.Context, fn func(tx Repository, medicines medicinerepo.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&repository{db: tx}, medicinerepo.NewRepository(tx))
	})
}

func toRecord(e *domain.InventoryEntry) record {
	return record{
		MedicineID: e.MedicineID,
		Direction:  e.Direction,
		Quantity:   e.Quantity,
		Reason:     e.Reason,
		CountedBy:  e.CountedBy.ID,
		Notes:      e.Notes,
	}
}

func toDomain(rec recordWithCounter) *domain.InventoryEntry {
	return &domain.InventoryEntry{
		ID:         rec.ID,
		MedicineID: rec.MedicineID,
		Direction:  rec.Direction,
		Quantity:   rec.Quantity,
		Reason:     rec.Reason,
		CountedBy: domain.InventoryEntryUser{
			ID:   rec.CountedBy,
			Name: rec.CountedByName,
		},
		Notes:     rec.Notes,
		CreatedAt: rec.CreatedAt,
	}
}
