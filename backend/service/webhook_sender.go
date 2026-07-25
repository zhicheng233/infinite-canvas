package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// webhookHTTPClient is a shared HTTP client with a short timeout for webhook delivery.
// http.Client is safe for concurrent use by multiple goroutines.
var webhookHTTPClient = &http.Client{Timeout: 10 * time.Second}

// WebhookSender sends a text message to a webhook URL.
type WebhookSender interface {
	Send(ctx context.Context, url string, message string) error
}

type webhookDeliveryResponse struct {
	OK         *bool `json:"ok"`
	ErrCode    *int  `json:"errcode"`
	Code       *int  `json:"code"`
	StatusCode *int  `json:"StatusCode"`
}

// --- Feishu (飞书) ---

type FeishuSender struct{}

func (s *FeishuSender) Send(ctx context.Context, url string, message string) error {
	body := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": message,
		},
	}
	return postWebhook(ctx, url, body, "feishu")
}

// --- DingTalk (钉钉) ---

type DingTalkSender struct{}

func (s *DingTalkSender) Send(ctx context.Context, url string, message string) error {
	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}
	return postWebhook(ctx, url, body, "dtalk")
}

// --- WeChat Work (企业微信) ---

type WecomSender struct{}

func (s *WecomSender) Send(ctx context.Context, url string, message string) error {
	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}
	return postWebhook(ctx, url, body, "wecom")
}

// --- Telegram ---

type TelegramSender struct{}

func (s *TelegramSender) Send(ctx context.Context, url string, message string) error {
	chatID, err := extractTelegramChatID(url)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	body := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}
	return postWebhook(ctx, url, body, "telegram")
}

func extractTelegramChatID(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid webhook url: %w", err)
	}
	chatID := parsed.Query().Get("chat_id")
	if chatID == "" {
		return "", fmt.Errorf("chat_id not found in webhook url")
	}
	return chatID, nil
}

// --- sender factory ---

// NewSender returns a platform-specific sender for the given platform identifier.
// Supported platforms: "feishu", "dtalk", "wecom", "telegram".
// Returns nil for unknown platforms.
func NewSender(platform string) WebhookSender {
	switch platform {
	case "feishu":
		return &FeishuSender{}
	case "dtalk":
		return &DingTalkSender{}
	case "wecom":
		return &WecomSender{}
	case "telegram":
		return &TelegramSender{}
	default:
		return nil
	}
}

// --- shared helper ---

func postWebhook(ctx context.Context, targetURL string, body interface{}, platform string) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return validateWebhookDeliveryResponse(platform, respBody)
}

func validateWebhookDeliveryResponse(platform string, body []byte) error {
	var response webhookDeliveryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("%s invalid response: %s", platform, string(body))
	}

	switch platform {
	case "telegram":
		if response.OK != nil && *response.OK {
			return nil
		}
	case "dtalk", "wecom":
		if response.ErrCode != nil && *response.ErrCode == 0 {
			return nil
		}
	case "feishu":
		if response.Code != nil {
			if *response.Code == 0 {
				return nil
			}
			break
		}
		if response.StatusCode != nil && *response.StatusCode == 0 {
			return nil
		}
	default:
		return fmt.Errorf("unsupported webhook platform: %s", platform)
	}

	return fmt.Errorf("%s rejected request: %s", platform, string(body))
}
