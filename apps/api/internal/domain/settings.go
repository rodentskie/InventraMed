package domain

// Settings is the inventory configuration.
type Settings struct {
	// WarningThresholdDays is how many days before its expiration date a
	// medicine counts as nearing expiration.
	WarningThresholdDays int
}
