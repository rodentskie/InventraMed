package inventory

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
	"apps/api/internal/service/inventory"
	"apps/api/pkg/apperror"
)

var secret = []byte("test-secret")

const validMedicineID = "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11"

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

// stubService records what the handler passed and returns the canned results.
type stubService struct {
	entry *domain.InventoryEntry
	page  *inventory.Page
	err   error

	createInput inventory.CreateInput
	filter      inventory.ListFilter
	gotID       string
	called      bool
}

func (s *stubService) Create(_ context.Context, input inventory.CreateInput) (*domain.InventoryEntry, error) {
	s.called = true
	s.createInput = input

	return s.entry, s.err
}

func (s *stubService) List(_ context.Context, filter inventory.ListFilter) (*inventory.Page, error) {
	s.called = true
	s.filter = filter

	return s.page, s.err
}

func (s *stubService) GetByID(_ context.Context, id string) (*domain.InventoryEntry, error) {
	s.called = true
	s.gotID = id

	return s.entry, s.err
}

var validBody = fmt.Sprintf(`{
	"medicine_id": "%s",
	"direction": "subtraction",
	"quantity": 5,
	"reason": "damaged",
	"notes": "water damage during storage"
}`, validMedicineID)

func created() *domain.InventoryEntry {
	notes := "water damage during storage"

	return &domain.InventoryEntry{
		ID:         "entry-1",
		MedicineID: validMedicineID,
		Direction:  "subtraction",
		Quantity:   5,
		Reason:     "damaged",
		CountedBy:  "user-1",
		Notes:      &notes,
		CreatedAt:  time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
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

	return call(t, h.Create, http.MethodPost, "/inventory-entries", body, nil)
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
	svc := &stubService{entry: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}

	body := decode(t, rec)
	if body["message"] != "inventory entry recorded" {
		t.Errorf("message: got %v", body["message"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data: got %v", body["data"])
	}
	want := map[string]any{
		"id":          "entry-1",
		"medicine_id": validMedicineID,
		"direction":   "subtraction",
		"quantity":    float64(5),
		"reason":      "damaged",
		"counted_by":  "user-1",
		"notes":       "water damage during storage",
		"created_at":  "2026-09-20T08:15:30Z",
	}
	for key, value := range want {
		if data[key] != value {
			t.Errorf("data.%s: got %v, want %v", key, data[key], value)
		}
	}
}

func TestCreate_PassesTrimmedInputAndCallerToService(t *testing.T) {
	svc := &stubService{entry: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, fmt.Sprintf(`{
		"medicine_id": "  %s ",
		"direction": " addition ",
		"quantity": 3,
		"reason": " count_adjustment ",
		"notes": "  "
	}`, validMedicineID))

	in := svc.createInput
	if in.MedicineID != validMedicineID {
		t.Errorf("medicine_id: got %q", in.MedicineID)
	}
	if in.Direction != "addition" {
		t.Errorf("direction: got %q", in.Direction)
	}
	if in.Reason != "count_adjustment" {
		t.Errorf("reason: got %q", in.Reason)
	}
	if in.Notes != nil {
		t.Errorf("notes: got %v, want nil for blank notes", *in.Notes)
	}
	if in.CountedBy != "user-1" {
		t.Errorf("counted_by: got %q, want the caller's user id", in.CountedBy)
	}
}

func TestCreate_CountedByInBodyIsIgnored(t *testing.T) {
	svc := &stubService{entry: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, fmt.Sprintf(`{
		"medicine_id": "%s",
		"direction": "addition",
		"quantity": 1,
		"reason": "count_adjustment",
		"counted_by": "someone-else"
	}`, validMedicineID))

	if svc.createInput.CountedBy != "user-1" {
		t.Errorf("counted_by: got %q, want caller's id regardless of body", svc.createInput.CountedBy)
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	tests := map[string]struct {
		body    string
		message string
	}{
		"invalid json": {
			body:    `{`,
			message: "invalid request body",
		},
		"missing medicine_id": {
			body:    `{"direction":"addition","quantity":1,"reason":"r"}`,
			message: "medicine_id is required",
		},
		"invalid medicine_id": {
			body:    `{"medicine_id":"not-a-uuid","direction":"addition","quantity":1,"reason":"r"}`,
			message: "invalid medicine_id",
		},
		"missing direction": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","quantity":1,"reason":"r"}`, validMedicineID),
			message: "direction is required",
		},
		"invalid direction": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","direction":"sideways","quantity":1,"reason":"r"}`, validMedicineID),
			message: "direction must be addition or subtraction",
		},
		"missing quantity": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","reason":"r"}`, validMedicineID),
			message: "quantity is required",
		},
		"zero quantity": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","quantity":0,"reason":"r"}`, validMedicineID),
			message: "quantity must be greater than zero",
		},
		"negative quantity": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","quantity":-1,"reason":"r"}`, validMedicineID),
			message: "quantity must be greater than zero",
		},
		"missing reason": {
			body:    fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","quantity":1}`, validMedicineID),
			message: "reason is required",
		},
		"reason too long": {
			body: fmt.Sprintf(
				`{"medicine_id":"%s","direction":"addition","quantity":1,"reason":"%s"}`,
				validMedicineID, strings.Repeat("r", 101),
			),
			message: "reason is too long",
		},
		"notes too long": {
			body: fmt.Sprintf(
				`{"medicine_id":"%s","direction":"addition","quantity":1,"reason":"r","notes":"%s"}`,
				validMedicineID, strings.Repeat("n", 501),
			),
			message: "notes is too long",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if svc.called {
				t.Error("service should not be called on validation failure")
			}
			if got := decode(t, rec)["error"]; got != tt.message {
				t.Errorf("message: got %v, want %q", got, tt.message)
			}
		})
	}
}

