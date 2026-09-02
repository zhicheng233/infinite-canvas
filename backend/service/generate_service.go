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
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

const (
	maxVideoReferenceImageBase64Chars = 460 * 1024
	maxLoggedRequestBodyChars         = 20000
	maxLoggedRequestStringChars       = 2000
)

type GenerateService struct {
	creditService      *CreditService
	creditRepo         pricingReader
	generationBilling  *GenerationBillingService
	logService         *ModelCallLogService
	repairService      *OnDemandRepairService
	channelSvc         channelKeyReader
	channelRepo        channelReader
	modelRepo          channelModelReader
	autoChannelService autoRoutingProvider
	webhookService     *WebhookService
	mergeGroupRepo     mergeGroupRepoReader
	estimateFuzzyRoute func(channelID uint, fuzzyGroupName, capability string) (*channelRouteContext, error)
	db                 *gorm.DB
	httpClient         *http.Client
	encryptKey         string
}

type autoRoutingProvider interface {
	AggregateModels(tenantIDs ...uint) ([]AggregatedModel, error)
	ResolveCandidates(poolID, tenantID uint, capability, modelName string) (*model.AutoRoutingPool, []AutoRouteCandidate, error)
	AcquireCandidate(candidate AutoRouteCandidate) bool
	ReleaseCandidate(channelModelID uint)
	RecordAttempt(item *model.GenerationAttempt) error
}

type channelReader interface {
	FindByID(id uint) (*model.Channel, error)
}

type channelModelReader interface {
	FindByID(id uint) (*model.ChannelModel, error)
}

type mergeGroupRepoReader interface {
	ListByChannel(channelID uint) ([]model.ModelMergeGroup, error)
}

type channelKeyReader interface {
	DecryptedApiKey(id uint) (string, error)
	Disable(id uint) error
}

type pricingReader interface {
	FindPricing(tenantID uint, modelName string, channelID uint) (*model.CreditPricing, error)
}

func NewGenerateService(creditService *CreditService, creditRepo *repository.CreditRepo, generationBilling *GenerationBillingService, logService *ModelCallLogService, encryptKey string, repairService *OnDemandRepairService, channelSvc *ChannelService, channelRepo *repository.ChannelRepo, modelRepo *repository.ChannelModelRepo, mergeGroupRepo *repository.MergeGroupRepo, db *gorm.DB, autoChannelService *AutoChannelService, webhookService *WebhookService) *GenerateService {
	return &GenerateService{
		creditService:      creditService,
		creditRepo:         creditRepo,
		generationBilling:  generationBilling,
		logService:         logService,
		repairService:      repairService,
		channelSvc:         channelSvc,
		channelRepo:        channelRepo,
		modelRepo:          modelRepo,
		mergeGroupRepo:     mergeGroupRepo,
		db:                 db,
		autoChannelService: autoChannelService,
		webhookService:     webhookService,
		httpClient:         &http.Client{Timeout: 10 * time.Minute},
		encryptKey:         encryptKey,
	}
}

type ProxyResult struct {
	StatusCode             int
	Body                   []byte
	Headers                http.Header
	Cost                   int
	Balance                int
	Refund                 int
	ResolvedChannelID      uint
	ResolvedChannelModelID uint
	ResolvedChannelName    string
	RequestID              string
}

type upstreamCallResult struct {
	StatusCode     int
	Body           []byte
	Headers        http.Header
	ResponseTimeMs int
}

const upstreamMaxResponseBytes = 512 << 20

type modelCallRequestSnapshot struct {
	UpstreamURL   string
	ContentType   string
	Body          string
	BodyTruncated bool
	Sent          bool
}

type ModelSelectionKind string

const (
	ModelSelectionPhysical ModelSelectionKind = "physical"
	ModelSelectionAuto     ModelSelectionKind = "auto"
	ModelSelectionMerge    ModelSelectionKind = "merge"
)

type ModelSelection struct {
	Kind              ModelSelectionKind
	ChannelID         uint
	ChannelModelID    uint
	ModelName         string
	VideoRoute        string
	MergeGroupName    string
	Capability        string
	AutoRoutingPoolID uint
}

type ChannelSelection = ModelSelection

func (selection ModelSelection) SelectionKind() ModelSelectionKind {
	if selection.Kind != "" {
		return selection.Kind
	}
	if strings.TrimSpace(selection.MergeGroupName) != "" {
		return ModelSelectionMerge
	}
	if selection.AutoRoutingPoolID > 0 {
		return ModelSelectionAuto
	}
	if selection.ChannelID > 0 || selection.ChannelModelID > 0 {
		return ModelSelectionPhysical
	}
	return ""
}

func validateModelSelection(selection ModelSelection) error {
	switch selection.SelectionKind() {
	case ModelSelectionAuto:
		if selection.AutoRoutingPoolID == 0 || selection.ChannelID != 0 || selection.ChannelModelID != 0 || strings.TrimSpace(selection.MergeGroupName) != "" {
			return errors.New("智能路由参数无效")
		}
	case ModelSelectionMerge:
		if selection.ChannelID == 0 || selection.ChannelModelID != 0 || selection.AutoRoutingPoolID != 0 || strings.TrimSpace(selection.MergeGroupName) == "" {
			return errors.New("模型合并组参数无效")
		}
	case ModelSelectionPhysical:
		if selection.ChannelID == 0 || selection.ChannelModelID == 0 || selection.AutoRoutingPoolID != 0 || strings.TrimSpace(selection.MergeGroupName) != "" {
			return errors.New("物理渠道参数无效")
		}
	default:
		return errors.New("请选择有效的渠道模型或智能路由池")
	}
	return nil
}

type ResolvedEstimateRoute struct {
	Selection    ChannelSelection
	PricingModel string
	Candidates   []ChannelSelection
}

type channelRouteContext struct {
	Channel        *model.Channel
	ChannelModel   *model.ChannelModel
	ApiKey         string
	ChannelID      *uint
	ChannelModelID *uint
}

type autoCandidatePlan struct {
	candidate  AutoRouteCandidate
	route      *channelRouteContext
	cost       int
	chargeType string
	pricing    CreditCostResult
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

func (s *GenerateService) resolveFuzzyMergeRoute(channelID uint, fuzzyGroupName string, capability string) (*channelRouteContext, error) {
	groups, err := s.mergeGroupRepo.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	var matchedGroup *model.ModelMergeGroup
	for i := range groups {
		if groups[i].Enabled && groups[i].GroupName == fuzzyGroupName {
			matchedGroup = &groups[i]
			break
		}
	}
	if matchedGroup == nil {
		return nil, fmt.Errorf("未找到合并组 %s", fuzzyGroupName)
	}
	var models []model.ChannelModel
	if err := s.db.Where("channel_id = ? AND model_name LIKE ? AND enabled = ?",
		channelID, matchedGroup.Pattern+"%", true).Find(&models).Error; err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("合并组 %s 内无可用模型", fuzzyGroupName)
	}
	type modelWithRate struct {
		model *model.ChannelModel
		rate  float64
	}
	ranked := make([]modelWithRate, 0, len(models))
	cutoff := time.Now().Add(-24 * time.Hour)
	for i := range models {
		var total, success int64
		s.db.Model(&model.ModelCallLog{}).Where("channel_model_id = ? AND created_at > ?", models[i].ID, cutoff).Count(&total)
		s.db.Model(&model.ModelCallLog{}).Where("channel_model_id = ? AND created_at > ? AND is_success = ?", models[i].ID, cutoff, true).Count(&success)
		rate := float64(0)
		if total > 0 {
			rate = float64(success) / float64(total) * 100
		}
		ranked = append(ranked, modelWithRate{&models[i], rate})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].rate > ranked[j].rate })
	best := ranked[0].model
	return s.resolveChannelRoute(
		ChannelSelection{ChannelID: channelID, ChannelModelID: best.ID},
		capability,
		best.ModelName,
	)
}

