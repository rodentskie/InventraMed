package login

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"go.uber.org/zap"

	"apps/api/internal/service/login"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

const maxBodyBytes = 1 << 20

type Handler struct {
	service login.Service
	log     *zap.Logger
}

func NewHandler(service login.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

type request struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	req, message := decodeRequest(w, r)
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	tokens, err := h.service.Login(r.Context(), req.Email, req.Password)
	if errors.Is(err, apperror.ErrUnauthorized) {
		h.write(response.Error(w, http.StatusUnauthorized, "invalid credentials"))
		return
	}
	if err != nil {
		h.log.Error("login failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
		return
	}

	h.write(response.JSON(w, http.StatusOK, tokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}))
}

// decodeRequest parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeRequest(w http.ResponseWriter, r *http.Request) (request, string) {
	var req request

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return request{}, "invalid request body"
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		return request{}, "email is required"
	}
	if addr, err := mail.ParseAddress(req.Email); err != nil || addr.Address != req.Email {
		return request{}, "email is invalid"
	}
	if req.Password == "" {
		return request{}, "password is required"
	}

	return req, ""
}

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write login response failed", zap.Error(err))
	}
}
