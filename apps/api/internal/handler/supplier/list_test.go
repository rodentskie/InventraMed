package supplier

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
	"apps/api/internal/service/supplier"
	"apps/api/pkg/apperror"
)

func get(t *testing.T, h *Handler, query string) map[string]any {
	t.Helper()

	rec := call(t, h.List, http.MethodGet, "/suppliers"+query, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body)
	}

	return decode(t, rec)
}

func TestList_Success(t *testing.T) {
	svc := &stubService{page: &supplier.Page{Suppliers: []*domain.Supplier{created()}, Total: 12}}
	h := NewHandler(svc, zap.NewNop())

	body := get(t, h, "?limit=10&offset=5")

	if body["total"] != float64(12) || body["limit"] != float64(10) || body["offset"] != float64(5) {
		t.Errorf("paging: got total %v limit %v offset %v", body["total"], body["limit"], body["offset"])
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data: got %v", body["data"])
	}
	item, _ := data[0].(map[string]any)
	for key, value := range wantData() {
		if item[key] != value {
			t.Errorf("data[0].%s: got %v, want %v", key, item[key], value)
		}
	}
}

func TestList_DefaultsAndFilters(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  supplier.ListFilter
	}{
		{"defaults", "", supplier.ListFilter{Limit: 20}},
		{"custom", "?limit=50&offset=100&name=acme", supplier.ListFilter{Limit: 50, Offset: 100, Name: "acme"}},
		{"bounds", "?limit=1&offset=0", supplier.ListFilter{Limit: 1}},
		{"max limit", "?limit=100", supplier.ListFilter{Limit: 100}},
		{"name is trimmed", "?name=%20acme%20", supplier.ListFilter{Limit: 20, Name: "acme"}},
		{"blank name is ignored", "?name=%20%20", supplier.ListFilter{Limit: 20}},
		{"empty params use defaults", "?limit=&offset=&name=", supplier.ListFilter{Limit: 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &supplier.Page{}}
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
		{"name too long", "?name=" + url.QueryEscape(strings.Repeat("a", 256)), "name is too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{page: &supplier.Page{}}
			h := NewHandler(svc, zap.NewNop())

			rec := call(t, h.List, http.MethodGet, "/suppliers"+tt.query, "", nil)

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

func TestList_MaxLengthNameAccepted(t *testing.T) {
	svc := &stubService{page: &supplier.Page{}}
	h := NewHandler(svc, zap.NewNop())

	get(t, h, "?name="+url.QueryEscape(strings.Repeat("é", 255)))

	if svc.filter.Name != strings.Repeat("é", 255) {
		t.Errorf("name: got %d characters", len([]rune(svc.filter.Name)))
	}
}

func TestList_EmptyPageIsAnEmptyArray(t *testing.T) {
	svc := &stubService{page: &supplier.Page{Total: 3}}
	h := NewHandler(svc, zap.NewNop())

	rec := call(t, h.List, http.MethodGet, "/suppliers?offset=40", "", nil)

	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body: got %s, want an empty array, not null", rec.Body)
	}
	if got := decode(t, rec)["total"]; got != float64(3) {
		t.Errorf("total: got %v, want the real total", got)
	}
}

func TestList_ServiceError(t *testing.T) {
	h := NewHandler(&stubService{err: errors.New("db down")}, zap.NewNop())

	rec := call(t, h.List, http.MethodGet, "/suppliers", "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("error: got %v", got)
	}
}

func getByID(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.GetByID, http.MethodGet, "/suppliers/id", "", map[string]string{"id": id})
}

func TestGetByID_Success(t *testing.T) {
	svc := &stubService{supplier: created()}
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
	for key, value := range wantData() {
		if data[key] != value {
			t.Errorf("data.%s: got %v, want %v", key, data[key], value)
		}
	}
}

func TestGetByID_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "supplier not found"},
		{"wrapped not found", fmt.Errorf("get: %w", apperror.ErrNotFound), http.StatusNotFound, "supplier not found"},
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