func (s *GenerateService) proxyWithAutoFailover(tenantID, userID uint, method, capability, path, contentType string, body []byte, modelName, requestedVideoRoute string, poolIDs ...uint) (*ProxyResult, error) {
	if s.autoChannelService == nil || len(poolIDs) == 0 || poolIDs[0] == 0 {
		return nil, errors.New("请选择有效的智能路由池")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	requestedVideoRoute = strings.ToLower(strings.TrimSpace(requestedVideoRoute))
	pool, candidates, err := s.autoChannelService.ResolveCandidates(poolIDs[0], tenantID, capability, modelName)
	if err != nil {
		return nil, err
	}

	plans := make([]autoCandidatePlan, 0, len(candidates))
	for _, candidate := range candidates {
		selection := ChannelSelection{ChannelID: candidate.Channel.ID, ChannelModelID: candidate.ChannelModel.ID}
		route, routeErr := s.resolveChannelRoute(selection, capability, modelName)
		if routeErr != nil {
			continue
		}
		if capability == "video" && requestedVideoRoute != "" && effectiveVideoRoute(route) != requestedVideoRoute {
			continue
		}
		if protocolErr := validateResolvedRequestProtocol(route, capability, method, path, contentType, body); protocolErr != nil {
			return nil, protocolErr
		}
		cost, chargeType, pricingResult, pricingErr := s.getProxyCostByGeneration(tenantID, candidate.Channel.ID, method, capability, contentType, body, modelName)
		if pricingErr != nil {
			continue
		}
		plans = append(plans, autoCandidatePlan{candidate: candidate, route: route, cost: cost, chargeType: chargeType, pricing: pricingResult})
	}
	if len(plans) == 0 {
		return nil, errors.New("智能路由没有合同和计费均匹配的候选")
	}
	sort.SliceStable(plans, func(i, j int) bool {
		left, right := plans[i].candidate, plans[j].candidate
		if left.CircuitStatus != right.CircuitStatus {
			return autoCircuitRank(left.CircuitStatus) < autoCircuitRank(right.CircuitStatus)
		}
		if left.SuccessRate != right.SuccessRate {
			return left.SuccessRate > right.SuccessRate
		}
		if left.Member.Priority != right.Member.Priority {
			return left.Member.Priority > right.Member.Priority
		}
		if left.P95LatencyMs != right.P95LatencyMs {
			return left.P95LatencyMs < right.P95LatencyMs
		}
		if plans[i].cost != plans[j].cost {
			return plans[i].cost < plans[j].cost
		}
		return left.ChannelModel.ID < right.ChannelModel.ID
	})
	maxCost := 0
	maxPricing := plans[0].pricing
	for _, plan := range plans {
		if plan.cost > maxCost {
			maxCost, maxPricing = plan.cost, plan.pricing
		}
	}
	job, balance, err := s.reserveGenerationCredits(tenantID, userID, maxCost, plans[0].chargeType, modelName, path, maxPricing, nil, pool.ID)
	if err != nil {
		return nil, err
	}
	requestID := generationRequestID(job)
	maxAttempts := pool.MaxAttempts
	if maxAttempts < 1 || maxAttempts > autoDefaultMaxAttempts {
		maxAttempts = autoDefaultMaxAttempts
	}
	if maxAttempts > len(plans) {
		maxAttempts = len(plans)
	}
	var firstErr error
	attemptNo := 0
	for _, plan := range plans {
		if attemptNo >= maxAttempts {
			break
		}
		if !s.autoChannelService.AcquireCandidate(plan.candidate) {
			continue
		}
		attemptNo++
		requestSnapshot := buildModelCallRequestSnapshot(plan.route, method, path, contentType, body)
		upstream, upstreamErr := s.doUpstreamRequest(method, plan.route.Channel.BaseUrl, plan.route.ApiKey, path, contentType, body)
		s.autoChannelService.ReleaseCandidate(plan.candidate.ChannelModel.ID)
		if upstreamErr != nil {
			category, retryable := classifyAutoFailure(0, nil, upstreamErr)
			s.recordAutoAttempt(requestID, attemptNo, pool.ID, plan, 0, 0, false, category, retryable, upstreamErr.Error())
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, capability, modelName, method, path, 0, nil, upstreamErr.Error(), plan.route, requestSnapshot)
			if firstErr == nil {
				firstErr = errors.New(autoFailureMessage(category))
			}
			if retryable {
				continue
			}
			break
		}
		alertStatus, alertMessage := s.handleUpstreamWebhookAlert(tenantID, modelName, upstream.Body, plan.route)
		if alertStatus != "" {
			category, retryable := "upstream_unavailable", true
			s.recordAutoAttempt(requestID, attemptNo, pool.ID, plan, upstream.StatusCode, upstream.ResponseTimeMs, false, category, retryable, alertMessage)
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, capability, modelName, method, path, upstream.StatusCode, upstream.Body, alertMessage, plan.route, requestSnapshot)
			if firstErr == nil {
				firstErr = errors.New(autoFailureMessage(category))
			}
			continue
		}
		if upstream.StatusCode >= http.StatusBadRequest {
			category, retryable := classifyAutoFailure(upstream.StatusCode, upstream.Body, nil)
			message := buildModelCallErrorSummary(upstream.StatusCode, upstream.Body, "")
			s.recordAutoAttempt(requestID, attemptNo, pool.ID, plan, upstream.StatusCode, upstream.ResponseTimeMs, false, category, retryable, message)
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, capability, modelName, method, path, upstream.StatusCode, upstream.Body, message, plan.route, requestSnapshot)
			if firstErr == nil {
				firstErr = errors.New(autoFailureMessage(category))
			}
			if retryable {
				continue
			}
			break
		}
		if failed, _, message := readFailedAsyncVideoTask(method, capability, upstream.Body); failed {
			s.recordAutoAttempt(requestID, attemptNo, pool.ID, plan, upstream.StatusCode, upstream.ResponseTimeMs, false, "upstream_task_failed", false, message)
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, capability, modelName, method, path, upstream.StatusCode, upstream.Body, message, plan.route, requestSnapshot)
			firstErr = errors.New("上游异步任务创建失败")
			break
		}
		s.recordAutoAttempt(requestID, attemptNo, pool.ID, plan, upstream.StatusCode, upstream.ResponseTimeMs, true, "", false, "")
		s.recordModelSuccessWithRouteAndRequest(tenantID, userID, capability, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, plan.route, requestSnapshot)
		responseBody := upstream.Body
		if converted, ok := transformImageResponseToChatFormat(path, responseBody); ok {
			responseBody = converted
		}
		refund := 0
		if job != nil {
			taskID := ""
			if capability == "video" && method == http.MethodPost {
				taskID = readAsyncVideoTaskID(upstream.Body)
			}
			settlement, settleErr := s.generationBilling.Settle(job, GenerationSettlementInput{Amount: plan.cost, ChannelID: plan.candidate.Channel.ID, ChannelModelID: plan.candidate.ChannelModel.ID, ChannelName: plan.candidate.Channel.Name, ChannelBaseURL: plan.candidate.Channel.BaseUrl, VideoRoute: effectiveVideoRoute(plan.route), UpstreamTaskID: taskID})
			if settleErr != nil {
				s.refundGenerationReservation(job, settleErr.Error())
				return nil, settleErr
			}
			balance, refund = settlement.Balance, settlement.Refund
		}
		return &ProxyResult{StatusCode: upstream.StatusCode, Body: responseBody, Headers: upstream.Headers, Cost: plan.cost, Balance: balance, Refund: refund, ResolvedChannelID: plan.candidate.Channel.ID, ResolvedChannelModelID: plan.candidate.ChannelModel.ID, ResolvedChannelName: plan.candidate.Channel.Name, RequestID: requestID}, nil
	}
	refund, balance := s.refundGenerationReservation(job, errorText(firstErr, "智能路由所有候选均失败"))
	_ = refund
	if firstErr == nil {
		firstErr = errors.New("智能路由所有候选均失败")
	}
	return nil, fmt.Errorf("智能路由请求失败（请求 ID %s，余额 %d）：%w", requestID, balance, firstErr)
}

