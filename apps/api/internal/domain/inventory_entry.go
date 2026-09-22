package domain

import "time"

// Inventory entry directions.
const (
	DirectionAddition    = "addition"
	DirectionSubtraction = "subtraction"
)

// InventoryEntryUser identifies the user who counted an inventory entry, by
// ID and name, so a caller doesn't need a separate lookup to display it.
type InventoryEntryUser struct {
	ID   string
	Name string
}

// InventoryEntry is a manual stock adjustment for a medicine. It is
// append-only: correcting a mistake is a new offsetting entry, not an edit.
type InventoryEntry struct {
	ID         string
	MedicineID string
	Direction  string
	Quantity   int
	Reason     string
	CountedBy  InventoryEntryUser
	Notes      *string
	CreatedAt  time.Time
}
