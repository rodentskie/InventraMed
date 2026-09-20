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

	return toDomain(rec), nil
}

func (r *repository) FindByID(ctx context.Context, id string) (*domain.InventoryEntry, error) {
	var rec record

	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error
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

	var records []record

	err := r.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
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
		CountedBy:  e.CountedBy,
		Notes:      e.Notes,
	}
}

func toDomain(rec record) *domain.InventoryEntry {
	return &domain.InventoryEntry{
		ID:         rec.ID,
		MedicineID: rec.MedicineID,
		Direction:  rec.Direction,
		Quantity:   rec.Quantity,
		Reason:     rec.Reason,
		CountedBy:  rec.CountedBy,
		Notes:      rec.Notes,
		CreatedAt:  rec.CreatedAt,
	}
}
