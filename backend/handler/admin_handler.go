package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type AdminHandler struct {
	tenantRepo    *repository.TenantRepo
	userRepo      *repository.UserRepo
	creditService *service.CreditService
	creditRepo    *repository.CreditRepo
	rechargeRepo  *repository.RechargeRepo
	modelLogRepo  *repository.ModelCallLogRepo
	modelLogSvc   *service.ModelCallLogService
	adminUserSvc  *service.AdminUserService
}

func NewAdminHandler(tenantRepo *repository.TenantRepo, userRepo *repository.UserRepo, creditService *service.CreditService, creditRepo *repository.CreditRepo, rechargeRepo *repository.RechargeRepo, modelLogRepo *repository.ModelCallLogRepo, modelLogSvc *service.ModelCallLogService) *AdminHandler {
	return &AdminHandler{tenantRepo: tenantRepo, userRepo: userRepo, creditService: creditService, creditRepo: creditRepo, rechargeRepo: rechargeRepo, modelLogRepo: modelLogRepo, modelLogSvc: modelLogSvc, adminUserSvc: service.NewAdminUserService(userRepo, creditRepo)}
}

func (h *AdminHandler) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	tenants, total, err := h.tenantRepo.List(page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, tenants, total, page, pageSize)
}

func (h *AdminHandler) ListAllUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	users, total, err := h.userRepo.ListAll(page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, users, total, page, pageSize)
}

func (h *AdminHandler) ListRecharges(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	orders, total, err := h.rechargeRepo.ListByTenant(claims.TenantID, page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, orders, total, page, pageSize)
}

func (h *AdminHandler) ListAllRecharges(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	orders, total, err := h.rechargeRepo.ListAll(page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, orders, total, page, pageSize)
}
func (h *AdminHandler) GetStats(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)

	// Total users in tenant
	_, totalUsers, err := h.userRepo.List(repository.UserListQuery{TenantID: claims.TenantID, Page: 1, PageSize: 1})
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	// Credit stats
	totalEarned, err := h.creditRepo.SumEarnedByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	totalSpent, err := h.creditRepo.SumSpentByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	// Recharge total
	rechargeTotal, err := h.rechargeRepo.SumCompletedByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	model.OK(c, gin.H{
		"total_users":          totalUsers,
		"total_credits_earned": totalEarned,
		"total_credits_spent":  totalSpent,
		"total_recharged":      rechargeTotal,
	})
}

func (h *AdminHandler) GetUsersWithBalance(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	query := (repository.UserListQuery{TenantID: claims.TenantID, Page: page, PageSize: pageSize, Keyword: c.Query("keyword")}).Normalize()
	items, total, err := h.adminUserSvc.ListUsersWithBalance(query)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, items, total, query.Page, query.PageSize)
}

func (h *AdminHandler) ListTransactions(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	txs, total, err := h.creditRepo.ListTransactionsByTenant(claims.TenantID, page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, txs, total, page, pageSize)
}

func (h *AdminHandler) ListModelCallLogs(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	items, total, err := h.modelLogRepo.List(claims.TenantID, repository.ModelCallLogQuery{
		Page:       page,
		PageSize:   pageSize,
		UserID:     uint(userID),
		Model:      c.Query("model"),
		Generation: c.Query("generation"),
		Keyword:    c.Query("keyword"),
		Status:     c.DefaultQuery("status", "failure"),
	})
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, items, total, page, pageSize)
}

func (h *AdminHandler) GetModelHealth(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	summary, err := h.modelLogSvc.HealthSummary(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, summary)
}
