package swagger

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

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

func TestHandlerRedirect(t *testing.T) {
	handler := NewHandler(zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	rec := httptest.NewRecorder()

	handler.Redirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != uiIndexPath {
		t.Fatalf("expected redirect to %q, got %q", uiIndexPath, got)
	}
}

func TestHandlerUI(t *testing.T) {
	handler := NewHandler(zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, uiIndexPath, nil)
	rec := httptest.NewRecorder()

	handler.UI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	// The index template JS-escapes slashes in the configured URL.
	escapedSpecPath := strings.ReplaceAll(specPath, "/", `\/`)
	if !strings.Contains(rec.Body.String(), escapedSpecPath) {
		t.Fatalf("expected index page to load %q", specPath)
	}
}

func TestHandlerSpec(t *testing.T) {
	handler := NewHandler(zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, specPath, nil)
	rec := httptest.NewRecorder()

	handler.Spec(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var doc struct {
		OpenAPI string                     `json:"openapi"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	if doc.OpenAPI == "" {
		t.Fatal("expected an openapi version")
	}
	for _, path := range []string{"/", "/login"} {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("expected spec to document %q", path)
		}
	}
}

func TestHandlerSpecLogsWriteFailure(t *testing.T) {
	handler := NewHandler(zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, specPath, nil)

	handler.Spec(&brokenWriter{}, req)
}
