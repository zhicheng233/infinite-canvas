package service

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas-server/model"
)

type webhookTestChannelService struct {
	disabledIDs []uint
}

func (s *webhookTestChannelService) DecryptedApiKey(uint) (string, error) {
	return "", nil
}

func (s *webhookTestChannelService) Disable(id uint) error {
	s.disabledIDs = append(s.disabledIDs, id)
	return nil
}

func TestClassifyUpstreamWebhookAlert(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus string
	}{
		{
			name:       "user quota response",
			body:       `{"error":{"message":"用户额度不足, 剩余额度: ¥0.000000 (request id: 202607242118503872757708268d9d6DPiDlVHo)","type":"new_api_error","param":"","code":"insufficient_user_quota"}}`,
			wantStatus: WebhookStatusUserQuotaInsufficient,
		},
		{
			name:       "user quota code",
			body:       `{"error":{"message":"quota unavailable","code":"insufficient_user_quota"}}`,
			wantStatus: WebhookStatusUserQuotaInsufficient,
		},
		{
			name:       "plain user quota message",
			body:       "用户额度不足",
			wantStatus: WebhookStatusUserQuotaInsufficient,
		},
		{
			name:       "failed task has no available channel",
			body:       `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"model_not_found\",\"message\":\"No available channel for model omni-fast under group default (distributor) (request id: 202607242047350891769018268d9d6jnRu8FJ0)\",\"type\":\"new_api_error\"}}","data":null}`,
			wantStatus: WebhookStatusModelUnavailable,
		},
		{name: "generic balance error is ignored", body: `{"error":{"message":"余额不足"}}`},
		{name: "generic quota code is ignored", body: `{"error":{"message":"quota exceeded","code":"insufficient_quota"}}`},
		{name: "ordinary failed task is ignored", body: `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"invalid_request\",\"message\":\"invalid request body\"}}"}`},
		{name: "direct model not found is ignored", body: `{"error":{"code":"model_not_found","message":"No available channel for model omni-fast"}}`},
		{name: "other model not found is ignored", body: `{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"model_not_found\",\"message\":\"model does not exist\"}}"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert, ok := classifyUpstreamWebhookAlert([]byte(tt.body))
			if tt.wantStatus == "" {
				if ok {
					t.Fatalf("unexpected alert: %#v", alert)
				}
				return
			}
			if !ok || alert.Status != tt.wantStatus || alert.Reason == "" {
				t.Fatalf("alert=%#v ok=%v, want status %q", alert, ok, tt.wantStatus)
			}
		})
	}
}

func TestUpstreamWebhookMessage(t *testing.T) {
	message := upstreamWebhookMessage(upstreamWebhookEvent{
		ChannelID:   12,
		ChannelName: "主渠道",
		ModelName:   "omni-fast",
		Status:      WebhookStatusModelUnavailable,
		Reason:      "No available channel for model omni-fast",
	}, time.Date(2026, 7, 25, 12, 30, 0, 0, time.UTC))

	for _, want := range []string{
		"上游模型无可用渠道",
		"渠道: 主渠道 (#12)",
		"模型: omni-fast",
		"原因: No available channel for model omni-fast",
		"时间: 2026-07-25T12:30:00Z",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q: %s", want, message)
		}
	}
}

func TestShouldDispatchUpstreamWebhookAlertAllowsDefaultTenant(t *testing.T) {
	event := upstreamWebhookEvent{
		TenantID: 0,
		Status:   WebhookStatusUserQuotaInsufficient,
	}
	if !shouldDispatchUpstreamWebhookAlert(event) {
		t.Fatal("default tenant webhook alert should be dispatched")
	}
}

func TestWebhookAlertInCooldownRequiresSuccessfulDelivery(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 30, 0, 0, time.UTC)
	failed := &model.WebhookLog{Success: false}
	failed.CreatedAt = now.Add(-time.Minute)
	if webhookAlertInCooldown(failed, nil, now, 10) {
		t.Fatal("failed delivery must not enter cooldown")
	}

	succeeded := &model.WebhookLog{Success: true}
	succeeded.CreatedAt = now.Add(-time.Minute)
	if !webhookAlertInCooldown(succeeded, nil, now, 10) {
		t.Fatal("successful delivery should enter cooldown")
	}
}

func TestHandleUpstreamWebhookAlertDisablesOnlyUserQuotaChannel(t *testing.T) {
	channelService := &webhookTestChannelService{}
	generateService := &GenerateService{channelSvc: channelService}
	route := &channelRouteContext{Channel: &model.Channel{BaseModel: model.BaseModel{ID: 12}, Name: "主渠道"}}

	status, clientMessage := generateService.handleUpstreamWebhookAlert(1, "omni-fast", []byte(`{"error":{"code":"insufficient_user_quota","message":"用户额度不足"}}`), route)
	if status != WebhookStatusUserQuotaInsufficient || len(channelService.disabledIDs) != 1 || channelService.disabledIDs[0] != 12 {
		t.Fatalf("status=%q disabled=%v", status, channelService.disabledIDs)
	}
	if clientMessage != "因上游问题被禁用" || strings.Contains(clientMessage, "用户额度不足") {
		t.Fatalf("unexpected client message: %q", clientMessage)
	}

	status, _ = generateService.handleUpstreamWebhookAlert(1, "omni-fast", []byte(`{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"model_not_found\",\"message\":\"No available channel for model omni-fast\"}}"}`), route)
	if status != WebhookStatusModelUnavailable || len(channelService.disabledIDs) != 1 {
		t.Fatalf("status=%q disabled=%v", status, channelService.disabledIDs)
	}
}
