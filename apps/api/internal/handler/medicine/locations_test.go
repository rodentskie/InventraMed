package medicine

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/service/medicine"
)

// locations calls the handler without the auth middleware, as it is
// registered: the route is public.
func locations(h *Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/medicines/locations", nil)
	rec := httptest.NewRecorder()

	h.Locations(rec, req)

	return rec
}

func TestLocations_Success(t *testing.T) {
	svc := &stubService{locations: []medicine.LocationStatus{
		{Location: 2, Status: domain.ExpirationStatusExpire},
		{Location: 7, Status: domain.ExpirationStatusNear},
		{Location: 12, Status: domain.ExpirationStatusGood},
	}}

	rec := locations(NewHandler(svc, zap.NewNop()))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body)
	}
	want := `[{"type":"http","location":2,"status":"expire"},` +
		`{"type":"http","location":7,"status":"near"},` +
		`{"type":"http","location":12,"status":"good"}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body:\n got %s\nwant %s", got, want)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type: got %q", got)
	}
}

func TestLocations_EmptyTrayIsAnEmptyArray(t *testing.T) {
	rec := locations(NewHandler(&stubService{}, zap.NewNop()))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body: got %s, want []", got)
	}
}

func TestLocations_ServiceError(t *testing.T) {
	rec := locations(NewHandler(&stubService{err: errors.New("db down")}, zap.NewNop()))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("error: got %v", got)
	}
}
