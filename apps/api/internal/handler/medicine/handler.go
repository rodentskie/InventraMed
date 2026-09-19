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

type createRequest struct {
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
	account, ok := h.caller(w, r)
	if !ok {
		return
	}

	input, message := decodeCreate(w, r)
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

// caller returns the authenticated account. When there is none, which means
// the route was registered without the auth middleware, it writes a 401 and
// returns false, so the handler fails closed.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (domain.AccountPayload, bool) {
	account, ok := middleware.AccountFromContext(r.Context())
	if !ok {
		h.write(response.Error(w, http.StatusUnauthorized, unauthorizedError))
	}

	return account, ok
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		h.write(response.Error(w, http.StatusNotFound, "medicine not found"))
	case errors.Is(err, apperror.ErrNameBatchExists):
		h.write(response.Error(w, http.StatusConflict, "medicine with this name and batch number already exists"))
	case errors.Is(err, apperror.ErrBarcodeExists):
		h.write(response.Error(w, http.StatusConflict, "medicine with this barcode already exists"))
	case errors.Is(err, apperror.ErrMedicineInPurchaseOrder):
		h.write(response.Error(w, http.StatusConflict, "medicine is used in a purchase order and cannot be deleted"))
	default:
		h.log.Error("medicine request failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
	}
}

// decodeCreate parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeCreate(w http.ResponseWriter, r *http.Request) (medicine.CreateInput, string) {
	var req createRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return medicine.CreateInput{}, "invalid request body"
	}

	d, message := validateDetails(req.Name, req.Barcode, req.BatchNumber, req.ExpirationDate)
	if message != "" {
		return medicine.CreateInput{}, message
	}

	if req.Quantity == nil {
		return medicine.CreateInput{}, "quantity is required"
	}
	if *req.Quantity < 0 {
		return medicine.CreateInput{}, "quantity must be zero or greater"
	}

	return medicine.CreateInput{
		Name:           d.name,
		Barcode:        d.barcode,
		BatchNumber:    d.batchNumber,
		ExpirationDate: d.expirationDate,
		Quantity:       *req.Quantity,
	}, ""
}

// details is the validated data create and update have in common.
type details struct {
	name           string
	barcode        string
	batchNumber    *string
	expirationDate time.Time
}

// validateDetails trims and validates the fields shared by create and update.
// It returns a non-empty validation message when one is invalid.
func validateDetails(name, barcode, batch, expiration string) (details, string) {
	name = strings.TrimSpace(name)
	barcode = strings.TrimSpace(barcode)
	batch = strings.TrimSpace(batch)

	if message := validateText(name, barcode, batch); message != "" {
		return details{}, message
	}

	date, message := parseExpirationDate(expiration)
	if message != "" {
		return details{}, message
	}

	d := details{name: name, barcode: barcode, expirationDate: date}
	if batch != "" {
		d.batchNumber = &batch
	}

	return d, ""
}

func validateText(name, barcode, batch string) string {
	switch {
	case name == "":
		return "name is required"
	case tooLong(name, maxNameLength):
		return "name is too long"
	}

	if message := validateBarcode(barcode); message != "" {
		return message
	}

	if tooLong(batch, maxBatchLength) {
		return "batch_number is too long"
	}

	return ""
}

func validateBarcode(barcode string) string {
	switch {
	case barcode == "":
		return "barcode is required"
	case tooLong(barcode, maxBarcodeLength):
		return "barcode is too long"
	}

	return ""
}

func tooLong(value string, limit int) bool {
	return utf8.RuneCountInString(value) > limit
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
