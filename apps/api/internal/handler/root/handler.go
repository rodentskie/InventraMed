package root

import (
	"net/http"

	"go.uber.org/zap"

	"apps/api/internal/service/root"
	"apps/api/pkg/response"
)

type Handler struct {
	service root.Service
	log     *zap.Logger
}

func NewHandler(service root.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	payload := map[string]string{"message": h.service.Greeting()}

	if err := response.JSON(w, http.StatusOK, payload); err != nil {
		h.log.Error("write root response failed", zap.Error(err))
	}
}
