package purchaseorder

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	medicinerepo "apps/api/internal/repository/medicine"
	"apps/api/internal/repository/purchaseorder"
	supplierrepo "apps/api/internal/repository/supplier"
	"apps/api/pkg/apperror"
)

// ItemInput is one medicine and quantity on a purchase order being created.
type ItemInput struct {
	MedicineID      string
	QuantityOrdered int
}

// CreateInput is the validated data needed to create a purchase order.
type CreateInput struct {
	SupplierID   string
	OrderDate    time.Time
	ExpectedDate *time.Time
	Notes        *string
	Items        []ItemInput
	// CreatedBy is the ID of the user creating the purchase order.
	CreatedBy string
}

// ReceiveInput is the validated data needed to receive a purchase order.
type ReceiveInput struct {
	PurchaseOrderID string
	Notes           *string
	// ReceivedBy is the ID of the user receiving the purchase order.
	ReceivedBy string
}

// ListFilter selects one page of purchase orders.
type ListFilter = purchaseorder.ListFilter

// Page is one page of purchase orders and the total number of them.
type Page struct {
	PurchaseOrders []*domain.PurchaseOrder
	Total          int64
}

// Service owns the purchase order business logic.
type Service interface {
	Create(ctx context.Context, input CreateInput) (*domain.PurchaseOrder, error)
	List(ctx context.Context, filter ListFilter) (*Page, error)
	GetByID(ctx context.Context, id string) (*domain.PurchaseOrder, error)
	Receive(ctx context.Context, input ReceiveInput) (*domain.PurchaseOrderReceipt, error)
}

type service struct {
	repo purchaseorder.Repository
	log  *zap.Logger
}

func NewService(repo purchaseorder.Repository, log *zap.Logger) Service {
	return &service{repo: repo, log: log}
}

// Create records a new purchase order in the draft status. It returns
// apperror.ErrSupplierNotFound when the supplier, or apperror.ErrMedicineNotFound
// when a medicine, is not active.
//
// The supplier and medicines are locked rather than just checked: the lock
// waits for a concurrent delete of the same row, so a supplier or medicine
// that is being deleted can't gain a purchase order at the same moment.
func (s *service) Create(ctx context.Context, input CreateInput) (*domain.PurchaseOrder, error) {
	var created *domain.PurchaseOrder

	err := s.repo.Transaction(ctx, input.CreatedBy, func(
		tx purchaseorder.Repository,
		medicines medicinerepo.Repository,
		suppliers supplierrepo.Repository,
	) error {
		if err := lockReferences(ctx, medicines, suppliers, input); err != nil {
			return err
		}

		var err error
		created, err = tx.Create(ctx, newPurchaseOrder(input))

		return err
	})
	if err != nil {
		return nil, s.fail("create", err)
	}

	return created, nil
}

// lockReferences locks the supplier, then each distinct medicine in ascending
// ID order, so two concurrent creates that share medicines can't deadlock.
func lockReferences(
	ctx context.Context,
	medicines medicinerepo.Repository,
	suppliers supplierrepo.Repository,
	input CreateInput,
) error {
	found, err := suppliers.LockByID(ctx, input.SupplierID)
	if err != nil {
		return err
	}
	if !found {
		return apperror.ErrSupplierNotFound
	}

	ids := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		ids = append(ids, item.MedicineID)
	}
	slices.Sort(ids)

	for _, id := range slices.Compact(ids) {
		found, err := medicines.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return apperror.ErrMedicineNotFound
		}
	}

	return nil
}

func newPurchaseOrder(input CreateInput) *domain.PurchaseOrder {
	items := make([]*domain.PurchaseOrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, &domain.PurchaseOrderItem{
			MedicineID:      item.MedicineID,
			QuantityOrdered: item.QuantityOrdered,
		})
	}

	return &domain.PurchaseOrder{
		SupplierID:   input.SupplierID,
		Status:       domain.PurchaseOrderStatusDraft,
		OrderDate:    input.OrderDate,
		ExpectedDate: input.ExpectedDate,
		CreatedBy:    input.CreatedBy,
		Notes:        input.Notes,
		Items:        items,
	}
}

// List returns one page of purchase orders and the total.
func (s *service) List(ctx context.Context, filter ListFilter) (*Page, error) {
	orders, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, s.fail("list", err)
	}

	return &Page{PurchaseOrders: orders, Total: total}, nil
}

// GetByID returns the purchase order with its items and receipts, or
// apperror.ErrNotFound.
func (s *service) GetByID(ctx context.Context, id string) (*domain.PurchaseOrder, error) {
	found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.fail("get", err)
	}

	return found, nil
}

// Receive receives every item of a draft or ordered purchase order in full and
// marks it received. It returns apperror.ErrNotFound for an unknown ID and
// apperror.ErrPurchaseOrderNotReceivable for any other status.
//
// It never writes medicines.quantity: the database trigger on
// purchase_order_receipt_items adds each received quantity.
func (s *service) Receive(ctx context.Context, input ReceiveInput) (*domain.PurchaseOrderReceipt, error) {
	var receipt *domain.PurchaseOrderReceipt

	err := s.repo.Transaction(ctx, input.ReceivedBy, func(
		tx purchaseorder.Repository,
		_ medicinerepo.Repository,
		_ supplierrepo.Repository,
	) error {
		status, found, err := tx.LockStatusByID(ctx, input.PurchaseOrderID)
		if err != nil {
			return err
		}
		if !found {
			return apperror.ErrNotFound
		}
		if !receivable(status) {
			return apperror.ErrPurchaseOrderNotReceivable
		}

		items, err := tx.ListItems(ctx, input.PurchaseOrderID)
		if err != nil {
			return err
		}

		receipt, err = tx.CreateReceipt(ctx, newReceipt(input, items))
		if err != nil {
			return err
		}

		return tx.UpdateStatus(ctx, input.PurchaseOrderID, domain.PurchaseOrderStatusReceived)
	})
	if err != nil {
		return nil, s.fail("receive", err)
	}

	return receipt, nil
}

func receivable(status string) bool {
	return status == domain.PurchaseOrderStatusDraft || status == domain.PurchaseOrderStatusOrdered
}

// newReceipt receives every item in full, in the order given, which keeps the
// quantity-sync trigger's row updates in a consistent order across orders.
func newReceipt(input ReceiveInput, items []*domain.PurchaseOrderItem) *domain.PurchaseOrderReceipt {
	receiptItems := make([]*domain.PurchaseOrderReceiptItem, 0, len(items))
	for _, item := range items {
		receiptItems = append(receiptItems, &domain.PurchaseOrderReceiptItem{
			PurchaseOrderItemID: item.ID,
			QuantityReceived:    item.QuantityOrdered,
		})
	}

	return &domain.PurchaseOrderReceipt{
		PurchaseOrderID: input.PurchaseOrderID,
		ReceivedBy:      input.ReceivedBy,
		Notes:           input.Notes,
		Items:           receiptItems,
	}
}

// fail returns not found and conflict errors unchanged, so the handler can
// map them. Anything else is logged and wrapped.
func (s *service) fail(operation string, err error) error {
	if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
		return err
	}

	s.log.Error(operation+" purchase order failed", zap.Error(err))

	return fmt.Errorf("%s purchase order: %w", operation, err)
}
