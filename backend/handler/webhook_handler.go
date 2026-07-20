package handler

import (
	"context"
	"strconv"
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
	configs, err := h.webhookRepo.ListEnabled(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, configs)
}

func (h *WebhookHandler) SaveConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var cfg model.WebhookConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	cfg.TenantID = claims.TenantID
	if err := h.webhookRepo.Save(&cfg); err != nil {
		model.Fail(c, 500, err.Error())
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
	_ = c.MustGet("claims").(*service.Claims)
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
	_ = c.MustGet("claims").(*service.Claims)
	model.OK(c, gin.H{
		"running":          h.poller.IsRunning(),
		"interval_seconds": h.poller.IntervalSeconds(),
	})
}
