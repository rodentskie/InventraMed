package purchaseorder

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apps/api/internal/domain"
	medicinerepo "apps/api/internal/repository/medicine"
	supplierrepo "apps/api/internal/repository/supplier"
	"apps/api/pkg/apperror"
)

// ListFilter selects one page of purchase orders.
type ListFilter struct {
	Limit  int
	Offset int
}

// Repository reads and writes purchase orders, their items and their receipts.
type Repository interface {
	// Create inserts the purchase order and its items. The returned order has
	// its Items, with IDs.
	Create(ctx context.Context, po *domain.PurchaseOrder) (*domain.PurchaseOrder, error)
	// List returns one page of active purchase orders without their items or
	// receipts, newest first, and the total number of active purchase orders.
	List(ctx context.Context, filter ListFilter) ([]*domain.PurchaseOrder, int64, error)
	// FindByID returns the active purchase order with its items and receipts
	// (each receipt with its items), or apperror.ErrNotFound. It runs a
	// constant number of queries, never one per row.
	FindByID(ctx context.Context, id string) (*domain.PurchaseOrder, error)
	// LockStatusByID returns the status of the active purchase order with the
	// ID and locks the row until the transaction ends. found is false when
	// there is none.
	LockStatusByID(ctx context.Context, id string) (status string, found bool, err error)
	// ListItems returns the purchase order's items ordered by medicine_id, id.
	ListItems(ctx context.Context, purchaseOrderID string) ([]*domain.PurchaseOrderItem, error)
	// CreateReceipt inserts the receipt and its items, in the order given.
	CreateReceipt(ctx context.Context, receipt *domain.PurchaseOrderReceipt) (*domain.PurchaseOrderReceipt, error)
	// UpdateStatus writes only the status. It returns apperror.ErrNotFound when
	// no active purchase order has the ID.
	UpdateStatus(ctx context.Context, id, status string) error
	// Transaction runs fn in a single transaction. Its first statement sets the
	// transaction-local app.actor_id to actorID, so the audit triggers record
	// who acted. fn gets a Repository plus a medicine and a supplier
	// Repository bound to the same transaction. It commits when fn returns nil
	// and rolls back otherwise.
	Transaction(
		ctx context.Context,
		actorID string,
		fn func(tx Repository, medicines medicinerepo.Repository, suppliers supplierrepo.Repository) error,
	) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, po *domain.PurchaseOrder) (*domain.PurchaseOrder, error) {
	order := toOrderRecord(po)
	items := make([]itemRecord, 0, len(po.Items))

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		for _, item := range po.Items {
			items = append(items, toItemRecord(order.ID, item))
		}

		return tx.Create(&items).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create purchase order: %w", err)
	}

	created := toOrderDomain(order)
	for _, item := range items {
		created.Items = append(created.Items, toItemDomain(item))
	}

	return created, nil
}

func (r *repository) List(ctx context.Context, filter ListFilter) ([]*domain.PurchaseOrder, int64, error) {
	var total int64

	if err := r.db.WithContext(ctx).Model(&orderRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count purchase orders: %w", err)
	}

	var records []orderRecord

	err := r.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list purchase orders: %w", err)
	}

	orders := make([]*domain.PurchaseOrder, 0, len(records))
	for _, rec := range records {
		orders = append(orders, toOrderDomain(rec))
	}

	return orders, total, nil
}

func (r *repository) FindByID(ctx context.Context, id string) (*domain.PurchaseOrder, error) {
	var rec orderRecord

	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find purchase order: %w", err)
	}

	order := toOrderDomain(rec)

	if order.Items, err = r.itemsOf(ctx, id, "created_at, id"); err != nil {
		return nil, err
	}
	if order.Receipts, err = r.receiptsOf(ctx, id); err != nil {
		return nil, err
	}

	return order, nil
}

