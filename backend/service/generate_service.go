package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

const maxVideoReferenceImageBase64Chars = 460 * 1024

type GenerateService struct {
	apiConfigRepo *repository.ApiConfigRepo
	creditService *CreditService
	creditRepo    pricingReader
	logService    *ModelCallLogService
	repairService *OnDemandRepairService
	channelSvc    channelKeyReader
	channelRepo   channelReader
	modelRepo     channelModelReader
	httpClient    *http.Client
	encryptKey    string
}

type channelReader interface {
	FindByID(id uint) (*model.Channel, error)
}

type channelModelReader interface {
	FindByID(id uint) (*model.ChannelModel, error)
}

type channelKeyReader interface {
	DecryptedApiKey(id uint) (string, error)
}

type pricingReader interface {
	FindPricing(tenantID uint, modelName string) (*model.CreditPricing, error)
}

func NewGenerateService(apiConfigRepo *repository.ApiConfigRepo, creditService *CreditService, creditRepo *repository.CreditRepo, logService *ModelCallLogService, encryptKey string, repairService *OnDemandRepairService, channelSvc *ChannelService, channelRepo *repository.ChannelRepo, modelRepo *repository.ChannelModelRepo) *GenerateService {
	return &GenerateService{
		apiConfigRepo: apiConfigRepo,
		creditService: creditService,
		creditRepo:    creditRepo,
		logService:    logService,
		repairService: repairService,
		channelSvc:    channelSvc,
		channelRepo:   channelRepo,
		modelRepo:     modelRepo,
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

type upstreamCallResult struct {
	StatusCode     int
	Body           []byte
	Headers        http.Header
	ResponseTimeMs int
}

type ChannelSelection struct {
	ChannelID      uint
	ChannelModelID uint
}

type channelRouteContext struct {
	Channel        *model.Channel
	ChannelModel   *model.ChannelModel
	ApiKey         string
	ChannelID      *uint
	ChannelModelID *uint
}

func (s *GenerateService) ProxyImage(tenantID, userID uint, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "image", "/v1/images/generations", contentType, body, selection)
}

func (s *GenerateService) ProxyText(tenantID, userID uint, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "text", "/v1/chat/completions", contentType, body, selection)
}

func (s *GenerateService) ProxyVideo(tenantID, userID uint, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "video", "/v1/video/generations", contentType, body, selection)
}

func (s *GenerateService) ProxyAudio(tenantID, userID uint, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	return s.proxy(tenantID, userID, "audio", "/v1/audio/speech", contentType, body, selection)
}

func (s *GenerateService) resolveChannelRoute(selection ChannelSelection, capability, modelName string) (*channelRouteContext, error) {
	if selection.ChannelID == 0 || selection.ChannelModelID == 0 {
		return nil, errors.New("请选择有效的渠道和模型")
	}
	if s.channelRepo == nil || s.modelRepo == nil || s.channelSvc == nil {
		return nil, errors.New("渠道服务未配置")
	}
	channel, err := s.channelRepo.FindByID(selection.ChannelID)
	if err != nil {
		return nil, errors.New("渠道不存在或不可用")
	}
	if !channel.Enabled {
		return nil, errors.New("渠道已禁用")
	}
	channelModel, err := s.modelRepo.FindByID(selection.ChannelModelID)
	if err != nil {
		return nil, errors.New("渠道模型不存在或不可用")
	}
	if channelModel.ChannelID != channel.ID {
		return nil, errors.New("渠道模型不属于所选渠道")
	}
	if !channelModel.Enabled {
		return nil, errors.New("渠道模型已禁用")
	}
	if strings.TrimSpace(channelModel.ModelName) != strings.TrimSpace(modelName) {
		return nil, errors.New("渠道模型与请求模型不匹配")
	}
	if !channelModelSupports(channelModel, capability) {
		return nil, errors.New("渠道模型不支持当前能力")
	}
	apiKey, err := s.channelSvc.DecryptedApiKey(channel.ID)
	if err != nil {
		return nil, err
	}
	return &channelRouteContext{
		Channel:        channel,
		ChannelModel:   channelModel,
		ApiKey:         apiKey,
		ChannelID:      uintPtr(channel.ID),
		ChannelModelID: uintPtr(channelModel.ID),
	}, nil
}

