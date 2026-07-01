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
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type GenerateService struct {
	apiConfigRepo *repository.ApiConfigRepo
	creditService *CreditService
	creditRepo    *repository.CreditRepo
	logService    *ModelCallLogService
	httpClient    *http.Client
	encryptKey    string
}

func NewGenerateService(apiConfigRepo *repository.ApiConfigRepo, creditService *CreditService, creditRepo *repository.CreditRepo, logService *ModelCallLogService, encryptKey string) *GenerateService {
	return &GenerateService{
		apiConfigRepo: apiConfigRepo,
		creditService: creditService,
		creditRepo:    creditRepo,
		logService:    logService,
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
		err := errors.New("请指定模型")
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
	}

	cost, pricingResult, err := s.getRequiredPricing(tenantID, genType, modelName, contentType, body)
	if err != nil {
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
	}

	account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
	if err != nil {
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
	}
	if account.Balance < cost {
		err := fmt.Errorf("积分不足，需要 %d 积分，当前余额 %d", cost, account.Balance)
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
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
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, fmt.Errorf("上游 API 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if resp.StatusCode >= 400 {
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, resp.StatusCode, respBytes, "")
		return &ProxyResult{
			StatusCode: resp.StatusCode,
			Body:       respBytes,
			Headers:    resp.Header,
		}, nil
	}

	if cost > 0 {
		metadata, note := buildCreditSpendDetail(genType, modelName, path, pricingResult)
		if err := s.creditService.SpendWithMetadata(0, userID, cost, genType, modelName, note, metadata); err != nil {
			return nil, err
		}
	}

	account, _ = s.creditService.GetOrCreateAccount(tenantID, userID)
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

func (s *GenerateService) getRequiredPricing(tenantID uint, genType, modelName, contentType string, body []byte) (int, CreditCostResult, error) {
	pricing, err := s.creditRepo.FindPricing(tenantID, modelName)
	if err != nil {
		return 0, CreditCostResult{}, fmt.Errorf("模型 %s 未配置计费，暂不可用", modelName)
	}
	result, err := CalculateCreditCost(pricing, genType, contentType, body)
	if err != nil {
		return 0, CreditCostResult{}, err
	}
	return result.TotalCost, result, nil
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

	modelName := extractModelName(contentType, body)
	cost, chargeType, pricingResult := s.getProxyCost(tenantID, method, path, contentType, body, modelName)
	if chargeType != "" {
		if modelName == "" {
			err := errors.New("请指定模型")
			s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
			return nil, err
		}
		if _, _, err := s.getRequiredPricing(tenantID, chargeType, modelName, contentType, body); err != nil {
			s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
			return nil, err
		}
	}

	if cost > 0 {
		account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
		if err != nil {
			s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
			return nil, err
		}
		if account.Balance < cost {
			err := fmt.Errorf("积分不足，需要 %d 积分，当前余额 %d", cost, account.Balance)
			s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
			return nil, err
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
		return nil, fmt.Errorf("上游 API 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, resp.StatusCode, nil, err.Error())
		return nil, err
	}

	if resp.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if resp.StatusCode >= 400 && chargeType != "" {
		s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, resp.StatusCode, respBytes, "")
	}
	if resp.StatusCode < 400 && strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet {
		if failed, responseModel, message := readFailedModelTaskResponse(respBytes); failed {
			if modelName == "" {
				modelName = responseModel
			}
			s.recordModelFailure(tenantID, userID, generationTypeFromPath(path), modelName, method, path, resp.StatusCode, respBytes, message)
		}
	}

	if resp.StatusCode < 400 && cost > 0 {
		metadata, note := buildCreditSpendDetail(chargeType, modelName, path, pricingResult)
		if err := s.creditService.SpendWithMetadata(0, userID, cost, chargeType, modelName, note, metadata); err != nil {
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

func (s *GenerateService) getProxyCost(tenantID uint, method, path, contentType string, body []byte, modelName string) (int, string, CreditCostResult) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return 0, "", CreditCostResult{}
	}
	chargeType := generationTypeFromPath(path)
	if chargeType == "" || modelName == "" {
		return 0, "", CreditCostResult{}
	}
	pricing, err := s.creditRepo.FindPricing(tenantID, modelName)
	if err != nil || pricing == nil {
		return 0, chargeType, CreditCostResult{}
	}
	result, err := CalculateCreditCost(pricing, chargeType, contentType, body)
	if err != nil {
		return 0, chargeType, CreditCostResult{}
	}
	return result.TotalCost, chargeType, result
}

func (s *GenerateService) recordModelFailure(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string) {
	if genType == "" {
		genType = generationTypeFromPath(path)
	}
	s.logService.RecordFailure(ModelCallLogInput{
		TenantID:     tenantID,
		UserID:       userID,
		Generation:   genType,
		Model:        modelName,
		Method:       method,
		Path:         path,
		StatusCode:   statusCode,
		ErrorMessage: fallback,
		ErrorBody:    body,
	})
}

func generationTypeFromPath(path string) string {
	cleanPath := strings.Split(strings.TrimSpace(path), "?")[0]
	switch {
	case strings.HasSuffix(cleanPath, "/images/generations"), strings.HasSuffix(cleanPath, "/images/edits"):
		return "image"
	case strings.Contains(cleanPath, "/video/generations"), strings.Contains(cleanPath, "/videos/generations"), strings.Contains(cleanPath, "/videos"), strings.Contains(cleanPath, "/contents/generations/tasks"):
		return "video"
	case strings.HasSuffix(cleanPath, "/audio/speech"):
		return "audio"
	case strings.HasSuffix(cleanPath, "/chat/completions"), strings.HasSuffix(cleanPath, "/responses"):
		return "text"
	default:
		return ""
	}
}

func extractImageCount(contentType string, body []byte) int {
	values := extractRequestFields(contentType, body)
	if value := intFromAny(values["n"]); value >= 1 {
		return value
	}
	return 1
}

func extractUsageCount(genType, contentType string, body []byte) int {
	if genType == "image" {
		return extractImageCount(contentType, body)
	}
	return 1
}

func buildCreditSpendDetail(genType, modelName, path string, cost CreditCostResult) (string, string) {
	if cost.Units <= 0 {
		cost.Units = 1
	}
	if cost.UnitCost <= 0 {
		cost.UnitCost = cost.TotalCost
	}
	label := generationTypeLabel(genType)
	note := fmt.Sprintf("%s · 模型 %s · 扣除 %d 积分", label, modelName, cost.TotalCost)
	if cost.UnitType != "" {
		note = fmt.Sprintf("%s · %s × %d", note, creditUnitLabel(cost.UnitType), cost.Units)
	}
	if cost.Formula != "" {
		note = fmt.Sprintf("%s · %s", note, cost.Formula)
	}
	payload := map[string]interface{}{
		"scene":      label,
		"generation": genType,
		"model":      modelName,
		"path":       strings.Split(strings.TrimSpace(path), "?")[0],
		"unit_type":  string(cost.UnitType),
		"unit_label": creditUnitLabel(cost.UnitType),
		"unit_cost":  cost.UnitCost,
		"units":      cost.Units,
		"total_cost": cost.TotalCost,
	}
	if cost.Seconds > 0 {
		payload["seconds"] = cost.Seconds
	}
	if cost.Resolution != "" {
		payload["resolution"] = cost.Resolution
	}
	if cost.Formula != "" {
		payload["formula"] = cost.Formula
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", note
	}
	return string(data), note
}

func generationTypeLabel(genType string) string {
	switch genType {
	case "image":
		return "图片生成"
	case "video":
		return "视频生成"
	case "audio":
		return "音频生成"
	case "text":
		return "文本生成"
	default:
		return "生成任务"
	}
}

func creditUnitLabel(unitType model.CreditPricingUnit) string {
	switch unitType {
	case model.UnitPerImage:
		return "按图片"
	case model.UnitPerVideo:
		return "按视频"
	case model.UnitPerVideoSecond:
		return "按秒"
	case model.UnitPerToken:
		return "按 Token"
	default:
		return "按次"
	}
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

func transformImageResponseToChatFormat(path string, respBytes []byte) ([]byte, bool) {
	cleanPath := strings.Split(strings.TrimSpace(path), "?")[0]
	if !strings.HasSuffix(cleanPath, "/chat/completions") {
		return respBytes, false
	}

	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &payload); err != nil || len(payload.Data) == 0 {
		return respBytes, false
	}

	lines := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		imageURL := ""
		if value, ok := item["url"].(string); ok && strings.TrimSpace(value) != "" {
			imageURL = strings.TrimSpace(value)
		}
		if imageURL == "" {
			if value, ok := item["b64_json"].(string); ok && strings.TrimSpace(value) != "" {
				encoded := strings.TrimSpace(value)
				if strings.HasPrefix(encoded, "http://") || strings.HasPrefix(encoded, "https://") || strings.HasPrefix(encoded, "data:image/") {
					imageURL = encoded
				} else {
					imageURL = "data:image/png;base64," + encoded
				}
			}
		}
		if imageURL != "" {
			lines = append(lines, fmt.Sprintf("![image](%s)", imageURL))
		}
	}
	if len(lines) == 0 {
		return respBytes, false
	}

	converted, err := json.Marshal(map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": strings.Join(lines, "\n\n"),
				},
				"finish_reason": "stop",
			},
		},
		"object": "chat.completion",
		"created": time.Now().Unix(),
	})
	if err != nil {
		return respBytes, false
	}
	return converted, true
}
