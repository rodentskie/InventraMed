package inventory

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/service/inventory"
	"apps/api/pkg/apperror"
)

func list(t *testing.T, h *Handler, query string) *httptest.ResponseRecorder {
	t.Helper()

	target := "/inventory-entries"
	if query != "" {
		target += "?" + query
	}

	return call(t, h.List, http.MethodGet, target, "", nil)
}

func getByID(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.GetByID, http.MethodGet, "/inventory-entries/"+id, "", map[string]string{"id": id})
}

func TestList_Defaults(t *testing.T) {
	svc := &stubService{page: &inventory.Page{Entries: []*domain.InventoryEntry{created()}, Total: 1}}
	h := NewHandler(svc, zap.NewNop())

	rec := list(t, h, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if svc.filter.Limit != defaultLimit || svc.filter.Offset != 0 {
		t.Errorf("filter: got %+v", svc.filter)
	}

	body := decode(t, rec)
	if body["total"] != float64(1) || body["limit"] != float64(defaultLimit) || body["offset"] != float64(0) {
		t.Errorf("body: got %+v", body)
	}
}

func TestList_CustomLimitAndOffset(t *testing.T) {
	svc := &stubService{page: &inventory.Page{}}
	h := NewHandler(svc, zap.NewNop())

	list(t, h, "limit=5&offset=10")

	if svc.filter.Limit != 5 || svc.filter.Offset != 10 {
		t.Errorf("filter: got %+v", svc.filter)
	}
}

func TestList_EmptyResultIsEmptyArrayNotNull(t *testing.T) {
	svc := &stubService{page: &inventory.Page{}}
	h := NewHandler(svc, zap.NewNop())

	rec := list(t, h, "")

	body := decode(t, rec)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data: got %v (%T), want an array", body["data"], body["data"])
	}
	if len(data) != 0 {
		t.Errorf("data: got %v, want empty", data)
	}
}

func TestList_InvalidLimitAndOffset(t *testing.T) {
	tests := map[string]struct {
		query   string
		message string
	}{
		"non-numeric limit":  {"limit=abc", "limit must be a whole number between 1 and 100"},
		"zero limit":         {"limit=0", "limit must be a whole number between 1 and 100"},
		"limit too large":    {"limit=101", "limit must be a whole number between 1 and 100"},
		"negative offset":    {"offset=-1", "offset must be a whole number of zero or greater"},
		"non-numeric offset": {"offset=abc", "offset must be a whole number of zero or greater"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			h := NewHandler(svc, zap.NewNop())

			rec := list(t, h, tt.query)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400", rec.Code)
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

func TestList_NoCallerIsUnauthorized(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/inventory-entries", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}

func TestList_ServiceError(t *testing.T) {
	svc := &stubService{err: errors.New("boom")}
	h := NewHandler(svc, zap.NewNop())

	rec := list(t, h, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestGetByID_Success(t *testing.T) {
	svc := &stubService{entry: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := getByID(t, h, validMedicineID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if svc.gotID != validMedicineID {
		t.Errorf("id: got %q", svc.gotID)
	}

	body := decode(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok || data["id"] != "entry-1" {
		t.Errorf("data: got %v", body["data"])
	}
}

func TestGetByID_InvalidID(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	rec := getByID(t, h, "not-a-uuid")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if svc.called {
		t.Error("service should not be called for an invalid id")
	}
	if got := decode(t, rec)["error"]; got != "invalid inventory entry id" {
		t.Errorf("message: got %v", got)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc := &stubService{err: apperror.ErrNotFound}
	h := NewHandler(svc, zap.NewNop())

	rec := getByID(t, h, validMedicineID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if got := decode(t, rec)["error"]; got != "inventory entry not found" {
		t.Errorf("message: got %v", got)
	}
}

func TestGetByID_NoCallerIsUnauthorized(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/inventory-entries/"+validMedicineID, nil)
	req.SetPathValue("id", validMedicineID)
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}

func TestGetByID_ServiceError(t *testing.T) {
	svc := &stubService{err: errors.New("boom")}
	h := NewHandler(svc, zap.NewNop())

	rec := getByID(t, h, validMedicineID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
