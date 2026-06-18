package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infinite-canvas-server/crypto"
	"infinite-canvas-server/repository"
)

type GenerateService struct {
	apiConfigRepo *repository.ApiConfigRepo
	creditService *CreditService
	creditRepo    *repository.CreditRepo
	httpClient    *http.Client
	encryptKey    string
}

func NewGenerateService(apiConfigRepo *repository.ApiConfigRepo, creditService *CreditService, creditRepo *repository.CreditRepo, encryptKey string) *GenerateService {
	return &GenerateService{
		apiConfigRepo: apiConfigRepo,
		creditService: creditService,
		creditRepo:    creditRepo,
		httpClient:    &http.Client{Timeout: 10 * time.Minute},
		encryptKey:    encryptKey,
	}
}

type ProxyResult struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Cost       int
	Balance    int
}

func (s *GenerateService) ProxyImage(tenantID, userID uint, contentType string, body []byte) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "image", "/v1/images/generations", contentType, body)
}

func (s *GenerateService) ProxyText(tenantID, userID uint, contentType string, body []byte) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "text", "/v1/chat/completions", contentType, body)
}

func (s *GenerateService) ProxyVideo(tenantID, userID uint, contentType string, body []byte) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "video", "/v1/video/generations", contentType, body)
}

func (s *GenerateService) ProxyAudio(tenantID, userID uint, contentType string, body []byte) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "audio", "/v1/audio/speech", contentType, body)
}

func (s *GenerateService) getDecryptedApiKey(tenantID uint) (string, error) {
	cfg, err := s.apiConfigRepo.FindByTenant(tenantID)
	if err != nil {
		return "", err
	}
	if s.encryptKey != "" {
		return crypto.Decrypt(s.encryptKey, cfg.ApiKey)
	}
	return cfg.ApiKey, nil
}

func (s *GenerateService) proxy(tenantID, userID uint, genType, path, contentType string, body []byte) (*ProxyResult, error) {
	cfg, err := s.apiConfigRepo.FindByTenant(tenantID)
	if err != nil {
		return nil, errors.New("租户未配置 API，请联系管理员")
	}

	apiKey, err := s.getDecryptedApiKey(tenantID)
	if err != nil {
		apiKey = cfg.ApiKey
	}

	modelName := extractModelName(contentType, body)
	if modelName == "" {
		return nil, errors.New("请指定模型")
	}

	cost := s.getCost(tenantID, genType, modelName)
	if cost > 0 {
		account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
		if err != nil {
			return nil, err
		}
		if account.Balance < cost {
			return nil, fmt.Errorf("积分不足，需要 %d 积分，当前余额 %d", cost, account.Balance)
		}
	}

	url := buildUpstreamURL(cfg.BaseUrl, path)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上游 API 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return &ProxyResult{
			StatusCode: resp.StatusCode,
			Body:       respBytes,
			Headers:    resp.Header,
		}, nil
	}

	if cost > 0 {
		if err := s.creditService.Spend(0, userID, cost, genType, modelName, fmt.Sprintf("生成 %s", genType)); err != nil {
			return nil, err
		}
	}

	account, _ := s.creditService.GetOrCreateAccount(tenantID, userID)
	balance := 0
	if account != nil {
		balance = account.Balance
	}

	return &ProxyResult{
		StatusCode: resp.StatusCode,
		Body:       respBytes,
		Headers:    resp.Header,
		Cost:       cost,
		Balance:    balance,
	}, nil
}

func (s *GenerateService) getCost(tenantID uint, genType, modelName string) int {
	pricing, err := s.creditRepo.FindPricing(tenantID, modelName)
	if err != nil {
		return 0
	}
	return pricing.CreditsPerUnit
}

func extractModelName(contentType string, body []byte) string {
	if strings.HasPrefix(contentType, "application/json") {
		var data map[string]interface{}
		if json.Unmarshal(body, &data) == nil {
			if m, ok := data["model"].(string); ok {
				return m
			}
		}
	}
	if strings.HasPrefix(contentType, "multipart/form-data") {
		boundary := extractBoundary(contentType)
		if boundary != "" {
			return extractModelFromMultipart(body, boundary)
		}
	}
	return ""
}

func extractBoundary(contentType string) string {
	parts := strings.Split(contentType, "boundary=")
	if len(parts) < 2 {
		return ""
	}
	return strings.Trim(parts[1], "\"")
}

func extractModelFromMultipart(body []byte, boundary string) string {
	delim := "--" + boundary
	parts := bytes.Split(body, []byte(delim))
	for _, part := range parts {
		if bytes.Contains(part, []byte("name=\"model\"")) {
			lines := bytes.Split(part, []byte("\r\n\r\n"))
			if len(lines) >= 2 {
				return strings.TrimSpace(string(lines[len(lines)-1]))
			}
		}
	}
	return ""
}

func (s *GenerateService) ProxyRaw(tenantID, userID uint, method, path, contentType string, body []byte) (*ProxyResult, error) {
	cfg, err := s.apiConfigRepo.FindByTenant(tenantID)
	if err != nil {
		return nil, errors.New("租户未配置 API，请联系管理员")
	}

	url := buildUpstreamURL(cfg.BaseUrl, path)

	apiKey, err := s.getDecryptedApiKey(tenantID)
	if err != nil {
		apiKey = cfg.ApiKey
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上游 API 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &ProxyResult{
		StatusCode: resp.StatusCode,
		Body:       respBytes,
		Headers:    resp.Header,
	}, nil
}

func buildUpstreamURL(baseURL, path string) string {
	normalizedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedPath := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if normalizedBase == "" {
		return normalizedPath
	}
	if strings.HasSuffix(normalizedBase, "/v1") || strings.Contains(normalizedPath, "/v1/") || normalizedPath == "/v1" {
		return normalizedBase + normalizedPath
	}
	return normalizedBase + "/v1" + normalizedPath
}
