package medicine

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/service/medicine"
	"apps/api/pkg/apperror"
)

func list(t *testing.T, h *Handler, query string) (status int, body string, decoded map[string]any) {
	t.Helper()

	rec := call(t, h.List, http.MethodGet, "/medicines?"+query, "", nil)
	body = rec.Body.String()
	if rec.Code == http.StatusOK || strings.HasPrefix(body, "{") {
		decoded = decode(t, rec)
	}

	return rec.Code, body, decoded
}

func TestList_Success(t *testing.T) {
	svc := &stubService{page: &medicine.Page{Medicines: []*domain.Medicine{created()}, Total: 57}}
	h := NewHandler(svc, zap.NewNop())

	status, _, body := list(t, h, "limit=10&offset=20")

	if status != http.StatusOK {
		t.Fatalf("status: got %d, want %d", status, http.StatusOK)
	}
	if body["total"] != float64(57) || body["limit"] != float64(10) || body["offset"] != float64(20) {
		t.Errorf("paging fields: got total %v limit %v offset %v", body["total"], body["limit"], body["offset"])
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data: got %v", body["data"])
	}
	first, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("data[0]: got %v", data[0])
	}
	if first["id"] != "med-1" || first["barcode"] != "8901234567890" || first["expiration_date"] != "2027-03-31" {
		t.Errorf("data[0]: got %v", first)
	}
}

func TestList_DefaultsAndFilters(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  medicine.ListFilter
	}{
		{"defaults", "", medicine.ListFilter{Limit: 20, Offset: 0}},
		{"empty values use the defaults", "limit=&offset=", medicine.ListFilter{Limit: 20, Offset: 0}},
		{"custom paging", "limit=5&offset=15", medicine.ListFilter{Limit: 5, Offset: 15}},
		{"bounds are allowed", "limit=100&offset=0", medicine.ListFilter{Limit: 100, Offset: 0}},
		{"lower bound", "limit=1", medicine.ListFilter{Limit: 1, Offset: 0}},
		{"filters are trimmed", "name=%20para%20&barcode=%20890%20", medicine.ListFilter{Limit: 20, Name: "para", Barcode: "890"}},
		{"blank filters are ignored", "name=%20%20&barcode=", medicine.ListFilter{Limit: 20}},
		{"wildcards are passed as text", "name=" + url.QueryEscape("100%_"), medicine.ListFilter{Limit: 20, Name: "100%_"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &medicine.Page{}}
			h := NewHandler(svc, zap.NewNop())

			status, _, _ := list(t, h, tt.query)

			if status != http.StatusOK {
				t.Fatalf("status: got %d, want %d", status, http.StatusOK)
			}
			if svc.filter != tt.want {
				t.Errorf("filter: got %+v, want %+v", svc.filter, tt.want)
			}
		})
	}
}

func TestList_BadRequest(t *testing.T) {
	const (
		limitMessage  = "limit must be a whole number between 1 and 100"
		offsetMessage = "offset must be a whole number of zero or greater"
	)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"non numeric limit", "limit=abc", limitMessage},
		{"fractional limit", "limit=1.5", limitMessage},
		{"zero limit", "limit=0", limitMessage},
		{"negative limit", "limit=-1", limitMessage},
		{"limit above the maximum", "limit=101", limitMessage},
		{"non numeric offset", "offset=abc", offsetMessage},
		{"fractional offset", "offset=1.5", offsetMessage},
		{"negative offset", "offset=-1", offsetMessage},
		{"offset overflow", "offset=99999999999999999999", offsetMessage},
		{"long name", "name=" + strings.Repeat("a", 256), "name is too long"},
		{"long barcode", "barcode=" + strings.Repeat("1", 129), "barcode is too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &medicine.Page{}}
			h := NewHandler(svc, zap.NewNop())

			status, _, body := list(t, h, tt.query)

			if status != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", status, http.StatusBadRequest)
			}
			if body["error"] != tt.want {
				t.Errorf("error: got %v, want %q", body["error"], tt.want)
			}
			if svc.called {
				t.Error("service must not be called for an invalid request")
			}
		})
	}
}

func TestList_MaxLengthFiltersAccepted(t *testing.T) {
	h := NewHandler(&stubService{page: &medicine.Page{}}, zap.NewNop())

	status, _, _ := list(t, h, "name="+strings.Repeat("a", 255)+"&barcode="+strings.Repeat("1", 128))

	if status != http.StatusOK {
		t.Fatalf("status: got %d, want %d", status, http.StatusOK)
	}
}

func TestList_EmptyPageIsAnEmptyArray(t *testing.T) {
	// A nil slice from the service must still be encoded as [] and not null.
	h := NewHandler(&stubService{page: &medicine.Page{Medicines: nil, Total: 0}}, zap.NewNop())

	status, raw, _ := list(t, h, "offset=500")

	if status != http.StatusOK {
		t.Fatalf("status: got %d, want %d", status, http.StatusOK)
	}
	if !strings.Contains(raw, `"data":[]`) {
		t.Errorf("body: got %s, want an empty data array", raw)
	}
}

func TestList_ServiceError(t *testing.T) {
	h := NewHandler(&stubService{err: errors.New("db down")}, zap.NewNop())

	status, _, body := list(t, h, "")

	if status != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", status, http.StatusInternalServerError)
	}
	if body["error"] != "internal server error" {
		t.Errorf("error: got %v", body["error"])
	}
}

func getByBarcode(t *testing.T, h *Handler, barcode string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.GetByBarcode, http.MethodGet, "/medicines/barcode/x", "", map[string]string{"barcode": barcode})
}

func TestGetByBarcode_Success(t *testing.T) {
	svc := &stubService{medicine: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := getByBarcode(t, h, " 8901234567890 ")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body)
	}
	if svc.barcode != "8901234567890" {
		t.Errorf("barcode passed to the service: got %q, want trimmed", svc.barcode)
	}

	data, ok := decode(t, rec)["data"].(map[string]any)
	if !ok {
		t.Fatal("data missing from the response")
	}
	want := map[string]any{
		"id":              "med-1",
		"name":            "Paracetamol 500mg",
		"barcode":         "8901234567890",
		"batch_number":    "B2026-001",
		"expiration_date": "2027-03-31",
		"quantity":        float64(120),
		"created_by":      "user-1",
	}
	for key, value := range want {
		if data[key] != value {
			t.Errorf("data.%s: got %v, want %v", key, data[key], value)
		}
	}
}

func TestGetByBarcode_BadRequest(t *testing.T) {
	tests := map[string]struct {
		barcode string
		want    string
	}{
		"blank":    {"   ", "barcode is required"},
		"too long": {strings.Repeat("1", 129), "barcode is too long"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{medicine: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := getByBarcode(t, h, tt.barcode)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if got := decode(t, rec)["error"]; got != tt.want {
				t.Errorf("error: got %v, want %q", got, tt.want)
			}
			if svc.called {
				t.Error("service must not be called for an invalid barcode")
			}
		})
	}
}

func TestGetByBarcode_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "medicine not found"},
		{"wrapped not found", fmt.Errorf("get: %w", apperror.ErrNotFound), http.StatusNotFound, "medicine not found"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := getByBarcode(t, h, "8901234567890")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}
