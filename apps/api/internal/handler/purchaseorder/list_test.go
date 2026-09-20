package purchaseorder

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/service/purchaseorder"
	"apps/api/pkg/apperror"
)

func get(t *testing.T, h *Handler, query string) map[string]any {
	t.Helper()

	rec := call(t, h.List, http.MethodGet, "/purchase-orders"+query, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body)
	}

	return decode(t, rec)
}

func getByID(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.GetByID, http.MethodGet, "/purchase-orders/id", "", map[string]string{"id": id})
}

func TestList_Success(t *testing.T) {
	svc := &stubService{page: &purchaseorder.Page{PurchaseOrders: []*domain.PurchaseOrder{created()}, Total: 12}}
	h := NewHandler(svc, zap.NewNop())

	decoded := get(t, h, "?limit=10&offset=5")

	if decoded["total"] != float64(12) || decoded["limit"] != float64(10) || decoded["offset"] != float64(5) {
		t.Errorf("paging: got total %v limit %v offset %v", decoded["total"], decoded["limit"], decoded["offset"])
	}
	data, ok := decoded["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data: got %v", decoded["data"])
	}
	item, _ := data[0].(map[string]any)
	assertFields(t, "data[0]", item, wantHeader())
	for _, key := range []string{"items", "receipts"} {
		if _, present := item[key]; present {
			t.Errorf("data[0] should be the header only, but has %q", key)
		}
	}
}

func TestList_DefaultsAndCustomValues(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  purchaseorder.ListFilter
	}{
		{"defaults", "", purchaseorder.ListFilter{Limit: 20}},
		{"custom", "?limit=50&offset=100", purchaseorder.ListFilter{Limit: 50, Offset: 100}},
		{"bounds", "?limit=1&offset=0", purchaseorder.ListFilter{Limit: 1}},
		{"max limit", "?limit=100", purchaseorder.ListFilter{Limit: 100}},
		{"empty params use defaults", "?limit=&offset=", purchaseorder.ListFilter{Limit: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &purchaseorder.Page{}}
			h := NewHandler(svc, zap.NewNop())

			get(t, h, tt.query)

			if svc.filter != tt.want {
				t.Errorf("filter: got %+v, want %+v", svc.filter, tt.want)
			}
		})
	}
}

func TestList_BadRequest(t *testing.T) {
	const limitMessage = "limit must be a whole number between 1 and 100"
	const offsetMessage = "offset must be a whole number of zero or greater"

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"limit zero", "?limit=0", limitMessage},
		{"limit negative", "?limit=-1", limitMessage},
		{"limit too big", "?limit=101", limitMessage},
		{"limit text", "?limit=abc", limitMessage},
		{"limit fractional", "?limit=1.5", limitMessage},
		{"offset negative", "?offset=-1", offsetMessage},
		{"offset text", "?offset=abc", offsetMessage},
		{"offset fractional", "?offset=1.5", offsetMessage},
		{"offset overflow", "?offset=99999999999999999999", offsetMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &purchaseorder.Page{}}
			h := NewHandler(svc, zap.NewNop())

			rec := call(t, h.List, http.MethodGet, "/purchase-orders"+tt.query, "", nil)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := decode(t, rec)["error"]; got != tt.want {
				t.Errorf("error: got %v, want %q", got, tt.want)
			}
			if svc.called {
				t.Error("service must not be called for an invalid query")
			}
		})
	}
}

func TestList_EmptyPageIsAnEmptyArray(t *testing.T) {
	svc := &stubService{page: &purchaseorder.Page{Total: 3}}
	h := NewHandler(svc, zap.NewNop())

	rec := call(t, h.List, http.MethodGet, "/purchase-orders?offset=40", "", nil)

	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body: got %s, want an empty array, not null", rec.Body)
	}
	if got := decode(t, rec)["total"]; got != float64(3) {
		t.Errorf("total: got %v, want the real total", got)
	}
}

func TestList_ServiceError(t *testing.T) {
	h := NewHandler(&stubService{err: errors.New("db down")}, zap.NewNop())

	rec := call(t, h.List, http.MethodGet, "/purchase-orders", "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("error: got %v", got)
	}
}

func received() *domain.PurchaseOrder {
	order := created()
	order.Status = domain.PurchaseOrderStatusReceived
	order.Receipts = []*domain.PurchaseOrderReceipt{receipt()}
	order.Receipts[0].Items = append(order.Receipts[0].Items, &domain.PurchaseOrderReceiptItem{
		ID: "ri-2", PurchaseOrderReceiptID: "receipt-1", PurchaseOrderItemID: "item-2",
		QuantityReceived: 50, QuantityDamaged: 1, QuantityReturned: 2, Notes: str("box crushed"),
		CreatedAt: time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
	})

	return order
}

func TestGetByID_Success(t *testing.T) {
	svc := &stubService{order: received()}
	h := NewHandler(svc, zap.NewNop())

	rec := getByID(t, h, validID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body)
	}
	if svc.id != validID {
		t.Errorf("id passed to the service: got %q", svc.id)
	}

	data, ok := decode(t, rec)["data"].(map[string]any)
	if !ok {
		t.Fatal("data missing")
	}
	want := wantHeader()
	want["status"] = "received"
	assertFields(t, "data", data, want)

	items, _ := data["items"].([]any)
	if len(items) != 2 {
		t.Errorf("items: got %d, want 2", len(items))
	}

	receipts, ok := data["receipts"].([]any)
	if !ok || len(receipts) != 1 {
		t.Fatalf("receipts: got %v", data["receipts"])
	}
	first, _ := receipts[0].(map[string]any)
	assertFields(t, "receipts[0]", first, wantReceipt())

	receiptItems, _ := first["items"].([]any)
	if len(receiptItems) != 2 {
		t.Fatalf("receipt items: got %v", first["items"])
	}
	second, _ := receiptItems[1].(map[string]any)
	assertFields(t, "receipts[0].items[1]", second, map[string]any{
		"id":                     "ri-2",
		"purchase_order_item_id": "item-2",
		"quantity_received":      float64(50),
		"quantity_damaged":       float64(1),
		"quantity_returned":      float64(2),
		"notes":                  "box crushed",
		"created_at":             "2026-09-20T08:15:30Z",
	})
}

func TestGetByID_NoReceiptsIsAnEmptyArray(t *testing.T) {
	h := NewHandler(&stubService{order: created()}, zap.NewNop())

	rec := getByID(t, h, validID)

	if !strings.Contains(rec.Body.String(), `"receipts":[]`) {
		t.Errorf("body: got %s, want receipts as an empty array, not null", rec.Body)
	}
}

func TestGetByID_ReceiptWithNoItemsHasAnEmptyArray(t *testing.T) {
	order := created()
	order.Receipts = []*domain.PurchaseOrderReceipt{{ID: "receipt-1", PurchaseOrderID: validID}}
	h := NewHandler(&stubService{order: order}, zap.NewNop())

	rec := getByID(t, h, validID)

	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("body: got %s, want the receipt's items as an empty array, not null", rec.Body)
	}
}

func TestGetByID_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "purchase order not found"},
		{"wrapped not found", fmt.Errorf("get: %w", apperror.ErrNotFound), http.StatusNotFound, "purchase order not found"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := getByID(t, h, validID)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}