func TestCreate_NoCallerIsUnauthorized(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/inventory-entries", strings.NewReader(validBody))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if svc.called {
		t.Error("service should not be called without a caller")
	}
}

func TestCreate_MedicineNotFound(t *testing.T) {
	svc := &stubService{err: apperror.ErrNotFound}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if got := decode(t, rec)["error"]; got != "medicine not found" {
		t.Errorf("message: got %v", got)
	}
}

func TestCreate_InsufficientQuantity(t *testing.T) {
	svc := &stubService{err: apperror.ErrInsufficientQuantity}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", rec.Code)
	}
	if got := decode(t, rec)["error"]; got != "insufficient quantity for subtraction" {
		t.Errorf("message: got %v", got)
	}
}

func TestCreate_InternalError(t *testing.T) {
	svc := &stubService{err: errors.New("boom")}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("message: got %v", got)
	}
}

func TestCreate_WriteFailure(t *testing.T) {
	h := NewHandler(&stubService{entry: created()}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/inventory-entries", strings.NewReader(validBody))
	req.Header.Set("Authorization", "Bearer "+accessToken(t))

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Create)(&brokenWriter{}, req)
}

func TestInvalidMedicineIDFormat(t *testing.T) {
	ids := map[string]string{
		"empty":            "",
		"not a uuid":       "abc",
		"sql injection":    "1' OR '1'='1",
		"too short":        "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e1",
		"too long":         "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e111",
		"no hyphens":       "0b8f3c626a1e4c3e9d0e5f1d2a7c9e11xxxx",
		"misplaced hyphen": "0b8f3c62x6a1e-4c3e-9d0e-5f1d2a7c9e11",
		"non hex":          "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e1g",
		"non ascii":        "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e1é",
		"braces":           "{0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e1}",
	}

	for name, id := range ids {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			body := fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","quantity":1,"reason":"r"}`, id)
			rec := post(t, h, body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400", rec.Code)
			}
			want := "invalid medicine_id"
			if id == "" {
				want = "medicine_id is required"
			}
			if got := decode(t, rec)["error"]; got != want {
				t.Errorf("error: got %v, want %q", got, want)
			}
			if svc.called {
				t.Error("service must not be called for an invalid medicine_id")
			}
		})
	}
}

func TestMedicineID_UppercaseUUIDIsAccepted(t *testing.T) {
	svc := &stubService{entry: created()}
	h := NewHandler(svc, zap.NewNop())
	id := strings.ToUpper(validMedicineID)

	body := fmt.Sprintf(`{"medicine_id":"%s","direction":"addition","quantity":1,"reason":"r"}`, id)
	rec := post(t, h, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if svc.createInput.MedicineID != id {
		t.Errorf("medicine_id: got %q, want %q", svc.createInput.MedicineID, id)
	}
}
