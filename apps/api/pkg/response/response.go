package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// JSON writes payload as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode json response: %w", err)
	}

	return nil
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes message as a JSON error response with the given status code.
func Error(w http.ResponseWriter, status int, message string) error {
	return JSON(w, status, map[string]string{"error": message})
}
