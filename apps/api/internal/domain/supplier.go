package domain

import "time"

// Supplier is a company medicines are purchased from.
type Supplier struct {
	ID          string
	Name        string
	ContactName *string
	Email       *string
	Phone       *string
	Address     *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
