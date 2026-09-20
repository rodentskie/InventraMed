package purchaseorder

import (
	"time"

	"gorm.io/gorm"

	"apps/api/internal/domain"
)

type orderRecord struct {
	ID           string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SupplierID   string `gorm:"type:uuid"`
	Status       string
	OrderDate    time.Time  `gorm:"type:date"`
	ExpectedDate *time.Time `gorm:"type:date"`
	CreatedBy    string     `gorm:"type:uuid"`
	Notes        *string
	CreatedAt    time.Time      `gorm:"autoCreateTime:false;default:now()"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime:false;default:now()"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (orderRecord) TableName() string {
	return "purchase_orders"
}

type itemRecord struct {
	ID              string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PurchaseOrderID string `gorm:"type:uuid"`
	MedicineID      string `gorm:"type:uuid"`
	QuantityOrdered int
	CreatedAt       time.Time `gorm:"autoCreateTime:false;default:now()"`
}

func (itemRecord) TableName() string {
	return "purchase_order_items"
}

type receiptRecord struct {
	ID              string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PurchaseOrderID string    `gorm:"type:uuid"`
	ReceivedBy      string    `gorm:"type:uuid"`
	ReceivedAt      time.Time `gorm:"autoCreateTime:false;default:now()"`
	Notes           *string
	CreatedAt       time.Time `gorm:"autoCreateTime:false;default:now()"`
}

func (receiptRecord) TableName() string {
	return "purchase_order_receipts"
}

type receiptItemRecord struct {
	ID                     string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PurchaseOrderReceiptID string `gorm:"type:uuid"`
	PurchaseOrderItemID    string `gorm:"type:uuid"`
	QuantityReceived       int
	QuantityDamaged        int
	QuantityReturned       int
	Notes                  *string
	CreatedAt              time.Time `gorm:"autoCreateTime:false;default:now()"`
}

func (receiptItemRecord) TableName() string {
	return "purchase_order_receipt_items"
}

func toOrderRecord(po *domain.PurchaseOrder) orderRecord {
	return orderRecord{
		SupplierID:   po.SupplierID,
		Status:       po.Status,
		OrderDate:    po.OrderDate,
		ExpectedDate: po.ExpectedDate,
		CreatedBy:    po.CreatedBy,
		Notes:        po.Notes,
	}
}

func toOrderDomain(rec orderRecord) *domain.PurchaseOrder {
	return &domain.PurchaseOrder{
		ID:           rec.ID,
		SupplierID:   rec.SupplierID,
		Status:       rec.Status,
		OrderDate:    rec.OrderDate,
		ExpectedDate: rec.ExpectedDate,
		CreatedBy:    rec.CreatedBy,
		Notes:        rec.Notes,
		CreatedAt:    rec.CreatedAt,
		UpdatedAt:    rec.UpdatedAt,
	}
}

func toItemRecord(purchaseOrderID string, item *domain.PurchaseOrderItem) itemRecord {
	return itemRecord{
		PurchaseOrderID: purchaseOrderID,
		MedicineID:      item.MedicineID,
		QuantityOrdered: item.QuantityOrdered,
	}
}

func toItemDomain(rec itemRecord) *domain.PurchaseOrderItem {
	return &domain.PurchaseOrderItem{
		ID:              rec.ID,
		PurchaseOrderID: rec.PurchaseOrderID,
		MedicineID:      rec.MedicineID,
		QuantityOrdered: rec.QuantityOrdered,
		CreatedAt:       rec.CreatedAt,
	}
}

func toReceiptRecord(receipt *domain.PurchaseOrderReceipt) receiptRecord {
	return receiptRecord{
		PurchaseOrderID: receipt.PurchaseOrderID,
		ReceivedBy:      receipt.ReceivedBy,
		Notes:           receipt.Notes,
	}
}

func toReceiptDomain(rec receiptRecord) *domain.PurchaseOrderReceipt {
	return &domain.PurchaseOrderReceipt{
		ID:              rec.ID,
		PurchaseOrderID: rec.PurchaseOrderID,
		ReceivedBy:      rec.ReceivedBy,
		ReceivedAt:      rec.ReceivedAt,
		Notes:           rec.Notes,
		CreatedAt:       rec.CreatedAt,
	}
}

func toReceiptItemRecord(receiptID string, item *domain.PurchaseOrderReceiptItem) receiptItemRecord {
	return receiptItemRecord{
		PurchaseOrderReceiptID: receiptID,
		PurchaseOrderItemID:    item.PurchaseOrderItemID,
		QuantityReceived:       item.QuantityReceived,
		QuantityDamaged:        item.QuantityDamaged,
		QuantityReturned:       item.QuantityReturned,
		Notes:                  item.Notes,
	}
}

func toReceiptItemDomain(rec receiptItemRecord) *domain.PurchaseOrderReceiptItem {
	return &domain.PurchaseOrderReceiptItem{
		ID:                     rec.ID,
		PurchaseOrderReceiptID: rec.PurchaseOrderReceiptID,
		PurchaseOrderItemID:    rec.PurchaseOrderItemID,
		QuantityReceived:       rec.QuantityReceived,
		QuantityDamaged:        rec.QuantityDamaged,
		QuantityReturned:       rec.QuantityReturned,
		Notes:                  rec.Notes,
		CreatedAt:              rec.CreatedAt,
	}
}
