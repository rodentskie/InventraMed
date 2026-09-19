package hash

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndCompare(t *testing.T) {
	t.Parallel()

	hashed, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if hashed == "secret" {
		t.Fatal("hash must not equal the plain password")
	}

	if err := Compare(hashed, "secret"); err != nil {
		t.Errorf("Compare: got %v, want nil", err)
	}
}

func TestCompare_WrongPassword(t *testing.T) {
	t.Parallel()

	hashed, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	err = Compare(hashed, "other")
	if !errors.Is(err, ErrMismatch) {
		t.Errorf("Compare: got %v, want ErrMismatch", err)
	}
}

func TestCompare_MalformedHash(t *testing.T) {
	t.Parallel()

	err := Compare("not-a-bcrypt-hash", "secret")
	if err == nil {
		t.Fatal("expected error for malformed hash, got nil")
	}

	if errors.Is(err, ErrMismatch) {
		t.Error("malformed hash must not be reported as a mismatch")
	}
}

func TestHashAndCompare_EmptyPassword(t *testing.T) {
	t.Parallel()

	hashed, err := Hash("")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if err := Compare(hashed, ""); err != nil {
		t.Errorf("Compare empty: got %v, want nil", err)
	}

	if err := Compare(hashed, "secret"); !errors.Is(err, ErrMismatch) {
		t.Errorf("Compare non-empty against empty hash: got %v, want ErrMismatch", err)
	}
}

func TestHash_Salted(t *testing.T) {
	t.Parallel()

	first, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	second, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if first == second {
		t.Error("two hashes of the same password must differ")
	}
}

func TestHash_TooLong(t *testing.T) {
	t.Parallel()

	_, err := Hash(strings.Repeat("a", 73))
	if err == nil {
		t.Fatal("expected error for password over bcrypt's 72 byte limit, got nil")
	}
}
