package purchaseorder

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/middleware"
	"apps/api/pkg/apperror"
)

func receive(t *testing.T, h *Handler, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	return call(t, h.Receive, http.MethodPost, "/purchase-orders/id/receive", body, map[string]string{"id": id})
}

func TestReceive_Success(t *testing.T) {
	svc := &stubService{receipt: receipt()}
	h := NewHandler(svc, zap.NewNop())

	rec := receive(t, h, validID, `{"notes": "Delivered by courier"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}

	decoded := decode(t, rec)
	if decoded["message"] != "purchase order received" {
		t.Errorf("message: got %v", decoded["message"])
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("data: got %v", decoded["data"])
	}
	assertFields(t, "data", data, wantReceipt())

	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items: got %v", data["items"])
	}
	first, _ := items[0].(map[string]any)
	assertFields(t, "items[0]", first, map[string]any{
		"id":                     "ri-1",
		"purchase_order_item_id": "item-1",
		"quantity_received":      float64(200),
		"quantity_damaged":       float64(0),
		"quantity_returned":      float64(0),
		"notes":                  nil,
		"created_at":             "2026-09-20T08:15:30Z",
	})
}

func TestReceive_PassesInputToService(t *testing.T) {
	svc := &stubService{receipt: receipt()}
	h := NewHandler(svc, zap.NewNop())

	receive(t, h, validID, `{"notes": "  Delivered by courier "}`)

	in := svc.receiveInput
	if in.PurchaseOrderID != validID {
		t.Errorf("purchase order id: got %q", in.PurchaseOrderID)
	}
	if in.Notes == nil || *in.Notes != "Delivered by courier" {
		t.Errorf("notes: got %v, want trimmed", in.Notes)
	}
	if in.ReceivedBy != "user-1" {
		t.Errorf("received by: got %q, want the caller from the token", in.ReceivedBy)
	}
}

func TestReceive_BodyIsOptional(t *testing.T) {
	for name, body := range map[string]string{
		"no body":       ``,
		"whitespace":    "  \n",
		"empty object":  `{}`,
		"empty notes":   `{"notes": ""}`,
		"blank notes":   `{"notes": "   "}`,
		"null notes":    `{"notes": null}`,
		"unknown field": `{"other": 1}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{receipt: receipt()}
			h := NewHandler(svc, zap.NewNop())

			rec := receive(t, h, validID, body)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
			}
			if svc.receiveInput.Notes != nil {
				t.Errorf("notes: got %q, want nil", *svc.receiveInput.Notes)
			}
		})
	}
}

func TestReceive_ReceivedByInBodyIsIgnored(t *testing.T) {
	svc := &stubService{receipt: receipt()}
	h := NewHandler(svc, zap.NewNop())

	receive(t, h, validID, `{"received_by": "attacker"}`)

	if svc.receiveInput.ReceivedBy != "user-1" {
		t.Errorf("received by: got %q, want the caller from the token", svc.receiveInput.ReceivedBy)
	}
}

func TestReceive_MaxNotesAccepted(t *testing.T) {
	svc := &stubService{receipt: receipt()}
	h := NewHandler(svc, zap.NewNop())

	rec := receive(t, h, validID, fmt.Sprintf(`{"notes": %q}`, strings.Repeat("é", 500)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
	}
}

func TestReceive_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed json", `{`, "invalid request body"},
		{"wrong type", `{"notes": 5}`, "invalid request body"},
		{"too large", `{"notes":"` + strings.Repeat("a", 1<<20) + `"}`, "invalid request body"},
		{"long notes", fmt.Sprintf(`{"notes": %q}`, strings.Repeat("a", 501)), "notes is too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{receipt: receipt()}
			h := NewHandler(svc, zap.NewNop())

			rec := receive(t, h, validID, tt.body)

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
		"too short":        "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e1",
		"too long":         "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e111",
		"no hyphens":       "e4f1a7b26a1e4c3e9d0e5f1d2a7c9e11xxxx",
		"misplaced hyphen": "e4f1a7b2x6a1e-4c3e-9d0e-5f1d2a7c9e11",
		"non hex":          "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e1g",
		"non ascii":        "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e1é",
		"braces":           "{e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e1}",
	}

	actions := map[string]func(*testing.T, *Handler, string) *httptest.ResponseRecorder{
		"get":     getByID,
		"receive": func(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder { return receive(t, h, id, `{}`) },
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
				if got := decode(t, rec)["error"]; got != "invalid purchase order id" {
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
	svc := &stubService{receipt: receipt()}
	h := NewHandler(svc, zap.NewNop())
	id := strings.ToUpper(validID)

	rec := receive(t, h, id, `{}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
	if svc.receiveInput.PurchaseOrderID != id {
		t.Errorf("id passed to the service: got %q", svc.receiveInput.PurchaseOrderID)
	}
}

func TestReceive_ServiceErrors(t *testing.T) {
	const notReceivable = "purchase order cannot be received in its current status"

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound, "purchase order not found"},
		{"not receivable", apperror.ErrPurchaseOrderNotReceivable, http.StatusConflict, notReceivable},
		{"wrapped not receivable", fmt.Errorf("receive: %w", apperror.ErrPurchaseOrderNotReceivable), http.StatusConflict, notReceivable},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError, "internal server error"},
		{"bare conflict is not a known one", apperror.ErrConflict, http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&stubService{err: tt.err}, zap.NewNop())

			rec := receive(t, h, validID, `{}`)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode(t, rec)["error"]; got != tt.wantError {
				t.Errorf("error: got %v, want %q", got, tt.wantError)
			}
		})
	}
}

func TestReceive_WriteFailure(t *testing.T) {
	h := NewHandler(&stubService{receipt: receipt()}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/purchase-orders/id/receive", strings.NewReader(`{}`))
	req.SetPathValue("id", validID)
	req.Header.Set("Authorization", "Bearer "+accessToken(t))

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Receive)(&brokenWriter{}, req)
}
