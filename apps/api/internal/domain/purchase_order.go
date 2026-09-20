package domain

import "time"

// Purchase order statuses. Only draft, ordered and received are reachable
// through the API today.
const (
	PurchaseOrderStatusDraft             = "draft"
	PurchaseOrderStatusOrdered           = "ordered"
	PurchaseOrderStatusPartiallyReceived = "partially_received"
	PurchaseOrderStatusReceived          = "received"
	PurchaseOrderStatusCancelled         = "cancelled"
)

// PurchaseOrder is an order of medicines from a supplier.
type PurchaseOrder struct {
	ID         string
	SupplierID string
	Status     string
	// OrderDate and ExpectedDate are calendar dates; only their year, month and
	// day are meaningful.
	OrderDate    time.Time
	ExpectedDate *time.Time
	CreatedBy    string
	Notes        *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// Items and Receipts are only filled by the repository's Create and FindByID.
	Items    []*PurchaseOrderItem
	Receipts []*PurchaseOrderReceipt
}

// PurchaseOrderItem is one medicine and quantity on a purchase order. It is
// immutable once created.
type PurchaseOrderItem struct {
	ID              string
	PurchaseOrderID string
	MedicineID      string
	QuantityOrdered int
	CreatedAt       time.Time
}

// PurchaseOrderReceipt is one receiving event on a purchase order.
type PurchaseOrderReceipt struct {
	ID              string
	PurchaseOrderID string
	ReceivedBy      string
	ReceivedAt      time.Time
	Notes           *string
	CreatedAt       time.Time
	Items           []*PurchaseOrderReceiptItem
}

// PurchaseOrderReceiptItem is what was received of one purchase order item in
// one receiving event.
type PurchaseOrderReceiptItem struct {
	ID                     string
	PurchaseOrderReceiptID string
	PurchaseOrderItemID    string
	QuantityReceived       int
	QuantityDamaged        int
	QuantityReturned       int
	Notes                  *string
	CreatedAt              time.Time
}
