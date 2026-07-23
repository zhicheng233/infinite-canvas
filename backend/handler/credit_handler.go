package handler

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type CreditHandler struct {
	creditService         *service.CreditService
	creditRepo            *repository.CreditRepo
	generateSvc           *service.GenerateService
	channelModelRepo      *repository.ChannelModelRepo
	channelRepo           *repository.ChannelRepo
	estimatePricingRepo   estimatePricingReader
	estimateRouteResolver estimateRouteResolver
}

type estimatePricingReader interface {
	FindPricing(tenantID uint, modelName string, channelID uint) (*model.CreditPricing, error)
}

type estimateRouteResolver interface {
	ResolveChannelRouteForEstimate(tenantID uint, selection service.ChannelSelection, capability, modelName, fuzzyGroupName string) (service.ResolvedEstimateRoute, error)
}

func NewCreditHandler(creditService *service.CreditService, creditRepo *repository.CreditRepo, generateSvc *service.GenerateService, channelModelRepo *repository.ChannelModelRepo, channelRepo *repository.ChannelRepo) *CreditHandler {
	return &CreditHandler{
		creditService: creditService, creditRepo: creditRepo, generateSvc: generateSvc, channelModelRepo: channelModelRepo, channelRepo: channelRepo,
		estimatePricingRepo: creditRepo, estimateRouteResolver: generateSvc,
	}
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
	channelID := parseUintQuery(c.Query("channel_id"))
	items, err := h.creditRepo.ListPricing(claims.TenantID, channelID)
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
	fuzzyGroupName := strings.TrimSpace(c.Query("fuzzy_group_name"))
	selection, err := parseEstimateChannelSelection(c, fuzzyGroupName)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	resolver := h.estimateRouteResolver
	if resolver == nil && h.generateSvc != nil {
		resolver = h.generateSvc
	}
	if resolver != nil {
		resolved, err := resolver.ResolveChannelRouteForEstimate(claims.TenantID, selection, genType, modelName, fuzzyGroupName)
		if err != nil {
			model.Fail(c, 400, err.Error())
			return
		}
		selection = resolved.Selection
		modelName = resolved.PricingModel
	}
	pricingRepo := h.estimatePricingRepo
	if pricingRepo == nil {
		pricingRepo = h.creditRepo
	}
	if pricingRepo == nil {
		model.Fail(c, 500, "查询模型计费失败")
		return
	}
	pricing, err := pricingRepo.FindPricing(claims.TenantID, modelName, selection.ChannelID)
	if err != nil {
		model.Fail(c, 500, "查询模型计费失败")
		return
	}
	if pricing == nil {
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

// ComparePricingResponse is returned by ComparePricing.
type ComparePricingResponse struct {
	ChannelID   uint                 `json:"channel_id"`
	ChannelName string               `json:"channel_name"`
	HasModel    bool                 `json:"has_model"`
	Pricing     *model.CreditPricing `json:"pricing,omitempty"`
}

func (h *CreditHandler) ComparePricing(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		model.Fail(c, 400, "请指定模型")
		return
	}

	channelModels, err := h.channelModelRepo.FindByModelName(modelName)
	if err != nil {
		model.Fail(c, 500, "查询渠道模型失败")
		return
	}

	channels := make([]ComparePricingResponse, 0, len(channelModels))
	for _, cm := range channelModels {
		channel, err := h.channelRepo.FindByID(cm.ChannelID)
		if err != nil {
			continue
		}
		entry := ComparePricingResponse{
			ChannelID:   cm.ChannelID,
			ChannelName: channel.Name,
			HasModel:    true,
		}
		// Try channel-specific pricing first, then fall back to global (channel_id=0)
		pricing, err := h.creditRepo.FindPricing(claims.TenantID, modelName, cm.ChannelID)
		if err != nil {
			pricing, err = h.creditRepo.FindPricing(claims.TenantID, modelName, 0)
		}
		if err == nil {
			entry.Pricing = pricing
		}
		channels = append(channels, entry)
	}

	model.OK(c, gin.H{"channels": channels})
}

func parseUintQuery(value string) uint {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return uint(parsed)
}

func parseEstimateChannelSelection(c *gin.Context, fuzzyGroupName string) (service.ChannelSelection, error) {
	rawChannelID, exists := c.GetQuery("channel_id")
	if !exists || strings.TrimSpace(rawChannelID) == "" {
		return service.ChannelSelection{}, errors.New("请指定有效的渠道")
	}
	channelID, err := strconv.ParseUint(strings.TrimSpace(rawChannelID), 10, strconv.IntSize)
	if err != nil {
		return service.ChannelSelection{}, errors.New("渠道参数无效")
	}
	selection := service.ChannelSelection{ChannelID: uint(channelID)}
	if rawChannelModelID, exists := c.GetQuery("channel_model_id"); exists && strings.TrimSpace(rawChannelModelID) != "" {
		channelModelID, err := strconv.ParseUint(strings.TrimSpace(rawChannelModelID), 10, strconv.IntSize)
		if err != nil {
			return service.ChannelSelection{}, errors.New("渠道模型参数无效")
		}
		selection.ChannelModelID = uint(channelModelID)
	}
	if fuzzyGroupName != "" {
		if selection.ChannelID == 0 || selection.ChannelModelID != 0 {
			return service.ChannelSelection{}, errors.New("模型合并组参数无效")
		}
		return selection, nil
	}
	if selection.ChannelID == 0 {
		if selection.ChannelModelID != 0 {
			return service.ChannelSelection{}, errors.New("Auto 渠道参数无效")
		}
		return selection, nil
	}
	if selection.ChannelModelID == 0 {
		return service.ChannelSelection{}, errors.New("请选择有效的渠道模型")
	}
	return selection, nil
}
