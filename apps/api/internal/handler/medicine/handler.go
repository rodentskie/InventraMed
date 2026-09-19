package medicine

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/middleware"
	"apps/api/internal/service/medicine"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

const (
	maxBodyBytes      = 1 << 20
	maxNameLength     = 255
	maxBarcodeLength  = 128
	maxBatchLength    = 64
	unauthorizedError = "unauthorized"
)

type Handler struct {
	service medicine.Service
	log     *zap.Logger
}

func NewHandler(service medicine.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

type request struct {
	Name           string `json:"name"`
	Barcode        string `json:"barcode"`
	BatchNumber    string `json:"batch_number"`
	ExpirationDate string `json:"expiration_date"`
	Quantity       *int   `json:"quantity"`
}

type medicineResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Barcode        string    `json:"barcode"`
	BatchNumber    *string   `json:"batch_number"`
	ExpirationDate string    `json:"expiration_date"`
	Quantity       int       `json:"quantity"`
	CreatedBy      *string   `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type createResponse struct {
	Message string           `json:"message"`
	Data    medicineResponse `json:"data"`
}

// Create handles POST /medicines. It must be wrapped by the auth middleware.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	account, ok := middleware.AccountFromContext(r.Context())
	if !ok {
		h.write(response.Error(w, http.StatusUnauthorized, unauthorizedError))
		return
	}

	input, message := decodeInput(w, r)
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}
	input.CreatedBy = account.UserID

	created, err := h.service.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}

	h.write(response.JSON(w, http.StatusCreated, createResponse{
		Message: "medicine created",
		Data:    toResponse(created),
	}))
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apperror.ErrNameBatchExists):
		h.write(response.Error(w, http.StatusConflict, "medicine with this name and batch number already exists"))
	case errors.Is(err, apperror.ErrBarcodeExists):
		h.write(response.Error(w, http.StatusConflict, "medicine with this barcode already exists"))
	default:
		h.log.Error("create medicine failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
	}
}

// decodeInput parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeInput(w http.ResponseWriter, r *http.Request) (medicine.CreateInput, string) {
	var req request

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return medicine.CreateInput{}, "invalid request body"
	}

	return validate(req)
}

func validate(req request) (medicine.CreateInput, string) {
	name := strings.TrimSpace(req.Name)
	barcode := strings.TrimSpace(req.Barcode)
	batch := strings.TrimSpace(req.BatchNumber)

	if message := validateText(name, barcode, batch); message != "" {
		return medicine.CreateInput{}, message
	}

	expiration, message := parseExpirationDate(req.ExpirationDate)
	if message != "" {
		return medicine.CreateInput{}, message
	}

	if req.Quantity == nil {
		return medicine.CreateInput{}, "quantity is required"
	}
	if *req.Quantity < 0 {
		return medicine.CreateInput{}, "quantity must be zero or greater"
	}

	input := medicine.CreateInput{
		Name:           name,
		Barcode:        barcode,
		ExpirationDate: expiration,
		Quantity:       *req.Quantity,
	}
	if batch != "" {
		input.BatchNumber = &batch
	}

	return input, ""
}

func validateText(name, barcode, batch string) string {
	switch {
	case name == "":
		return "name is required"
	case utf8.RuneCountInString(name) > maxNameLength:
		return "name is too long"
	case barcode == "":
		return "barcode is required"
	case utf8.RuneCountInString(barcode) > maxBarcodeLength:
		return "barcode is too long"
	case utf8.RuneCountInString(batch) > maxBatchLength:
		return "batch_number is too long"
	}

	return ""
}

func parseExpirationDate(value string) (time.Time, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, "expiration_date is required"
	}

	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, "expiration_date must be a valid date in YYYY-MM-DD format"
	}

	return date, ""
}

func toResponse(m *domain.Medicine) medicineResponse {
	return medicineResponse{
		ID:             m.ID,
		Name:           m.Name,
		Barcode:        m.Barcode,
		BatchNumber:    m.BatchNumber,
		ExpirationDate: m.ExpirationDate.Format(time.DateOnly),
		Quantity:       m.Quantity,
		CreatedBy:      m.CreatedBy,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write medicine response failed", zap.Error(err))
	}
}
