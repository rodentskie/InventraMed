package medicine

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"apps/api/internal/service/medicine"
	"apps/api/pkg/response"
)

const (
	defaultLimit = 20
	minLimit     = 1
	maxLimit     = 100
)

type listResponse struct {
	Data   []medicineResponse `json:"data"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

type getResponse struct {
	Data medicineResponse `json:"data"`
}

// List handles GET /medicines. It must be wrapped by the auth middleware.
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
		h.writeError(w, err)
		return
	}

	// Never nil, so an empty page is encoded as [] and not null.
	data := make([]medicineResponse, 0, len(page.Medicines))
	for _, m := range page.Medicines {
		data = append(data, toResponse(m))
	}

	h.write(response.JSON(w, http.StatusOK, listResponse{
		Data:   data,
		Total:  page.Total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}))
}

// GetByBarcode handles GET /medicines/barcode/{barcode}. It must be wrapped by
// the auth middleware.
func (h *Handler) GetByBarcode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}

	barcode := strings.TrimSpace(r.PathValue("barcode"))
	if message := validateBarcode(barcode); message != "" {
		h.write(response.Error(w, http.StatusBadRequest, message))
		return
	}

	found, err := h.service.GetByBarcode(r.Context(), barcode)
	if err != nil {
		h.writeError(w, err)
		return
	}

	h.write(response.JSON(w, http.StatusOK, getResponse{Data: toResponse(found)}))
}

// parseListFilter validates the list query parameters. It returns a non-empty
// validation message when one is invalid.
func parseListFilter(query url.Values) (medicine.ListFilter, string) {
	limit, ok := parseWholeNumber(query.Get("limit"), defaultLimit, minLimit, maxLimit)
	if !ok {
		return medicine.ListFilter{}, "limit must be a whole number between 1 and 100"
	}

	offset, ok := parseWholeNumber(query.Get("offset"), 0, 0, math.MaxInt)
	if !ok {
		return medicine.ListFilter{}, "offset must be a whole number of zero or greater"
	}

	name := strings.TrimSpace(query.Get("name"))
	if tooLong(name, maxNameLength) {
		return medicine.ListFilter{}, "name is too long"
	}

	barcode := strings.TrimSpace(query.Get("barcode"))
	if tooLong(barcode, maxBarcodeLength) {
		return medicine.ListFilter{}, "barcode is too long"
	}

	return medicine.ListFilter{Limit: limit, Offset: offset, Name: name, Barcode: barcode}, ""
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
