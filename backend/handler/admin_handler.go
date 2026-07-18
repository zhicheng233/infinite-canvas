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
}

func NewAdminHandler(tenantRepo *repository.TenantRepo, userRepo *repository.UserRepo, creditService *service.CreditService, creditRepo *repository.CreditRepo, rechargeRepo *repository.RechargeRepo, modelLogRepo *repository.ModelCallLogRepo, modelLogSvc *service.ModelCallLogService) *AdminHandler {
	return &AdminHandler{tenantRepo: tenantRepo, userRepo: userRepo, creditService: creditService, creditRepo: creditRepo, rechargeRepo: rechargeRepo, modelLogRepo: modelLogRepo, modelLogSvc: modelLogSvc}
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

type AdjustCreditsInput struct {
	UserID uint   `json:"user_id"`
	Amount int    `json:"amount"`
	Note   string `json:"note"`
}

func (h *AdminHandler) AdjustCredits(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input AdjustCreditsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Amount == 0 {
		model.Fail(c, 400, "金额不能为零")
		return
	}
	note := input.Note
	if note == "" {
		note = "管理员调整积分"
	}
	metadata := service.BuildCreditMetadata(map[string]interface{}{
		"scene":            "后台调整",
		"operator_user_id": claims.UserID,
		"target_user_id":   input.UserID,
		"adjustment":       input.Amount,
	})
	if input.Amount > 0 {
		if err := h.creditService.EarnWithMetadata(input.UserID, input.Amount, "adjust", "", note, metadata); err != nil {
			model.Fail(c, 500, err.Error())
			return
		}
	} else {
		if err := h.creditService.SpendWithMetadata(0, input.UserID, -input.Amount, "adjust", "", note, metadata); err != nil {
			model.Fail(c, 500, err.Error())
			return
		}
	}
	model.OK(c, gin.H{"adjusted": true})
}

type RechargeInput struct {
	UserID  uint   `json:"user_id"`
	Credits int    `json:"credits"`
	Note    string `json:"note"`
}

func (h *AdminHandler) RechargeCredits(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input RechargeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Credits <= 0 {
		model.Fail(c, 400, "积分必须为正数")
		return
	}

	order := &model.RechargeOrder{
		TenantID: claims.TenantID,
		UserID:   input.UserID,
		Amount:   0,
		Credits:  input.Credits,
		Status:   "completed",
		Note:     input.Note,
	}
	if err := h.rechargeRepo.Create(order); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	note := input.Note
	if note == "" {
		note = "管理员充值"
	}
	metadata := service.BuildCreditMetadata(map[string]interface{}{
		"scene":             "后台充值",
		"operator_user_id":  claims.UserID,
		"target_user_id":    input.UserID,
		"recharge_order_id": order.ID,
		"credits":           input.Credits,
	})
	if err := h.creditService.EarnWithMetadata(input.UserID, input.Credits, "recharge", strconv.FormatUint(uint64(order.ID), 10), note, metadata); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	model.OK(c, order)
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
	_, totalUsers, err := h.userRepo.List(claims.TenantID, 1, 1)
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

type UserWithBalance struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Balance     int    `json:"balance"`
}

func (h *AdminHandler) GetUsersWithBalance(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	users, total, err := h.userRepo.List(claims.TenantID, page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	// Collect user IDs
	userIDs := make([]uint, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	// Fetch balances
	balances, _ := h.creditRepo.GetBalancesByUserIDs(userIDs)

	// Build response
	items := make([]UserWithBalance, len(users))
	for i, u := range users {
		items[i] = UserWithBalance{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Role:        string(u.Role),
			Status:      string(u.Status),
			Balance:     balances[u.ID],
		}
	}

	model.OKPage(c, items, total, page, pageSize)
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
