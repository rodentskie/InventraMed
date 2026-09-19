// Package hash hashes and verifies passwords with bcrypt.
package hash

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrMismatch is returned by Compare when the plain text does not match the hash.
var ErrMismatch = bcrypt.ErrMismatchedHashAndPassword

// Hash returns the bcrypt hash of password at bcrypt.DefaultCost.
func Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hashed), nil
}

// Compare reports whether plain matches hashed. A mismatch wraps ErrMismatch
// so callers can tell a wrong password from a malformed hash with errors.Is.
func Compare(hashed, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)); err != nil {
		return fmt.Errorf("compare password: %w", err)
	}

	return nil
}