func (s *GenerateService) ResolveChannelRouteForEstimate(selection ChannelSelection, capability, modelName string) error {
	_, err := s.resolveChannelRoute(selection, capability, modelName)
	return err
}

func uintPtr(value uint) *uint {
	return &value
}

func channelModelSupports(item *model.ChannelModel, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return true
	}
	capabilities := parseChannelCapabilities(item.Capabilities)
	if len(capabilities) == 0 {
		capabilities = defaultChannelModelCapabilities()
	}
	for _, item := range capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func parseChannelCapabilities(raw string) []string {
	items := make([]string, 0)
	if strings.TrimSpace(raw) == "" {
		return items
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		_ = json.Unmarshal([]byte(raw), &items)
	} else {
		items = strings.Split(raw, ",")
	}
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func defaultChannelModelCapabilities() []string {
	return []string{string(model.CapabilityImage), string(model.CapabilityVideo), string(model.CapabilityText), string(model.CapabilityAudio)}
}

func defaultChannelModelCapabilitiesJSON() string {
	encoded, _ := json.Marshal(defaultChannelModelCapabilities())
	return string(encoded)
}

func mergeSelection(primary, fallback ChannelSelection) ChannelSelection {
	if primary.ChannelID == 0 {
		primary.ChannelID = fallback.ChannelID
	}
	if primary.ChannelModelID == 0 {
		primary.ChannelModelID = fallback.ChannelModelID
	}
	return primary
}

func extractChannelSelection(contentType string, body []byte, path string) ChannelSelection {
	selection := channelSelectionFromQuery(path)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json") && len(body) > 0 {
		var payload map[string]interface{}
		if json.Unmarshal(body, &payload) == nil {
			selection = mergeSelection(selection, ChannelSelection{ChannelID: uintFromAny(payload["channel_id"]), ChannelModelID: uintFromAny(payload["channel_model_id"])})
		}
	}
	return selection
}

func channelSelectionFromQuery(path string) ChannelSelection {
	parsed, err := url.Parse(path)
	if err != nil {
		return ChannelSelection{}
	}
	values := parsed.Query()
	return ChannelSelection{ChannelID: parseUintParam(values.Get("channel_id")), ChannelModelID: parseUintParam(values.Get("channel_model_id"))}
}

func parseUintParam(value string) uint {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return uint(parsed)
}

func uintFromAny(value interface{}) uint {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return uint(typed)
		}
	case string:
		return parseUintParam(typed)
	}
	return 0
}

func stripChannelIdentityQuery(path string) string {
	parsed, err := url.Parse(path)
	if err != nil || parsed.RawQuery == "" {
		return path
	}
	values := parsed.Query()
	values.Del("channel_id")
	values.Del("channel_model_id")
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func stripJSONChannelIdentity(contentType string, body []byte) []byte {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json") || len(body) == 0 {
		return body
	}
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	if _, ok := payload["channel_id"]; !ok {
		if _, ok := payload["channel_model_id"]; !ok {
			return body
		}
	}
	delete(payload, "channel_id")
	delete(payload, "channel_model_id")
	updated, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return updated
}

