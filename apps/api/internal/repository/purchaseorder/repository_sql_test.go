package purchaseorder

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests check the SQL that GORM generates for each repository method.
// GORM runs in DryRun mode, so nothing connects to a database. They guard the
// parts that are easy to get subtly wrong (soft-delete scoping, locking,
// ordering, which columns an update writes, the audit actor); they do not
// replace running the queries against PostgreSQL.

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

// setActor is what Transaction runs first. DryRun can't open a transaction, so
// the statement is checked on its own.
func TestSQL_SetActorIsLocalToTheTransaction(t *testing.T) {
	got := generated(t, func(r *repository) {
		_ = setActor(r.db, "user-1")
	})

	assertSQL(t, got[0], []string{`SELECT set_config('app.actor_id', 'user-1', true)`}, []string{"SET LOCAL"})
}

func TestSQL_LockStatusByIDLocksTheRow(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.LockStatusByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		"id = 'id-1'",
		`"purchase_orders"."deleted_at" IS NULL`,
		"FOR UPDATE",
		"status",
	}, nil)
}

func TestSQL_ListIsActiveOnlyAndNewestFirst(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20, Offset: 40})
	})

	if len(got) != 2 {
		t.Fatalf("expected a count and a page query, got %d statements: %v", len(got), got)
	}

	scope := `"purchase_orders"."deleted_at" IS NULL`
	assertSQL(t, got[0], []string{"count(*)", scope}, []string{"ORDER BY", "LIMIT"})
	assertSQL(t, got[1], []string{scope, "ORDER BY created_at DESC, id DESC", "LIMIT 20", "OFFSET 40"}, nil)
}

func TestSQL_FindByIDIsActiveOnly(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.FindByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{"id = 'id-1'", `"purchase_orders"."deleted_at" IS NULL`}, nil)
}

func TestSQL_ItemsAreOrdered(t *testing.T) {
	t.Run("for a read", func(t *testing.T) {
		got := generated(t, func(r *repository) {
			_, _ = r.itemsOf(context.Background(), "po-1", "created_at, id")
		})

		assertSQL(t, got[0], []string{
			`FROM "purchase_order_items"`,
			"purchase_order_id = 'po-1'",
			"ORDER BY created_at, id",
		}, []string{"deleted_at"})
	})

	t.Run("for a receive, by medicine so the trigger locks rows in a consistent order", func(t *testing.T) {
		got := generated(t, func(r *repository) {
			_, _ = r.ListItems(context.Background(), "po-1")
		})

		assertSQL(t, got[0], []string{"purchase_order_id = 'po-1'", "ORDER BY medicine_id, id"}, nil)
	})
}

func TestSQL_ReceiptsAreOrdered(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.receiptsOf(context.Background(), "po-1")
	})

	assertSQL(t, got[0], []string{
		`FROM "purchase_order_receipts"`,
		"purchase_order_id = 'po-1'",
		"ORDER BY received_at, id",
	}, nil)
}

func TestSQL_UpdateStatusWritesOnlyTheStatus(t *testing.T) {
	got := generated(t, func(r *repository) {
		_ = r.UpdateStatus(context.Background(), "id-1", "received")
	})

	assertSQL(t, got[0], []string{
		`UPDATE "purchase_orders" SET "status"='received'`,
		"WHERE id = 'id-1'",
		`"purchase_orders"."deleted_at" IS NULL`,
	}, []string{"updated_at", "notes", "supplier_id", "order_date"})
}
