package router

import (
	"infinite-canvas-server/handler"
	"infinite-canvas-server/middleware"
	"infinite-canvas-server/service"

	"github.com/gin-gonic/gin"
)

func Setup(r *gin.Engine, authService *service.AuthService, authHandler *handler.AuthHandler, adminHandler *handler.AdminHandler, userHandler *handler.UserHandler, creditHandler *handler.CreditHandler, generateHandler *handler.GenerateHandler, apiConfigHandler *handler.ApiConfigHandler, proxyHandler *handler.ProxyHandler, canvasHandler *handler.CanvasHandler, rechargeHandler *handler.RechargeHandler) {
	r.Use(middleware.Cors())

	api := r.Group("/api")

	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	auth := api.Group("")
	auth.Use(middleware.AuthRequired(authService))
	{
		auth.GET("/auth/me", authHandler.Me)

		auth.GET("/credits/balance", creditHandler.GetBalance)
		auth.GET("/credits/transactions", creditHandler.GetTransactions)
		auth.GET("/credits/estimate", creditHandler.EstimateCost)

		auth.GET("/api-config", apiConfigHandler.Get)
		auth.POST("/api-config", apiConfigHandler.Save)

		auth.POST("/generate/image", generateHandler.Image)
		auth.POST("/generate/text", generateHandler.Text)
		auth.POST("/generate/video", generateHandler.Video)
		auth.POST("/generate/audio", generateHandler.Audio)

		auth.POST("/proxy", proxyHandler.Proxy)
		auth.GET("/proxy", proxyHandler.ProxyGet)
		auth.GET("/proxy/*path", proxyHandler.ProxyGetPath)

		// Canvas cloud storage
		auth.POST("/canvas/save", canvasHandler.Save)
		auth.GET("/canvas/:id", canvasHandler.Load)
		auth.GET("/canvas", canvasHandler.List)
		auth.DELETE("/canvas/:id", canvasHandler.Delete)
		auth.POST("/canvas/delete-batch", canvasHandler.DeleteBatch)

		// User recharge
		auth.GET("/recharge/payouts", rechargeHandler.ListPayouts)
		auth.POST("/recharge/order", rechargeHandler.CreateOrder)
		auth.GET("/recharge/orders", rechargeHandler.ListMyOrders)

		admin := auth.Group("")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", userHandler.List)
			admin.GET("/credits/pricing", creditHandler.ListPricing)
			admin.POST("/credits/pricing", creditHandler.SavePricing)
			admin.DELETE("/credits/pricing/:id", creditHandler.DeletePricing)
			admin.POST("/credits/recharge", creditHandler.Recharge)
			admin.GET("/recharges", adminHandler.ListRecharges)
			admin.GET("/stats", adminHandler.GetStats)
			admin.GET("/users-with-balance", adminHandler.GetUsersWithBalance)
			admin.GET("/transactions", adminHandler.ListTransactions)
		}

		superAdmin := auth.Group("")
		superAdmin.Use(middleware.SuperAdminRequired())
		{
			superAdmin.GET("/admin/tenants", adminHandler.ListTenants)
			superAdmin.GET("/admin/users", adminHandler.ListAllUsers)
			superAdmin.POST("/admin/credits/adjust", adminHandler.AdjustCredits)
			superAdmin.GET("/admin/recharges", adminHandler.ListAllRecharges)
		}
	}
}
