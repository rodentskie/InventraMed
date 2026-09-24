package medicine

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"apps/api/pkg/apperror"
)

func TestEscapeLike(t *testing.T) {
	tests := map[string]string{
		"para":        "para",
		"100%":        `100\%`,
		"a_b":         `a\_b`,
		`back\slash`:  `back\\slash`,
		`%_\`:         `\%\_\\`,
		"":            "",
		"Paracetamol": "Paracetamol",
	}

	for term, want := range tests {
		if got := escapeLike(term); got != want {
			t.Errorf("escapeLike(%q): got %q, want %q", term, got, want)
		}
	}
}

func TestTranslateError(t *testing.T) {
	plain := errors.New("connection reset")

	tests := []struct {
		name      string
		err       error
		want      error
		conflicts bool
	}{
		{
			name:      "barcode unique violation",
			err:       &pgconn.PgError{Code: "23505", ConstraintName: "medicines_barcode_key"},
			want:      apperror.ErrBarcodeExists,
			conflicts: true,
		},
		{
			name:      "name and batch unique violation",
			err:       &pgconn.PgError{Code: "23505", ConstraintName: "uq_medicines_name_batch"},
			want:      apperror.ErrNameBatchExists,
			conflicts: true,
		},
		{
			name:      "location unique violation",
			err:       &pgconn.PgError{Code: "23505", ConstraintName: "uq_medicines_location"},
			want:      apperror.ErrLocationTaken,
			conflicts: true,
		},
		{
			name:      "wrapped unique violation",
			err:       fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "medicines_barcode_key"}),
			want:      apperror.ErrBarcodeExists,
			conflicts: true,
		},
		{
			name: "unique violation on another constraint",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "something_else"},
		},
		{
			name: "other postgres error",
			err:  &pgconn.PgError{Code: "23503", ConstraintName: "medicines_barcode_key"},
		},
		{
			name: "non postgres error",
			err:  plain,
			want: plain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateError(tt.err)

			if got == nil {
				t.Fatal("expected an error")
			}
			if tt.want != nil && !errors.Is(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if isConflict := errors.Is(got, apperror.ErrConflict); isConflict != tt.conflicts {
				t.Errorf("conflict: got %v, want %v (err: %v)", isConflict, tt.conflicts, got)
			}
			if tt.want == nil && !errors.Is(got, tt.err) {
				t.Errorf("original error not wrapped: %v", got)
			}
		})
	}
}