func autoCircuitRank(status string) int {
	switch status {
	case "closed":
		return 0
	case "half_open":
		return 1
	default:
		return 2
	}
}

func autoFailureMessage(category string) string {
	switch category {
	case "timeout":
		return "上游请求超时"
	case "network":
		return "上游网络连接失败"
	case "auth":
		return "上游渠道鉴权失败"
	case "rate_limited":
		return "上游渠道请求受限"
	case "upstream_unavailable":
		return "上游渠道暂时不可用"
	case "content_rejected":
		return "请求内容被上游拒绝"
	default:
		return "请求参数不受当前模型支持"
	}
}

func classifyAutoFailure(statusCode int, body []byte, requestErr error) (string, bool) {
	if requestErr != nil {
		if errors.Is(requestErr, context.DeadlineExceeded) || strings.Contains(strings.ToLower(requestErr.Error()), "timeout") {
			return "timeout", true
		}
		return "network", true
	}
	switch {
	case statusCode == http.StatusRequestTimeout:
		return "timeout", true
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return "auth", true
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited", true
	case statusCode >= 500:
		return "upstream_unavailable", true
	case strings.Contains(strings.ToLower(readErrorMessage(body)), "content") || strings.Contains(strings.ToLower(readErrorMessage(body)), "policy"):
		return "content_rejected", false
	default:
		return "request_invalid", false
	}
}

func errorText(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}

func (s *GenerateService) recordAutoAttempt(requestID string, attemptNo int, poolID uint, plan autoCandidatePlan, statusCode, responseTime int, success bool, category string, retryable bool, message string) {
	if s.autoChannelService == nil || requestID == "" {
		return
	}
	_ = s.autoChannelService.RecordAttempt(&model.GenerationAttempt{RequestID: requestID, AttemptNo: attemptNo, PoolID: poolID, ChannelID: plan.candidate.Channel.ID, ChannelModelID: plan.candidate.ChannelModel.ID, StatusCode: statusCode, ResponseTimeMs: responseTime, Success: success, FailureCategory: category, Retryable: retryable, CountsForHealth: success || retryable, ErrorMessage: cleanShort(message, 500)})
}

func (s *GenerateService) ResolveChannelRouteForEstimate(tenantID uint, selection ChannelSelection, capability, modelName, fuzzyGroupName string) (ResolvedEstimateRoute, error) {
	if strings.TrimSpace(fuzzyGroupName) != "" {
		resolver := s.resolveFuzzyMergeRoute
		if s.estimateFuzzyRoute != nil {
			resolver = s.estimateFuzzyRoute
		}
		route, err := resolver(selection.ChannelID, fuzzyGroupName, capability)
		return resolvedEstimateRoute(route, err)
	}
	if selection.SelectionKind() == ModelSelectionAuto {
		if s.autoChannelService == nil {
			return ResolvedEstimateRoute{}, errors.New("智能路由服务不可用")
		}
		if selection.AutoRoutingPoolID == 0 {
			return ResolvedEstimateRoute{}, errors.New("请选择有效的智能路由池")
		}
		_, candidates, err := s.autoChannelService.ResolveCandidates(selection.AutoRoutingPoolID, tenantID, capability, modelName)
		if err != nil {
			return ResolvedEstimateRoute{}, err
		}
		selections := make([]ChannelSelection, 0, len(candidates))
		for _, candidate := range candidates {
			selections = append(selections, ChannelSelection{Kind: ModelSelectionPhysical, ChannelID: candidate.Channel.ID, ChannelModelID: candidate.ChannelModel.ID})
		}
		return ResolvedEstimateRoute{Selection: selection, PricingModel: modelName, Candidates: selections}, nil
	}
	route, err := s.resolveChannelRoute(selection, capability, modelName)
	return resolvedEstimateRoute(route, err)
}

func resolvedEstimateRoute(route *channelRouteContext, err error) (ResolvedEstimateRoute, error) {
	if err != nil {
		return ResolvedEstimateRoute{}, err
	}
	if route == nil || route.ChannelModel == nil {
		return ResolvedEstimateRoute{}, errors.New("未解析到具体渠道模型")
	}
	return ResolvedEstimateRoute{
		Selection:    selectionFromRoute(route),
		PricingModel: route.ChannelModel.ModelName,
	}, nil
}

func selectionFromRoute(route *channelRouteContext) ChannelSelection {
	if route == nil || route.ChannelID == nil || route.ChannelModelID == nil {
		return ChannelSelection{}
	}
	return ChannelSelection{ChannelID: *route.ChannelID, ChannelModelID: *route.ChannelModelID}
}

func pricingIdentityFromRoute(route *channelRouteContext, fallbackChannelID uint, fallbackModel string) (uint, string) {
	channelID := fallbackChannelID
	modelName := fallbackModel
	if route != nil {
		if route.ChannelID != nil {
			channelID = *route.ChannelID
		}
		if route.ChannelModel != nil && strings.TrimSpace(route.ChannelModel.ModelName) != "" {
			modelName = route.ChannelModel.ModelName
		}
	}
	return channelID, modelName
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
	if primary.Kind == "" {
		primary.Kind = fallback.Kind
	}
	if primary.SelectionKind() != ModelSelectionAuto && primary.ChannelID == 0 {
		primary.ChannelID = fallback.ChannelID
	}
	if primary.SelectionKind() == ModelSelectionPhysical && primary.ChannelModelID == 0 {
		primary.ChannelModelID = fallback.ChannelModelID
	}
	if primary.AutoRoutingPoolID == 0 {
		primary.AutoRoutingPoolID = fallback.AutoRoutingPoolID
	}
	if strings.TrimSpace(primary.ModelName) == "" {
		primary.ModelName = fallback.ModelName
	}
	if strings.TrimSpace(primary.VideoRoute) == "" {
		primary.VideoRoute = fallback.VideoRoute
	}
	if strings.TrimSpace(primary.MergeGroupName) == "" {
		primary.MergeGroupName = fallback.MergeGroupName
	}
	if strings.TrimSpace(primary.Capability) == "" {
		primary.Capability = fallback.Capability
	}
	return primary
}

func normalizeVideoRoute(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "auto"
	}
	return value
}