func (s *GenerateService) doUpstreamRequest(method, baseURL, apiKey, path, contentType string, body []byte) (*upstreamCallResult, error) {
	url := buildUpstreamURL(baseURL, path)
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

	startTime := time.Now()
	resp, err := s.httpClient.Do(req)
	responseTimeMs := int(time.Since(startTime).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &upstreamCallResult{
		StatusCode:     resp.StatusCode,
		Body:           respBytes,
		Headers:        resp.Header,
		ResponseTimeMs: responseTimeMs,
	}, nil
}

func (s *GenerateService) proxy(tenantID, userID uint, genType, path, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	selection = mergeSelection(selection, extractChannelSelection(contentType, body, path))
	path = stripChannelIdentityQuery(path)
	body = stripJSONChannelIdentity(contentType, body)
	if normalizedBody, changed := normalizeVideoReferenceImages(http.MethodPost, path, contentType, body); changed {
		log.Printf("compressed video reference image payload path=%s", cleanPath(path))
		body = normalizedBody
	}

	modelName := extractModelName(contentType, body)
	if modelName == "" {
		err := errors.New("请指定模型")
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
	}
	route, err := s.resolveChannelRoute(selection, genType, modelName)
	if err != nil {
		s.recordModelFailure(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
		return nil, err
	}

	cost, pricingResult, err := s.getRequiredPricing(tenantID, genType, modelName, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		return nil, err
	}

	account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		return nil, err
	}
	if account.Balance < cost {
		err := fmt.Errorf("积分不足，需要 %d 积分，当前余额 %d", cost, account.Balance)
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		return nil, err
	}

	upstream, err := s.doUpstreamRequest(http.MethodPost, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, genType, modelName, http.MethodPost, path, contentType, body, 0, nil, err.Error(), route); ok {
			upstream = retry
		} else {
			return nil, fmt.Errorf("涓婃父 API 璇锋眰澶辫触: %v", err)
		}
	}
	if upstream == nil {
		return nil, errors.New("upstream request failed")
	}
	respBytes := upstream.Body
	if upstream.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if upstream.StatusCode >= 400 {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, respBytes, "", route)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, genType, modelName, http.MethodPost, path, contentType, body, upstream.StatusCode, respBytes, "", route); ok {
			upstream = retry
			respBytes = upstream.Body
			if upstream.StatusCode < 400 {
				if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
					respBytes = converted
				}
			}
		}
	}
	if upstream.StatusCode >= 400 {
		return &ProxyResult{
			StatusCode: upstream.StatusCode,
			Body:       respBytes,
			Headers:    upstream.Headers,
		}, nil
	}

	s.recordModelSuccessWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, upstream.ResponseTimeMs, route)

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
		StatusCode: upstream.StatusCode,
		Body:       respBytes,
		Headers:    upstream.Headers,
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

func (s *GenerateService) ProxyRaw(tenantID, userID uint, method, path, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	selection = mergeSelection(selection, extractChannelSelection(contentType, body, path))
	path = stripChannelIdentityQuery(path)
	body = stripJSONChannelIdentity(contentType, body)
	if normalizedBody, changed := normalizeVideoReferenceImages(method, path, contentType, body); changed {
		log.Printf("compressed video reference image payload path=%s", cleanPath(path))
		body = normalizedBody
	}

	modelName := extractModelName(contentType, body)
	chargeType := generationTypeFromPath(path)
	if modelName == "" && strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet && selection.ChannelModelID != 0 && s.modelRepo != nil {
		if item, err := s.modelRepo.FindByID(selection.ChannelModelID); err == nil {
			modelName = item.ModelName
		}
	}
	if chargeType == "" {
		return nil, errors.New("无法识别代理请求能力")
	}
	if modelName == "" {
		err := errors.New("请指定模型")
		s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
		return nil, err
	}
	route, err := s.resolveChannelRoute(selection, chargeType, modelName)
	if err != nil {
		s.recordModelFailure(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error())
		return nil, err
	}
	cost, _, pricingResult, err := s.getProxyCostByGeneration(tenantID, method, chargeType, contentType, body, modelName)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
		return nil, err
	}

	if cost > 0 {
		account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
		if err != nil {
			s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
			return nil, err
		}
		if account.Balance < cost {
			err := fmt.Errorf("积分不足，需要 %d 积分，当前余额 %d", cost, account.Balance)
			s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
			return nil, err
		}
	}

	upstream, err := s.doUpstreamRequest(method, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
		return nil, fmt.Errorf("上游 API 请求失败: %v", err)
	}
	respBytes := upstream.Body

	if upstream.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if upstream.StatusCode >= 400 && chargeType != "" {
		s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, upstream.StatusCode, respBytes, "", route)
	}
	if upstream.StatusCode < 400 && strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet {
		if failed, responseModel, message := readFailedModelTaskResponse(respBytes); failed {
			if modelName == "" {
				modelName = responseModel
			}
			s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, upstream.StatusCode, respBytes, message, route)
		} else if chargeType != "" && modelName != "" {
			s.recordModelSuccessWithRoute(tenantID, userID, chargeType, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route)
		}
	} else if upstream.StatusCode < 400 && chargeType != "" && modelName != "" {
		s.recordModelSuccessWithRoute(tenantID, userID, chargeType, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route)
	}

	if upstream.StatusCode < 400 && cost > 0 {
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
		StatusCode: upstream.StatusCode,
		Body:       respBytes,
		Headers:    upstream.Headers,
		Cost:       cost,
		Balance:    balance,
	}, nil
}

