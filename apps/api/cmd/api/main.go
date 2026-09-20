package main

import (
	"net/http"

	"github.com/rodentskiedev/go-libraries/lib/logger"
	"go.uber.org/zap"

	"apps/api/config"
	"apps/api/internal/database"
	inventoryhandler "apps/api/internal/handler/inventory"
	loginhandler "apps/api/internal/handler/login"
	medicinehandler "apps/api/internal/handler/medicine"
	roothandler "apps/api/internal/handler/root"
	supplierhandler "apps/api/internal/handler/supplier"
	swaggerhandler "apps/api/internal/handler/swagger"
	"apps/api/internal/middleware"
	inventoryrepository "apps/api/internal/repository/inventory"
	medicinerepository "apps/api/internal/repository/medicine"
	supplierrepository "apps/api/internal/repository/supplier"
	userrepository "apps/api/internal/repository/user"
	"apps/api/internal/router"
	inventoryservice "apps/api/internal/service/inventory"
	loginservice "apps/api/internal/service/login"
	medicineservice "apps/api/internal/service/medicine"
	rootservice "apps/api/internal/service/root"
	supplierservice "apps/api/internal/service/supplier"
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
	inventorySvc := inventoryservice.NewService(inventoryrepository.NewRepository(db), log)
	supplierSvc := supplierservice.NewService(supplierrepository.NewRepository(db), log)

	rootHandler := roothandler.NewHandler(rootservice.NewService(), log)
	loginHandler := loginhandler.NewHandler(loginSvc, log)
	medicineHandler := medicinehandler.NewHandler(medicineSvc, log)
	inventoryHandler := inventoryhandler.NewHandler(inventorySvc, log)
	supplierHandler := supplierhandler.NewHandler(supplierSvc, log)
	swaggerHandler := swaggerhandler.NewHandler(log)
	auth := middleware.Auth([]byte(cfg.JWTSecret), log)

	r := router.New(cfg.APIPrefix)
	r.HandleExempt("GET /", rootHandler.Get)
	r.HandleExempt("GET /swagger", swaggerHandler.Redirect)
	r.HandleExempt("GET /swagger/", swaggerHandler.UI)
	r.HandleExempt("GET /swagger/openapi.json", swaggerHandler.Spec)
	r.Handle("POST /login", loginHandler.Login)
	r.Handle("POST /medicines", auth(medicineHandler.Create))
	r.Handle("GET /medicines", auth(medicineHandler.List))
	r.Handle("GET /medicines/barcode/{barcode}", auth(medicineHandler.GetByBarcode))
	r.Handle("PUT /medicines/{id}", auth(medicineHandler.Update))
	r.Handle("DELETE /medicines/{id}", auth(medicineHandler.Delete))
	r.Handle("POST /inventory-entries", auth(inventoryHandler.Create))
	r.Handle("GET /inventory-entries", auth(inventoryHandler.List))
	r.Handle("GET /inventory-entries/{id}", auth(inventoryHandler.GetByID))
	r.Handle("POST /suppliers", auth(supplierHandler.Create))
	r.Handle("GET /suppliers", auth(supplierHandler.List))
	r.Handle("GET /suppliers/{id}", auth(supplierHandler.GetByID))
	r.Handle("PUT /suppliers/{id}", auth(supplierHandler.Update))
	r.Handle("DELETE /suppliers/{id}", auth(supplierHandler.Delete))

	log.Info("starting api server", zap.String("port", cfg.Port), zap.String("prefix", cfg.APIPrefix))

	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal("api server stopped", zap.Error(err))
	}
}
