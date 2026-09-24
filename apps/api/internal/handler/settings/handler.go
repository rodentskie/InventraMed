package settings

import (
	"errors"
	"net/http"

	"go.uber.org/zap"

	"apps/api/internal/service/settings"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

type Handler struct {
	service settings.Service
	log     *zap.Logger
}

func NewHandler(service settings.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

type settingsResponse struct {
	WarningThresholdDays int `json:"warning_threshold_days"`
}

type getResponse struct {
	Data settingsResponse `json:"data"`
}

// Get handles GET /settings. Public: registered without the auth middleware,
// because the scanner page needs the warning threshold without a logged-in
// session. The settings hold nothing sensitive.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.Get(r.Context())
	if errors.Is(err, apperror.ErrNotFound) {
		h.write(response.Error(w, http.StatusNotFound, "settings not found"))
		return
	}
	if err != nil {
		h.log.Error("settings request failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
		return
	}

	h.write(response.JSON(w, http.StatusOK, getResponse{
		Data: settingsResponse{WarningThresholdDays: found.WarningThresholdDays},
	}))
}

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write settings response failed", zap.Error(err))
	}
}