func (s *GenerateService) ProxyRawWithRepair(tenantID, userID uint, method, path, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	selection = mergeSelection(selection, extractChannelSelection(contentType, body, path))
	path = stripChannelIdentityQuery(path)
	body = stripJSONChannelIdentity(contentType, body)
	if normalizedBody, changed := normalizeVideoReferenceImages(method, path, contentType, body); changed {
		log.Printf("compressed video reference image payload path=%s", cleanPath(path))
		body = normalizedBody
	}

	modelName := extractModelName(contentType, body)
	generation := generationTypeFromPath(path)
	if modelName == "" && strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet && selection.ChannelModelID != 0 && s.modelRepo != nil {
		if item, err := s.modelRepo.FindByID(selection.ChannelModelID); err == nil {
			modelName = item.ModelName
		}
	}
	if generation == "" {
		return nil, errors.New("unknown proxy generation")
	}
	if modelName == "" {
		err := errors.New("model is required")
		s.recordModelFailure(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error())
		return nil, err
	}
	route, err := s.resolveChannelRoute(selection, generation, modelName)
	if err != nil {
		s.recordModelFailure(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error())
		return nil, err
	}
	cost, chargeType, pricingResult, err := s.getProxyCostByGeneration(tenantID, method, generation, contentType, body, modelName)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route)
		return nil, err
	}

	if cost > 0 {
		account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
		if err != nil {
			s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
			return nil, err
		}
		if account.Balance < cost {
			err := fmt.Errorf("insufficient credits, need %d, current balance %d", cost, account.Balance)
			s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
			return nil, err
		}
	}

	upstream, err := s.doUpstreamRequest(method, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, generation, modelName, method, path, contentType, body, 0, nil, err.Error(), route); ok {
			upstream = retry
		} else {
			return nil, fmt.Errorf("upstream API request failed: %v", err)
		}
	}
	if upstream == nil {
		return nil, errors.New("upstream request failed")
	}

	respBytes := upstream.Body
	if upstream.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if upstream.StatusCode >= 400 && generation != "" {
		s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, respBytes, "", route)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, generation, modelName, method, path, contentType, body, upstream.StatusCode, respBytes, "", route); ok {
			upstream = retry
			respBytes = upstream.Body
			if upstream.StatusCode < 400 {
				if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
					respBytes = converted
				}
			}
		}
	}

	if upstream.StatusCode < 400 && strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet {
		if failed, responseModel, message := readFailedModelTaskResponse(respBytes); failed {
			if modelName == "" {
				modelName = responseModel
			}
			s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, respBytes, message, route)
			requestContext := buildRepairRequestContext(generation, method, path, "application/json", respBytes)
			s.triggerOnDemandRepairAsync(generation, modelName, message, requestContext)
		} else if generation != "" && modelName != "" {
			s.recordModelSuccessWithRoute(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route)
		}
	} else if upstream.StatusCode < 400 && generation != "" && modelName != "" {
		s.recordModelSuccessWithRoute(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route)
	}

	if upstream.StatusCode < 400 && cost > 0 {
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
		StatusCode: upstream.StatusCode,
		Body:       respBytes,
		Headers:    upstream.Headers,
		Cost:       cost,
		Balance:    balance,
	}, nil
}

