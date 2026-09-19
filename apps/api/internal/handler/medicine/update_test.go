package medicine

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"apps/api/pkg/apperror"
)

const validUpdateBody = `{
	"name": "Paracetamol 500mg",
	"barcode": "8901234567890",
	"batch_number": "B2026-002",
	"expiration_date": "2027-06-30"
}`

func put(t *testing.T, h *Handler, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Update, http.MethodPut, "/medicines/id", body, map[string]string{"id": id})
}

func remove(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Delete, http.MethodDelete, "/medicines/id", "", map[string]string{"id": id})
}

func TestUpdate_Success(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	rec := put(t, h, validID, validUpdateBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusNoContent, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body: got %q, want empty", rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Errorf("content type: got %q, want none", got)
	}
	if svc.id != validID {
		t.Errorf("id passed to the service: got %q", svc.id)
	}
}

func TestUpdate_PassesValidatedInputToService(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	put(t, h, validID, `{
		"name": "  Paracetamol 500mg ",
		"barcode": " 8901234567890\n",
		"batch_number": " B2026-002 ",
		"expiration_date": " 2027-06-30 "
	}`)

	in := svc.updateInput
	if in.Name != "Paracetamol 500mg" || in.Barcode != "8901234567890" {
		t.Errorf("name %q barcode %q, want trimmed", in.Name, in.Barcode)
	}
	if in.BatchNumber == nil || *in.BatchNumber != "B2026-002" {
		t.Errorf("batch number: got %v, want trimmed", in.BatchNumber)
	}
	if want := time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC); !in.ExpirationDate.Equal(want) {
		t.Errorf("expiration date: got %v, want %v", in.ExpirationDate, want)
	}
}

func TestUpdate_OmittedBatchClearsIt(t *testing.T) {
	for name, batch := range map[string]string{
		"missing": ``,
		"empty":   `"batch_number": "",`,
		"null":    `"batch_number": null,`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := put(t, h, validID, fmt.Sprintf(
				`{"name":"P","barcode":"1",%s"expiration_date":"2027-03-31"}`, batch,
			))

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
			}
			if svc.updateInput.BatchNumber != nil {
				t.Errorf("batch number: got %q, want nil", *svc.updateInput.BatchNumber)
			}
		})
	}
}

func TestUpdate_QuantityAndCreatedByInBodyAreIgnored(t *testing.T) {
	// Even a value that would fail create validation must not cause a 400.
	for name, extra := range map[string]string{
		"quantity":         `"quantity": 95,`,
		"invalid quantity": `"quantity": -5,`,
		"quantity text":    `"quantity": "many",`,
		"created_by":       `"created_by": "someone-else",`,
		"id and timestamp": `"id": "x", "created_at": "2026-01-01T00:00:00Z",`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := put(t, h, validID, fmt.Sprintf(
				`{%s"name":"P","barcode":"1","expiration_date":"2027-03-31"}`, extra,
			))

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusNoContent, rec.Body)
			}
			if svc.updateInput.Name != "P" || svc.updateInput.Barcode != "1" {
				t.Errorf("update input: got %+v", svc.updateInput)
			}
		})
	}
}

func TestUpdate_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed json", `{`, "invalid request body"},
		{"empty body", ``, "invalid request body"},
		{"wrong type", `{"name": 5}`, "invalid request body"},
		{"missing name", `{"barcode":"1","expiration_date":"2027-03-31"}`, "name is required"},
		{"blank name", `{"name":"  ","barcode":"1","expiration_date":"2027-03-31"}`, "name is required"},
		{"long name", fmt.Sprintf(`{"name":%q,"barcode":"1","expiration_date":"2027-03-31"}`, strings.Repeat("a", 256)), "name is too long"},
		{"missing barcode", `{"name":"P","expiration_date":"2027-03-31"}`, "barcode is required"},
		{"blank barcode", `{"name":"P","barcode":"\n","expiration_date":"2027-03-31"}`, "barcode is required"},
		{"long barcode", fmt.Sprintf(`{"name":"P","barcode":%q,"expiration_date":"2027-03-31"}`, strings.Repeat("1", 129)), "barcode is too long"},
		{"long batch", fmt.Sprintf(`{"name":"P","barcode":"1","batch_number":%q,"expiration_date":"2027-03-31"}`, strings.Repeat("b", 65)), "batch_number is too long"},
		{"missing expiration date", `{"name":"P","barcode":"1"}`, "expiration_date is required"},
		{"blank expiration date", `{"name":"P","barcode":"1","expiration_date":" "}`, "expiration_date is required"},
		{"wrong date format", `{"name":"P","barcode":"1","expiration_date":"31/03/2027"}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
		{"impossible date", `{"name":"P","barcode":"1","expiration_date":"2027-02-30"}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
		{"timestamp instead of date", `{"name":"P","barcode":"1","expiration_date":"2027-03-31T00:00:00Z"}`, "expiration_date must be a valid date in YYYY-MM-DD format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := put(t, h, validID, tt.body)

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

func TestInvalidPathID(t *testing.T) {
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
		for _, action := range []string{"update", "delete"} {
			t.Run(action+" "+name, func(t *testing.T) {
				svc := &stubService{}
				h := NewHandler(svc, zap.NewNop())

				var rec *httptest.ResponseRecorder
				if action == "update" {
					rec = put(t, h, id, validUpdateBody)
				} else {
					rec = remove(t, h, id)
				}

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
				}
				if got := decode(t, rec)["error"]; got != "invalid medicine id" {
					t.Errorf("error: got %v", got)
				}
				if svc.called {
					t.Error("service must not be called for an invalid id")
				}
			})
		}
	}
}

func TestPathID_UppercaseUUIDIsAccepted(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())
	id := strings.ToUpper(validID)

	rec := remove(t, h, id)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	if svc.id != id {
		t.Errorf("id passed to the service: got %q", svc.id)
	}
}

func TestUpdate_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "medicine not found"},
		{"name and batch conflict", apperror.ErrNameBatchExists, http.StatusConflict, "medicine with this name and batch number already exists"},
		{"barcode conflict", apperror.ErrBarcodeExists, http.StatusConflict, "medicine with this barcode already exists"},
		{"wrapped conflict", fmt.Errorf("update: %w", apperror.ErrBarcodeExists), http.StatusConflict, "medicine with this barcode already exists"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := put(t, h, validID, validUpdateBody)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}

func TestDelete_Success(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	rec := remove(t, h, validID)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusNoContent, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body: got %q, want empty", rec.Body)
	}
	if svc.id != validID {
		t.Errorf("id passed to the service: got %q", svc.id)
	}
}

func TestDelete_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "medicine not found"},
		{"used in a purchase order", apperror.ErrMedicineInPurchaseOrder, http.StatusConflict, "medicine is used in a purchase order and cannot be deleted"},
		{"wrapped purchase order conflict", fmt.Errorf("delete: %w", apperror.ErrMedicineInPurchaseOrder), http.StatusConflict, "medicine is used in a purchase order and cannot be deleted"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := remove(t, h, validID)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}
