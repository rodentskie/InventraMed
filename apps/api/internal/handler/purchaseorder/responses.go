package purchaseorder

import (
	"time"

	"apps/api/internal/domain"
)

type purchaseOrderResponse struct {
	ID           string    `json:"id"`
	SupplierID   string    `json:"supplier_id"`
	Status       string    `json:"status"`
	OrderDate    string    `json:"order_date"`
	ExpectedDate *string   `json:"expected_date"`
	CreatedBy    string    `json:"created_by"`
	Notes        *string   `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// detailResponse is a purchase order with its items and receipts.
type detailResponse struct {
	purchaseOrderResponse
	Items    []itemResponse    `json:"items"`
	Receipts []receiptResponse `json:"receipts"`
}

type itemResponse struct {
	ID              string    `json:"id"`
	MedicineID      string    `json:"medicine_id"`
	QuantityOrdered int       `json:"quantity_ordered"`
	CreatedAt       time.Time `json:"created_at"`
}

type receiptResponse struct {
	ID              string                `json:"id"`
	PurchaseOrderID string                `json:"purchase_order_id"`
	ReceivedBy      string                `json:"received_by"`
	ReceivedAt      time.Time             `json:"received_at"`
	Notes           *string               `json:"notes"`
	CreatedAt       time.Time             `json:"created_at"`
	Items           []receiptItemResponse `json:"items"`
}

type receiptItemResponse struct {
	ID                  string    `json:"id"`
	PurchaseOrderItemID string    `json:"purchase_order_item_id"`
	QuantityReceived    int       `json:"quantity_received"`
	QuantityDamaged     int       `json:"quantity_damaged"`
	QuantityReturned    int       `json:"quantity_returned"`
	Notes               *string   `json:"notes"`
	CreatedAt           time.Time `json:"created_at"`
}

func toResponse(po *domain.PurchaseOrder) purchaseOrderResponse {
	var expected *string
	if po.ExpectedDate != nil {
		formatted := po.ExpectedDate.Format(time.DateOnly)
		expected = &formatted
	}

	return purchaseOrderResponse{
		ID:           po.ID,
		SupplierID:   po.SupplierID,
		Status:       po.Status,
		OrderDate:    po.OrderDate.Format(time.DateOnly),
		ExpectedDate: expected,
		CreatedBy:    po.CreatedBy,
		Notes:        po.Notes,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}

// toDetailResponse never leaves Items or Receipts nil, so they are encoded as
// [] and not null.
func toDetailResponse(po *domain.PurchaseOrder) detailResponse {
	items := make([]itemResponse, 0, len(po.Items))
	for _, item := range po.Items {
		items = append(items, itemResponse{
			ID:              item.ID,
			MedicineID:      item.MedicineID,
			QuantityOrdered: item.QuantityOrdered,
			CreatedAt:       item.CreatedAt,
		})
	}

	receipts := make([]receiptResponse, 0, len(po.Receipts))
	for _, receipt := range po.Receipts {
		receipts = append(receipts, toReceiptResponse(receipt))
	}

	return detailResponse{purchaseOrderResponse: toResponse(po), Items: items, Receipts: receipts}
}

func toReceiptResponse(receipt *domain.PurchaseOrderReceipt) receiptResponse {
	items := make([]receiptItemResponse, 0, len(receipt.Items))
	for _, item := range receipt.Items {
		items = append(items, receiptItemResponse{
			ID:                  item.ID,
			PurchaseOrderItemID: item.PurchaseOrderItemID,
			QuantityReceived:    item.QuantityReceived,
			QuantityDamaged:     item.QuantityDamaged,
			QuantityReturned:    item.QuantityReturned,
			Notes:               item.Notes,
			CreatedAt:           item.CreatedAt,
		})
	}

	return receiptResponse{
		ID:              receipt.ID,
		PurchaseOrderID: receipt.PurchaseOrderID,
		ReceivedBy:      receipt.ReceivedBy,
		ReceivedAt:      receipt.ReceivedAt,
		Notes:           receipt.Notes,
		CreatedAt:       receipt.CreatedAt,
		Items:           items,
	}
}
