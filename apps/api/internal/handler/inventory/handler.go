package inventory

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
	"apps/api/internal/service/inventory"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

const (
	maxBodyBytes      = 1 << 20
	maxReasonLength   = 100
	maxNotesLength    = 500
	unauthorizedError = "unauthorized"
)

type Handler struct {
	service inventory.Service
	log     *zap.Logger
}

func NewHandler(service inventory.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

type createRequest struct {
	MedicineID string `json:"medicine_id"`
	Direction  string `json:"direction"`
	Quantity   *int   `json:"quantity"`
	Reason     string `json:"reason"`
	Notes      string `json:"notes"`
}

type entryResponse struct {
	ID         string    `json:"id"`
	MedicineID string    `json:"medicine_id"`
	Direction  string    `json:"direction"`
	Quantity   int       `json:"quantity"`
	Reason     string    `json:"reason"`
	CountedBy  string    `json:"counted_by"`
	Notes      *string   `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

type createResponse struct {
	Message string        `json:"message"`
	Data    entryResponse `json:"data"`
}

// Create handles POST /inventory-entries. It must be wrapped by the auth
// middleware.
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
	input.CountedBy = account.UserID

	created, err := h.service.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, err, "medicine not found")
		return
	}

	h.write(response.JSON(w, http.StatusCreated, createResponse{
		Message: "inventory entry recorded",
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

// writeError maps err to a response. notFoundMessage is used for
// apperror.ErrNotFound, since the same error means different things
// depending on the caller (an unknown medicine_id on create, an unknown
// entry on get).
func (h *Handler) writeError(w http.ResponseWriter, err error, notFoundMessage string) {
	switch {
	case errors.Is(err, apperror.ErrInsufficientQuantity):
		h.write(response.Error(w, http.StatusConflict, "insufficient quantity for subtraction"))
	case errors.Is(err, apperror.ErrNotFound):
		h.write(response.Error(w, http.StatusNotFound, notFoundMessage))
	default:
		h.log.Error("inventory entry request failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
	}
}

// decodeCreate parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeCreate(w http.ResponseWriter, r *http.Request) (inventory.CreateInput, string) {
	var req createRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return inventory.CreateInput{}, "invalid request body"
	}

	medicineID := strings.TrimSpace(req.MedicineID)
	direction := strings.TrimSpace(req.Direction)
	reason := strings.TrimSpace(req.Reason)
	notes := strings.TrimSpace(req.Notes)

	if message := validateCreate(medicineID, direction, req.Quantity, reason, notes); message != "" {
		return inventory.CreateInput{}, message
	}

	input := inventory.CreateInput{
		MedicineID: medicineID,
		Direction:  direction,
		Quantity:   *req.Quantity,
		Reason:     reason,
	}
	if notes != "" {
		input.Notes = &notes
	}

	return input, ""
}

func validateCreate(medicineID, direction string, quantity *int, reason, notes string) string {
	switch {
	case medicineID == "":
		return "medicine_id is required"
	case !isUUID(medicineID):
		return "invalid medicine_id"
	case direction == "":
		return "direction is required"
	case direction != domain.DirectionAddition && direction != domain.DirectionSubtraction:
		return "direction must be addition or subtraction"
	case quantity == nil:
		return "quantity is required"
	case *quantity <= 0:
		return "quantity must be greater than zero"
	case reason == "":
		return "reason is required"
	case tooLong(reason, maxReasonLength):
		return "reason is too long"
	case tooLong(notes, maxNotesLength):
		return "notes is too long"
	}

	return ""
}

func tooLong(value string, limit int) bool {
	return utf8.RuneCountInString(value) > limit
}

// isUUID reports whether value is a UUID in its canonical 8-4-4-4-12 form.
func isUUID(value string) bool {
	const length = 36

	if len(value) != length {
		return false
	}

	for i, c := range value {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHexDigit(c) {
				return false
			}
		}
	}

	return true
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func toResponse(e *domain.InventoryEntry) entryResponse {
	return entryResponse{
		ID:         e.ID,
		MedicineID: e.MedicineID,
		Direction:  e.Direction,
		Quantity:   e.Quantity,
		Reason:     e.Reason,
		CountedBy:  e.CountedBy,
		Notes:      e.Notes,
		CreatedAt:  e.CreatedAt,
	}
}

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write inventory entry response failed", zap.Error(err))
	}
}
