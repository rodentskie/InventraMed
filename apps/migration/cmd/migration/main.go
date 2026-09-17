package main

import (
	"apps/migration/config"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
	"github.com/rodentskiedev/go-libraries/lib/logger"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const migrationsDir = "transactions"

func main() {
	log := logger.New()
	defer log.Sync() //nolint:errcheck

	cfg := config.LoadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("open db", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("get sql.DB", zap.Error(err))
	}
	defer sqlDB.Close() //nolint:errcheck

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("set dialect", zap.Error(err))
	}

	switch cfg.Action {
	case "up":
		if err := goose.Up(sqlDB, migrationsDir); err != nil {
			log.Fatal("migration up", zap.Error(err))
		}
		log.Info("migrations applied to latest")
	case "down":
		if err := goose.DownTo(sqlDB, migrationsDir, 0); err != nil {
			log.Fatal("migration down", zap.Error(err))
		}
		log.Info("all migrations rolled back")
	case "rollback":
		if err := goose.Down(sqlDB, migrationsDir); err != nil {
			log.Fatal("migration rollback", zap.Error(err))
		}
		log.Info("rolled back one migration")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nusage: migration <up|down|rollback>\n", cfg.Action)
		os.Exit(1)
	}
}
