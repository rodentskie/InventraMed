package supplier

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
	"apps/api/internal/service/supplier"
	"apps/api/pkg/apperror"
)

var secret = []byte("test-secret")

const validID = "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11"

// stubService records what the handler passed and returns the canned results.
type stubService struct {
	supplier *domain.Supplier
	page     *supplier.Page
	err      error

	input  supplier.CreateInput
	filter supplier.ListFilter
	id     string
	called bool
}

func (s *stubService) Create(_ context.Context, input supplier.CreateInput) (*domain.Supplier, error) {
	s.called = true
	s.input = input

	return s.supplier, s.err
}

func (s *stubService) List(_ context.Context, filter supplier.ListFilter) (*supplier.Page, error) {
	s.called = true
	s.filter = filter

	return s.page, s.err
}

func (s *stubService) GetByID(_ context.Context, id string) (*domain.Supplier, error) {
	s.called = true
	s.id = id

	return s.supplier, s.err
}

func (s *stubService) Update(_ context.Context, id string, input supplier.UpdateInput) error {
	s.called = true
	s.id = id
	s.input = input

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
	"name": "Acme Pharma Distribution",
	"contact_name": "Jane Cruz",
	"email": "jane@acmepharma.example",
	"phone": "+63 917 123 4567",
	"address": "123 Industrial Ave, Quezon City"
}`

func str(value string) *string {
	return &value
}

func created() *domain.Supplier {
	return &domain.Supplier{
		ID:          "sup-1",
		Name:        "Acme Pharma Distribution",
		ContactName: str("Jane Cruz"),
		Email:       str("jane@acmepharma.example"),
		Phone:       str("+63 917 123 4567"),
		Address:     str("123 Industrial Ave, Quezon City"),
		CreatedAt:   time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 9, 20, 8, 15, 30, 0, time.UTC),
	}
}

func wantData() map[string]any {
	return map[string]any{
		"id":           "sup-1",
		"name":         "Acme Pharma Distribution",
		"contact_name": "Jane Cruz",
		"email":        "jane@acmepharma.example",
		"phone":        "+63 917 123 4567",
		"address":      "123 Industrial Ave, Quezon City",
		"created_at":   "2026-09-20T08:15:30Z",
		"updated_at":   "2026-09-20T08:15:30Z",
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

	return call(t, h.Create, http.MethodPost, "/suppliers", body, nil)
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
	svc := &stubService{supplier: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}

	body := decode(t, rec)
	if body["message"] != "supplier created" {
		t.Errorf("message: got %v", body["message"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data: got %v", body["data"])
	}
	for key, value := range wantData() {
		if data[key] != value {
			t.Errorf("data.%s: got %v, want %v", key, data[key], value)
		}
	}
}

func TestCreate_PassesValidatedInputToService(t *testing.T) {
	svc := &stubService{supplier: created()}
	h := NewHandler(svc, zap.NewNop())

	post(t, h, `{
		"name": "  Acme Pharma Distribution ",
		"contact_name": " Jane Cruz\n",
		"email": " jane@acmepharma.example ",
		"phone": " +63 917 123 4567 ",
		"address": " 123 Industrial Ave, Quezon City "
	}`)

	in := svc.input
	if in.Name != "Acme Pharma Distribution" {
		t.Errorf("name: got %q, want trimmed", in.Name)
	}
	got := map[string]*string{
		"contact_name": in.ContactName, "email": in.Email, "phone": in.Phone, "address": in.Address,
	}
	want := map[string]string{
		"contact_name": "Jane Cruz",
		"email":        "jane@acmepharma.example",
		"phone":        "+63 917 123 4567",
		"address":      "123 Industrial Ave, Quezon City",
	}
	for field, value := range want {
		if got[field] == nil || *got[field] != value {
			t.Errorf("%s: got %v, want %q trimmed", field, got[field], value)
		}
	}
}

func TestCreate_EmptyOptionalFieldsAreNil(t *testing.T) {
	for name, optional := range map[string]string{
		"missing": ``,
		"empty":   `"contact_name":"","email":"","phone":"","address":"",`,
		"blank":   `"contact_name":" ","email":" ","phone":"\t","address":"\n",`,
		"null":    `"contact_name":null,"email":null,"phone":null,"address":null,`,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{supplier: created()}
			h := NewHandler(svc, zap.NewNop())

			rec := post(t, h, fmt.Sprintf(`{%s"name":"Acme"}`, optional))

			if rec.Code != http.StatusCreated {
				t.Fatalf("status: got %d (body %s)", rec.Code, rec.Body)
			}
			in := svc.input
			if in.ContactName != nil || in.Email != nil || in.Phone != nil || in.Address != nil {
				t.Errorf("optional fields: got %+v, want all nil", in)
			}
		})
	}
}

func TestCreate_NullFieldsAreEncodedAsNull(t *testing.T) {
	svc := &stubService{supplier: &domain.Supplier{ID: "sup-1", Name: "Acme"}}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, `{"name":"Acme"}`)

	data, _ := decode(t, rec)["data"].(map[string]any)
	for _, field := range []string{"contact_name", "email", "phone", "address"} {
		value, present := data[field]
		if !present || value != nil {
			t.Errorf("data.%s: got %v (present %v), want null", field, value, present)
		}
	}
}

func longString(n int) string {
	return strings.Repeat("a", n)
}

// invalidBodies are the body cases both create and update reject with a 400.
func invalidBodies() []struct {
	name string
	body string
	want string
} {
	return []struct {
		name string
		body string
		want string
	}{
		{"malformed json", `{`, "invalid request body"},
		{"empty body", ``, "invalid request body"},
		{"wrong type", `{"name": 5}`, "invalid request body"},
		{"too large", `{"name":"` + strings.Repeat("a", 1<<20) + `"}`, "invalid request body"},
		{"missing name", `{"email":"a@b.com"}`, "name is required"},
		{"blank name", `{"name":"  "}`, "name is required"},
		{"long name", fmt.Sprintf(`{"name":%q}`, longString(256)), "name is too long"},
		{"long contact name", fmt.Sprintf(`{"name":"A","contact_name":%q}`, longString(256)), "contact_name is too long"},
		{"long email", fmt.Sprintf(`{"name":"A","email":%q}`, longString(250)+"@b.com"), "email is too long"},
		{"email without at sign", `{"name":"A","email":"jane"}`, "email must be a valid email address"},
		{"email without domain", `{"name":"A","email":"jane@"}`, "email must be a valid email address"},
		{"email with display name", `{"name":"A","email":"Jane <jane@acme.example>"}`, "email must be a valid email address"},
		{"long phone", fmt.Sprintf(`{"name":"A","phone":%q}`, longString(33)), "phone is too long"},
		{"long address", fmt.Sprintf(`{"name":"A","address":%q}`, longString(501)), "address is too long"},
	}
}

func TestCreate_BadRequest(t *testing.T) {
	for _, tt := range invalidBodies() {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{supplier: created()}
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
	svc := &stubService{supplier: created()}
	h := NewHandler(svc, zap.NewNop())

	rec := post(t, h, fmt.Sprintf(
		`{"name":%q,"contact_name":%q,"email":%q,"phone":%q,"address":%q}`,
		strings.Repeat("é", 255), strings.Repeat("é", 255), strings.Repeat("a", 243)+"@example.com",
		strings.Repeat("1", 32), strings.Repeat("é", 500),
	))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body)
	}
}

func TestNoCallerInContext(t *testing.T) {
	handlers := map[string]func(*Handler) http.HandlerFunc{
		"create":    func(h *Handler) http.HandlerFunc { return h.Create },
		"list":      func(h *Handler) http.HandlerFunc { return h.List },
		"get by id": func(h *Handler) http.HandlerFunc { return h.GetByID },
		"update":    func(h *Handler) http.HandlerFunc { return h.Update },
		"delete":    func(h *Handler) http.HandlerFunc { return h.Delete },
	}

	for name, pick := range handlers {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{supplier: created(), page: &supplier.Page{}}
			handler := pick(NewHandler(svc, zap.NewNop()))

			req := httptest.NewRequest(http.MethodPost, "/suppliers", strings.NewReader(validBody))
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
	h := NewHandler(&stubService{supplier: created()}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/suppliers", strings.NewReader(validBody))
	req.Header.Set("Authorization", "Bearer "+accessToken(t))

	// Must not panic when the response can't be written.
	middleware.Auth(secret, zap.NewNop())(h.Create)(&brokenWriter{}, req)
}