func effectiveVideoRoute(route *channelRouteContext) string {
	if route != nil && route.Channel != nil && normalizeChannelVideoAPIStandard(route.Channel.VideoAPIStandard) == model.VideoAPIStandardBinghuo {
		return model.VideoAPIStandardBinghuo
	}
	if route == nil || route.ChannelModel == nil {
		return "auto"
	}
	return normalizeVideoRoute(route.ChannelModel.VideoRoute)
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
	selection := ChannelSelection{ChannelID: parseUintParam(values.Get("channel_id")), ChannelModelID: parseUintParam(values.Get("channel_model_id")), AutoRoutingPoolID: parseUintParam(values.Get("routing_pool_id")), MergeGroupName: strings.TrimSpace(values.Get("fuzzy_group_name"))}
	if selection.MergeGroupName != "" {
		selection.Kind = ModelSelectionMerge
	} else if selection.AutoRoutingPoolID > 0 {
		selection.Kind = ModelSelectionAuto
	} else if selection.ChannelID > 0 || selection.ChannelModelID > 0 {
		selection.Kind = ModelSelectionPhysical
	}
	return selection
}

func extractFuzzyGroupName(path string) string {
	parsed, err := url.Parse(path)
	if err != nil {
		return ""
	}
	return parsed.Query().Get("fuzzy_group_name")
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
	values.Del("routing_model")
	values.Del("routing_video_route")
	values.Del("routing_capability")
	values.Del("routing_pool_id")
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
	changed := false
	for _, key := range []string{"channel_id", "channel_model_id", "routing_pool_id", "routing_model", "routing_capability", "routing_video_route", "fuzzy_group_name"} {
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	if !changed {
		return body
	}
	updated, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return updated
}

func buildModelCallRequestSnapshot(route *channelRouteContext, method, path, contentType string, body []byte) *modelCallRequestSnapshot {
	if route == nil || route.Channel == nil {
		return nil
	}
	return &modelCallRequestSnapshot{
		UpstreamURL:   sanitizeLoggedUpstreamURL(buildUpstreamURL(route.Channel.BaseUrl, path)),
		ContentType:   strings.TrimSpace(contentType),
		Body:          formatRawLoggedRequestBody(body),
		BodyTruncated: false,
		Sent:          true,
	}
}

func formatRawLoggedRequestBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if utf8.Valid(body) {
		return string(body)
	}
	return "[base64 encoded raw request body]\n" + base64.StdEncoding.EncodeToString(body)
}

func sanitizeLoggedUpstreamURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return truncateString(rawURL, 1000)
	}
	parsed.User = nil
	values := parsed.Query()
	for key := range values {
		if isSensitiveLogKey(key) {
			values.Set(key, "[redacted]")
		}
	}
	parsed.RawQuery = values.Encode()
	return truncateString(parsed.String(), 1000)
}

func formatLoggedRequestBody(contentType string, body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
		params = map[string]string{}
	}
	mediaType = strings.ToLower(mediaType)
	var text string
	var truncated bool
	switch {
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		text, truncated = formatLoggedJSONBody(body)
	case mediaType == "multipart/form-data":
		text, truncated = formatLoggedMultipartBody(params["boundary"], body)
	case mediaType == "application/x-www-form-urlencoded":
		text, truncated = formatLoggedFormBody(body)
	case isTextRequestBody(mediaType, body):
		text, truncated = truncateLoggedText(string(body), maxLoggedRequestBodyChars)
	default:
		text = fmt.Sprintf("[binary body omitted, %d bytes]", len(body))
	}
	text, finalTruncated := truncateLoggedText(text, maxLoggedRequestBodyChars)
	return text, truncated || finalTruncated
}

func formatLoggedJSONBody(body []byte) (string, bool) {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return truncateLoggedText(string(body), maxLoggedRequestBodyChars)
	}
	sanitized, truncated := sanitizeLoggedJSONValue(payload)
	encoded, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return truncateLoggedText(string(body), maxLoggedRequestBodyChars)
	}
	text, finalTruncated := truncateLoggedText(string(encoded), maxLoggedRequestBodyChars)
	return text, truncated || finalTruncated
}

func sanitizeLoggedJSONValue(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		truncated := false
		for key, item := range typed {
			if isSensitiveLogKey(key) {
				result[key] = "[redacted]"
				truncated = true
				continue
			}
			next, nextTruncated := sanitizeLoggedJSONValue(item)
			result[key] = next
			truncated = truncated || nextTruncated
		}
		return result, truncated
	case []interface{}:
		result := make([]interface{}, len(typed))
		truncated := false
		for index, item := range typed {
			next, nextTruncated := sanitizeLoggedJSONValue(item)
			result[index] = next
			truncated = truncated || nextTruncated
		}
		return result, truncated
	case string:
		return sanitizeLoggedString(typed)
	default:
		return typed, false
	}
}

func sanitizeLoggedString(value string) (string, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return value, false
	}
	if strings.HasPrefix(strings.ToLower(text), "data:") {
		return fmt.Sprintf("[data url omitted, %d chars]", len([]rune(value))), true
	}
	if looksLikeLongBase64(text) {
		return fmt.Sprintf("[base64 omitted, %d chars]", len([]rune(value))), true
	}
	return truncateLoggedText(value, maxLoggedRequestStringChars)
}

func looksLikeLongBase64(value string) bool {
	if len(value) < 200 || strings.ContainsAny(value, " \r\n\t:/?&") {
		return false
	}
	for _, ch := range value {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '+' || ch == '/' || ch == '=' || ch == '-' || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func formatLoggedMultipartBody(boundary string, body []byte) (string, bool) {
	if boundary == "" {
		return fmt.Sprintf("[multipart body omitted, missing boundary, %d bytes]", len(body)), true
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	lines := []string{}
	truncated := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Sprintf("[multipart body omitted, parse failed, %d bytes]", len(body)), true
		}
		name := part.FormName()
		filename := part.FileName()
		partContentType := part.Header.Get("Content-Type")
		partBody, _ := io.ReadAll(part)
		if filename != "" {
			lines = append(lines, fmt.Sprintf("%s: [file filename=%q content_type=%q size=%d bytes]", name, filename, partContentType, len(partBody)))
			truncated = true
			continue
		}
		value, valueTruncated := sanitizeLoggedString(string(partBody))
		lines = append(lines, fmt.Sprintf("%s: %s", name, value))
		truncated = truncated || valueTruncated
	}
	return strings.Join(lines, "\n"), truncated
}

func formatLoggedFormBody(body []byte) (string, bool) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return truncateLoggedText(string(body), maxLoggedRequestBodyChars)
	}
	result := map[string]interface{}{}
	truncated := false
	for key, items := range values {
		if isSensitiveLogKey(key) {
			result[key] = "[redacted]"
			truncated = true
			continue
		}
		if len(items) == 1 {
			value, valueTruncated := sanitizeLoggedString(items[0])
			result[key] = value
			truncated = truncated || valueTruncated
		} else {
			array := make([]interface{}, len(items))
			for index, item := range items {
				value, valueTruncated := sanitizeLoggedString(item)
				array[index] = value
				truncated = truncated || valueTruncated
			}
			result[key] = array
		}
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return truncateLoggedText(string(body), maxLoggedRequestBodyChars)
	}
	text, finalTruncated := truncateLoggedText(string(encoded), maxLoggedRequestBodyChars)
	return text, truncated || finalTruncated
}

func isTextRequestBody(mediaType string, body []byte) bool {
	if mediaType == "application/octet-stream" || mediaType == "binary/octet-stream" || strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "video/") || strings.HasPrefix(mediaType, "audio/") {
		return false
	}
	if strings.HasPrefix(mediaType, "text/") || strings.Contains(mediaType, "xml") {
		return true
	}
	for _, b := range body {
		if b == 0 {
			return false
		}
	}
	return true
}

func isSensitiveLogKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "apikey") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "authorization") ||
		lower == "key"
}

func truncateLoggedText(value string, limit int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]) + fmt.Sprintf("\n[truncated, %d chars omitted]", len(runes)-limit), true
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

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, upstreamMaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(respBytes) > upstreamMaxResponseBytes {
		return nil, errors.New("上游响应内容过大")
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
	if err := validateModelSelection(selection); err != nil {
		return nil, err
	}
	path = stripChannelIdentityQuery(path)
	body = stripJSONChannelIdentity(contentType, body)
	if normalizedBody, changed := normalizeVideoReferenceImages(http.MethodPost, path, contentType, body); changed {
		log.Printf("compressed video reference image payload path=%s", cleanPath(path))
		body = normalizedBody
	}

	modelName := extractProxyModelName(contentType, body, selection)
	if modelName == "" {
		err := errors.New("请指定模型")
		s.recordModelFailureWithSelection(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), selection)
		return nil, err
	}

	// Auto channel routing with success-rate priority failover
	if selection.SelectionKind() == ModelSelectionAuto {
		if s.autoChannelService == nil {
			return nil, errors.New("智能路由服务不可用")
		}
		result, err := s.proxyWithAutoFailover(tenantID, userID, http.MethodPost, genType, path, contentType, body, modelName, selection.VideoRoute, selection.AutoRoutingPoolID)
		if err != nil {
			s.recordModelFailureWithAutoSelection(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error())
			return nil, err
		}
		return result, nil
	}

	var route *channelRouteContext
	var err error
	fuzzyGroupName := strings.TrimSpace(selection.MergeGroupName)
	if fuzzyGroupName == "" {
		fuzzyGroupName = extractFuzzyGroupName(path)
	}
	if fuzzyGroupName != "" {
		route, err = s.resolveFuzzyMergeRoute(selection.ChannelID, fuzzyGroupName, genType)
		if err != nil && strings.Contains(err.Error(), "未找到合并组") {
			// Group not found, fall through to normal route
		} else if err != nil {
			s.recordModelFailureWithSelection(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), selection)
			return nil, err
		}
	}
	if route == nil {
		route, err = s.resolveChannelRoute(selection, genType, modelName)
	}
	if err != nil {
		s.recordModelFailureWithSelection(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), selection)
		return nil, err
	}
	if err := validateResolvedRequestProtocol(route, genType, http.MethodPost, path, contentType, body); err != nil {
		return nil, err
	}

	pricingChannelID, pricingModel := pricingIdentityFromRoute(route, selection.ChannelID, modelName)
	cost, pricingResult, err := s.getRequiredPricing(tenantID, pricingChannelID, genType, pricingModel, contentType, body)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		return nil, err
	}

	job, balance, err := s.reserveGenerationCredits(tenantID, userID, cost, genType, pricingModel, path, pricingResult, route)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route)
		return nil, err
	}

	requestSnapshot := buildModelCallRequestSnapshot(route, http.MethodPost, path, contentType, body)
	upstream, err := s.doUpstreamRequest(http.MethodPost, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, genType, modelName, http.MethodPost, path, 0, nil, err.Error(), route, requestSnapshot)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, genType, modelName, http.MethodPost, path, contentType, body, 0, nil, err.Error(), route); ok {
			upstream = retry
		} else {
			s.refundGenerationReservation(job, err.Error())
			return nil, fmt.Errorf("上游 API 请求失败: %v", err)
		}
	}
	if upstream == nil {
		s.refundGenerationReservation(job, "upstream request failed")
		return nil, errors.New("upstream request failed")
	}
	respBytes := upstream.Body
	if upstream.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if upstream.StatusCode >= 400 {
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, respBytes, "", route, requestSnapshot)
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
	alertStatus, alertMessage := s.handleUpstreamWebhookAlert(tenantID, modelName, respBytes, route)
	if alertStatus != "" {
		if upstream.StatusCode < 400 {
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, respBytes, alertMessage, route, requestSnapshot)
		}
		s.refundGenerationReservation(job, alertMessage)
		return nil, errors.New(alertMessage)
	}
	if upstream.StatusCode >= 400 {
		refund, refundBalance := s.refundGenerationReservation(job, fmt.Sprintf("上游返回 %d", upstream.StatusCode))
		return &ProxyResult{
			StatusCode: upstream.StatusCode,
			Body:       respBytes,
			Headers:    upstream.Headers,
			Balance:    refundBalance,
			Refund:     refund,
			RequestID:  generationRequestID(job),
		}, nil
	}

	if failed, responseModel, message := readFailedAsyncVideoTask(http.MethodPost, genType, respBytes); failed {
		if modelName == "" {
			modelName = responseModel
		}
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, respBytes, message, route, requestSnapshot)
		refund, refundBalance := s.refundGenerationReservation(job, message)
		return &ProxyResult{
			StatusCode:             upstream.StatusCode,
			Body:                   respBytes,
			Headers:                upstream.Headers,
			Cost:                   0,
			Balance:                refundBalance,
			Refund:                 refund,
			ResolvedChannelID:      selectionFromRoute(route).ChannelID,
			ResolvedChannelModelID: selectionFromRoute(route).ChannelModelID,
			ResolvedChannelName:    resolvedChannelName(route),
			RequestID:              generationRequestID(job),
		}, nil
	}

	s.recordModelSuccessWithRouteAndRequest(tenantID, userID, genType, modelName, http.MethodPost, path, upstream.StatusCode, upstream.ResponseTimeMs, route, requestSnapshot)

	if err := s.succeedGenerationReservation(job, respBytes); err != nil {
		log.Printf("failed to mark generation reservation succeeded: %v", err)
	}

	return &ProxyResult{
		StatusCode:             upstream.StatusCode,
		Body:                   respBytes,
		Headers:                upstream.Headers,
		Cost:                   cost,
		Balance:                balance,
		ResolvedChannelID:      selectionFromRoute(route).ChannelID,
		ResolvedChannelModelID: selectionFromRoute(route).ChannelModelID,
		ResolvedChannelName:    resolvedChannelName(route),
	}, nil
}

