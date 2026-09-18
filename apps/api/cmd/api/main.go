package main

import (
	"net/http"

	"github.com/rodentskiedev/go-libraries/lib/logger"
	"go.uber.org/zap"

	"apps/api/config"
	roothandler "apps/api/internal/handler/root"
	rootservice "apps/api/internal/service/root"
)

func main() {
	log := logger.New()
	defer log.Sync() //nolint:errcheck

	cfg := config.LoadConfig()

	handler := roothandler.NewHandler(rootservice.NewService(), log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handler.Get)

	log.Info("starting api server", zap.String("port", cfg.Port))

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal("api server stopped", zap.Error(err))
	}
}
