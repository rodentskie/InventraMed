package supplier

import (
	"net/http"

	"apps/api/pkg/response"
)

// Update handles PUT /suppliers/{id}. It must be wrapped by the auth middleware.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	id, ok := h.pathID(w, r)
	if !ok {
		return
	}

	input, message := decodeInput(w, r)
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

// Delete handles DELETE /suppliers/{id}. It must be wrapped by the auth middleware.
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
		h.write(response.Error(w, http.StatusBadRequest, "invalid supplier id"))
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