func (s *GenerateService) getProxyCostByGeneration(tenantID uint, method, generation, contentType string, body []byte, modelName string) (int, string, CreditCostResult, error) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return 0, generation, CreditCostResult{}, nil
	}
	if generation == "" || modelName == "" {
		return 0, generation, CreditCostResult{}, nil
	}
	cost, result, err := s.getRequiredPricing(tenantID, generation, modelName, contentType, body)
	if err != nil {
		return 0, generation, CreditCostResult{}, err
	}
	return cost, generation, result, nil
}

func (s *GenerateService) repairAndRetryUpstream(tenantID, userID uint, generation, modelName, method, path, contentType string, body []byte, statusCode int, responseBody []byte, fallback string, route *channelRouteContext) (*upstreamCallResult, bool) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return nil, false
	}
	if route == nil || route.Channel == nil {
		return nil, false
	}
	requestContext := buildRepairRequestContext(generation, method, path, contentType, body)
	if !s.shouldAttemptOnDemandRepair(generation, modelName, statusCode, responseBody, fallback, requestContext) {
		return nil, false
	}
	reason := buildRepairReason(method, path, statusCode, responseBody, fallback)
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	result, err := s.repairService.Repair(ctx, generation, modelName, reason, requestContext)
	if err != nil {
		log.Printf("on-demand repair failed generation=%s model=%s: %v", generation, modelName, err)
		return nil, false
	}
	if result == nil || !result.Repaired {
		return nil, false
	}
	retry, err := s.doUpstreamRequest(method, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route)
		log.Printf("retry after on-demand repair failed generation=%s model=%s: %v", generation, modelName, err)
		return nil, false
	}
	return retry, true
}

func (s *GenerateService) triggerOnDemandRepairAsync(generation, modelName, reason string, requestContext *RepairRequestContext) {
	if !s.shouldAttemptOnDemandRepair(generation, modelName, 0, nil, reason, requestContext) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		if _, err := s.repairService.Repair(ctx, generation, modelName, reason, requestContext); err != nil {
			log.Printf("async on-demand repair failed generation=%s model=%s: %v", generation, modelName, err)
		}
	}()
}

