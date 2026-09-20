package apperror

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
)

// Medicine conflicts. Both wrap ErrConflict, so errors.Is(err, ErrConflict)
// matches either while callers can still tell them apart.
var (
	ErrNameBatchExists = fmt.Errorf("medicine name and batch number already exist: %w", ErrConflict)
	ErrBarcodeExists   = fmt.Errorf("medicine barcode already exists: %w", ErrConflict)
	// ErrMedicineInPurchaseOrder is returned when deleting a medicine that a
	// purchase order still uses.
	ErrMedicineInPurchaseOrder = fmt.Errorf("medicine is used in a purchase order: %w", ErrConflict)
)

// ErrInsufficientQuantity is returned when a subtraction inventory entry
// would take a medicine's quantity below zero.
var ErrInsufficientQuantity = fmt.Errorf("insufficient quantity for subtraction: %w", ErrConflict)