func (s *GenerateService) getRequiredPricing(tenantID uint, channelID uint, genType, modelName, contentType string, body []byte) (int, CreditCostResult, error) {
	pricing, err := s.creditRepo.FindPricing(tenantID, modelName, channelID)
	if err != nil {
		return 0, CreditCostResult{}, err
	}
	if pricing == nil {
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

func extractProxyModelName(contentType string, body []byte, selection ChannelSelection) string {
	if modelName := strings.TrimSpace(extractModelName(contentType, body)); modelName != "" {
		return modelName
	}
	return strings.TrimSpace(selection.ModelName)
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
	return s.ProxyRawWithRepair(tenantID, userID, method, path, contentType, body, selection)
}

func (s *GenerateService) ProxyRawWithRepair(tenantID, userID uint, method, path, contentType string, body []byte, selection ChannelSelection) (*ProxyResult, error) {
	selection = mergeSelection(selection, extractChannelSelection(contentType, body, path))
	if err := validateModelSelection(selection); err != nil {
		return nil, err
	}
	path = stripChannelIdentityQuery(path)
	body = stripJSONChannelIdentity(contentType, body)
	if normalizedBody, changed := normalizeVideoReferenceImages(method, path, contentType, body); changed {
		log.Printf("compressed video reference image payload path=%s", cleanPath(path))
		body = normalizedBody
	}

	modelName := extractProxyModelName(contentType, body, selection)
	generation := generationTypeForSelection(selection, path)
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
		s.recordModelFailureWithSelection(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), selection)
		return nil, err
	}

	// Auto channel routing with success-rate priority failover
	if selection.SelectionKind() == ModelSelectionAuto {
		if s.autoChannelService == nil {
			return nil, errors.New("智能路由服务不可用")
		}
		result, err := s.proxyWithAutoFailover(tenantID, userID, method, generation, path, contentType, body, modelName, selection.VideoRoute, selection.AutoRoutingPoolID)
		if err != nil {
			s.recordModelFailureWithAutoSelection(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error())
			return nil, err
		}
		return result, nil
	}

	var route *channelRouteContext
	var err error
	fuzzyGroupName := strings.TrimSpace(selection.MergeGroupName)
	if fuzzyGroupName == "" {
		fuzzyGroupName = extractFuzzyGroupName(path)
	}
	if fuzzyGroupName != "" {
		route, err = s.resolveFuzzyMergeRoute(selection.ChannelID, fuzzyGroupName, generation)
		if err != nil && strings.Contains(err.Error(), "未找到合并组") {
			// Group not found, fall through to normal route
		} else if err != nil {
			s.recordModelFailureWithSelection(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), selection)
			return nil, err
		}
	}
	if route == nil {
		route, err = s.resolveChannelRoute(selection, generation, modelName)
	}
	if err != nil {
		s.recordModelFailureWithSelection(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), selection)
		return nil, err
	}
	if err := validateResolvedRequestProtocol(route, generation, method, path, contentType, body); err != nil {
		return nil, err
	}
	pricingChannelID, pricingModel := pricingIdentityFromRoute(route, selection.ChannelID, modelName)
	cost, chargeType, pricingResult, err := s.getProxyCostByGeneration(tenantID, pricingChannelID, method, generation, contentType, body, pricingModel)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route)
		return nil, err
	}

	job, balance, err := s.reserveGenerationCredits(tenantID, userID, cost, chargeType, pricingModel, path, pricingResult, route)
	if err != nil {
		s.recordModelFailureWithRoute(tenantID, userID, chargeType, modelName, method, path, 0, nil, err.Error(), route)
		return nil, err
	}

	requestSnapshot := buildModelCallRequestSnapshot(route, method, path, contentType, body)
	upstream, err := s.doUpstreamRequest(method, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route, requestSnapshot)
		if retry, ok := s.repairAndRetryUpstream(tenantID, userID, generation, modelName, method, path, contentType, body, 0, nil, err.Error(), route); ok {
			upstream = retry
		} else {
			s.refundGenerationReservation(job, err.Error())
			return nil, fmt.Errorf("upstream API request failed: %v", err)
		}
	}
	if upstream == nil {
		s.refundGenerationReservation(job, "upstream request failed")
		return nil, errors.New("upstream request failed")
	}

	respBytes := upstream.Body
	if upstream.StatusCode < 400 {
		if converted, ok := transformImageResponseToChatFormat(path, respBytes); ok {
			respBytes = converted
		}
	}

	if upstream.StatusCode >= 400 && generation != "" {
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, respBytes, "", route, requestSnapshot)
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

	alertStatus, alertMessage := s.handleUpstreamWebhookAlert(tenantID, modelName, respBytes, route)
	if alertStatus == WebhookStatusUserQuotaInsufficient {
		s.refundGenerationReservation(job, alertMessage)
		return nil, errors.New(alertMessage)
	}
	if alertStatus == WebhookStatusModelUnavailable && strings.ToUpper(strings.TrimSpace(method)) != http.MethodGet {
		if upstream.StatusCode < 400 {
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, respBytes, alertMessage, route, requestSnapshot)
		}
		s.refundGenerationReservation(job, alertMessage)
		return nil, errors.New(alertMessage)
	}

	refund := 0
	asyncFailed := false
	if upstream.StatusCode < 400 {
		if failed, responseModel, message := readFailedAsyncVideoTask(method, generation, respBytes); failed {
			asyncFailed = true
			if modelName == "" {
				modelName = responseModel
			}
			s.recordModelFailureWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, respBytes, message, route, requestSnapshot)
			requestContext := buildRepairRequestContext(generation, method, path, "application/json", respBytes)
			s.triggerOnDemandRepairAsync(generation, modelName, message, requestContext)
			if strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet {
				refund, balance = s.refundFailedAsyncTask(tenantID, userID, generation, modelName, path, respBytes, message, route)
			} else {
				refund, balance = s.refundGenerationReservation(job, message)
			}
		} else if strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet && generation != "" && modelName != "" {
			s.recordModelSuccessWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route, requestSnapshot)
		} else if generation != "" && modelName != "" {
			s.recordModelSuccessWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, upstream.StatusCode, upstream.ResponseTimeMs, route, requestSnapshot)
		}
	}

	if upstream.StatusCode >= 400 {
		refund, balance = s.refundGenerationReservation(job, fmt.Sprintf("上游返回 %d", upstream.StatusCode))
	} else if !asyncFailed {
		if strings.ToUpper(strings.TrimSpace(method)) == http.MethodPost {
			if err := s.succeedGenerationReservation(job, respBytes); err != nil {
				log.Printf("failed to mark generation reservation succeeded: %v", err)
			}
		} else {
			s.completeAsyncGenerationTask(tenantID, userID, route, path, respBytes)
		}
	}
	resultCost := cost
	if upstream.StatusCode >= 400 || asyncFailed && strings.ToUpper(strings.TrimSpace(method)) == http.MethodPost {
		resultCost = 0
	}

	return &ProxyResult{
		StatusCode:             upstream.StatusCode,
		Body:                   respBytes,
		Headers:                upstream.Headers,
		Cost:                   resultCost,
		Balance:                balance,
		Refund:                 refund,
		ResolvedChannelID:      selectionFromRoute(route).ChannelID,
		ResolvedChannelModelID: selectionFromRoute(route).ChannelModelID,
		ResolvedChannelName:    resolvedChannelName(route),
		RequestID:              generationRequestID(job),
	}, nil
}

func resolvedChannelName(route *channelRouteContext) string {
	if route == nil || route.Channel == nil {
		return ""
	}
	return route.Channel.Name
}

func (s *GenerateService) getProxyCostByGeneration(tenantID uint, channelID uint, method, generation, contentType string, body []byte, modelName string) (int, string, CreditCostResult, error) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return 0, generation, CreditCostResult{}, nil
	}
	if generation == "" || modelName == "" {
		return 0, generation, CreditCostResult{}, nil
	}
	cost, result, err := s.getRequiredPricing(tenantID, channelID, generation, modelName, contentType, body)
	if err != nil {
		return 0, generation, CreditCostResult{}, err
	}
	return cost, generation, result, nil
}

func asyncVideoTaskRefID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}
	return "video_task:" + taskID
}

func asyncVideoSpendIdempotencyKey(userID uint, refID string) string {
	if strings.TrimSpace(refID) == "" {
		return ""
	}
	return fmt.Sprintf("spend:video:%d:%s", userID, refID)
}

func asyncVideoRefundIdempotencyKey(userID uint, refID string) string {
	if strings.TrimSpace(refID) == "" {
		return ""
	}
	return fmt.Sprintf("refund:video:%d:%s", userID, refID)
}

func readAsyncVideoTaskID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	return readStringPath(payload, "id", "task_id", "request_id", "data.id", "data.task_id", "data.request_id")
}

