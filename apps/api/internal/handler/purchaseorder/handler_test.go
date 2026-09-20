package purchaseorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/jwt"
	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/middleware"
	"apps/api/internal/service/purchaseorder"
	"apps/api/pkg/apperror"
)

var secret = []byte("test-secret")

const (
	validID    = "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e11"
	supplierID = "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11"
	medicineA  = "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11"
	medicineB  = "5c7d9e21-3b5c-4e8a-a1f6-9c0b2d4e6f88"
)

// stubService records what the handler passed and returns the canned results.
type stubService struct {
	order   *domain.PurchaseOrder
	page    *purchaseorder.Page
	receipt *domain.PurchaseOrderReceipt
	err     error

	createInput  purchaseorder.CreateInput
	receiveInput purchaseorder.ReceiveInput
	filter       purchaseorder.ListFilter
	id           string
	called       bool
}

func (s *stubService) Create(_ context.Context, input purchaseorder.CreateInput) (*domain.PurchaseOrder, error) {
	s.called = true
	s.createInput = input

	return s.order, s.err
}

func (s *stubService) List(_ context.Context, filter purchaseorder.ListFilter) (*purchaseorder.Page, error) {
	s.called = true
	s.filter = filter

	return s.page, s.err
}

func (s *stubService) GetByID(_ context.Context, id string) (*domain.PurchaseOrder, error) {
	s.called = true
	s.id = id

	return s.order, s.err
}

func (s *stubService) Receive(_ context.Context, input purchaseorder.ReceiveInput) (*domain.PurchaseOrderReceipt, error) {
	s.called = true
	s.receiveInput = input

	return s.receipt, s.err
}

type brokenWriter struct {
	header http.Header
}

func (w *brokenWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *brokenWriter) WriteHeader(int) {}

func str(value string) *string {
	return &value
}

var at = time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC)

func created() *domain.PurchaseOrder {
	expected := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	return &domain.PurchaseOrder{
		ID:           validID,
		SupplierID:   supplierID,
		Status:       domain.PurchaseOrderStatusDraft,
		OrderDate:    time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		ExpectedDate: &expected,
		CreatedBy:    "user-1",
		Notes:        str("Quarterly restock"),
		CreatedAt:    at,
		UpdatedAt:    at,
		Items: []*domain.PurchaseOrderItem{
			{ID: "item-1", PurchaseOrderID: validID, MedicineID: medicineA, QuantityOrdered: 200, CreatedAt: at},
			{ID: "item-2", PurchaseOrderID: validID, MedicineID: medicineB, QuantityOrdered: 50, CreatedAt: at},
		},
	}
}

func wantHeader() map[string]any {
	return map[string]any{
		"id":            validID,
		"supplier_id":   supplierID,
		"status":        "draft",
		"order_date":    "2026-09-20",
		"expected_date": "2026-09-30",
		"created_by":    "user-1",
		"notes":         "Quarterly restock",
		"created_at":    "2026-09-20T08:15:30Z",
		"updated_at":    "2026-09-20T08:15:30Z",
	}
}

func receipt() *domain.PurchaseOrderReceipt {
	return &domain.PurchaseOrderReceipt{
		ID:              "receipt-1",
		PurchaseOrderID: validID,
		ReceivedBy:      "user-1",
		ReceivedAt:      at,
		Notes:           str("Delivered by courier"),
		CreatedAt:       at,
		Items: []*domain.PurchaseOrderReceiptItem{
			{
				ID: "ri-1", PurchaseOrderReceiptID: "receipt-1", PurchaseOrderItemID: "item-1",
				QuantityReceived: 200, CreatedAt: at,
			},
		},
	}
}

func wantReceipt() map[string]any {
	return map[string]any{
		"id":                "receipt-1",
		"purchase_order_id": validID,
		"received_by":       "user-1",
		"received_at":       "2026-09-20T08:15:30Z",
		"notes":             "Delivered by courier",
		"created_at":        "2026-09-20T08:15:30Z",
	}
}

func accessToken(t *testing.T) string {
	t.Helper()

	access, _, err := jwt.BuildTokenPair(
		domain.AccountPayload{UserID: "user-1", Email: "a@b.com"}, "user-1", secret, time.Hour, time.Hour,
	)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	return access
}

