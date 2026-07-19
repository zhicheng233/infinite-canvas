package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type CreditHandler struct {
	creditService *service.CreditService
	creditRepo    *repository.CreditRepo
	generateSvc   *service.GenerateService
}

func NewCreditHandler(creditService *service.CreditService, creditRepo *repository.CreditRepo, generateSvc *service.GenerateService) *CreditHandler {
	return &CreditHandler{creditService: creditService, creditRepo: creditRepo, generateSvc: generateSvc}
}

func (h *CreditHandler) GetBalance(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	account, err := h.creditService.GetOrCreateAccount(claims.TenantID, claims.UserID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, gin.H{
		"balance":      account.Balance,
		"total_earned": account.TotalEarned,
		"total_spent":  account.TotalSpent,
	})
}

func (h *CreditHandler) GetTransactions(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	account, err := h.creditService.GetOrCreateAccount(claims.TenantID, claims.UserID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	txs, total, err := h.creditRepo.ListTransactions(account.ID, page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, txs, total, page, pageSize)
}

func (h *CreditHandler) ListPricing(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	items, err := h.creditRepo.ListPricing(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, items)
}

func (h *CreditHandler) SavePricing(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var pricing model.CreditPricing
	if err := c.ShouldBindJSON(&pricing); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	pricing.Model = strings.TrimSpace(pricing.Model)
	if pricing.Model == "" {
		model.Fail(c, 400, "模型名称不能为空")
		return
	}
	if pricing.PricingMode == "" {
		pricing.PricingMode = model.PricingModePerUnit
	}
	if pricing.PricingMode == model.PricingModeVideoDynamic || pricing.UnitType == model.UnitPerVideoSecond {
		pricing.PricingMode = model.PricingModeVideoDynamic
		pricing.UnitType = model.UnitPerVideoSecond
		var rule model.VideoPricingRule
		if err := json.Unmarshal([]byte(pricing.PricingRule), &rule); err != nil {
			model.Fail(c, 400, "视频动态计费规则格式错误")
			return
		}
		if !hasPositiveVideoRate(rule.ResolutionSecondRates) {
			model.Fail(c, 400, "请至少配置一个分辨率秒单价")
			return
		}
	} else if pricing.CreditsPerUnit <= 0 {
		model.Fail(c, 400, "每次消耗积分必须大于 0")
		return
	}
	pricing.TenantID = claims.TenantID
	if err := h.creditRepo.SavePricing(&pricing); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, pricing)
}

func hasPositiveVideoRate(items map[string]int) bool {
	for _, value := range items {
		if value > 0 {
			return true
		}
	}
	return false
}

func (h *CreditHandler) DeletePricing(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, 400, "无效的ID")
		return
	}
	if err := h.creditRepo.DeletePricing(uint(id)); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, nil)
}

type CreditRechargeInput struct {
	UserID uint   `json:"user_id"`
	Amount int    `json:"amount"`
	Note   string `json:"note"`
}

func (h *CreditHandler) Recharge(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input CreditRechargeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Amount <= 0 {
		model.Fail(c, 400, "金额必须为正数")
		return
	}

	// Verify the target user belongs to the same tenant
	_, err := h.creditService.GetOrCreateAccount(claims.TenantID, input.UserID)
	if err != nil {
		model.Fail(c, 500, "未找到用户账户")
		return
	}

	note := input.Note
	if note == "" {
		note = "管理员充值"
	}
	metadata := service.BuildCreditMetadata(map[string]interface{}{
		"scene":            "后台充值",
		"operator_user_id": claims.UserID,
		"target_user_id":   input.UserID,
		"credits":          input.Amount,
	})
	if err := h.creditService.EarnWithMetadata(input.UserID, input.Amount, "recharge", "", note, metadata); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	// Get updated balance
	account, _ := h.creditService.GetOrCreateAccount(claims.TenantID, input.UserID)
	model.OK(c, gin.H{
		"user_id": input.UserID,
		"amount":  input.Amount,
		"balance": account.Balance,
		"message": "充值成功",
	})
}

func (h *CreditHandler) EstimateCost(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		model.Fail(c, 400, "请指定模型")
		return
	}
	genType := strings.TrimSpace(c.DefaultQuery("type", ""))
	if h.generateSvc != nil {
		selection := service.ChannelSelection{ChannelID: parseUintQuery(c.Query("channel_id")), ChannelModelID: parseUintQuery(c.Query("channel_model_id"))}
		if err := h.generateSvc.ResolveChannelRouteForEstimate(selection, genType, modelName); err != nil {
			model.Fail(c, 400, err.Error())
			return
		}
	}
	pricing, err := h.creditRepo.FindPricing(claims.TenantID, modelName)
	if err != nil {
		model.Fail(c, 403, "该模型未配置计费，暂不可用")
		return
	}
	fields := map[string]interface{}{}
	if seconds := strings.TrimSpace(c.Query("seconds")); seconds != "" {
		fields["seconds"] = seconds
	}
	if duration := strings.TrimSpace(c.Query("duration")); duration != "" {
		fields["duration"] = duration
	}
	if resolution := strings.TrimSpace(c.Query("resolution")); resolution != "" {
		fields["resolution"] = resolution
	}
	if size := strings.TrimSpace(c.Query("size")); size != "" {
		fields["size"] = size
	}
	if count := strings.TrimSpace(c.Query("count")); count != "" {
		fields["n"] = count
	}
	body, _ := json.Marshal(fields)
	if genType == "" {
		if pricing.PricingMode == model.PricingModeVideoDynamic || pricing.UnitType == model.UnitPerVideo || pricing.UnitType == model.UnitPerVideoSecond {
			genType = "video"
		} else if pricing.UnitType == model.UnitPerImage {
			genType = "image"
		}
	}
	cost, err := service.CalculateCreditCost(pricing, genType, "application/json", body)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, gin.H{
		"model":            pricing.Model,
		"credits_per_unit": pricing.CreditsPerUnit,
		"unit_type":        pricing.UnitType,
		"pricing_mode":     pricing.PricingMode,
		"pricing_rule":     pricing.PricingRule,
		"total_cost":       cost.TotalCost,
		"unit_cost":        cost.UnitCost,
		"units":            cost.Units,
		"seconds":          cost.Seconds,
		"resolution":       cost.Resolution,
		"formula":          cost.Formula,
	})
}

func parseUintQuery(value string) uint {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return uint(parsed)
}
