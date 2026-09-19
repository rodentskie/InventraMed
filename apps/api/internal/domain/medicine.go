package domain

import "time"

// Medicine is a registered medicine in the inventory.
type Medicine struct {
	ID          string
	Name        string
	Barcode     string
	BatchNumber *string
	// ExpirationDate is a calendar date; only its year, month and day are meaningful.
	ExpirationDate time.Time
	Quantity       int
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