func readAsyncVideoTaskIDFromPath(path string) string {
	clean := strings.Trim(strings.Split(strings.TrimSpace(path), "?")[0], "/")
	if clean == "" {
		return ""
	}
	parts := strings.Split(clean, "/")
	for i, part := range parts {
		switch part {
		case "videos":
			if i+1 < len(parts) && parts[i+1] != "generations" && parts[i+1] != "content" {
				return parts[i+1]
			}
		case "generations":
			if i > 0 && parts[i-1] == "video" && i+1 < len(parts) {
				return parts[i+1]
			}
		case "tasks":
			if i > 0 && parts[i-1] == "generations" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}

func readFailedAsyncVideoTask(method, generation string, body []byte) (bool, string, string) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if generation != "video" || (normalizedMethod != http.MethodGet && normalizedMethod != http.MethodPost) {
		return false, "", ""
	}
	return readFailedModelTaskResponse(body)
}

func buildCreditSpendDetailForResponse(genType, modelName, path string, cost CreditCostResult, responseBody []byte, route *channelRouteContext) (string, string, string) {
	metadata, note := buildCreditSpendDetail(genType, modelName, path, cost)
	refID := modelName
	if genType != "video" {
		return metadata, note, refID
	}
	taskID := readAsyncVideoTaskID(responseBody)
	if taskID == "" {
		return metadata, note, refID
	}
	refID = asyncVideoTaskRefID(taskID)
	if refID == "" {
		return metadata, note, modelName
	}
	values := map[string]interface{}{
		"task_id":           taskID,
		"async_task_ref_id": refID,
	}
	if route != nil {
		if route.ChannelID != nil {
			values["channel_id"] = *route.ChannelID
		}
		if route.ChannelModelID != nil {
			values["channel_model_id"] = *route.ChannelModelID
		}
	}
	metadata = mergeCreditMetadata(metadata, values)
	note = fmt.Sprintf("%s · 任务 %s", note, cleanShort(taskID, 60))
	return metadata, note, refID
}

func (s *GenerateService) reserveGenerationCredits(tenantID, userID uint, amount int, genType, modelName, path string, cost CreditCostResult, route *channelRouteContext, autoPoolIDs ...uint) (*model.GenerationJob, int, error) {
	if amount <= 0 {
		if s.creditService == nil {
			return nil, 0, nil
		}
		account, err := s.creditService.GetOrCreateAccount(tenantID, userID)
		if err != nil || account == nil {
			return nil, 0, err
		}
		return nil, account.Balance, nil
	}
	if s.generationBilling == nil {
		return nil, 0, errors.New("生成计费服务未配置")
	}
	metadata, note := buildCreditSpendDetail(genType, modelName, path, cost)
	autoPoolID := uint(0)
	if len(autoPoolIDs) > 0 {
		autoPoolID = autoPoolIDs[0]
	}
	selection := selectionFromRoute(route)
	channelName := ""
	channelBaseURL := ""
	videoRoute := ""
	if route != nil {
		if route.Channel != nil {
			channelName = route.Channel.Name
			channelBaseURL = route.Channel.BaseUrl
		}
		videoRoute = effectiveVideoRoute(route)
		metadata = mergeCreditMetadata(metadata, map[string]interface{}{
			"channel_id": selection.ChannelID, "channel_model_id": selection.ChannelModelID,
		})
	}
	return s.generationBilling.Reserve(GenerationReservationInput{
		TenantID: tenantID, UserID: userID, Capability: genType, ModelName: modelName, AutoRoutingPoolID: autoPoolID,
		ChannelID: selection.ChannelID, ChannelModelID: selection.ChannelModelID,
		ChannelName: channelName, ChannelBaseURL: channelBaseURL, VideoRoute: videoRoute,
		Amount: amount, Note: note, Metadata: metadata,
	})
}

func (s *GenerateService) succeedGenerationReservation(job *model.GenerationJob, responseBody []byte) error {
	if job == nil {
		return nil
	}
	taskID := ""
	if job.Capability == "video" {
		taskID = readAsyncVideoTaskID(responseBody)
	}
	return s.generationBilling.Succeed(job, taskID)
}

func (s *GenerateService) refundGenerationReservation(job *model.GenerationJob, reason string) (int, int) {
	if job == nil || s.generationBilling == nil {
		return 0, 0
	}
	result, err := s.generationBilling.Refund(job, reason)
	if err != nil {
		log.Printf("failed to refund generation reservation request=%s: %v", job.RequestID, err)
		return 0, 0
	}
	if result == nil {
		return 0, 0
	}
	if !result.Refunded {
		return 0, result.Balance
	}
	return result.Amount, result.Balance
}

func generationRequestID(job *model.GenerationJob) string {
	if job == nil {
		return ""
	}
	return job.RequestID
}

func mergeCreditMetadata(metadata string, values map[string]interface{}) string {
	payload := map[string]interface{}{}
	if strings.TrimSpace(metadata) != "" {
		_ = json.Unmarshal([]byte(metadata), &payload)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	for key, value := range values {
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return metadata
	}
	return string(data)
}

func (s *GenerateService) refundFailedAsyncTask(tenantID, userID uint, generation, modelName, path string, responseBody []byte, message string, route *channelRouteContext) (int, int) {
	if s == nil || s.creditService == nil || generation != "video" || strings.TrimSpace(modelName) == "" {
		return 0, 0
	}
	taskID := readAsyncVideoTaskID(responseBody)
	if taskID == "" {
		taskID = readAsyncVideoTaskIDFromPath(path)
	}
	refID := asyncVideoTaskRefID(taskID)
	if refID == "" {
		log.Printf("skip async video refund: missing task id user=%d model=%s path=%s", userID, modelName, cleanPath(path))
		return 0, 0
	}
	if s.generationBilling != nil && route != nil && route.ChannelModelID != nil {
		result, err := s.generationBilling.RefundTask(tenantID, userID, *route.ChannelModelID, taskID, message)
		if err == nil && result != nil {
			if result.Refunded {
				return result.Amount, result.Balance
			}
			return 0, result.Balance
		}
		if err != nil && !errors.Is(err, repository.ErrGenerationJobNotFound) {
			log.Printf("failed to refund generation job user=%d task=%s model=%s: %v", userID, taskID, modelName, err)
		}
	}
	metadata := BuildCreditMetadata(map[string]interface{}{
		"scene":      generationTypeLabel(generation),
		"generation": generation,
		"model":      modelName,
		"path":       cleanPath(path),
		"task_id":    taskID,
		"status":     "failed",
		"error":      cleanShort(message, 500),
	})
	if route != nil {
		values := map[string]interface{}{}
		if route.ChannelID != nil {
			values["channel_id"] = *route.ChannelID
		}
		if route.ChannelModelID != nil {
			values["channel_model_id"] = *route.ChannelModelID
		}
		if len(values) > 0 {
			metadata = mergeCreditMetadata(metadata, values)
		}
	}
	note := fmt.Sprintf("视频生成失败自动退款: %s", cleanShort(message, 300))
	result, err := s.creditService.RefundAsyncSpendOnce(userID, generation, refID, asyncVideoRefundIdempotencyKey(userID, refID), note, metadata)
	if err != nil {
		log.Printf("failed to refund async video task user=%d task=%s model=%s: %v", userID, taskID, modelName, err)
		return 0, 0
	}
	if result == nil {
		return 0, 0
	}
	if result.AlreadyExists {
		return 0, result.Balance
	}
	if !result.SpendFound {
		log.Printf("skip async video refund: original spend not found user=%d task=%s model=%s", userID, taskID, modelName)
		return 0, result.Balance
	}
	if result.Refunded {
		return result.Amount, result.Balance
	}
	return 0, result.Balance
}

func (s *GenerateService) completeAsyncGenerationTask(tenantID, userID uint, route *channelRouteContext, path string, responseBody []byte) {
	if s.generationBilling == nil || route == nil || route.ChannelModelID == nil {
		return
	}
	taskID := readAsyncVideoTaskID(responseBody)
	if taskID == "" {
		taskID = readAsyncVideoTaskIDFromPath(path)
	}
	if taskID == "" {
		return
	}
	if err := s.generationBilling.CompleteTask(tenantID, userID, *route.ChannelModelID, taskID); err != nil {
		log.Printf("failed to complete generation job user=%d task=%s: %v", userID, taskID, err)
	}
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
	requestSnapshot := buildModelCallRequestSnapshot(route, method, path, contentType, body)
	retry, err := s.doUpstreamRequest(method, route.Channel.BaseUrl, route.ApiKey, path, contentType, body)
	if err != nil {
		s.recordModelFailureWithRouteAndRequest(tenantID, userID, generation, modelName, method, path, 0, nil, err.Error(), route, requestSnapshot)
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
	if IsUpstreamBalanceError(message) {
		return false
	}
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

// UpstreamBalanceErrorKeywords controls HTTP error passthrough and repair decisions.
// Automatic webhook alerts use the stricter classifier in webhook_service.go.
var UpstreamBalanceErrorKeywords = []string{
	"扣费额度失败",
	"余额不足",
	"额度不足",
	"用户额度不足",
	"insufficient balance",
	"insufficient_quota",
	"insufficient_user_quota",
	"quota exceeded",
	"billing failed",
}

// IsUpstreamBalanceError checks if the response body indicates an upstream balance/credit issue.
func IsUpstreamBalanceError(body string) bool {
	lower := strings.ToLower(body)
	for _, kw := range UpstreamBalanceErrorKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func (s *GenerateService) handleUpstreamWebhookAlert(tenantID uint, modelName string, body []byte, route *channelRouteContext) (string, string) {
	alert, ok := classifyUpstreamWebhookAlert(body)
	if !ok {
		return "", ""
	}
	event := upstreamWebhookEvent{
		TenantID:  tenantID,
		ModelName: strings.TrimSpace(modelName),
		Status:    alert.Status,
		Reason:    alert.Reason,
	}
	if route != nil && route.Channel != nil {
		event.ChannelID = route.Channel.ID
		event.ChannelName = route.Channel.Name
	}
	clientMessage := alert.Reason
	if alert.Status == WebhookStatusUserQuotaInsufficient {
		clientMessage = "因上游问题被禁用"
		switch {
		case event.ChannelID == 0:
			event.Action = "未定位到具体渠道，未执行自动禁用"
		case s.channelSvc == nil:
			event.Action = "渠道服务不可用，自动禁用失败"
		default:
			if err := s.channelSvc.Disable(event.ChannelID); err != nil {
				event.Action = "自动禁用渠道失败: " + err.Error()
				log.Printf("auto-disable channel %d after upstream user quota alert: %v", event.ChannelID, err)
			} else {
				event.Action = "已自动禁用渠道"
				log.Printf("auto-disabled channel %d after upstream user quota alert", event.ChannelID)
			}
		}
	}
	s.webhookService.NotifyUpstreamAlertAsync(event)
	return alert.Status, clientMessage
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
	case "image", "images", "image_url", "image_urls", "first_image", "first_image_url", "last_image", "last_image_url", "start_frame", "end_frame", "input_reference", "reference_image", "reference_images", "reference_image_urls", "reference_video", "reference_videos", "reference_video_url", "reference_video_urls", "reference_audio", "reference_audios", "video", "video_url", "references", "inline_data", "filedata", "file_data":
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
	for _, key := range []string{"video", "video_url", "reference_video", "reference_videos", "reference_video_url", "reference_video_urls", "input_video"} {
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

func (s *GenerateService) recordModelFailureWithSelection(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string, selection ChannelSelection) {
	s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, method, path, statusCode, body, fallback, routeIdentityFromSelection(selection, false))
}

func (s *GenerateService) recordModelFailureWithAutoSelection(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string) {
	s.recordModelFailureWithRoute(tenantID, userID, genType, modelName, method, path, statusCode, body, fallback, routeIdentityFromSelection(ChannelSelection{}, true))
}

func (s *GenerateService) recordModelFailureWithRoute(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string, route *channelRouteContext) {
	s.recordModelFailureWithRouteAndRequest(tenantID, userID, genType, modelName, method, path, statusCode, body, fallback, route, nil)
}

func (s *GenerateService) recordModelFailureWithRouteAndRequest(tenantID, userID uint, genType, modelName, method, path string, statusCode int, body []byte, fallback string, route *channelRouteContext, request *modelCallRequestSnapshot) {
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
	input := ModelCallLogInput{
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
	}
	if request != nil {
		input.UpstreamURL = request.UpstreamURL
		input.RequestContentType = request.ContentType
		input.RequestBody = request.Body
		input.RequestBodyTruncated = request.BodyTruncated
		input.RequestSent = request.Sent
	}
	s.logService.RecordFailure(input)
}

func routeIdentityFromSelection(selection ChannelSelection, includeAuto bool) *channelRouteContext {
	if selection.ChannelID == 0 && selection.ChannelModelID == 0 {
		if includeAuto {
			channelID := uint(0)
			return &channelRouteContext{ChannelID: &channelID}
		}
		return nil
	}
	route := &channelRouteContext{}
	if selection.ChannelID > 0 {
		route.ChannelID = uintPtr(selection.ChannelID)
	}
	if selection.ChannelModelID > 0 {
		route.ChannelModelID = uintPtr(selection.ChannelModelID)
	}
	return route
}

func (s *GenerateService) recordModelSuccess(tenantID, userID uint, genType, modelName, method, path string, statusCode, responseTimeMs int) {
	s.recordModelSuccessWithRoute(tenantID, userID, genType, modelName, method, path, statusCode, responseTimeMs, nil)
}

func (s *GenerateService) recordModelSuccessWithRoute(tenantID, userID uint, genType, modelName, method, path string, statusCode, responseTimeMs int, route *channelRouteContext) {
	s.recordModelSuccessWithRouteAndRequest(tenantID, userID, genType, modelName, method, path, statusCode, responseTimeMs, route, nil)
}

func (s *GenerateService) recordModelSuccessWithRouteAndRequest(tenantID, userID uint, genType, modelName, method, path string, statusCode, responseTimeMs int, route *channelRouteContext, request *modelCallRequestSnapshot) {
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
	input := ModelCallLogInput{
		TenantID:       tenantID,
		UserID:         userID,
		Generation:     genType,
		Model:          modelName,
		Method:         method,
		Path:           path,
		StatusCode:     statusCode,
		ChannelID:      channelID,
		ChannelModelID: channelModelID,
	}
	if request != nil {
		input.UpstreamURL = request.UpstreamURL
		input.RequestContentType = request.ContentType
		input.RequestBody = request.Body
		input.RequestBodyTruncated = request.BodyTruncated
		input.RequestSent = request.Sent
	}
	s.logService.RecordSuccess(input, responseTimeMs)
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

func generationTypeForSelection(selection ModelSelection, path string) string {
	capability := strings.ToLower(strings.TrimSpace(selection.Capability))
	switch capability {
	case "image", "video", "text", "audio":
		return capability
	default:
		return generationTypeFromPath(path)
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
