package domain

import "time"

// Expiration statuses, one per LED color: green, yellow and red.
const (
	ExpirationStatusGood   = "good"
	ExpirationStatusNear   = "near"
	ExpirationStatusExpire = "expire"
)

// Medicine is a registered medicine in the inventory.
type Medicine struct {
	ID          string
	Name        string
	Barcode     string
	BatchNumber *string
	// ExpirationDate is a calendar date; only its year, month and day are meaningful.
	ExpirationDate time.Time
	Quantity       int
	// Location is the tray compartment (1–12), or nil when not placed.
	Location  *int
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}
