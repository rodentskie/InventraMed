package medicine

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
	"apps/api/internal/service/medicine"
	"apps/api/pkg/apperror"
)

var secret = []byte("test-secret")

const validID = "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11"

// stubService records what the handler passed and returns the canned results.
type stubService struct {
	medicine  *domain.Medicine
	page      *medicine.Page
	locations []medicine.LocationStatus
	err       error

	createInput medicine.CreateInput
	updateInput medicine.UpdateInput
	filter      medicine.ListFilter
	id          string
	barcode     string
	called      bool
}

func (s *stubService) Create(_ context.Context, input medicine.CreateInput) (*domain.Medicine, error) {
	s.called = true
	s.createInput = input

	return s.medicine, s.err
}

func (s *stubService) List(_ context.Context, filter medicine.ListFilter) (*medicine.Page, error) {
	s.called = true
	s.filter = filter

	return s.page, s.err
}

func (s *stubService) GetByBarcode(_ context.Context, barcode string) (*domain.Medicine, error) {
	s.called = true
	s.barcode = barcode

	return s.medicine, s.err
}

func (s *stubService) Locations(_ context.Context) ([]medicine.LocationStatus, error) {
	s.called = true

	return s.locations, s.err
}

func (s *stubService) Update(_ context.Context, id string, input medicine.UpdateInput) error {
	s.called = true
	s.id = id
	s.updateInput = input

	return s.err
}

func (s *stubService) Delete(_ context.Context, id string) error {
	s.called = true
	s.id = id

	return s.err
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

const validBody = `{
	"name": "Paracetamol 500mg",
	"barcode": "8901234567890",
	"batch_number": "B2026-001",
	"expiration_date": "2027-03-31",
	"quantity": 120
}`

func created() *domain.Medicine {
	batch := "B2026-001"
	by := "user-1"
	location := 7

	return &domain.Medicine{
		ID:             "med-1",
		Name:           "Paracetamol 500mg",
		Barcode:        "8901234567890",
		BatchNumber:    &batch,
		ExpirationDate: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		Quantity:       120,
		Location:       &location,
		CreatedBy:      &by,
		CreatedAt:      time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
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

	return call(t, h.Create, http.MethodPost, "/medicines", body, nil)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return body
}

func TestCreate_Success(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}

	body := decode(t, rec)
	if body["message"] != "medicine created" {
		t.Errorf("message: got %v", body["message"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data: got %v", body["data"])
	}
	want := map[string]any{
		"id":              "med-1",
		"name":            "Paracetamol 500mg",
		"barcode":         "8901234567890",
		"batch_number":    "B2026-001",
		"expiration_date": "2027-03-31",
		"quantity":        float64(120),
		"location":        float64(7),
		"created_by":      "user-1",
		"created_at":      "2026-09-20T08:15:30Z",
		"updated_at":      "2026-09-20T08:15:30Z",
	}
	for key, value := range want {
		if data[key] != value {
			t.Errorf("data.%s: got %v, want %v", key, data[key], value)
		}
	}
}

func TestCreate_PassesValidatedInputToService(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, `{
		"name": "  Paracetamol 500mg ",
		"barcode": " 8901234567890\n",
		"batch_number": " B2026-001 ",
		"expiration_date": " 2027-03-31 ",
		"quantity": 0
	}`)

	in := svc.createInput
	if in.Name != "Paracetamol 500mg" || in.Barcode != "8901234567890" {
		t.Errorf("name %q barcode %q, want trimmed", in.Name, in.Barcode)
	}
	if in.BatchNumber == nil || *in.BatchNumber != "B2026-001" {
		t.Errorf("batch number: got %v, want trimmed", in.BatchNumber)
	}
	if want := time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC); !in.ExpirationDate.Equal(want) {
		t.Errorf("expiration date: got %v, want %v", in.ExpirationDate, want)
	}
	if in.Quantity != 0 {
		t.Errorf("quantity: got %d, want 0 accepted", in.Quantity)
	}
	if in.CreatedBy != "user-1" {
		t.Errorf("created by: got %q, want the caller from the token", in.CreatedBy)
	}
}

func TestCreate_EmptyBatchIsNil(t *testing.T) {
	for name, batch := range map[string]string{
		"missing": ``,
		"empty":   `"batch_number": "",`,
		"blank":   `"batch_number": "   ",`,
		"null":    `"batch_number": null,`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{medicine: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, fmt.Sprintf(
				`{"name":"P","barcode":"1",%s"expiration_date":"2027-03-31","quantity":1}`, batch,
			))

			if rec.Code != http.StatusCreated {
				t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
			}
			if svc.createInput.BatchNumber != nil {
				t.Errorf("batch number: got %q, want nil", *svc.createInput.BatchNumber)
			}
		})
	}
}

func TestCreate_Location(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  *int
	}{
		{"missing", ``, nil},
		{"null", `"location": null,`, nil},
		{"first compartment", `"location": 1,`, new(1)},
		{"last compartment", `"location": 12,`, new(12)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{medicine: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, fmt.Sprintf(
				`{"name":"P","barcode":"1",%s"expiration_date":"2027-03-31","quantity":1}`, tt.field,
			))

			if rec.Code != http.StatusCreated {
				t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
			}
			assertLocation(t, svc.createInput.Location, tt.want)
		})
	}
}

