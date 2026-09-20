package supplier

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/middleware"
	"apps/api/internal/service/supplier"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

const (
	maxBodyBytes      = 1 << 20
	maxNameLength     = 255
	maxContactLength  = 255
	maxEmailLength    = 255
	maxPhoneLength    = 32
	maxAddressLength  = 500
	unauthorizedError = "unauthorized"
)

type Handler struct {
	service supplier.Service
	log     *zap.Logger
}

func NewHandler(service supplier.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// supplierRequest is the body of both create and update; an update replaces
// every field, so an omitted optional field is cleared.
type supplierRequest struct {
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
}

type supplierResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContactName *string   `json:"contact_name"`
	Email       *string   `json:"email"`
	Phone       *string   `json:"phone"`
	Address     *string   `json:"address"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type createResponse struct {
	Message string           `json:"message"`
	Data    supplierResponse `json:"data"`
}

// Create handles POST /suppliers. It must be wrapped by the auth middleware.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	input, message := decodeInput(w, r)
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	created, err := h.service.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}

	h.write(response.JSON(w, http.StatusCreated, createResponse{
		Message: "supplier created",
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
		h.write(response.Error(w, http.StatusNotFound, "supplier not found"))
	case errors.Is(err, apperror.ErrSupplierInPurchaseOrder):
		h.write(response.Error(w, http.StatusConflict, "supplier is used in a purchase order and cannot be deleted"))
	default:
		h.log.Error("supplier request failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
	}
}

// decodeInput parses and validates the create and update body. It returns a
// non-empty validation message when the request is invalid.
func decodeInput(w http.ResponseWriter, r *http.Request) (supplier.CreateInput, string) {
	var req supplierRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return supplier.CreateInput{}, "invalid request body"
	}

	name := strings.TrimSpace(req.Name)
	contact := strings.TrimSpace(req.ContactName)
	email := strings.TrimSpace(req.Email)
	phone := strings.TrimSpace(req.Phone)
	address := strings.TrimSpace(req.Address)

	if message := validate(name, contact, email, phone, address); message != "" {
		return supplier.CreateInput{}, message
	}

	return supplier.CreateInput{
		Name:        name,
		ContactName: optional(contact),
		Email:       optional(email),
		Phone:       optional(phone),
		Address:     optional(address),
	}, ""
}

// validate checks the trimmed fields. It returns a non-empty validation
// message when one is invalid.
func validate(name, contact, email, phone, address string) string {
	switch {
	case name == "":
		return "name is required"
	case tooLong(name, maxNameLength):
		return "name is too long"
	case tooLong(contact, maxContactLength):
		return "contact_name is too long"
	case tooLong(email, maxEmailLength):
		return "email is too long"
	case email != "" && !validEmail(email):
		return "email must be a valid email address"
	case tooLong(phone, maxPhoneLength):
		return "phone is too long"
	case tooLong(address, maxAddressLength):
		return "address is too long"
	}

	return ""
}

// validEmail reports whether value is a bare address. mail.ParseAddress also
// accepts a display name ("Jane <jane@x.com>"), which is not an address.
func validEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)

	return err == nil && parsed.Address == value
}

func tooLong(value string, limit int) bool {
	return utf8.RuneCountInString(value) > limit
}

// optional returns nil for an empty value, so it is stored as NULL.
func optional(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func toResponse(s *domain.Supplier) supplierResponse {
	return supplierResponse{
		ID:          s.ID,
		Name:        s.Name,
		ContactName: s.ContactName,
		Email:       s.Email,
		Phone:       s.Phone,
		Address:     s.Address,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write supplier response failed", zap.Error(err))
	}
}