func (s *GenerateService) shouldAttemptOnDemandRepair(generation, modelName string, statusCode int, responseBody []byte, fallback string, requestContext *RepairRequestContext) bool {
	if s.repairService == nil || !s.repairService.Enabled() {
		return false
	}
	generation = strings.TrimSpace(generation)
	if generation != "image" && generation != "video" {
		return false
	}
	if strings.TrimSpace(modelName) == "" {
		return false
	}
	if statusCode == 0 && strings.TrimSpace(fallback) != "" {
		return true
	}
	message := strings.ToLower(buildModelCallErrorSummary(statusCode, responseBody, fallback))
	if requestContext != nil && requestContext.Operation != "" && isCapabilityMismatchMessage(message) {
		return true
	}
	nonChannelPatterns := []string{
		"prompt length",
		"prompt too long",
		"too long",
		"maximum",
		"最多",
		"超过上限",
		"参数",
		"invalid",
		"must be",
		"requires",
		"required",
		"reference image",
		"reference_images",
		"至少需要",
		"必须提供",
		"seconds is invalid",
		"video_length",
		"unsupported",
	}
	for _, pattern := range nonChannelPatterns {
		if strings.Contains(message, pattern) {
			return false
		}
	}
	if statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		return true
	}
	transientPatterns := []string{
		"overload",
		"overloaded",
		"too many requests",
		"rate limit",
		"ratelimit",
		"capacity",
		"busy",
		"timeout",
		"timed out",
		"temporarily",
		"try again",
		"quota",
		"insufficient_quota",
		"负载",
		"限流",
		"超时",
		"稍后",
		"繁忙",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

func isCapabilityMismatchMessage(message string) bool {
	patterns := []string{
		"not support",
		"not supported",
		"unsupported",
		"only support",
		"only supports",
		"duration",
		"seconds",
		"aspect",
		"ratio",
		"resolution",
		"size",
		"image-to-video",
		"image to video",
		"video-to-video",
		"video to video",
		"first frame",
		"last frame",
		"reference",
		"首帧",
		"尾帧",
		"参考图",
		"参考视频",
		"竖屏",
		"横屏",
		"尺寸",
		"比例",
		"时长",
		"仅支持",
		"不支持",
	}
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

func buildRepairRequestContext(generation, method, path, contentType string, body []byte) *RepairRequestContext {
	generation = strings.TrimSpace(generation)
	if generation != "image" && generation != "video" {
		return nil
	}
	ctx := &RepairRequestContext{
		Method:      strings.ToUpper(strings.TrimSpace(method)),
		Path:        cleanPath(path),
		ContentType: strings.TrimSpace(strings.Split(contentType, ";")[0]),
	}

	payload := map[string]interface{}{}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json") && len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}

	ctx.Size = firstPayloadString(payload, "size", "resolution", "resolution_name", "vquality")
	if ctx.Size == "" {
		ctx.Size = sizeFromWidthHeight(payload)
	}
	ctx.AspectRatio = firstPayloadString(payload, "aspect_ratio", "ratio")
	if ctx.AspectRatio == "" {
		ctx.AspectRatio = aspectRatioFromSize(ctx.Size)
	}
	ctx.Seconds = firstPayloadInt(payload, "seconds", "duration", "video_length")
	ctx.ReferenceCount = countRequestReferences(payload)
	if ctx.ReferenceCount == 0 && strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data") {
		ctx.ReferenceCount = countMultipartReferences(body)
	}
	ctx.HasReferences = ctx.ReferenceCount > 0

	cleanPath := ctx.Path
	switch generation {
	case "image":
		ctx.Operation = "image_generate"
		if strings.HasSuffix(cleanPath, "/images/edits") || ctx.HasReferences {
			ctx.Operation = "image_edit"
		}
	case "video":
		ctx.Operation = "text_to_video"
		if hasVideoReference(payload) {
			ctx.Operation = "video_to_video"
			ctx.HasReferences = true
			if ctx.ReferenceCount == 0 {
				ctx.ReferenceCount = 1
			}
		} else if ctx.HasReferences {
			ctx.Operation = "image_to_video"
		}
	}
	return ctx
}

func firstPayloadString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64:
			if typed > 0 {
				return strconv.Itoa(int(typed))
			}
		}
	}
	return ""
}

func firstPayloadInt(payload map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int(typed)
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func sizeFromWidthHeight(payload map[string]interface{}) string {
	width := firstPayloadInt(payload, "width")
	height := firstPayloadInt(payload, "height")
	if width > 0 && height > 0 {
		return fmt.Sprintf("%dx%d", width, height)
	}
	return ""
}

func aspectRatioFromSize(size string) string {
	size = strings.ToLower(strings.TrimSpace(size))
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}
	switch {
	case width == height:
		return "1:1"
	case width*9 == height*16:
		return "16:9"
	case width*16 == height*9:
		return "9:16"
	case width*3 == height*4:
		return "4:3"
	case width*4 == height*3:
		return "3:4"
	default:
		return fmt.Sprintf("%d:%d", width/gcd(width, height), height/gcd(width, height))
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func countRequestReferences(payload map[string]interface{}) int {
	if len(payload) == 0 {
		return 0
	}
	count := 0
	var visit func(key string, value interface{})
	visit = func(key string, value interface{}) {
		lowerKey := strings.ToLower(key)
		if isReferenceKey(lowerKey) {
			count += referenceValueCount(value)
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			for childKey, childValue := range typed {
				visit(childKey, childValue)
			}
		case []interface{}:
			for _, childValue := range typed {
				visit("", childValue)
			}
		}
	}
	for key, value := range payload {
		visit(key, value)
	}
	return count
}

func isReferenceKey(key string) bool {
	switch key {
	case "image", "images", "image_url", "image_urls", "first_image", "first_image_url", "last_image", "last_image_url", "input_reference", "reference_image", "reference_images", "reference_image_urls", "reference_video", "reference_video_url", "reference_video_urls", "video", "video_url", "references", "inline_data", "filedata", "file_data":
		return true
	default:
		return false
	}
}

func referenceValueCount(value interface{}) int {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		if strings.Contains(trimmed, "|") {
			return len(strings.Split(trimmed, "|"))
		}
		return 1
	case []interface{}:
		if len(typed) == 0 {
			return 0
		}
		return len(typed)
	case map[string]interface{}:
		if len(typed) == 0 {
			return 0
		}
		return 1
	default:
		return 0
	}
}

func countMultipartReferences(body []byte) int {
	count := 0
	lower := bytes.ToLower(body)
	for _, marker := range [][]byte{
		[]byte(`name="image"`),
		[]byte(`name="image[]"`),
		[]byte(`name="images"`),
		[]byte(`name="file"`),
		[]byte(`name="first_image"`),
		[]byte(`name="last_image"`),
		[]byte(`name="video"`),
	} {
		count += bytes.Count(lower, marker)
	}
	return count
}

func hasVideoReference(payload map[string]interface{}) bool {
	for _, key := range []string{"video", "video_url", "reference_video", "reference_video_url", "reference_video_urls", "input_video"} {
		if referenceValueCount(payload[key]) > 0 {
			return true
		}
	}
	return false
}

func buildRepairReason(method, path string, statusCode int, responseBody []byte, fallback string) string {
	message := buildModelCallErrorSummary(statusCode, responseBody, fallback)
	if message == "" {
		message = "upstream request failed"
	}
	return fmt.Sprintf("%s %s status=%d: %s", strings.ToUpper(strings.TrimSpace(method)), cleanPath(path), statusCode, message)
}

func (s *GenerateService) recordModelFailure(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string) {
	s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, method, path, statusCode, body, fallback, nil)
}