// assertLocation compares a location the handler passed to the service.
func assertLocation(t *testing.T, got, want *int) {
	t.Helper()

	switch {
	case want == nil && got != nil:
		t.Errorf("location: got %d, want nil", *got)
	case want != nil && (got == nil || *got != *want):
		t.Errorf("location: got %v, want %d", got, *want)
	}
}

func TestCreate_PastExpirationDateAllowed(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, `{"name":"P","barcode":"1","expiration_date":"2001-01-01","quantity":1}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestCreate_CreatedByInBodyIsIgnored(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"created_by":"someone-else"}`)

	if svc.createInput.CreatedBy != "user-1" {
		t.Errorf("created by: got %q, want user-1", svc.createInput.CreatedBy)
	}
}

func TestCreate_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed json", `{`, "invalid request body"},
		{"empty body", ``, "invalid request body"},
		{"wrong type", `{"name": 5}`, "invalid request body"},
		{"fractional quantity", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1.5}`, "invalid request body"},
		{"missing name", `{"barcode":"1","expiration_date":"2027-03-31","quantity":1}`, "name is required"},
		{"blank name", `{"name":"  ","barcode":"1","expiration_date":"2027-03-31","quantity":1}`, "name is required"},
		{"long name", fmt.Sprintf(`{"name":%q,"barcode":"1","expiration_date":"2027-03-31","quantity":1}`, strings.Repeat("a", 256)), "name is too long"},
		{"missing barcode", `{"name":"P","expiration_date":"2027-03-31","quantity":1}`, "barcode is required"},
		{"blank barcode", `{"name":"P","barcode":"\n","expiration_date":"2027-03-31","quantity":1}`, "barcode is required"},
		{"long barcode", fmt.Sprintf(`{"name":"P","barcode":%q,"expiration_date":"2027-03-31","quantity":1}`, strings.Repeat("1", 129)), "barcode is too long"},
		{"long batch", fmt.Sprintf(`{"name":"P","barcode":"1","batch_number":%q,"expiration_date":"2027-03-31","quantity":1}`, strings.Repeat("b", 65)), "batch_number is too long"},
		{"missing expiration date", `{"name":"P","barcode":"1","quantity":1}`, "expiration_date is required"},
		{"blank expiration date", `{"name":"P","barcode":"1","expiration_date":" ","quantity":1}`, "expiration_date is required"},
		{"wrong date format", `{"name":"P","barcode":"1","expiration_date":"31/03/2027","quantity":1}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
		{"impossible date", `{"name":"P","barcode":"1","expiration_date":"2027-02-30","quantity":1}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
		{"timestamp instead of date", `{"name":"P","barcode":"1","expiration_date":"2027-03-31T00:00:00Z","quantity":1}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
		{"missing quantity", `{"name":"P","barcode":"1","expiration_date":"2027-03-31"}`, "quantity is required"},
		{"null quantity", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":null}`, "quantity is required"},
		{"negative quantity", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":-1}`, "quantity must be zero or greater"},
		{"location zero", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"location":0}`, "location must be between 1 and 12"},
		{"location above the tray", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"location":13}`, "location must be between 1 and 12"},
		{"negative location", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"location":-1}`, "location must be between 1 and 12"},
		{"location as text", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"location":"7"}`, "invalid request body"},
		{"fractional location", `{"name":"P","barcode":"1","expiration_date":"2027-03-31","quantity":1,"location":7.5}`, "invalid request body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{medicine: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
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

func TestCreate_MaxLengthsAccepted(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, fmt.Sprintf(
		`{"name":%q,"barcode":%q,"batch_number":%q,"expiration_date":"2027-03-31","quantity":1}`,
		strings.Repeat("é", 255), strings.Repeat("1", 128), strings.Repeat("b", 64),
	))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
}

func TestNoCallerInContext(t *testing.T) {
	// GetByBarcode is intentionally not here: it's public and never checks
	// for a caller, see TestGetByBarcode_NoCallerInContext.
	handlers := map[string]func(*Handler) http.HandlerFunc{
		"create": func(h *Handler) http.HandlerFunc { return h.Create },
		"list":   func(h *Handler) http.HandlerFunc { return h.List },
		"update": func(h *Handler) http.HandlerFunc { return h.Update },
		"delete": func(h *Handler) http.HandlerFunc { return h.Delete },
	}

	for name, pick := range handlers {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{medicine: created(), page: &medicine.Page{}}
			handler := pick(NewHandler(svc, zap.NewNop()))

			req := httptest.NewRequest(http.MethodPost, "/medicines", strings.NewReader(validBody))
			req.SetPathValue("id", validID)
			req.SetPathValue("barcode", "8901234567890")
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
		{"name and batch conflict", apperror.ErrNameBatchExists, http.StatusConflict, "medicine with this name and batch number already exists"},
		{"barcode conflict", apperror.ErrBarcodeExists, http.StatusConflict, "medicine with this barcode already exists"},
		{"location conflict", apperror.ErrLocationTaken, http.StatusConflict, "another medicine is already in this location"},
		{"wrapped conflict", fmt.Errorf("create: %w", apperror.ErrBarcodeExists), http.StatusConflict, "medicine with this barcode already exists"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
		{"bare conflict is not a known one", apperror.ErrConflict, http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := post(t, h, validBody)

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
	h := NewHandler(&stubService{medicine: created()}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/medicines", strings.NewReader(validBody))
	req.Header.Set("Authorization", "Bearer "+accessToken(t))

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Create)(&brokenWriter{}, req)
}
