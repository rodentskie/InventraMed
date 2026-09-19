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

type stubService struct {
	medicine *domain.Medicine
	err      error
	input    medicine.CreateInput
	called   bool
}

func (s *stubService) Create(_ context.Context, input medicine.CreateInput) (*domain.Medicine, error) {
	s.called = true
	s.input = input

	return s.medicine, s.err
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

	return &domain.Medicine{
		ID:             "med-1",
		Name:           "Paracetamol 500mg",
		Barcode:        "8901234567890",
		BatchNumber:    &batch,
		ExpirationDate: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		Quantity:       120,
		CreatedBy:      &by,
		CreatedAt:      time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
	}
}

// post sends body through the real auth middleware with a valid access token,
// so the handler sees the caller exactly as it does in production.
func post(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	access, _, err := jwt.BuildTokenPair(
		domain.AccountPayload{UserID: "user-1", Email: "a@b.com"}, "user-1", secret, time.Hour, time.Hour,
	)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/medicines", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()

	middleware.Auth(secret, zap.NewNop())(h.Create)(rec, req)

	return rec
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

	in := svc.input
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
			if svc.input.BatchNumber != nil {
				t.Errorf("batch number: got %q, want nil", *svc.input.BatchNumber)
			}
		})
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

	if svc.input.CreatedBy != "user-1" {
		t.Errorf("created by: got %q, want user-1", svc.input.CreatedBy)
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

func TestCreate_NoCallerInContext(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/medicines", strings.NewReader(validBody))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := decode(t, rec)["error"]; got != "unauthorized" {
		t.Errorf("error: got %v", got)
	}
	if svc.called {
		t.Error("service must not be called without a caller")
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

	access, _, err := jwt.BuildTokenPair(
		domain.AccountPayload{UserID: "user-1"}, "user-1", secret, time.Hour, time.Hour,
	)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/medicines", strings.NewReader(validBody))
	req.Header.Set("Authorization", "Bearer "+access)

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Create)(&brokenWriter{}, req)
}
