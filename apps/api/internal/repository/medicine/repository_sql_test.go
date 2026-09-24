package medicine

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
// which columns an update writes); they do not replace running the queries
// against PostgreSQL.

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

func TestSQL_ExistsByNameAndBatch(t *testing.T) {
	batch := "B-1"

	t.Run("with a batch, excluding a medicine", func(t *testing.T) {
		got := generated(t, func(r *repository) {
			_, _ = r.ExistsByNameAndBatch(context.Background(), "Para", &batch, "id-1")
		})

		assertSQL(t, got[0], []string{
			"lower(name) = lower('Para')",
			"lower(batch_number) = lower('B-1')",
			"id <> 'id-1'",
			`"medicines"."deleted_at" IS NULL`,
		}, nil)
	})

	t.Run("without a batch or exclusion", func(t *testing.T) {
		got := generated(t, func(r *repository) {
			_, _ = r.ExistsByNameAndBatch(context.Background(), "Para", nil, "")
		})

		assertSQL(t, got[0], []string{"batch_number IS NULL", `"medicines"."deleted_at" IS NULL`}, []string{"id <>"})
	})
}

func TestSQL_ExistsByBarcodeIncludesSoftDeleted(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.ExistsByBarcode(context.Background(), "890", "id-1")
	})

	assertSQL(t, got[0], []string{"barcode = '890'", "id <> 'id-1'"}, []string{"deleted_at"})

	got = generated(t, func(r *repository) {
		_, _ = r.ExistsByBarcode(context.Background(), "890", "")
	})

	assertSQL(t, got[0], []string{"barcode = '890'"}, []string{"deleted_at", "id <>"})
}

func TestSQL_ExistsByLocationIgnoresSoftDeleted(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.ExistsByLocation(context.Background(), 7, "id-1")
	})

	assertSQL(t, got[0], []string{"location = 7", "id <> 'id-1'", `"medicines"."deleted_at" IS NULL`}, nil)

	got = generated(t, func(r *repository) {
		_, _ = r.ExistsByLocation(context.Background(), 7, "")
	})

	assertSQL(t, got[0], []string{"location = 7", `"medicines"."deleted_at" IS NULL`}, []string{"id <>"})
}

func TestSQL_ExistsByIDIgnoresSoftDeleted(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.ExistsByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{"id = 'id-1'", `"medicines"."deleted_at" IS NULL`}, nil)
}

func TestSQL_LockByIDLocksTheRow(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.LockByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{"id = 'id-1'", `"medicines"."deleted_at" IS NULL`, "FOR UPDATE"}, nil)
}

func TestSQL_LockQuantityByIDLocksTheRow(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.LockQuantityByID(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{"id = 'id-1'", `"medicines"."deleted_at" IS NULL`, "FOR UPDATE", "quantity"}, nil)
}

func TestSQL_ExistsInPurchaseOrder(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.ExistsInPurchaseOrder(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		"JOIN purchase_orders ON purchase_orders.id = purchase_order_items.purchase_order_id",
		"purchase_order_items.medicine_id = 'id-1'",
		"purchase_orders.deleted_at IS NULL",
	}, []string{"status"})
}

func TestSQL_List(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20, Offset: 40, Name: "100%_", Barcode: "890"})
	})

	if len(got) != 2 {
		t.Fatalf("expected a count and a page query, got %d statements: %v", len(got), got)
	}

	filters := []string{
		`name ILIKE '%100\%\_%' ESCAPE '\'`,
		`barcode ILIKE '%890%' ESCAPE '\'`,
		`"medicines"."deleted_at" IS NULL`,
	}
	assertSQL(t, got[0], append([]string{"count(*)"}, filters...), []string{"ORDER BY", "LIMIT"})
	assertSQL(t, got[1], append([]string{"ORDER BY created_at DESC, id DESC", "LIMIT 20", "OFFSET 40"}, filters...), nil)
}

func TestSQL_ListWithoutFilters(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _, _ = r.List(context.Background(), ListFilter{Limit: 20})
	})

	for _, statement := range got {
		assertSQL(t, statement, []string{`"medicines"."deleted_at" IS NULL`}, []string{"ILIKE"})
	}
}

func TestSQL_FindByBarcodeIsExactAndActiveOnly(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.FindByBarcode(context.Background(), "890")
	})

	assertSQL(t, got[0], []string{"barcode = '890'", `"medicines"."deleted_at" IS NULL`}, []string{"ILIKE"})
}

func TestSQL_UpdateWritesOnlyTheDetails(t *testing.T) {
	got := generated(t, func(r *repository) {
		_ = r.Update(context.Background(), &domain.Medicine{
			ID:             "id-1",
			Name:           "N",
			Barcode:        "B",
			BatchNumber:    nil,
			Quantity:       99,
			ExpirationDate: time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
		})
	})

	assertSQL(t, got[0], []string{
		`UPDATE "medicines" SET "name"='N',"barcode"='B',"batch_number"=NULL,"expiration_date"='2027-01-02`,
		`"location"=NULL`,
		"WHERE id = 'id-1'",
		`"medicines"."deleted_at" IS NULL`,
	}, []string{"quantity", "created_by", "created_at", "updated_at"})
}

func TestSQL_UpdateWritesTheLocation(t *testing.T) {
	location := 7

	got := generated(t, func(r *repository) {
		_ = r.Update(context.Background(), &domain.Medicine{ID: "id-1", Name: "N", Barcode: "B", Location: &location})
	})

	assertSQL(t, got[0], []string{`"location"=7`}, nil)
}

func TestSQL_DeleteIsASoftDelete(t *testing.T) {
	got := generated(t, func(r *repository) {
		_ = r.Delete(context.Background(), "id-1")
	})

	assertSQL(t, got[0], []string{
		`UPDATE "medicines" SET "deleted_at"=`,
		"WHERE id = 'id-1'",
		`"medicines"."deleted_at" IS NULL`,
	}, []string{"DELETE FROM"})
}

func TestSQL_CreateLeavesTimestampsAndIDToTheDatabase(t *testing.T) {
	got := generated(t, func(r *repository) {
		_, _ = r.Create(context.Background(), &domain.Medicine{Name: "N", Barcode: "B", Quantity: 3})
	})

	assertSQL(t, got[0], []string{
		`INSERT INTO "medicines" ("name","barcode","batch_number","expiration_date","quantity","location","created_by"`,
		`RETURNING "id","created_at","updated_at"`,
	}, nil)

	// Only the insert's column list, not the RETURNING clause, must leave these out.
	columns, _, _ := strings.Cut(got[0], " VALUES ")
	assertSQL(t, columns, nil, []string{`"id"`, `"created_at"`, `"updated_at"`})
}
