package purchaseorder

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"apps/api/internal/service/purchaseorder"
	"apps/api/pkg/response"
)

// receiveRequest has no received_by: the receiver is always the caller, so one
// in the body is ignored like any other unknown field.
type receiveRequest struct {
	Notes string `json:"notes"`
}

type receiveResponse struct {
	Message string          `json:"message"`
	Data    receiptResponse `json:"data"`
}

// Receive handles POST /purchase-orders/{id}/receive. It must be wrapped by the
// auth middleware.
func (h *Handler) Receive(w http.ResponseWriter, r *http.Request) {
	account, ok := h.caller(w, r)
	if !ok {
		return
	}

	id, ok := h.pathID(w, r)
	if !ok {
		return
	}

	notes, message := decodeReceive(w, r)
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	receipt, err := h.service.Receive(r.Context(), purchaseorder.ReceiveInput{
		PurchaseOrderID: id,
		Notes:           notes,
		ReceivedBy:      account.UserID,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	h.write(response.JSON(w, http.StatusCreated, receiveResponse{
		Message: "purchase order received",
		Data:    toReceiptResponse(receipt),
	}))
}

// decodeReceive parses and validates the optional body. An empty body is the
// same as {}. It returns a non-empty validation message when the request is
// invalid.
func decodeReceive(w http.ResponseWriter, r *http.Request) (*string, string) {
	var req receiveRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return nil, "invalid request body"
	}

	notes := strings.TrimSpace(req.Notes)
	if tooLong(notes, maxNotesLength) {
		return nil, "notes is too long"
	}

	return optional(notes), ""
}

// pathID returns the {id} path value. When it is not a UUID it writes a 400
// and returns false, so a bad value never reaches the database.
func (h *Handler) pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !isUUID(id) {
		h.write(response.Error(w, http.StatusBadRequest, "invalid purchase order id"))
		return "", false
	}

	return id, true
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
