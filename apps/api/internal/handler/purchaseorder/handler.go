package purchaseorder

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/middleware"
	"apps/api/internal/service/purchaseorder"
	"apps/api/pkg/apperror"
	"apps/api/pkg/response"
)

const (
	maxBodyBytes      = 1 << 20
	maxNotesLength    = 500
	maxItems          = 100
	maxQuantity       = 1<<31 - 1
	unauthorizedError = "unauthorized"
)

type Handler struct {
	service purchaseorder.Service
	log     *zap.Logger
}

func NewHandler(service purchaseorder.Service, log *zap.Logger) *Handler {
	return &Handler{service: service, log: log}
}

type createRequest struct {
	SupplierID   string        `json:"supplier_id"`
	OrderDate    string        `json:"order_date"`
	ExpectedDate string        `json:"expected_date"`
	Notes        string        `json:"notes"`
	Items        []itemRequest `json:"items"`
}

type itemRequest struct {
	MedicineID      string `json:"medicine_id"`
	QuantityOrdered *int   `json:"quantity_ordered"`
}

type createResponse struct {
	Message string         `json:"message"`
	Data    detailResponse `json:"data"`
}

// Create handles POST /purchase-orders. It must be wrapped by the auth middleware.
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
		Message: "purchase order created",
		Data:    toDetailResponse(created),
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

// writeError maps a service error to a response. The supplier and medicine
// errors wrap ErrNotFound, so they are matched first.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apperror.ErrSupplierNotFound):
		h.write(response.Error(w, http.StatusNotFound, "supplier not found"))
	case errors.Is(err, apperror.ErrMedicineNotFound):
		h.write(response.Error(w, http.StatusNotFound, "medicine not found"))
	case errors.Is(err, apperror.ErrNotFound):
		h.write(response.Error(w, http.StatusNotFound, "purchase order not found"))
	case errors.Is(err, apperror.ErrPurchaseOrderNotReceivable):
		h.write(response.Error(w, http.StatusConflict, "purchase order cannot be received in its current status"))
	default:
		h.log.Error("purchase order request failed", zap.Error(err))
		h.write(response.Error(w, http.StatusInternalServerError, "internal server error"))
	}
}

// decodeCreate parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeCreate(w http.ResponseWriter, r *http.Request) (purchaseorder.CreateInput, string) {
	var req createRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return purchaseorder.CreateInput{}, "invalid request body"
	}

	return validateCreate(req)
}

// validateCreate checks the fields in the order the API documents them: the
// order's own fields first, then the items. It returns a non-empty validation
// message when one is invalid.
func validateCreate(req createRequest) (purchaseorder.CreateInput, string) {
	supplierID, message := requiredUUID(req.SupplierID, "supplier_id is required", "invalid supplier_id")
	if message != "" {
		return purchaseorder.CreateInput{}, message
	}

	orderDate, expectedDate, notes, message := validateOrderFields(req)
	if message != "" {
		return purchaseorder.CreateInput{}, message
	}

	items, message := validateItems(req.Items)
	if message != "" {
		return purchaseorder.CreateInput{}, message
	}

	return purchaseorder.CreateInput{
		SupplierID:   supplierID,
		OrderDate:    orderDate,
		ExpectedDate: expectedDate,
		Notes:        notes,
		Items:        items,
	}, ""
}

// validateOrderFields checks order_date, expected_date and notes.
func validateOrderFields(req createRequest) (time.Time, *time.Time, *string, string) {
	orderValue := strings.TrimSpace(req.OrderDate)
	if orderValue == "" {
		return time.Time{}, nil, nil, "order_date is required"
	}

	orderDate, err := time.Parse(time.DateOnly, orderValue)
	if err != nil {
		return time.Time{}, nil, nil, "order_date must be a valid date in YYYY-MM-DD format"
	}

	var expectedDate *time.Time
	if value := strings.TrimSpace(req.ExpectedDate); value != "" {
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return time.Time{}, nil, nil, "expected_date must be a valid date in YYYY-MM-DD format"
		}
		if parsed.Before(orderDate) {
			return time.Time{}, nil, nil, "expected_date must not be before order_date"
		}
		expectedDate = &parsed
	}

	notes := strings.TrimSpace(req.Notes)
	if tooLong(notes, maxNotesLength) {
		return time.Time{}, nil, nil, "notes is too long"
	}

	return orderDate, expectedDate, optional(notes), ""
}

// validateItems checks the items. In the messages, i is the zero-based position.
func validateItems(requests []itemRequest) ([]purchaseorder.ItemInput, string) {
	switch {
	case len(requests) == 0:
		return nil, "items is required"
	case len(requests) > maxItems:
		return nil, "items must have at most 100 entries"
	}

	items := make([]purchaseorder.ItemInput, 0, len(requests))
	seen := make(map[string]struct{}, len(requests))

	for i, req := range requests {
		item, message := validateItem(i, req)
		if message != "" {
			return nil, message
		}
		if _, repeated := seen[item.MedicineID]; repeated {
			return nil, itemField(i, "medicine_id is repeated")
		}
		seen[item.MedicineID] = struct{}{}

		items = append(items, item)
	}

	return items, ""
}

func validateItem(i int, req itemRequest) (purchaseorder.ItemInput, string) {
	medicineID, message := requiredUUID(
		req.MedicineID, itemField(i, "medicine_id is required"), itemField(i, "medicine_id is invalid"),
	)
	if message != "" {
		return purchaseorder.ItemInput{}, message
	}

	switch {
	case req.QuantityOrdered == nil:
		return purchaseorder.ItemInput{}, itemField(i, "quantity_ordered is required")
	case *req.QuantityOrdered < 1:
		return purchaseorder.ItemInput{}, itemField(i, "quantity_ordered must be greater than zero")
	case *req.QuantityOrdered > maxQuantity:
		return purchaseorder.ItemInput{}, itemField(i, "quantity_ordered is too large")
	}

	return purchaseorder.ItemInput{MedicineID: medicineID, QuantityOrdered: *req.QuantityOrdered}, ""
}

// itemField returns message prefixed with the items[i]. path, e.g.
// itemField(0, "medicine_id is required") is "items[0].medicine_id is required".
func itemField(i int, message string) string {
	return "items[" + strconv.Itoa(i) + "]." + message
}

// requiredUUID trims and lowercases value, so the same UUID in a different
// case is the same value. It returns the missing or invalid message when the
// value is empty or not a UUID.
func requiredUUID(value, missing, invalid string) (string, string) {
	value = strings.TrimSpace(value)

	switch {
	case value == "":
		return "", missing
	case !isUUID(value):
		return "", invalid
	}

	return strings.ToLower(value), ""
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

func (h *Handler) write(err error) {
	if err != nil {
		h.log.Error("write purchase order response failed", zap.Error(err))
	}
}
