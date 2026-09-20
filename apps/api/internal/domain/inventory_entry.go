package domain

import "time"

// Inventory entry directions.
const (
	DirectionAddition    = "addition"
	DirectionSubtraction = "subtraction"
)

// InventoryEntry is a manual stock adjustment for a medicine. It is
// append-only: correcting a mistake is a new offsetting entry, not an edit.
type InventoryEntry struct {
	ID         string
	MedicineID string
	Direction  string
	Quantity   int
	Reason     string
	CountedBy  string
	Notes      *string
	CreatedAt  time.Time
}
