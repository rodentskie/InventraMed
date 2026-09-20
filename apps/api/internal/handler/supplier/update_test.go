package supplier

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"apps/api/pkg/apperror"
)

func put(t *testing.T, h *Handler, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Update, http.MethodPut, "/suppliers/id", body, map[string]string{"id": id})
}

func remove(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Delete, http.MethodDelete, "/suppliers/id", "", map[string]string{"id": id})
}

func TestUpdate_Success(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	rec := put(t, h, validID, validBody)

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
	if svc.input.Name != "Acme Pharma Distribution" || svc.input.Email == nil || *svc.input.Email != "jane@acmepharma.example" {
		t.Errorf("input passed to the service: got %+v", svc.input)
	}
}

func TestUpdate_OmittedOptionalFieldsAreCleared(t *testing.T) {
	svc := &stubService{}
	h := NewHandler(svc, zap.NewNop())

	rec := put(t, h, validID, `{"name":"Acme"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
	}
	in := svc.input
	if in.ContactName != nil || in.Email != nil || in.Phone != nil || in.Address != nil {
		t.Errorf("optional fields: got %+v, want all nil", in)
	}
}

func TestUpdate_BadRequest(t *testing.T) {
	for _, tt := range invalidBodies() {
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
		"too short":        "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e1",
		"too long":         "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e111",
		"no hyphens":       "b1a2c3d46a1e4c3e9d0e5f1d2a7c9e11xxxx",
		"misplaced hyphen": "b1a2c3d4x6a1e-4c3e-9d0e-5f1d2a7c9e11",
		"non hex":          "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e1g",
		"non ascii":        "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e1é",
		"braces":           "{b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e1}",
	}

	actions := map[string]func(*testing.T, *Handler, string) *httptest.ResponseRecorder{
		"get":    getByID,
		"update": func(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder { return put(t, h, id, validBody) },
		"delete": remove,
	}

	for name, id := range ids {
		for action, run := range actions {
			t.Run(action+" "+name, func(t *testing.T) {
				svc := &stubService{}
				h := NewHandler(svc, zap.NewNop())

				rec := run(t, h, id)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
				}
				if got := decode(t, rec)["error"]; got != "invalid supplier id" {
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
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "supplier not found"},
		{"wrapped not found", fmt.Errorf("update: %w", apperror.ErrNotFound), http.StatusNotFound, "supplier not found"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := put(t, h, validID, validBody)

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
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "supplier not found"},
		{"in a purchase order", apperror.ErrSupplierInPurchaseOrder, http.StatusConflict, "supplier is used in a purchase order and cannot be deleted"},
		{"wrapped in a purchase order", fmt.Errorf("delete: %w", apperror.ErrSupplierInPurchaseOrder), http.StatusConflict, "supplier is used in a purchase order and cannot be deleted"},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
		{"bare conflict is not a known one", apperror.ErrConflict, http.StatusInternalServerError, "internal server error"},
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
