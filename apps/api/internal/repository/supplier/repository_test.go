package supplier

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"apps/api/pkg/apperror"
)

func TestTranslateError(t *testing.T) {
	plain := errors.New("connection reset")

	tests := []struct {
		name         string
		err          error
		want         error
		wantConflict bool
	}{
		{
			name:         "name unique violation",
			err:          &pgconn.PgError{Code: "23505", ConstraintName: "uq_suppliers_name"},
			want:         apperror.ErrSupplierNameExists,
			wantConflict: true,
		},
		{
			name:         "wrapped name unique violation",
			err:          fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "uq_suppliers_name"}),
			want:         apperror.ErrSupplierNameExists,
			wantConflict: true,
		},
		{
			name: "unique violation on another constraint",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "something_else"},
		},
		{
			name: "other postgres error on the name constraint",
			err:  &pgconn.PgError{Code: "23503", ConstraintName: "uq_suppliers_name"},
		},
		{
			name: "non postgres error",
			err:  plain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateError(tt.err)

			if tt.want != nil {
				if got != tt.want {
					t.Fatalf("error: got %v, want %v unchanged", got, tt.want)
				}
			} else if !errors.Is(got, tt.err) {
				t.Fatalf("error: got %v, want it to wrap %v", got, tt.err)
			}
			if conflict := errors.Is(got, apperror.ErrConflict); conflict != tt.wantConflict {
				t.Errorf("wraps ErrConflict: got %v, want %v", conflict, tt.wantConflict)
			}
		})
	}
}