func (s *GenerateService) recordModelFailureWithRoute(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string, route *channelRouteContext) {
	if s.logService == nil {
		return
	}
	if genType == "" {
		genType = generationTypeFromPath(path)
	}
	var channelID *uint
	var channelModelID *uint
	if route != nil {
		channelID = route.ChannelID
		channelModelID = route.ChannelModelID
	}
	s.logService.RecordFailure(ModelCallLogInput{
		TenantID:       tenantID,
		UserID:         userID,
		Generation:     genType,
		Model:          modelName,
		Method:         method,
		Path:           path,
		StatusCode:     statusCode,
		ErrorMessage:   fallback,
		ErrorBody:      body,
		ChannelID:      channelID,
		ChannelModelID: channelModelID,
	})
}

func (s *GenerateService) recordModelSuccess(tenantID, userID uint, genType, modelName, method, path string, statusCode, responseTimeMs int) {
	s.recordModelSuccessWithRoute(tenantID, userID, genType, modelName, method, path, statusCode, responseTimeMs, nil)
}

func (s *GenerateService) recordModelSuccessWithRoute(tenantID, userID uint, genType, modelName, method, path string, statusCode, responseTimeMs int, route *channelRouteContext) {
	if s.logService == nil {
		return
	}
	if genType == "" {
		genType = generationTypeFromPath(path)
	}
	var channelID *uint
	var channelModelID *uint
	if route != nil {
		channelID = route.ChannelID
		channelModelID = route.ChannelModelID
	}
	s.logService.RecordSuccess(ModelCallLogInput{
		TenantID:       tenantID,
		UserID:         userID,
		Generation:     genType,
		Model:          modelName,
		Method:         method,
		Path:           path,
		StatusCode:     statusCode,
		ChannelID:      channelID,
		ChannelModelID: channelModelID,
	}, responseTimeMs)
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

func normalizeVideoReferenceImages(method, path, contentType string, body []byte) ([]byte, bool) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return body, false
	}
	if generationTypeFromPath(path) != "video" {
		return body, false
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json") || len(body) == 0 {
		return body, false
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, false
	}
	durationChanged := normalizeVeoOmniFlashDuration(payload)
	updated, imageChanged := normalizeDataURLImages(payload)
	changed := durationChanged || imageChanged
	if !changed {
		return body, false
	}
	normalizedBody, err := json.Marshal(updated)
	if err != nil {
		return body, false
	}
	return normalizedBody, true
}

