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

// ErrSupplierInPurchaseOrder is returned when deleting a supplier that a
// purchase order still uses. It wraps ErrConflict.
var ErrSupplierInPurchaseOrder = fmt.Errorf("supplier is used in a purchase order: %w", ErrConflict)

// ErrInsufficientQuantity is returned when a subtraction inventory entry
// would take a medicine's quantity below zero.
var ErrInsufficientQuantity = fmt.Errorf("insufficient quantity for subtraction: %w", ErrConflict)

// Purchase order errors. ErrSupplierNotFound and ErrMedicineNotFound are
// returned by create when the body references a supplier or medicine that is
// not active; they wrap ErrNotFound, so callers that don't care which one is
// missing can still match it.
var (
	ErrSupplierNotFound = fmt.Errorf("supplier not found: %w", ErrNotFound)
	ErrMedicineNotFound = fmt.Errorf("medicine not found: %w", ErrNotFound)
	// ErrPurchaseOrderNotReceivable is returned when receiving a purchase order
	// whose status is not draft or ordered.
	ErrPurchaseOrderNotReceivable = fmt.Errorf("purchase order cannot be received: %w", ErrConflict)
)
