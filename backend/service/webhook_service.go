package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

const (
	WebhookStatusUserQuotaInsufficient = "user_quota_insufficient"
	WebhookStatusModelUnavailable      = "model_unavailable"
)

type WebhookService struct {
	repo          *repository.WebhookRepo
	senderFactory func(string) WebhookSender
}

type WebhookConfigPatch struct {
	Platform        string
	WebhookURL      *string
	Enabled         *bool
	CooldownMinutes *int
}

type WebhookTestResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type upstreamWebhookAlert struct {
	Status string
	Reason string
}

type upstreamWebhookEvent struct {
	TenantID    uint
	ChannelID   uint
	ChannelName string
	ModelName   string
	Status      string
	Reason      string
	Action      string
}

func NewWebhookService(repo *repository.WebhookRepo) *WebhookService {
	return &WebhookService{repo: repo, senderFactory: NewSender}
}

func (s *WebhookService) ListConfigs(tenantID uint) ([]model.WebhookConfig, error) {
	return s.repo.ListByTenant(tenantID)
}

func (s *WebhookService) SaveConfig(tenantID uint, input WebhookConfigPatch) (*model.WebhookConfig, error) {
	platform := strings.TrimSpace(input.Platform)
	if platform == "" {
		return nil, errors.New("platform 不能为空")
	}
	if s.senderFactory(platform) == nil {
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
	updates := map[string]interface{}{}
	if input.WebhookURL != nil {
		updates["webhook_url"] = strings.TrimSpace(*input.WebhookURL)
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if input.CooldownMinutes != nil {
		if *input.CooldownMinutes < 0 {
			return nil, errors.New("冷却时间不能小于 0")
		}
		updates["cooldown_minutes"] = *input.CooldownMinutes
	}
	return s.repo.SavePatch(tenantID, platform, updates)
}

func (s *WebhookService) TestSend(ctx context.Context, tenantID uint, platform, message string) (*WebhookTestResult, error) {
	platform = strings.TrimSpace(platform)
	message = strings.TrimSpace(message)
	if platform == "" || message == "" {
		return nil, errors.New("platform 和 message 不能为空")
	}
	cfg, err := s.repo.GetByPlatform(tenantID, platform)
	if err != nil {
		return nil, err
	}
	sender := s.senderFactory(platform)
	if sender == nil {
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	sendErr := sender.Send(sendCtx, cfg.WebhookURL, message)
	cancel()

	entry := &model.WebhookLog{
		TenantID: tenantID,
		Platform: platform,
		Status:   "test",
		Message:  message,
		Success:  sendErr == nil,
	}
	result := &WebhookTestResult{Success: sendErr == nil}
	if sendErr != nil {
		result.Error = sendErr.Error()
		entry.ResponseBody = result.Error
	}
	if err := s.repo.InsertLog(entry); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *WebhookService) ListLogs(tenantID uint, limit int) ([]model.WebhookLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListLogs(tenantID, limit)
}

func shouldDispatchUpstreamWebhookAlert(event upstreamWebhookEvent) bool {
	return strings.TrimSpace(event.Status) != ""
}

func (s *WebhookService) NotifyUpstreamAlertAsync(event upstreamWebhookEvent) {
	if s == nil || s.repo == nil || !shouldDispatchUpstreamWebhookAlert(event) {
		return
	}
	go s.notifyUpstreamAlert(event)
}

func webhookAlertInCooldown(lastLog *model.WebhookLog, err error, now time.Time, cooldownMinutes int) bool {
	return err == nil &&
		lastLog != nil &&
		lastLog.Success &&
		now.Before(lastLog.CreatedAt.Add(time.Duration(cooldownMinutes)*time.Minute))
}

func (s *WebhookService) notifyUpstreamAlert(event upstreamWebhookEvent) {
	configs, err := s.repo.ListEnabled(event.TenantID)
	if err != nil {
		log.Printf("webhook alert: query configs: %v", err)
		return
	}
	now := time.Now()
	message := upstreamWebhookMessage(event, now)
	for _, cfg := range configs {
		if cfg.CooldownMinutes > 0 {
			lastLog, err := s.repo.LastAlert(cfg.TenantID, cfg.Platform, event.ChannelID, event.ModelName, event.Status)
			if webhookAlertInCooldown(lastLog, err, now, cfg.CooldownMinutes) {
				_ = s.repo.InsertLog(webhookLogFromEvent(cfg.Platform, event, message, false, true))
				continue
			}
		}
		sender := s.senderFactory(cfg.Platform)
		if sender == nil {
			log.Printf("webhook alert: unknown platform %q for tenant %d", cfg.Platform, cfg.TenantID)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		sendErr := sender.Send(ctx, cfg.WebhookURL, message)
		cancel()

		entry := webhookLogFromEvent(cfg.Platform, event, message, sendErr == nil, false)
		if sendErr != nil {
			entry.ResponseBody = sendErr.Error()
			log.Printf("webhook alert: send %s to tenant %d: %v", cfg.Platform, cfg.TenantID, sendErr)
		}
		if err := s.repo.InsertLog(entry); err != nil {
			log.Printf("webhook alert: insert log: %v", err)
		}
	}
}

func webhookLogFromEvent(platform string, event upstreamWebhookEvent, message string, success, skipped bool) *model.WebhookLog {
	return &model.WebhookLog{
		TenantID:        event.TenantID,
		Platform:        platform,
		ChannelID:       event.ChannelID,
		ChannelName:     event.ChannelName,
		ModelName:       event.ModelName,
		Status:          event.Status,
		Message:         message,
		Success:         success,
		CooldownSkipped: skipped,
	}
}

func classifyUpstreamWebhookAlert(body []byte) (upstreamWebhookAlert, bool) {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return upstreamWebhookAlert{}, false
	}
	payloads := decodeUpstreamPayloads(body)
	for _, payload := range payloads {
		code := strings.ToLower(readStringPath(payload, "error.code", "data.error.code", "code", "data.code"))
		message := readStringPath(payload, "error.message", "data.error.message", "message", "data.message", "msg", "detail")
		if code == "insufficient_user_quota" || strings.Contains(message, "用户额度不足") {
			return upstreamWebhookAlert{Status: WebhookStatusUserQuotaInsufficient, Reason: upstreamWebhookReason(body)}, true
		}
	}
	if len(payloads) == 0 && strings.Contains(raw, "用户额度不足") {
		return upstreamWebhookAlert{Status: WebhookStatusUserQuotaInsufficient, Reason: truncateString(raw, 500)}, true
	}

	if len(payloads) < 2 || !strings.EqualFold(readStringPath(payloads[0], "code"), "fail_to_fetch_task") {
		return upstreamWebhookAlert{}, false
	}
	for _, payload := range payloads[1:] {
		code := strings.ToLower(readStringPath(payload, "error.code", "data.error.code", "code", "data.code"))
		message := readStringPath(payload, "error.message", "data.error.message", "message", "data.message", "msg", "detail")
		if code == "model_not_found" && strings.Contains(strings.ToLower(message), "no available channel for model") {
			return upstreamWebhookAlert{Status: WebhookStatusModelUnavailable, Reason: truncateString(message, 500)}, true
		}
	}
	return upstreamWebhookAlert{}, false
}

func decodeUpstreamPayloads(body []byte) []map[string]interface{} {
	raw := body
	payloads := make([]map[string]interface{}, 0, 3)
	for range 3 {
		var payload map[string]interface{}
		if json.Unmarshal(raw, &payload) != nil {
			break
		}
		payloads = append(payloads, payload)
		embedded := readStringPath(payload, "message", "error.message", "data.message", "data.error.message", "msg", "detail")
		if !strings.HasPrefix(strings.TrimSpace(embedded), "{") {
			break
		}
		raw = []byte(embedded)
	}
	return payloads
}

func upstreamWebhookReason(body []byte) string {
	reason := buildModelCallErrorSummary(200, body, "")
	if reason == "" {
		return "用户额度不足"
	}
	return reason
}

func upstreamWebhookMessage(event upstreamWebhookEvent, now time.Time) string {
	title := "上游模型无可用渠道"
	if event.Status == WebhookStatusUserQuotaInsufficient {
		title = "上游用户额度不足"
	}
	parts := []string{title}
	if channel := webhookChannelLabel(event.ChannelID, event.ChannelName); channel != "" {
		parts = append(parts, "渠道: "+channel)
	}
	if strings.TrimSpace(event.ModelName) != "" {
		parts = append(parts, "模型: "+strings.TrimSpace(event.ModelName))
	}
	if strings.TrimSpace(event.Reason) != "" {
		parts = append(parts, "原因: "+strings.TrimSpace(event.Reason))
	}
	if strings.TrimSpace(event.Action) != "" {
		parts = append(parts, "处理: "+strings.TrimSpace(event.Action))
	}
	parts = append(parts, "时间: "+now.Format(time.RFC3339))
	return strings.Join(parts, "\n")
}

func webhookChannelLabel(channelID uint, channelName string) string {
	channelName = strings.TrimSpace(channelName)
	if channelName != "" && channelID > 0 {
		return fmt.Sprintf("%s (#%d)", channelName, channelID)
	}
	if channelName != "" {
		return channelName
	}
	if channelID > 0 {
		return fmt.Sprintf("渠道 #%d", channelID)
	}
	return ""
}
