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

// --- Feishu (飞书) ---

type FeishuSender struct{}

func (s *FeishuSender) Send(ctx context.Context, url string, message string) error {
	body := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": message,
		},
	}
	return postWebhook(ctx, url, body)
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
	return postWebhook(ctx, url, body)
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
	return postWebhook(ctx, url, body)
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
	return postWebhook(ctx, url, body)
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

func postWebhook(ctx context.Context, targetURL string, body interface{}) error {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