// call sends a request through the real auth middleware with a valid access
// token, so the handler sees the caller exactly as it does in production.
func call(
	t *testing.T, handler http.HandlerFunc, method, target, body string, pathValues map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+accessToken(t))
	for name, value := range pathValues {
		req.SetPathValue(name, value)
	}
	rec := httptest.NewRecorder()

	middleware.Auth(secret, zap.NewNop())(handler)(rec, req)

	return rec
}

func post(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Create, http.MethodPost, "/purchase-orders", body, nil)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return body
}

func assertFields(t *testing.T, name string, got map[string]any, want map[string]any) {
	t.Helper()

	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s.%s: got %v, want %v", name, key, got[key], value)
		}
	}
}

// body builds a valid create body, with overrides applied on top. A nil
// override removes the field.
func body(overrides map[string]any) string {
	fields := map[string]any{
		"supplier_id":   supplierID,
		"order_date":    "2026-09-20",
		"expected_date": "2026-09-30",
		"notes":         "Quarterly restock",
		"items": []any{
			map[string]any{"medicine_id": medicineA, "quantity_ordered": 200},
			map[string]any{"medicine_id": medicineB, "quantity_ordered": 50},
		},
	}
	for name, value := range overrides {
		if value == nil {
			delete(fields, name)
			continue
		}
		fields[name] = value
	}

	encoded, _ := json.Marshal(fields)

	return string(encoded)
}

func oneItem(item map[string]any) map[string]any {
	return map[string]any{"items": []any{item}}
}

func TestCreate_Success(t *testing.T) {
	svc := &stubService{order: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, body(nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}

	raw := rec.Body.String()
	decoded := decode(t, rec)
	if decoded["message"] != "purchase order created" {
		t.Errorf("message: got %v", decoded["message"])
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("data: got %v", decoded["data"])
	}
	assertFields(t, "data", data, wantHeader())

	items, ok := data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items: got %v", data["items"])
	}
	first, _ := items[0].(map[string]any)
	assertFields(t, "items[0]", first, map[string]any{
		"id": "item-1", "medicine_id": medicineA, "quantity_ordered": float64(200), "created_at": "2026-09-20T08:15:30Z",
	})
	if _, present := first["purchase_order_id"]; present {
		t.Error("an item is nested in its order, so it should not repeat purchase_order_id")
	}
	if !strings.Contains(raw, `"receipts":[]`) {
		t.Errorf("body: got %s, want receipts as an empty array, not null", raw)
	}
}

func TestCreate_NullOptionalFieldsAreEncodedAsNull(t *testing.T) {
	order := created()
	order.ExpectedDate = nil
	order.Notes = nil
	h := NewHandler(&stubService{order: order}, zap.NewNop())

	data, _ := decode(t, post(t, h, body(nil)))["data"].(map[string]any)

	for _, field := range []string{"expected_date", "notes"} {
		value, present := data[field]
		if !present || value != nil {
			t.Errorf("data.%s: got %v (present %v), want null", field, value, present)
		}
	}
}

func TestCreate_PassesValidatedInputToService(t *testing.T) {
	svc := &stubService{order: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, body(map[string]any{
		"supplier_id":   "  " + strings.ToUpper(supplierID) + " ",
		"order_date":    " 2026-09-20 ",
		"expected_date": " 2026-09-30\n",
		"notes":         "  Quarterly restock ",
		"items": []any{
			map[string]any{"medicine_id": " " + strings.ToUpper(medicineA), "quantity_ordered": 200},
			map[string]any{"medicine_id": medicineB, "quantity_ordered": 2147483647},
		},
	}))

	in := svc.createInput
	if in.SupplierID != supplierID {
		t.Errorf("supplier id: got %q, want trimmed and lowercased", in.SupplierID)
	}
	if want := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC); !in.OrderDate.Equal(want) {
		t.Errorf("order date: got %v, want %v", in.OrderDate, want)
	}
	if want := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC); in.ExpectedDate == nil || !in.ExpectedDate.Equal(want) {
		t.Errorf("expected date: got %v, want %v", in.ExpectedDate, want)
	}
	if in.Notes == nil || *in.Notes != "Quarterly restock" {
		t.Errorf("notes: got %v, want trimmed", in.Notes)
	}
	want := []purchaseorder.ItemInput{{MedicineID: medicineA, QuantityOrdered: 200}, {MedicineID: medicineB, QuantityOrdered: 2147483647}}
	if len(in.Items) != 2 || in.Items[0] != want[0] || in.Items[1] != want[1] {
		t.Errorf("items: got %+v, want %+v", in.Items, want)
	}
	if in.CreatedBy != "user-1" {
		t.Errorf("created by: got %q, want the caller from the token", in.CreatedBy)
	}
}

