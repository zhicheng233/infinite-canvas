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
		&model.GenerationRecord{},
		&model.TenantApiConfig{},
		&model.RechargeOrder{},
		&model.CanvasProject{},
		&model.ModelCallLog{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	creditRepo := repository.NewCreditRepo(db)
	rechargeRepo := repository.NewRechargeRepo(db)
	apiConfigRepo := repository.NewApiConfigRepo(db)
	canvasRepo := repository.NewCanvasRepo(db)
	generationRecordRepo := repository.NewGenerationRecordRepo(db)
	modelCallLogRepo := repository.NewModelCallLogRepo(db)

	captchaService := service.NewCaptchaService()

	authService := service.NewAuthService(cfg, userRepo, tenantRepo, creditRepo, captchaService)
	if err := authService.EnsureInitialAdmin(); err != nil {
		log.Fatalf("failed to bootstrap initial admin: %v", err)
	}
	userService := service.NewUserService(userRepo)
	creditService := service.NewCreditService(creditRepo)
	modelCallLogService := service.NewModelCallLogService(modelCallLogRepo, userRepo)
	onDemandRepairService := service.NewOnDemandRepairService(cfg.OnDemandRepairURL, cfg.OnDemandRepairUser, cfg.OnDemandRepairPass, cfg.OnDemandRepairTimeoutSeconds)
	generateService := service.NewGenerateService(apiConfigRepo, creditService, creditRepo, modelCallLogService, cfg.ApiKeyEncryptKey, onDemandRepairService)
	tempMediaService := service.NewTempMediaService(cfg)
	channelStatusService := service.NewChannelStatusService(modelCallLogRepo, apiConfigRepo)
	paymentGateway := service.NewMockPaymentGateway(rechargeRepo, creditService)

	authHandler := handler.NewAuthHandler(authService, userService)
	adminHandler := handler.NewAdminHandler(tenantRepo, userRepo, creditService, creditRepo, rechargeRepo, modelCallLogRepo, modelCallLogService)
	userHandler := handler.NewUserHandler(userService)
	creditHandler := handler.NewCreditHandler(creditService, creditRepo)
	generateHandler := handler.NewGenerateHandler(generateService)
	apiConfigHandler := handler.NewApiConfigHandler(apiConfigRepo, creditRepo, generateService, cfg)
	proxyHandler := handler.NewProxyHandler(generateService)
	canvasHandler := handler.NewCanvasHandler(canvasRepo)
	generationRecordHandler := handler.NewGenerationRecordHandler(generationRecordRepo)
	rechargeHandler := handler.NewRechargeHandler(rechargeRepo, paymentGateway, creditService)
	captchaHandler := handler.NewCaptchaHandler(captchaService)
	tempMediaHandler := handler.NewTempMediaHandler(tempMediaService)
	channelStatusHandler := handler.NewChannelStatusHandler(channelStatusService)

	r := gin.Default()
	router.Setup(r, authService, authHandler, adminHandler, userHandler, creditHandler, generateHandler, apiConfigHandler, proxyHandler, canvasHandler, generationRecordHandler, rechargeHandler, captchaHandler, tempMediaHandler, channelStatusHandler)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
