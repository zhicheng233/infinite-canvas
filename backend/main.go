package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"infinite-canvas-server/config"
	"infinite-canvas-server/handler"
	"infinite-canvas-server/migrations"
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
		&model.Channel{},
		&model.ChannelModel{},
		&model.VideoConfigPreset{},
		&model.MetricsConfig{},
		&model.RechargeOrder{},
		&model.ModelCallLog{},
		&model.ModelMergeGroup{},
		&model.WebhookConfig{},
		&model.WebhookLog{},
		&model.SiteAnnouncement{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to access database connection: %v", err)
	}
	if err := migrations.Up(sqlDB); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	userRepo := repository.NewUserRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	creditRepo := repository.NewCreditRepo(db)
	generationJobRepo := repository.NewGenerationJobRepo(db)
	autoRoutingRepo := repository.NewAutoRoutingRepo(db)
	rechargeRepo := repository.NewRechargeRepo(db)
	canvasRepo := repository.NewCanvasRepo(db)
	generationRecordRepo := repository.NewGenerationRecordRepo(db)
	modelCallLogRepo := repository.NewModelCallLogRepo(db)
	channelRepo := repository.NewChannelRepo(db)
	channelModelRepo := repository.NewChannelModelRepo(db)
	videoConfigPresetRepo := repository.NewVideoConfigPresetRepo(db)
	metricsConfigRepo := repository.NewMetricsConfigRepo(db)
	webhookRepo := repository.NewWebhookRepo(db)
	mergeGroupRepo := repository.NewMergeGroupRepo(db)
	apiConfigTransferRepo := repository.NewAPIConfigTransferRepo(db)
	siteAnnouncementRepo := repository.NewSiteAnnouncementRepo(db)
	modelConfigRepo := repository.NewModelConfigRepo(db)
	apiConfigRepo := repository.NewApiConfigRepo(db)

	captchaService := service.NewCaptchaService()

	authService := service.NewAuthService(cfg, userRepo, tenantRepo, creditRepo, captchaService)
	if err := authService.EnsureInitialAdmin(); err != nil {
		log.Fatalf("failed to bootstrap initial admin: %v", err)
	}
	userService := service.NewUserService(userRepo)
	creditService := service.NewCreditService(creditRepo)
	generationBillingService := service.NewGenerationBillingService(generationJobRepo)
	channelService := service.NewChannelService(channelRepo, cfg.ApiKeyEncryptKey)
	channelModelService := service.NewChannelModelService(channelService, channelRepo, channelModelRepo, creditRepo)
	videoConfigPresetService := service.NewVideoConfigPresetService(videoConfigPresetRepo)
	metricsService := service.NewMetricsService(metricsConfigRepo, channelRepo, channelModelRepo)
	modelCallLogService := service.NewModelCallLogService(modelCallLogRepo, userRepo)
	onDemandRepairService := service.NewOnDemandRepairService(cfg.OnDemandRepairURL, cfg.OnDemandRepairUser, cfg.OnDemandRepairPass, cfg.OnDemandRepairTimeoutSeconds)
	autoChannelService := service.NewAutoChannelService(db, channelRepo, channelModelRepo, creditRepo, autoRoutingRepo)
	webhookService := service.NewWebhookService(webhookRepo)
	siteAnnouncementService := service.NewSiteAnnouncementService(siteAnnouncementRepo)
	generateService := service.NewGenerateService(creditService, creditRepo, generationBillingService, modelCallLogService, cfg.ApiKeyEncryptKey, onDemandRepairService, channelService, channelRepo, channelModelRepo, mergeGroupRepo, db, autoChannelService, webhookService)
	tempMediaService := service.NewTempMediaService(cfg)
	channelStatusService := service.NewChannelStatusService(modelCallLogRepo)
	paymentGateway := service.NewMockPaymentGateway(rechargeRepo, creditService)
	mergeGroupService := service.NewMergeGroupService(mergeGroupRepo)
	apiConfigTransferService := service.NewAPIConfigTransferService(apiConfigTransferRepo, cfg.ApiKeyEncryptKey)
	modelConfigService := service.NewModelConfigService(modelConfigRepo, channelService, generateService)
	legacyAPIConfigService := service.NewLegacyAPIConfigService(db)
	if err := legacyAPIConfigService.MigrateAll(); err != nil {
		log.Fatalf("failed to migrate legacy API config: %v", err)
	}

	authHandler := handler.NewAuthHandler(authService, userService)
	adminHandler := handler.NewAdminHandler(tenantRepo, userRepo, creditService, creditRepo, rechargeRepo, modelCallLogRepo, modelCallLogService)
	userHandler := handler.NewUserHandler(userService)
	creditHandler := handler.NewCreditHandler(creditService, creditRepo, generateService, channelModelRepo, channelRepo)
	generateHandler := handler.NewGenerateHandler(generateService)
	apiConfigHandler := handler.NewApiConfigHandler(apiConfigRepo, creditRepo, channelModelService, generateService, legacyAPIConfigService, cfg)
	proxyHandler := handler.NewProxyHandler(generateService)
	canvasHandler := handler.NewCanvasHandler(canvasRepo)
	generationRecordHandler := handler.NewGenerationRecordHandler(generationRecordRepo)
	rechargeHandler := handler.NewRechargeHandler(rechargeRepo, paymentGateway, creditService)
	captchaHandler := handler.NewCaptchaHandler(captchaService)
	tempMediaHandler := handler.NewTempMediaHandler(tempMediaService)
	channelStatusHandler := handler.NewChannelStatusHandler(channelStatusService)
	channelHandler := handler.NewChannelHandler(channelService, autoChannelService)
	channelModelHandler := handler.NewChannelModelHandler(channelModelService)
	videoConfigPresetHandler := handler.NewVideoConfigPresetHandler(videoConfigPresetService)
	metricsHandler := handler.NewMetricsHandler(metricsService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	mergeGroupHandler := handler.NewMergeGroupHandler(mergeGroupService)
	apiConfigTransferHandler := handler.NewAPIConfigTransferHandler(apiConfigTransferService)
	autoRoutingHandler := handler.NewAutoRoutingHandler(autoChannelService)
	siteAnnouncementHandler := handler.NewSiteAnnouncementHandler(siteAnnouncementService)
	modelConfigHandler := handler.NewModelConfigHandler(modelConfigService)

	r := gin.Default()
	router.Setup(r, router.Dependencies{
		AuthService: authService, AuthHandler: authHandler, AdminHandler: adminHandler, UserHandler: userHandler,
		CreditHandler: creditHandler, GenerateHandler: generateHandler, APIConfigHandler: apiConfigHandler,
		VideoConfigPresetHandler: videoConfigPresetHandler, ProxyHandler: proxyHandler, CanvasHandler: canvasHandler,
		GenerationRecordHandler: generationRecordHandler, RechargeHandler: rechargeHandler, CaptchaHandler: captchaHandler,
		TempMediaHandler: tempMediaHandler, ChannelStatusHandler: channelStatusHandler, ChannelHandler: channelHandler,
		ChannelModelHandler: channelModelHandler, MetricsHandler: metricsHandler, WebhookHandler: webhookHandler,
		MergeGroupHandler: mergeGroupHandler, APIConfigTransferHandler: apiConfigTransferHandler,
		AutoRoutingHandler:      autoRoutingHandler,
		SiteAnnouncementHandler: siteAnnouncementHandler,
		ModelConfigHandler:      modelConfigHandler,
	})

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
