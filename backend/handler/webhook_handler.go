package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type WebhookHandler struct {
	webhookRepo *repository.WebhookRepo
	poller      *service.WebhookPoller
	sender      service.WebhookSender
}

func NewWebhookHandler(webhookRepo *repository.WebhookRepo, poller *service.WebhookPoller, sender service.WebhookSender) *WebhookHandler {
	return &WebhookHandler{webhookRepo: webhookRepo, poller: poller, sender: sender}
}

func (h *WebhookHandler) ListConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	configs, err := h.webhookRepo.ListByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, configs)
}

type webhookConfigInput struct {
	Platform        string  `json:"platform"`
	WebhookURL      *string `json:"webhook_url"`
	Enabled         *bool   `json:"enabled"`
	TemplateDown    *string `json:"template_down"`
	TemplateUp      *string `json:"template_up"`
	IntervalSeconds *int    `json:"interval_seconds"`
	CooldownMinutes *int    `json:"cooldown_minutes"`
}

func (h *WebhookHandler) SaveConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input webhookConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	updates := map[string]interface{}{}
	if input.WebhookURL != nil {
		updates["webhook_url"] = strings.TrimSpace(*input.WebhookURL)
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if input.TemplateDown != nil {
		updates["template_down"] = *input.TemplateDown
	}
	if input.TemplateUp != nil {
		updates["template_up"] = *input.TemplateUp
	}
	if input.IntervalSeconds != nil {
		if *input.IntervalSeconds <= 0 {
			model.Fail(c, 400, "轮询间隔必须大于 0")
			return
		}
		updates["interval_seconds"] = *input.IntervalSeconds
	}
	if input.CooldownMinutes != nil {
		if *input.CooldownMinutes < 0 {
			model.Fail(c, 400, "冷却时间不能小于 0")
			return
		}
		updates["cooldown_minutes"] = *input.CooldownMinutes
	}

	platform := strings.TrimSpace(input.Platform)
	if platform == "" {
		if input.IntervalSeconds != nil && len(updates) == 1 {
			h.poller.SetIntervalSeconds(*input.IntervalSeconds)
			model.OK(c, gin.H{"interval_seconds": *input.IntervalSeconds})
			return
		}
		model.Fail(c, 400, "platform 不能为空")
		return
	}
	cfg, err := h.webhookRepo.SavePatch(claims.TenantID, platform, updates)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	if input.IntervalSeconds != nil {
		h.poller.SetIntervalSeconds(*input.IntervalSeconds)
	}
	model.OK(c, cfg)
}

type testSendInput struct {
	Platform string `json:"platform"`
	Message  string `json:"message"`
}

func (h *WebhookHandler) TestSend(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input testSendInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Platform == "" || input.Message == "" {
		model.Fail(c, 400, "platform 和 message 不能为空")
		return
	}

	cfg, err := h.webhookRepo.GetByPlatform(claims.TenantID, input.Platform)
	if err != nil {
		model.Fail(c, 404, "未找到该平台的 webhook 配置")
		return
	}

	sender := service.NewSender(input.Platform)
	if sender == nil {
		model.Fail(c, 400, "不支持的平台: "+input.Platform)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sendErr := sender.Send(ctx, cfg.WebhookURL, input.Message)

	logEntry := &model.WebhookLog{
		TenantID:  claims.TenantID,
		Platform:  input.Platform,
		ModelName: "",
		Status:    "test",
		Message:   input.Message,
		Success:   sendErr == nil,
	}
	if sendErr != nil {
		logEntry.ResponseBody = sendErr.Error()
	}
	if logErr := h.webhookRepo.InsertLog(logEntry); logErr != nil {
		model.Fail(c, 500, logErr.Error())
		return
	}

	model.OK(c, gin.H{
		"success": sendErr == nil,
		"error": func() string {
			if sendErr != nil {
				return sendErr.Error()
			}
			return ""
		}(),
	})
}

func (h *WebhookHandler) ListLogs(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, err := h.webhookRepo.ListLogs(claims.TenantID, limit)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, logs)
}

func (h *WebhookHandler) StartPoller(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	h.poller.SetIntervalSeconds(h.webhookRepo.IntervalSeconds(claims.TenantID))
	if err := h.poller.Start(); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, gin.H{"started": true})
}

func (h *WebhookHandler) StopPoller(c *gin.Context) {
	_ = c.MustGet("claims").(*service.Claims)
	h.poller.Stop()
	model.OK(c, gin.H{"stopped": true})
}

func (h *WebhookHandler) PollerStatus(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	h.poller.SetIntervalSeconds(h.webhookRepo.IntervalSeconds(claims.TenantID))
	model.OK(c, gin.H{
		"running":          h.poller.IsRunning(),
		"interval_seconds": h.poller.IntervalSeconds(),
	})
}