func normalizeVeoOmniFlashDuration(value interface{}) bool {
	payload, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	modelName, _ := payload["model"].(string)
	if strings.TrimSpace(modelName) != "veo-omni-flash" {
		return false
	}

	changed := false
	if payload["duration"] != float64(10) {
		payload["duration"] = 10
		changed = true
	}
	if _, exists := payload["seconds"]; exists && payload["seconds"] != "10" {
		payload["seconds"] = "10"
		changed = true
	}
	return changed
}

func normalizeDataURLImages(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		changed := false
		for key, child := range typed {
			updated, childChanged := normalizeDataURLImages(child)
			if childChanged {
				typed[key] = updated
				changed = true
			}
		}
		return typed, changed
	case []interface{}:
		changed := false
		for idx, child := range typed {
			updated, childChanged := normalizeDataURLImages(child)
			if childChanged {
				typed[idx] = updated
				changed = true
			}
		}
		return typed, changed
	case string:
		return compressDataURLImage(typed)
	default:
		return value, false
	}
}

func compressDataURLImage(value string) (string, bool) {
	prefix, encoded, ok := splitBase64ImageDataURL(value)
	if !ok || len(encoded) <= maxVideoReferenceImageBase64Chars {
		return value, false
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return value, false
	}
	compressed, ok := compressImageBytesForBase64Limit(raw, maxVideoReferenceImageBase64Chars)
	if !ok {
		return value, false
	}
	compressedEncoded := base64.StdEncoding.EncodeToString(compressed)
	if len(compressedEncoded) >= len(encoded) {
		return value, false
	}
	return prefix + compressedEncoded, true
}

func splitBase64ImageDataURL(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "data:image/") {
		return "", "", false
	}
	commaIdx := strings.Index(trimmed, ",")
	if commaIdx < 0 {
		return "", "", false
	}
	prefix := trimmed[:commaIdx+1]
	if !strings.Contains(strings.ToLower(prefix), ";base64") {
		return "", "", false
	}
	encoded := stripBase64Whitespace(trimmed[commaIdx+1:])
	if encoded == "" {
		return "", "", false
	}
	return "data:image/jpeg;base64,", encoded, true
}

func stripBase64Whitespace(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch r {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func compressImageBytesForBase64Limit(raw []byte, maxEncodedChars int) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, false
	}

	qualities := []int{82, 72, 62, 52, 42, 34, 28}
	scales := []float64{1, 0.85, 0.7, 0.55, 0.45, 0.35, 0.25}
	var smallest []byte
	for _, scale := range scales {
		candidateImage := img
		if scale < 1 {
			scaledWidth := int(float64(width) * scale)
			scaledHeight := int(float64(height) * scale)
			if scaledWidth < 1 {
				scaledWidth = 1
			}
			if scaledHeight < 1 {
				scaledHeight = 1
			}
			candidateImage = resizeNearest(img, scaledWidth, scaledHeight)
		}

		for _, quality := range qualities {
			var buffer bytes.Buffer
			if err := jpeg.Encode(&buffer, candidateImage, &jpeg.Options{Quality: quality}); err != nil {
				continue
			}
			candidate := buffer.Bytes()
			if len(smallest) == 0 || len(candidate) < len(smallest) {
				smallest = append([]byte(nil), candidate...)
			}
			if base64.StdEncoding.EncodedLen(len(candidate)) <= maxEncodedChars {
				return candidate, true
			}
		}
	}
	if len(smallest) > 0 && len(smallest) < len(raw) {
		return smallest, true
	}
	return nil, false
}

func resizeNearest(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + y*srcHeight/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
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
	cleanPath := strings.TrimSpace(path)
	if strings.HasPrefix(cleanPath, "http://") || strings.HasPrefix(cleanPath, "https://") {
		if parsed, err := url.Parse(cleanPath); err == nil {
			cleanPath = parsed.RequestURI()
		}
	}
	normalizedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedPath := "/" + strings.TrimLeft(cleanPath, "/")
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
		"object":  "chat.completion",
		"created": time.Now().Unix(),
	})
	if err != nil {
		return respBytes, false
	}
	return converted, true
}
