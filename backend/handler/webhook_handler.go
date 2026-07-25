package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type WebhookHandler struct {
	service *service.WebhookService
}

func NewWebhookHandler(webhookService *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: webhookService}
}

func (h *WebhookHandler) ListConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	configs, err := h.service.ListConfigs(claims.TenantID)
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
	CooldownMinutes *int    `json:"cooldown_minutes"`
}

func (h *WebhookHandler) SaveConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input webhookConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	cfg, err := h.service.SaveConfig(claims.TenantID, service.WebhookConfigPatch{
		Platform:        input.Platform,
		WebhookURL:      input.WebhookURL,
		Enabled:         input.Enabled,
		CooldownMinutes: input.CooldownMinutes,
	})
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
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
	result, err := h.service.TestSend(c.Request.Context(), claims.TenantID, input.Platform, input.Message)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, result)
}

func (h *WebhookHandler) ListLogs(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, err := h.service.ListLogs(claims.TenantID, limit)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, logs)
}
