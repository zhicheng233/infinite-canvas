package router

import (
	"infinite-canvas-server/handler"
	"infinite-canvas-server/middleware"
	"infinite-canvas-server/service"

	"github.com/gin-gonic/gin"
)

func Setup(r *gin.Engine, authService *service.AuthService, authHandler *handler.AuthHandler, adminHandler *handler.AdminHandler, userHandler *handler.UserHandler, creditHandler *handler.CreditHandler, generateHandler *handler.GenerateHandler, apiConfigHandler *handler.ApiConfigHandler, proxyHandler *handler.ProxyHandler, canvasHandler *handler.CanvasHandler, generationRecordHandler *handler.GenerationRecordHandler, rechargeHandler *handler.RechargeHandler, captchaHandler *handler.CaptchaHandler, tempMediaHandler *handler.TempMediaHandler, channelStatusHandler *handler.ChannelStatusHandler, channelHandler *handler.ChannelHandler, channelModelHandler *handler.ChannelModelHandler, metricsHandler *handler.MetricsHandler, webhookHandler *handler.WebhookHandler, mergeGroupHandler *handler.MergeGroupHandler) {
	r.Use(middleware.Cors())

	api := r.Group("/backend-api")
	api.GET("/media/tmp/:filename", tempMediaHandler.Serve)
	api.GET("/channel-status", channelStatusHandler.GetChannelStatus)

	api.GET("/auth/captcha", captchaHandler.Generate)
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	auth := api.Group("")
	auth.Use(middleware.AuthRequired(authService))
	{
		auth.GET("/auth/me", authHandler.Me)
		auth.PUT("/auth/password", authHandler.ChangePassword)
		auth.PUT("/auth/profile", authHandler.UpdateProfile)

		auth.GET("/credits/balance", creditHandler.GetBalance)
		auth.GET("/credits/transactions", creditHandler.GetTransactions)
		auth.GET("/credits/estimate", creditHandler.EstimateCost)
		auth.GET("/api-config/catalog", apiConfigHandler.Catalog)
		auth.GET("/channels", channelHandler.List)
		auth.GET("/channels/metrics", metricsHandler.Read)
		auth.GET("/channels/:id/models", channelModelHandler.List)
		auth.POST("/media/tmp", tempMediaHandler.UploadImage)

		auth.POST("/generate/image", generateHandler.Image)
		auth.POST("/generate/text", generateHandler.Text)
		auth.POST("/generate/video", generateHandler.Video)
		auth.POST("/generate/audio", generateHandler.Audio)

		auth.POST("/proxy", proxyHandler.Proxy)
		auth.GET("/proxy", proxyHandler.ProxyGet)
		auth.GET("/proxy/*path", proxyHandler.ProxyGetPath)

		auth.POST("/canvas/save", canvasHandler.Save)
		auth.GET("/canvas/:id", canvasHandler.Load)
		auth.GET("/canvas", canvasHandler.List)
		auth.DELETE("/canvas/:id", canvasHandler.Delete)
		auth.POST("/canvas/delete-batch", canvasHandler.DeleteBatch)

		auth.POST("/generation-records/save", generationRecordHandler.Save)
		auth.GET("/generation-records", generationRecordHandler.List)
		auth.DELETE("/generation-records/:id", generationRecordHandler.Delete)
		auth.POST("/generation-records/delete-batch", generationRecordHandler.DeleteBatch)

		auth.GET("/recharge/payouts", rechargeHandler.ListPayouts)
		auth.POST("/recharge/order", rechargeHandler.CreateOrder)
		auth.GET("/recharge/orders", rechargeHandler.ListMyOrders)

		admin := auth.Group("")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", userHandler.List)
			admin.GET("/api-config", apiConfigHandler.Get)
			admin.POST("/api-config", apiConfigHandler.Save)
			admin.POST("/api-config/test-model", apiConfigHandler.TestModel)
			admin.GET("/credits/pricing", creditHandler.ListPricing)
			admin.GET("/credits/pricing/compare", creditHandler.ComparePricing)
			admin.POST("/credits/pricing", creditHandler.SavePricing)
			admin.DELETE("/credits/pricing/:id", creditHandler.DeletePricing)
			admin.POST("/credits/recharge", creditHandler.Recharge)
			admin.GET("/recharges", adminHandler.ListRecharges)
			admin.GET("/stats", adminHandler.GetStats)
			admin.GET("/users-with-balance", adminHandler.GetUsersWithBalance)
			admin.GET("/transactions", adminHandler.ListTransactions)
			admin.GET("/model-health", adminHandler.GetModelHealth)
			admin.GET("/model-call-logs", adminHandler.ListModelCallLogs)
		}

		superAdmin := auth.Group("")
		superAdmin.Use(middleware.SuperAdminRequired())
		{
			superAdmin.GET("/admin/tenants", adminHandler.ListTenants)
			superAdmin.GET("/admin/users", adminHandler.ListAllUsers)
			superAdmin.POST("/admin/credits/adjust", adminHandler.AdjustCredits)
			superAdmin.GET("/admin/recharges", adminHandler.ListAllRecharges)
			superAdmin.GET("/admin/channels", channelHandler.ListAdmin)
			superAdmin.POST("/admin/channels", channelHandler.Create)
			superAdmin.PUT("/admin/channels/:id", channelHandler.Update)
			superAdmin.POST("/admin/channels/:id/disable", channelHandler.Disable)
			superAdmin.POST("/admin/channels/:id/enable", channelHandler.Enable)
			superAdmin.DELETE("/admin/channels/:id", channelHandler.Delete)
			superAdmin.GET("/admin/channels/:id/models", channelModelHandler.ListAdmin)
			superAdmin.POST("/admin/channels/:id/models/sync", channelModelHandler.Sync)
			superAdmin.PUT("/admin/channels/:id/models/:modelId", channelModelHandler.Update)
			superAdmin.GET("/admin/channels/:id/merge-groups", mergeGroupHandler.List)
			superAdmin.POST("/admin/channels/:id/merge-groups", mergeGroupHandler.Create)
			superAdmin.DELETE("/admin/channels/:id/merge-groups/:groupId", mergeGroupHandler.Delete)
			superAdmin.POST("/admin/channels/:id/merge-groups/auto", mergeGroupHandler.AutoCreate)
			superAdmin.GET("/admin/metrics-config", metricsHandler.GetConfig)
			superAdmin.POST("/admin/metrics-config", metricsHandler.SaveConfig)

			superAdmin.GET("/admin/webhook/config", webhookHandler.ListConfig)
			superAdmin.PUT("/admin/webhook/config", webhookHandler.SaveConfig)
			superAdmin.POST("/admin/webhook/test", webhookHandler.TestSend)
			superAdmin.GET("/admin/webhook/logs", webhookHandler.ListLogs)
			superAdmin.POST("/admin/webhook/poller/start", webhookHandler.StartPoller)
			superAdmin.POST("/admin/webhook/poller/stop", webhookHandler.StopPoller)
			superAdmin.GET("/admin/webhook/poller/status", webhookHandler.PollerStatus)
		}
	}
}
