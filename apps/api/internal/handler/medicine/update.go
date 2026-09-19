package medicine

import (
	"encoding/json"
	"net/http"

	"apps/api/internal/service/medicine"
	"apps/api/pkg/response"
)

// updateRequest has no quantity: stock is not edited through an update, so a
// quantity in the body is ignored like any other unknown field.
type updateRequest struct {
	Name           string `json:"name"`
	Barcode        string `json:"barcode"`
	BatchNumber    string `json:"batch_number"`
	ExpirationDate string `json:"expiration_date"`
}

// Update handles PUT /medicines/{id}. It must be wrapped by the auth middleware.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	id, ok := h.pathID(w, r)
	if !ok {
		return
	}

	input, message := decodeUpdate(w, r)
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	if err := h.service.Update(r.Context(), id, input); err != nil {
		h.writeError(w, err)
		return
	}

	response.NoContent(w)
}

// Delete handles DELETE /medicines/{id}. It must be wrapped by the auth middleware.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	id, ok := h.pathID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}

	response.NoContent(w)
}

// pathID returns the {id} path value. When it is not a UUID it writes a 400
// and returns false, so a bad value never reaches the database.
func (h *Handler) pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !isUUID(id) {
		h.write(response.Error(w, http.StatusBadRequest, "invalid medicine id"))
		return "", false
	}

	return id, true
}

// decodeUpdate parses and validates the body. It returns a non-empty
// validation message when the request is invalid.
func decodeUpdate(w http.ResponseWriter, r *http.Request) (medicine.UpdateInput, string) {
	var req updateRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return medicine.UpdateInput{}, "invalid request body"
	}

	d, message := validateDetails(req.Name, req.Barcode, req.BatchNumber, req.ExpirationDate)
	if message != "" {
		return medicine.UpdateInput{}, message
	}

	return medicine.UpdateInput{
		Name:           d.name,
		Barcode:        d.barcode,
		BatchNumber:    d.batchNumber,
		ExpirationDate: d.expirationDate,
	}, ""
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
