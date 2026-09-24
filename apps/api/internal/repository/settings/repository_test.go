package settings

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This test checks the SQL that GORM generates. GORM runs in DryRun mode, so
// nothing connects to a database.

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

func TestSQL_GetReadsTheFirstRow(t *testing.T) {
	rec := &recorder{}
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=localhost user=dry dbname=dry"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: rec},
	)
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}

	_, _ = NewRepository(db).Get(context.Background())

	if len(rec.statements) != 1 {
		t.Fatalf("statements: got %v", rec.statements)
	}
	for _, part := range []string{`FROM "settings"`, "ORDER BY id", "LIMIT 1"} {
		if !strings.Contains(rec.statements[0], part) {
			t.Errorf("SQL should contain %q:\n%s", part, rec.statements[0])
		}
	}
	if strings.Contains(rec.statements[0], "deleted_at") {
		t.Errorf("settings has no deleted_at:\n%s", rec.statements[0])
	}
}