func TestCreate_EmptyOptionalFieldsAreNil(t *testing.T) {
	for name, overrides := range map[string]map[string]any{
		"missing": {"expected_date": nil, "notes": nil},
		"empty":   {"expected_date": "", "notes": ""},
		"blank":   {"expected_date": "  ", "notes": "\t"},
		"null":    {"expected_date": json.RawMessage("null"), "notes": json.RawMessage("null")},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{order: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, body(overrides))

			if rec.Code != http.StatusCreated {
				t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
			}
			if svc.createInput.ExpectedDate != nil || svc.createInput.Notes != nil {
				t.Errorf("optional fields: got %+v, want nil", svc.createInput)
			}
		})
	}
}

func TestCreate_CreatedByInBodyIsIgnored(t *testing.T) {
	svc := &stubService{order: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, body(map[string]any{"created_by": "attacker", "status": "received"}))

	if svc.createInput.CreatedBy != "user-1" {
		t.Errorf("created by: got %q, want the caller from the token", svc.createInput.CreatedBy)
	}
}

func TestCreate_PastDatesAndSameDayAreAccepted(t *testing.T) {
	svc := &stubService{order: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, body(map[string]any{"order_date": "2020-01-15", "expected_date": "2020-01-15"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
	}
}

func TestCreate_MaxSizesAccepted(t *testing.T) {
	svc := &stubService{order: created()}
	h := NewHandler(svc, zap.NewNop())

	items := make([]any, 0, 100)
	for i := 0; i < 100; i++ {
		items = append(items, map[string]any{
			"medicine_id":      fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			"quantity_ordered": 2147483647,
		})
	}

	rec := post(t, h, body(map[string]any{"notes": strings.Repeat("é", 500), "items": items}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if len(svc.createInput.Items) != 100 {
		t.Errorf("items: got %d, want 100", len(svc.createInput.Items))
	}
}

// invalidBodies are the create body cases rejected with a 400.
func invalidBodies() []struct {
	name string
	body string
	want string
} {
	item := func(id string, quantity any) map[string]any {
		return oneItem(map[string]any{"medicine_id": id, "quantity_ordered": quantity})
	}

	tooMany := make([]any, 0, 101)
	for i := 0; i < 101; i++ {
		tooMany = append(tooMany, map[string]any{
			"medicine_id":      fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			"quantity_ordered": 1,
		})
	}

	return []struct {
		name string
		body string
		want string
	}{
		{"malformed json", `{`, "invalid request body"},
		{"empty body", ``, "invalid request body"},
		{"wrong type", `{"supplier_id": 5}`, "invalid request body"},
		{"too large", `{"notes":"` + strings.Repeat("a", 1<<20) + `"}`, "invalid request body"},
		{"fractional quantity", strings.Replace(body(nil), `"quantity_ordered":200`, `"quantity_ordered":1.5`, 1), "invalid request body"},
		{"missing supplier", body(map[string]any{"supplier_id": nil}), "supplier_id is required"},
		{"blank supplier", body(map[string]any{"supplier_id": "  "}), "supplier_id is required"},
		{"invalid supplier", body(map[string]any{"supplier_id": "abc"}), "invalid supplier_id"},
		{"missing order date", body(map[string]any{"order_date": nil}), "order_date is required"},
		{"blank order date", body(map[string]any{"order_date": " "}), "order_date is required"},
		{"wrong order date format", body(map[string]any{"order_date": "20/09/2026"}), "order_date must be a valid date in YYYY-MM-DD format"},
		{"impossible order date", body(map[string]any{"order_date": "2026-02-30"}), "order_date must be a valid date in YYYY-MM-DD format"},
		{"order date timestamp", body(map[string]any{"order_date": "2026-09-20T00:00:00Z"}), "order_date must be a valid date in YYYY-MM-DD format"},
		{"wrong expected date format", body(map[string]any{"expected_date": "30/09/2026"}), "expected_date must be a valid date in YYYY-MM-DD format"},
		{"expected before order", body(map[string]any{"order_date": "2026-09-20", "expected_date": "2026-09-19"}), "expected_date must not be before order_date"},
		{"long notes", body(map[string]any{"notes": strings.Repeat("a", 501)}), "notes is too long"},
		{"missing items", body(map[string]any{"items": nil}), "items is required"},
		{"null items", body(map[string]any{"items": json.RawMessage("null")}), "items is required"},
		{"empty items", body(map[string]any{"items": []any{}}), "items is required"},
		{"too many items", body(map[string]any{"items": tooMany}), "items must have at most 100 entries"},
		{"missing medicine", body(oneItem(map[string]any{"quantity_ordered": 1})), "items[0].medicine_id is required"},
		{"null item", body(map[string]any{"items": []any{nil}}), "items[0].medicine_id is required"},
		{"invalid medicine", body(item("abc", 1)), "items[0].medicine_id is invalid"},
		{"missing quantity", body(oneItem(map[string]any{"medicine_id": medicineA})), "items[0].quantity_ordered is required"},
		{"null quantity", body(item(medicineA, nil)), "items[0].quantity_ordered is required"},
		{"zero quantity", body(item(medicineA, 0)), "items[0].quantity_ordered must be greater than zero"},
		{"negative quantity", body(item(medicineA, -1)), "items[0].quantity_ordered must be greater than zero"},
		{"too large quantity", body(item(medicineA, 2147483648)), "items[0].quantity_ordered is too large"},
		{
			"repeated medicine",
			body(map[string]any{"items": []any{
				map[string]any{"medicine_id": medicineA, "quantity_ordered": 1},
				map[string]any{"medicine_id": medicineB, "quantity_ordered": 1},
				map[string]any{"medicine_id": medicineA, "quantity_ordered": 2},
			}}),
			"items[2].medicine_id is repeated",
		},
		{
			"repeated medicine in another case",
			body(map[string]any{"items": []any{
				map[string]any{"medicine_id": medicineA, "quantity_ordered": 1},
				map[string]any{"medicine_id": strings.ToUpper(medicineA), "quantity_ordered": 2},
			}}),
			"items[1].medicine_id is repeated",
		},
		{
			"error names the failing position",
			body(map[string]any{"items": []any{
				map[string]any{"medicine_id": medicineA, "quantity_ordered": 1},
				map[string]any{"medicine_id": medicineB, "quantity_ordered": 0},
			}}),
			"items[1].quantity_ordered must be greater than zero",
		},
		{
			"order fields are checked before items",
			body(map[string]any{"notes": strings.Repeat("a", 501), "items": nil}),
			"notes is too long",
		},
	}
}

func TestCreate_BadRequest(t *testing.T) {
	for _, tt := range invalidBodies() {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{order: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusBadRequest, rec.Body)
			}
			if got := decode(t, rec)["error"]; got != tt.want {
				t.Errorf("error: got %v, want %q", got, tt.want)
			}
			if svc.called {
				t.Error("service must not be called for an invalid request")
			}
		})
	}
}

func TestNoCallerInContext(t *testing.T) {
	handlers := map[string]func(*Handler) http.HandlerFunc{
		"create":    func(h *Handler) http.HandlerFunc { return h.Create },
		"list":      func(h *Handler) http.HandlerFunc { return h.List },
		"get by id": func(h *Handler) http.HandlerFunc { return h.GetByID },
		"receive":   func(h *Handler) http.HandlerFunc { return h.Receive },
	}

	for name, pick := range handlers {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{order: created(), page: &purchaseorder.Page{}, receipt: receipt()}
			handler := pick(NewHandler(svc, zap.NewNop()))

			req := httptest.NewRequest(http.MethodPost, "/purchase-orders", strings.NewReader(body(nil)))
			req.SetPathValue("id", validID)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if got := decode(t, rec)["error"]; got != "unauthorized" {
				t.Errorf("error: got %v", got)
			}
			if svc.called {
				t.Error("service must not be called without a caller")
			}
		})
	}
}

func TestCreate_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"supplier not found", apperror.ErrSupplierNotFound, http.StatusNotFound, "supplier not found"},
		{"medicine not found", apperror.ErrMedicineNotFound, http.StatusNotFound, "medicine not found"},
		{"wrapped supplier not found", fmt.Errorf("create: %w", apperror.ErrSupplierNotFound), http.StatusNotFound, "supplier not found"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
		{"bare conflict is not a known one", apperror.ErrConflict, http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := post(t, h, body(nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}

func TestCreate_WriteFailure(t *testing.T) {
	h := NewHandler(&stubService{order: created()}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/purchase-orders", strings.NewReader(body(nil)))
	req.Header.Set("Authorization", "Bearer "+accessToken(t))

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Create)(&brokenWriter{}, req)
}
