package main

import (
	"net/http"

	"github.com/rodentskiedev/go-libraries/lib/logger"
	"go.uber.org/zap"

	"apps/api/config"
	"apps/api/internal/database"
	loginhandler "apps/api/internal/handler/login"
	medicinehandler "apps/api/internal/handler/medicine"
	roothandler "apps/api/internal/handler/root"
	swaggerhandler "apps/api/internal/handler/swagger"
	"apps/api/internal/middleware"
	medicinerepository "apps/api/internal/repository/medicine"
	userrepository "apps/api/internal/repository/user"
	"apps/api/internal/router"
	loginservice "apps/api/internal/service/login"
	medicineservice "apps/api/internal/service/medicine"
	rootservice "apps/api/internal/service/root"
)

func main() {
	log := logger.New()
	defer log.Sync() //nolint:errcheck

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("connect database", zap.Error(err))
	}

	loginSvc := loginservice.NewService(userrepository.NewRepository(db), loginservice.TokenConfig{
		Secret:        []byte(cfg.JWTSecret),
		AccessExpiry:  cfg.JWTAccessExpiry,
		RefreshExpiry: cfg.JWTRefreshExpiry,
	}, log)

	medicineSvc := medicineservice.NewService(medicinerepository.NewRepository(db), log)

	rootHandler := roothandler.NewHandler(rootservice.NewService(), log)
	loginHandler := loginhandler.NewHandler(loginSvc, log)
	medicineHandler := medicinehandler.NewHandler(medicineSvc, log)
	swaggerHandler := swaggerhandler.NewHandler(log)
	auth := middleware.Auth([]byte(cfg.JWTSecret), log)

	r := router.New(cfg.APIPrefix)
	r.HandleExempt("GET /", rootHandler.Get)
	r.HandleExempt("GET /swagger", swaggerHandler.Redirect)
	r.HandleExempt("GET /swagger/", swaggerHandler.UI)
	r.HandleExempt("GET /swagger/openapi.json", swaggerHandler.Spec)
	r.Handle("POST /login", loginHandler.Login)
	r.Handle("POST /medicines", auth(medicineHandler.Create))

	log.Info("starting api server", zap.String("port", cfg.Port), zap.String("prefix", cfg.APIPrefix))

	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal("api server stopped", zap.Error(err))
	}
}
