package inventory

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"apps/api/internal/domain"
)

// These tests check the SQL that GORM generates for each repository method.
// GORM runs in DryRun mode, so nothing connects to a database.

type recorder struct {
	logger.Interface
	statements []string
}

func (r *recorder) LogMode(logger.LogLevel) logger.Interface { return r }

func (r *recorder) Info(context.Context, string, ...any) {}

func (r *recorder) Warn(context.Context, string, ...any) {}

func (r *recorder) Error(context.Context, string, ...any) {}

func (r *recorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	statement, _ := fc()
	r.statements = append(r.statements, statement)
}

// generated runs fn against a DryRun repository and returns every SQL
// statement it produced.
func generated(t *testing.T, fn func(r *repository)) []string {
	t.Helper()

	rec := &recorder{}
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=localhost user=dry dbname=dry"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: rec},
	)
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}

	fn(&repository{db: db})

	if len(rec.statements) == 0 {
		t.Fatal("no SQL was generated")
	}

	return rec.statements
}

func assertSQL(t *testing.T, statement string, contains, excludes []string) {
	t.Helper()

	for _, part := range contains {
		if !strings.Contains(statement, part) {
			t.Errorf("SQL should contain %q:\n%s", part, statement)
		}
	}
	for _, part := range excludes {
		if strings.Contains(statement, part) {
			t.Errorf("SQL should not contain %q:\n%s", part, statement)
		}
	}
}

func TestSQL_CreateLeavesIDAndCreatedAtToTheDatabase(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.Create(context.Background(), &domain.InventoryEntry{
			MedicineID: "med-1",
			Direction:  domain.DirectionAddition,
			Quantity:   5,
			Reason:     "count_adjustment",
			CountedBy:  domain.InventoryEntryUser{ID: "user-1"},
		})
	})

	assertSQL(t, got[0], []string{
		`INSERT INTO "inventory_entries" ("medicine_id","direction","quantity","reason","counted_by","notes")`,
		`RETURNING "id","created_at"`,
	}, nil)
}

func TestSQL_FindByID(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.FindByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		`"inventory_entries"`,
		"LEFT JOIN users ON users.id = inventory_entries.counted_by",
		"COALESCE(users.name, '') AS counted_by_name",
		"inventory_entries.id = 'id-1'",
	}, nil)
}

func TestSQL_CreateReloadsWithCounterName(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.Create(context.Background(), &domain.InventoryEntry{
			MedicineID: "med-1",
			Direction:  domain.DirectionAddition,
			Quantity:   5,
			Reason:     "count_adjustment",
			CountedBy:  domain.InventoryEntryUser{ID: "user-1"},
		})
	})

	// got[0] is the INSERT, got[1] the reload that joins in the user's name.
	assertSQL(t, got[1], []string{
		"LEFT JOIN users ON users.id = inventory_entries.counted_by",
	}, nil)
}

func TestSQL_ListOrdersAndPaginates(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20, Offset: 40})
	})

	// got[0] is the Count query, got[1] the page query.
	assertSQL(t, got[1], []string{
		"LEFT JOIN users ON users.id = inventory_entries.counted_by",
		"ORDER BY inventory_entries.created_at DESC, inventory_entries.id DESC",
		"LIMIT 20",
		"OFFSET 40",
	}, nil)
}
