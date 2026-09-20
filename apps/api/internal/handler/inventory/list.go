package inventory

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"apps/api/internal/service/inventory"
	"apps/api/pkg/response"
)

const (
	defaultLimit = 20
	minLimit     = 1
	maxLimit     = 100
)

type listResponse struct {
	Data   []entryResponse `json:"data"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type getResponse struct {
	Data entryResponse `json:"data"`
}

// List handles GET /inventory-entries. It must be wrapped by the auth middleware.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	filter, message := parseListFilter(r.URL.Query())
	if message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	page, err := h.service.List(r.Context(), filter)
	if err != nil {
		h.writeError(w, err, "")
		return
	}

	// Never nil, so an empty page is encoded as [] and not null.
	data := make([]entryResponse, 0, len(page.Entries))
	for _, e := range page.Entries {
		data = append(data, toResponse(e))
	}

	h.write(response.JSON(w, http.StatusOK, listResponse{
		Data:   data,
		Total:  page.Total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}))
}

// GetByID handles GET /inventory-entries/{id}. It must be wrapped by the auth
// middleware.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	if !isUUID(id) {
		h.write(response.Error(w, http.StatusBadRequest, "invalid inventory entry id"))
		return
	}

	found, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.writeError(w, err, "inventory entry not found")
		return
	}

	h.write(response.JSON(w, http.StatusOK, getResponse{Data: toResponse(found)}))
}

// parseListFilter validates the list query parameters. It returns a non-empty
// validation message when one is invalid.
func parseListFilter(query url.Values) (inventory.ListFilter, string) {
	limit, ok := parseWholeNumber(query.Get("limit"), defaultLimit, minLimit, maxLimit)
	if !ok {
		return inventory.ListFilter{}, "limit must be a whole number between 1 and 100"
	}

	offset, ok := parseWholeNumber(query.Get("offset"), 0, 0, math.MaxInt)
	if !ok {
		return inventory.ListFilter{}, "offset must be a whole number of zero or greater"
	}

	return inventory.ListFilter{Limit: limit, Offset: offset}, ""
}

// parseWholeNumber parses value as an integer between lower and upper. An empty
// value gives fallback. It reports false when value is not a whole number in range.
func parseWholeNumber(value string, fallback, lower, upper int) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, true
	}

	number, err := strconv.Atoi(value)
	if err != nil || number < lower || number > upper {
		return 0, false
	}

	return number, true
}
