package swagger

import (
	_ "embed"
	"encoding/json"
	"net/http"

	httpswagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"

	"apps/api/pkg/response"
)

const (
	uiIndexPath = "/swagger/index.html"
	specPath    = "/swagger/openapi.json"
)

//go:embed openapi.json
var spec []byte

type Handler struct {
	ui  http.HandlerFunc
	log *zap.Logger
}

func NewHandler(log *zap.Logger) *Handler {
	return &Handler{
		ui:  httpswagger.Handler(httpswagger.URL(specPath)),
		log: log,
	}
}

// Redirect sends GET /swagger to the Swagger UI, whose assets are resolved
// relative to /swagger/.
func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, uiIndexPath, http.StatusFound)
}

// UI serves the Swagger UI page and its static assets under /swagger/.
func (h *Handler) UI(w http.ResponseWriter, r *http.Request) {
	h.ui(w, r)
}

// Spec serves the OpenAPI document that the Swagger UI renders.
func (h *Handler) Spec(w http.ResponseWriter, r *http.Request) {
	if err := response.JSON(w, http.StatusOK, json.RawMessage(spec)); err != nil {
		h.log.Error("write swagger spec failed", zap.Error(err))
	}
}