// itemsOf returns the purchase order's items in the given ORDER BY.
func (r *repository) itemsOf(ctx context.Context, purchaseOrderID, orderBy string) ([]*domain.PurchaseOrderItem, error) {
	var records []itemRecord

	err := r.db.WithContext(ctx).
		Where("purchase_order_id = ?", purchaseOrderID).
		Order(orderBy).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list purchase order items: %w", err)
	}

	items := make([]*domain.PurchaseOrderItem, 0, len(records))
	for _, rec := range records {
		items = append(items, toItemDomain(rec))
	}

	return items, nil
}

// receiptsOf returns the purchase order's receipts, each with its items, using
// one query for the receipts and one for all of their items.
func (r *repository) receiptsOf(ctx context.Context, purchaseOrderID string) ([]*domain.PurchaseOrderReceipt, error) {
	var records []receiptRecord

	err := r.db.WithContext(ctx).
		Where("purchase_order_id = ?", purchaseOrderID).
		Order("received_at, id").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list purchase order receipts: %w", err)
	}

	receipts := make([]*domain.PurchaseOrderReceipt, 0, len(records))
	byID := make(map[string]*domain.PurchaseOrderReceipt, len(records))
	ids := make([]string, 0, len(records))

	for _, rec := range records {
		receipt := toReceiptDomain(rec)
		receipts = append(receipts, receipt)
		byID[receipt.ID] = receipt
		ids = append(ids, receipt.ID)
	}
	if len(ids) == 0 {
		return receipts, nil
	}

	var itemRecords []receiptItemRecord

	err = r.db.WithContext(ctx).
		Where("purchase_order_receipt_id IN ?", ids).
		Order("created_at, id").
		Find(&itemRecords).Error
	if err != nil {
		return nil, fmt.Errorf("list purchase order receipt items: %w", err)
	}

	for _, rec := range itemRecords {
		receipt := byID[rec.PurchaseOrderReceiptID]
		receipt.Items = append(receipt.Items, toReceiptItemDomain(rec))
	}

	return receipts, nil
}

func (r *repository) LockStatusByID(ctx context.Context, id string) (string, bool, error) {
	var rec orderRecord

	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status").
		Where("id = ?", id).
		Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lock purchase order: %w", err)
	}

	return rec.Status, true, nil
}

func (r *repository) ListItems(ctx context.Context, purchaseOrderID string) ([]*domain.PurchaseOrderItem, error) {
	return r.itemsOf(ctx, purchaseOrderID, "medicine_id, id")
}

func (r *repository) CreateReceipt(ctx context.Context, receipt *domain.PurchaseOrderReceipt) (*domain.PurchaseOrderReceipt, error) {
	rec := toReceiptRecord(receipt)
	items := make([]receiptItemRecord, 0, len(receipt.Items))

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}

		for _, item := range receipt.Items {
			items = append(items, toReceiptItemRecord(rec.ID, item))
		}

		return tx.Create(&items).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create purchase order receipt: %w", err)
	}

	created := toReceiptDomain(rec)
	for _, item := range items {
		created.Items = append(created.Items, toReceiptItemDomain(item))
	}

	return created, nil
}

func (r *repository) UpdateStatus(ctx context.Context, id, status string) error {
	// Select limits the write to status; updated_at is set by the database trigger.
	result := r.db.WithContext(ctx).
		Model(&orderRecord{}).
		Where("id = ?", id).
		Select("status").
		Updates(orderRecord{Status: status})
	if result.Error != nil {
		return fmt.Errorf("update purchase order status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperror.ErrNotFound
	}

	return nil
}

func (r *repository) Transaction(
	ctx context.Context,
	actorID string,
	fn func(tx Repository, medicines medicinerepo.Repository, suppliers supplierrepo.Repository) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := setActor(tx, actorID); err != nil {
			return err
		}

		return fn(&repository{db: tx}, medicinerepo.NewRepository(tx), supplierrepo.NewRepository(tx))
	})
}

// setActor sets app.actor_id for the current transaction only (the third
// argument, is_local), which is what the audit triggers read. SET LOCAL can't
// take a bind parameter, so set_config is used instead.
func setActor(tx *gorm.DB, actorID string) error {
	if err := tx.Exec("SELECT set_config('app.actor_id', ?, true)", actorID).Error; err != nil {
		return fmt.Errorf("set audit actor: %w", err)
	}

	return nil
}
