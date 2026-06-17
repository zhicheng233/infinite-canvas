package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"infinite-canvas-server/config"
	"infinite-canvas-server/handler"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/router"
	"infinite-canvas-server/service"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(mysql.Open(cfg.DBDsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Tenant{},
		&model.User{},
		&model.CreditAccount{},
		&model.CreditTransaction{},
		&model.CreditPricing{},
		&model.TenantApiConfig{},
		&model.RechargeOrder{},
		&model.CanvasProject{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	creditRepo := repository.NewCreditRepo(db)
	rechargeRepo := repository.NewRechargeRepo(db)
	apiConfigRepo := repository.NewApiConfigRepo(db)
	canvasRepo := repository.NewCanvasRepo(db)

	authService := service.NewAuthService(cfg, userRepo, tenantRepo, creditRepo)
	userService := service.NewUserService(userRepo)
	creditService := service.NewCreditService(creditRepo)
	generateService := service.NewGenerateService(apiConfigRepo, creditService, creditRepo)
	paymentGateway := service.NewMockPaymentGateway(rechargeRepo, creditService)

	authHandler := handler.NewAuthHandler(authService, userService)
	adminHandler := handler.NewAdminHandler(tenantRepo, userRepo, creditService, creditRepo, rechargeRepo)
	userHandler := handler.NewUserHandler(userService)
	creditHandler := handler.NewCreditHandler(creditService, creditRepo)
	generateHandler := handler.NewGenerateHandler(generateService)
	apiConfigHandler := handler.NewApiConfigHandler(apiConfigRepo)
	proxyHandler := handler.NewProxyHandler(generateService)
	canvasHandler := handler.NewCanvasHandler(canvasRepo)
	rechargeHandler := handler.NewRechargeHandler(rechargeRepo, paymentGateway, creditService)

	r := gin.Default()
	router.Setup(r, authService, authHandler, adminHandler, userHandler, creditHandler, generateHandler, apiConfigHandler, proxyHandler, canvasHandler, rechargeHandler)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
