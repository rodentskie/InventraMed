package supplier

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
// GORM runs in DryRun mode, so nothing connects to a database. They guard the
// parts that are easy to get subtly wrong (soft-delete scoping, escaping,
// locking, which columns an update writes); they do not replace running the
// queries against PostgreSQL.

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

func TestSQL_LockByIDLocksTheRow(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.LockByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{"id = 'id-1'", `"suppliers"."deleted_at" IS NULL`, "FOR UPDATE"}, nil)
}

func TestSQL_ExistsInPurchaseOrder(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.ExistsInPurchaseOrder(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		`FROM "purchase_orders"`,
		"supplier_id = 'id-1'",
		"deleted_at IS NULL",
	}, []string{"JOIN", "status"})
}

func TestSQL_ListEscapesTheNameFilter(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20, Offset: 40, Name: `100%_\`})
	})

	if len(got) != 2 {
		t.Fatalf("expected a count and a page query, got %d statements: %v", len(got), got)
	}

	filters := []string{`name ILIKE '%100\%\_\\%' ESCAPE '\'`, `"suppliers"."deleted_at" IS NULL`}
	assertSQL(t, got[0], append([]string{"count(*)"}, filters...), []string{"ORDER BY", "LIMIT"})
	assertSQL(t, got[1], append([]string{"ORDER BY created_at DESC, id DESC", "LIMIT 20", "OFFSET 40"}, filters...), nil)
}

func TestSQL_ListWithoutNameHasNoFilter(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20})
	})

	for _, statement := range got {
		assertSQL(t, statement, []string{`"suppliers"."deleted_at" IS NULL`}, []string{"ILIKE"})
	}
}

func TestSQL_UpdateWritesEveryDetailIncludingNulls(t *testing.T) {
	phone := "+63"
	got := generated(t, func(r *repository) {
		_ = r.Update(context.Background(), &domain.Supplier{ID: "id-1", Name: "N", Phone: &phone})
	})

	assertSQL(t, got[0], []string{
		`UPDATE "suppliers" SET "name"='N',"contact_name"=NULL,"email"=NULL,"phone"='+63',"address"=NULL`,
		"WHERE id = 'id-1'",
		`"suppliers"."deleted_at" IS NULL`,
	}, []string{"created_at"})
}

func TestSQL_DeleteIsASoftDelete(t *testing.T) {
	got := generated(t, func(r *repository) {
		_ = r.Delete(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		`UPDATE "suppliers" SET "deleted_at"=`,
		"WHERE id = 'id-1'",
		`"suppliers"."deleted_at" IS NULL`,
	}, []string{"DELETE FROM"})
}
