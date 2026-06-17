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
}

func NewAdminHandler(tenantRepo *repository.TenantRepo, userRepo *repository.UserRepo, creditService *service.CreditService, creditRepo *repository.CreditRepo, rechargeRepo *repository.RechargeRepo) *AdminHandler {
	return &AdminHandler{tenantRepo: tenantRepo, userRepo: userRepo, creditService: creditService, creditRepo: creditRepo, rechargeRepo: rechargeRepo}
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
	var input AdjustCreditsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Amount == 0 {
		model.Fail(c, 400, "金额不能为零")
		return
	}
	if input.Amount > 0 {
		if err := h.creditService.Earn(input.UserID, input.Amount, "adjust", "", input.Note); err != nil {
			model.Fail(c, 500, err.Error())
			return
		}
	} else {
		if err := h.creditService.Spend(0, input.UserID, -input.Amount, "adjust", "", input.Note); err != nil {
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

	if err := h.creditService.Earn(input.UserID, input.Credits, "recharge", strconv.FormatUint(uint64(order.ID), 10), input.Note); err != nil {
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
		"total_users":           totalUsers,
		"total_credits_earned":  totalEarned,
		"total_credits_spent":   totalSpent,
		"total_recharged":       rechargeTotal,
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
