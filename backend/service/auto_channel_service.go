package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

const (
	autoHealthWindow       = 24 * time.Hour
	autoCircuitWindow      = 5 * time.Minute
	autoCircuitCooldown    = 2 * time.Minute
	autoCircuitThreshold   = 3
	autoDefaultMaxAttempts = 2
)

type AutoChannelService struct {
	channelRepo      *repository.ChannelRepo
	channelModelRepo *repository.ChannelModelRepo
	creditRepo       *repository.CreditRepo
	routingRepo      *repository.AutoRoutingRepo
	halfOpenMu       sync.Mutex
	halfOpen         map[uint]bool
}

func NewAutoChannelService(_ *gorm.DB, channelRepo *repository.ChannelRepo, channelModelRepo *repository.ChannelModelRepo, creditRepo *repository.CreditRepo, routingRepo *repository.AutoRoutingRepo) *AutoChannelService {
	return &AutoChannelService{channelRepo: channelRepo, channelModelRepo: channelModelRepo, creditRepo: creditRepo, routingRepo: routingRepo, halfOpen: map[uint]bool{}}
}

type AggregatedChannelRef struct {
	ChannelID         uint    `json:"channel_id"`
	ChannelModelID    uint    `json:"channel_model_id"`
	ChannelName       string  `json:"channel_name"`
	SuccessRate       float64 `json:"success_rate"`
	SampleCount       int     `json:"sample_count"`
	P95LatencyMs      int     `json:"p95_latency_ms"`
	Priority          int     `json:"priority"`
	CircuitStatus     string  `json:"circuit_status"`
	Available         bool    `json:"available"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
}

type AggregatedModel struct {
	PoolID         uint                   `json:"pool_id"`
	Model          string                 `json:"model"`
	Capability     string                 `json:"capability"`
	ContractKey    string                 `json:"contract_key"`
	Channels       []AggregatedChannelRef `json:"channels"`
	MemberCount    int                    `json:"member_count"`
	AvailableCount int                    `json:"available_count"`
	MinPrice       int                    `json:"min_price"`
	MaxPrice       int                    `json:"max_price"`
	Reliability    float64                `json:"reliability"`
}

type AutoRoutingSuggestion struct {
	Model       string                  `json:"model"`
	Capability  string                  `json:"capability"`
	ContractKey string                  `json:"contract_key"`
	Members     []AutoRoutingMemberInfo `json:"members"`
}

type AutoRoutingMemberInfo struct {
	ID                uint    `json:"id"`
	ChannelModelID    uint    `json:"channel_model_id"`
	ChannelID         uint    `json:"channel_id"`
	ChannelName       string  `json:"channel_name"`
	ModelName         string  `json:"model_name"`
	Priority          int     `json:"priority"`
	Enabled           bool    `json:"enabled"`
	ContractValid     bool    `json:"contract_valid"`
	UnavailableReason string  `json:"unavailable_reason,omitempty"`
	SuccessRate       float64 `json:"success_rate"`
	SampleCount       int     `json:"sample_count"`
	P95LatencyMs      int     `json:"p95_latency_ms"`
	CircuitStatus     string  `json:"circuit_status"`
}

type AutoRoutingPoolInfo struct {
	ID          uint                    `json:"id"`
	Model       string                  `json:"model"`
	Capability  string                  `json:"capability"`
	ContractKey string                  `json:"contract_key"`
	Enabled     bool                    `json:"enabled"`
	MaxAttempts int                     `json:"max_attempts"`
	Members     []AutoRoutingMemberInfo `json:"members"`
}

type AutoRouteCandidate struct {
	Pool          *model.AutoRoutingPool
	Member        *model.AutoRoutingPoolMember
	Channel       *model.Channel
	ChannelModel  *model.ChannelModel
	SuccessRate   float64
	SampleCount   int
	P95LatencyMs  int
	CircuitStatus string
}

type SaveAutoRoutingPoolInput struct {
	Model           string `json:"model"`
	Capability      string `json:"capability"`
	ContractKey     string `json:"contract_key"`
	ChannelModelIDs []uint `json:"channel_model_ids"`
}

type UpdateAutoRoutingPoolInput struct {
	Enabled         *bool   `json:"enabled,omitempty"`
	MaxAttempts     *int    `json:"max_attempts,omitempty"`
	ContractKey     *string `json:"contract_key,omitempty"`
	ChannelModelIDs []uint  `json:"channel_model_ids,omitempty"`
}

type UpdateAutoRoutingMemberInput struct {
	Enabled  *bool `json:"enabled,omitempty"`
	Priority *int  `json:"priority,omitempty"`
}

func (s *AutoChannelService) AggregateModels(tenantIDs ...uint) ([]AggregatedModel, error) {
	tenantID := uint(0)
	if len(tenantIDs) > 0 {
		tenantID = tenantIDs[0]
	}
	pools, err := s.routingRepo.ListPools()
	if err != nil {
		return nil, err
	}
	result := make([]AggregatedModel, 0, len(pools))
	for i := range pools {
		pool := &pools[i]
		if !pool.Enabled {
			continue
		}
		candidates, _ := s.resolvePoolCandidates(pool)
		item := AggregatedModel{PoolID: pool.ID, Model: pool.PublicModelName, Capability: pool.Capability, ContractKey: pool.ContractKey, MemberCount: len(pool.Members), Channels: make([]AggregatedChannelRef, 0, len(pool.Members))}
		for _, candidate := range candidates {
			price, priceErr := s.creditRepo.FindPricing(tenantID, candidate.ChannelModel.ModelName, candidate.Channel.ID)
			available := candidate.CircuitStatus != "open" && priceErr == nil && price != nil && price.HasValidPricingRule()
			reason := ""
			if !available {
				reason = "当前候选不可用"
			}
			if available {
				item.AvailableCount++
				if item.AvailableCount == 1 {
					item.Reliability = candidate.SuccessRate
				}
				if item.MinPrice == 0 || price.CreditsPerUnit < item.MinPrice {
					item.MinPrice = price.CreditsPerUnit
				}
				if price.CreditsPerUnit > item.MaxPrice {
					item.MaxPrice = price.CreditsPerUnit
				}
			}
			item.Channels = append(item.Channels, AggregatedChannelRef{ChannelID: candidate.Channel.ID, ChannelModelID: candidate.ChannelModel.ID, ChannelName: candidate.Channel.Name, SuccessRate: candidate.SuccessRate, SampleCount: candidate.SampleCount, P95LatencyMs: candidate.P95LatencyMs, Priority: candidate.Member.Priority, CircuitStatus: candidate.CircuitStatus, Available: available, UnavailableReason: reason})
		}
		if item.AvailableCount == 0 {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *AutoChannelService) Suggestions() ([]AutoRoutingSuggestion, error) {
	channels, err := s.channelRepo.ListEnabled()
	if err != nil {
		return nil, err
	}
	type groupKey struct{ model, capability, contract string }
	groups := map[groupKey][]AutoRoutingMemberInfo{}
	for _, channelInfo := range channels {
		channel, err := s.channelRepo.FindByID(channelInfo.ID)
		if err != nil {
			continue
		}
		models, err := s.channelModelRepo.ListByChannel(channel.ID, true)
		if err != nil {
			return nil, err
		}
		for i := range models {
			for _, capability := range channelModelCapabilities(&models[i]) {
				contract, contractErr := autoRoutingContract(channel, &models[i], capability)
				if contractErr != nil {
					continue
				}
				key := groupKey{strings.TrimSpace(models[i].ModelName), capability, contract}
				groups[key] = append(groups[key], AutoRoutingMemberInfo{ChannelModelID: models[i].ID, ChannelID: channel.ID, ChannelName: channel.Name, ModelName: models[i].ModelName, Enabled: true, ContractValid: true, CircuitStatus: "closed"})
			}
		}
	}
	result := make([]AutoRoutingSuggestion, 0)
	for key, members := range groups {
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool { return members[i].ChannelModelID < members[j].ChannelModelID })
		result = append(result, AutoRoutingSuggestion{Model: key.model, Capability: key.capability, ContractKey: key.contract, Members: members})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Model != result[j].Model {
			return result[i].Model < result[j].Model
		}
		if result[i].Capability != result[j].Capability {
			return result[i].Capability < result[j].Capability
		}
		return result[i].ContractKey < result[j].ContractKey
	})
	return result, nil
}

func (s *AutoChannelService) ListPools() ([]AutoRoutingPoolInfo, error) {
	pools, err := s.routingRepo.ListPools()
	if err != nil {
		return nil, err
	}
	result := make([]AutoRoutingPoolInfo, 0, len(pools))
	for i := range pools {
		pool := &pools[i]
		info := AutoRoutingPoolInfo{ID: pool.ID, Model: pool.PublicModelName, Capability: pool.Capability, ContractKey: pool.ContractKey, Enabled: pool.Enabled, MaxAttempts: pool.MaxAttempts, Members: make([]AutoRoutingMemberInfo, 0, len(pool.Members))}
		for memberIndex := range pool.Members {
			info.Members = append(info.Members, s.memberInfo(pool, &pool.Members[memberIndex]))
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *AutoChannelService) CreatePool(input SaveAutoRoutingPoolInput) (*AutoRoutingPoolInfo, error) {
	pool, members, err := s.validatedPoolInput(input)
	if err != nil {
		return nil, err
	}
	if err := s.routingRepo.SavePool(pool, members); err != nil {
		return nil, err
	}
	return s.poolInfo(pool.ID)
}

func (s *AutoChannelService) UpdatePool(id uint, input UpdateAutoRoutingPoolInput) (*AutoRoutingPoolInfo, error) {
	pool, err := s.routingRepo.FindPool(id)
	if err != nil {
		return nil, err
	}
	if input.MaxAttempts != nil {
		if *input.MaxAttempts < 1 || *input.MaxAttempts > autoDefaultMaxAttempts {
			return nil, errors.New("智能路由最大尝试次数必须为 1 或 2")
		}
		pool.MaxAttempts = *input.MaxAttempts
	}
	contractChanged := false
	if input.ChannelModelIDs != nil {
		targetContract := pool.ContractKey
		if input.ContractKey != nil {
			targetContract = strings.TrimSpace(*input.ContractKey)
			if targetContract == "" {
				return nil, errors.New("智能路由协议合同不能为空")
			}
		}
		validated, memberIDs, validationErr := s.validatedPoolInput(SaveAutoRoutingPoolInput{Model: pool.PublicModelName, Capability: pool.Capability, ContractKey: targetContract, ChannelModelIDs: input.ChannelModelIDs})
		if validationErr != nil {
			return nil, validationErr
		}
		if validated.ContractKey != targetContract {
			return nil, errors.New("更新候选与路由池协议合同不一致")
		}
		contractChanged = validated.ContractKey != pool.ContractKey
		pool.ContractKey = validated.ContractKey
		enabledMembers := enabledMemberCountAfterReplacement(pool.Members, memberIDs)
		if contractChanged {
			pool.Enabled = false
		}
		if input.Enabled != nil {
			if contractChanged && *input.Enabled {
				return nil, errors.New("协议合同更新后请检查候选并重新启用智能路由池")
			}
			if *input.Enabled && enabledMembers < 2 {
				return nil, errors.New("至少需要两个合同有效的候选才能启用智能路由池")
			}
			pool.Enabled = *input.Enabled
		} else if pool.Enabled && enabledMembers < 2 {
			pool.Enabled = false
		}
		if err := s.routingRepo.ReplaceMembers(pool, memberIDs); err != nil {
			return nil, err
		}
	} else if input.ContractKey != nil {
		return nil, errors.New("更新协议合同时必须同时提交候选")
	} else if input.Enabled != nil {
		if *input.Enabled && s.validMemberCount(pool) < 2 {
			return nil, errors.New("至少需要两个合同有效的候选才能启用智能路由池")
		}
		pool.Enabled = *input.Enabled
	} else if pool.Enabled && s.validMemberCount(pool) < 2 {
		pool.Enabled = false
	}
	if input.ChannelModelIDs == nil {
		if err := s.routingRepo.UpdatePool(pool); err != nil {
			return nil, err
		}
	}
	return s.poolInfo(id)
}

func enabledMemberCountAfterReplacement(current []model.AutoRoutingPoolMember, channelModelIDs []uint) int {
	settings := make(map[uint]bool, len(current))
	for _, member := range current {
		settings[member.ChannelModelID] = member.Enabled
	}
	count := 0
	for _, channelModelID := range channelModelIDs {
		if enabled, exists := settings[channelModelID]; !exists || enabled {
			count++
		}
	}
	return count
}

func (s *AutoChannelService) UpdateMember(poolID, memberID uint, input UpdateAutoRoutingMemberInput) (*AutoRoutingPoolInfo, error) {
	member, err := s.routingRepo.FindMember(poolID, memberID)
	if err != nil {
		return nil, err
	}
	if input.Enabled != nil {
		member.Enabled = *input.Enabled
	}
	if input.Priority != nil {
		member.Priority = *input.Priority
	}
	if err := s.routingRepo.UpdateMember(member); err != nil {
		return nil, err
	}
	pool, err := s.routingRepo.FindPool(poolID)
	if err == nil && pool.Enabled && s.validMemberCount(pool) < 2 {
		pool.Enabled = false
		err = s.routingRepo.UpdatePool(pool)
	}
	if err != nil {
		return nil, err
	}
	return s.poolInfo(poolID)
}

func (s *AutoChannelService) DeletePool(id uint) error { return s.routingRepo.DeletePool(id) }

func (s *AutoChannelService) ResolveCandidates(poolID, tenantID uint, capability, modelName string) (*model.AutoRoutingPool, []AutoRouteCandidate, error) {
	pool, err := s.routingRepo.FindPool(poolID)
	if err != nil {
		return nil, nil, errors.New("智能路由池不存在")
	}
	if !pool.Enabled || pool.Capability != strings.TrimSpace(capability) || pool.PublicModelName != strings.TrimSpace(modelName) {
		return nil, nil, errors.New("智能路由池与当前模型或能力不匹配")
	}
	candidates, err := s.resolvePoolCandidates(pool)
	if err != nil {
		return nil, nil, err
	}
	available := candidates[:0]
	for _, candidate := range candidates {
		if candidate.CircuitStatus == "open" {
			continue
		}
		if pricing, pricingErr := s.creditRepo.FindPricing(tenantID, candidate.ChannelModel.ModelName, candidate.Channel.ID); pricingErr == nil && pricing != nil && pricing.HasValidPricingRule() {
			available = append(available, candidate)
		}
	}
	if len(available) == 0 {
		return nil, nil, errors.New("智能路由当前没有可用候选")
	}
	return pool, available, nil
}

func (s *AutoChannelService) AcquireCandidate(candidate AutoRouteCandidate) bool {
	if candidate.CircuitStatus != "half_open" {
		return true
	}
	s.halfOpenMu.Lock()
	defer s.halfOpenMu.Unlock()
	if s.halfOpen[candidate.ChannelModel.ID] {
		return false
	}
	s.halfOpen[candidate.ChannelModel.ID] = true
	return true
}

func (s *AutoChannelService) ReleaseCandidate(channelModelID uint) {
	s.halfOpenMu.Lock()
	delete(s.halfOpen, channelModelID)
	s.halfOpenMu.Unlock()
}

func (s *AutoChannelService) RecordAttempt(item *model.GenerationAttempt) error {
	return s.routingRepo.CreateAttempt(item)
}

func (s *AutoChannelService) resolvePoolCandidates(pool *model.AutoRoutingPool) ([]AutoRouteCandidate, error) {
	result := make([]AutoRouteCandidate, 0, len(pool.Members))
	for memberIndex := range pool.Members {
		member := &pool.Members[memberIndex]
		if !member.Enabled {
			continue
		}
		channelModel, err := s.channelModelRepo.FindByID(member.ChannelModelID)
		if err != nil || !channelModel.Enabled || channelModel.ModelName != pool.PublicModelName {
			continue
		}
		channel, err := s.channelRepo.FindByID(channelModel.ChannelID)
		if err != nil || !channel.Enabled {
			continue
		}
		contract, err := autoRoutingContract(channel, channelModel, pool.Capability)
		if err != nil || contract != pool.ContractKey {
			continue
		}
		rate, samples, p95, circuit := s.health(channelModel.ID, time.Now())
		result = append(result, AutoRouteCandidate{Pool: pool, Member: member, Channel: channel, ChannelModel: channelModel, SuccessRate: rate, SampleCount: samples, P95LatencyMs: p95, CircuitStatus: circuit})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CircuitStatus != result[j].CircuitStatus {
			return autoCircuitRank(result[i].CircuitStatus) < autoCircuitRank(result[j].CircuitStatus)
		}
		if result[i].SuccessRate != result[j].SuccessRate {
			return result[i].SuccessRate > result[j].SuccessRate
		}
		if result[i].Member.Priority != result[j].Member.Priority {
			return result[i].Member.Priority > result[j].Member.Priority
		}
		if result[i].P95LatencyMs != result[j].P95LatencyMs {
			return result[i].P95LatencyMs < result[j].P95LatencyMs
		}
		return result[i].ChannelModel.ID < result[j].ChannelModel.ID
	})
	return result, nil
}

func (s *AutoChannelService) health(channelModelID uint, now time.Time) (float64, int, int, string) {
	attempts, err := s.routingRepo.ListHealthAttempts(channelModelID, now.Add(-autoHealthWindow))
	if err != nil {
		return 90, 0, 0, "closed"
	}
	return summarizeAutoHealth(attempts, now)
}

func summarizeAutoHealth(attempts []model.GenerationAttempt, now time.Time) (float64, int, int, string) {
	successes := 0
	latencies := make([]int, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.Success {
			successes++
		}
		if attempt.ResponseTimeMs > 0 {
			latencies = append(latencies, attempt.ResponseTimeMs)
		}
	}
	rate := float64(successes+9) / float64(len(attempts)+10) * 100
	sort.Ints(latencies)
	p95 := 0
	if len(latencies) > 0 {
		p95 = latencies[(len(latencies)*95-1)/100]
	}
	consecutive := 0
	var latestFailure time.Time
	for _, attempt := range attempts {
		if attempt.CreatedAt.Before(now.Add(-autoCircuitWindow)) || attempt.Success || !attempt.Retryable {
			break
		}
		if consecutive == 0 {
			latestFailure = attempt.CreatedAt
		}
		consecutive++
	}
	circuit := "closed"
	if consecutive >= autoCircuitThreshold {
		circuit = "open"
		if now.Sub(latestFailure) >= autoCircuitCooldown {
			circuit = "half_open"
		}
	}
	return rate, len(attempts), p95, circuit
}

func (s *AutoChannelService) validatedPoolInput(input SaveAutoRoutingPoolInput) (*model.AutoRoutingPool, []uint, error) {
	modelName := strings.TrimSpace(input.Model)
	capability := strings.TrimSpace(input.Capability)
	if modelName == "" || !validAutoCapability(capability) || len(input.ChannelModelIDs) < 2 {
		return nil, nil, errors.New("智能路由池必须包含模型、能力和至少两个候选")
	}
	seen := map[uint]struct{}{}
	memberIDs := make([]uint, 0, len(input.ChannelModelIDs))
	contract := ""
	for _, id := range input.ChannelModelIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			return nil, nil, errors.New("智能路由候选不能重复")
		}
		seen[id] = struct{}{}
		channelModel, err := s.channelModelRepo.FindByID(id)
		if err != nil || !channelModel.Enabled || strings.TrimSpace(channelModel.ModelName) != modelName {
			return nil, nil, errors.New("智能路由候选模型无效")
		}
		channel, err := s.channelRepo.FindByID(channelModel.ChannelID)
		if err != nil || !channel.Enabled {
			return nil, nil, errors.New("智能路由候选渠道无效")
		}
		key, err := autoRoutingContract(channel, channelModel, capability)
		if err != nil {
			return nil, nil, err
		}
		if contract == "" {
			contract = key
		}
		if key != contract || strings.TrimSpace(input.ContractKey) != "" && key != strings.TrimSpace(input.ContractKey) {
			return nil, nil, errors.New("智能路由候选的协议合同不一致")
		}
		memberIDs = append(memberIDs, id)
	}
	if len(memberIDs) < 2 {
		return nil, nil, errors.New("至少需要两个有效候选")
	}
	return &model.AutoRoutingPool{PublicModelName: modelName, Capability: capability, ContractKey: contract, Enabled: false, MaxAttempts: autoDefaultMaxAttempts}, memberIDs, nil
}

func (s *AutoChannelService) validMemberCount(pool *model.AutoRoutingPool) int {
	count := 0
	for memberIndex := range pool.Members {
		if s.memberInfo(pool, &pool.Members[memberIndex]).ContractValid && pool.Members[memberIndex].Enabled {
			count++
		}
	}
	return count
}

func (s *AutoChannelService) memberInfo(pool *model.AutoRoutingPool, member *model.AutoRoutingPoolMember) AutoRoutingMemberInfo {
	info := AutoRoutingMemberInfo{ID: member.ID, ChannelModelID: member.ChannelModelID, Priority: member.Priority, Enabled: member.Enabled, CircuitStatus: "closed"}
	channelModel, err := s.channelModelRepo.FindByID(member.ChannelModelID)
	if err != nil {
		info.UnavailableReason = "渠道模型不存在"
		return info
	}
	info.ChannelID = channelModel.ChannelID
	info.ModelName = channelModel.ModelName
	channel, err := s.channelRepo.FindByID(channelModel.ChannelID)
	if err != nil {
		info.UnavailableReason = "渠道不存在"
		return info
	}
	info.ChannelName = channel.Name
	contract, err := autoRoutingContract(channel, channelModel, pool.Capability)
	info.ContractValid = err == nil && contract == pool.ContractKey && channel.Enabled && channelModel.Enabled && channelModel.ModelName == pool.PublicModelName
	if !info.ContractValid {
		info.UnavailableReason = "模型配置已变化，需要重新确认"
	}
	info.SuccessRate, info.SampleCount, info.P95LatencyMs, info.CircuitStatus = s.health(channelModel.ID, time.Now())
	return info
}

func (s *AutoChannelService) poolInfo(id uint) (*AutoRoutingPoolInfo, error) {
	items, err := s.ListPools()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func autoRoutingContract(channel *model.Channel, channelModel *model.ChannelModel, capability string) (string, error) {
	if channel == nil || channelModel == nil || !containsStringValue(channelModelCapabilities(channelModel), capability) {
		return "", errors.New("渠道模型不支持当前能力")
	}
	payload := map[string]any{"capability": capability}
	switch capability {
	case string(model.CapabilityImage):
		generateRoute, err := model.NormalizeImageGenerateRoute(channelModel.ImageGenerateRoute)
		if err != nil {
			return "", err
		}
		editRoute, err := model.NormalizeImageEditRoute(channelModel.ImageEditRoute)
		if err != nil {
			return "", err
		}
		payload["generate_route"], payload["edit_route"] = generateRoute, editRoute
	case string(model.CapabilityVideo):
		payload["api_standard"] = normalizeChannelVideoAPIStandard(channel.VideoAPIStandard)
		route := channelModel.VideoRoute
		if channel.VideoAPIStandard == model.VideoAPIStandardBinghuo {
			route = "binghuo"
		}
		normalizedRoute, err := model.NormalizeVideoRoute(route)
		if err != nil {
			return "", err
		}
		var durations []int
		if strings.TrimSpace(channelModel.VideoDurations) != "" && json.Unmarshal([]byte(channelModel.VideoDurations), &durations) != nil {
			return "", errors.New("视频时长配置无效")
		}
		sort.Ints(durations)
		payload["route"], payload["durations"], payload["customizable"] = normalizedRoute, durations, channelModel.VideoCustomizable
		if normalizedRoute == "custom" {
			var config model.CustomVideoConfig
			if json.Unmarshal([]byte(channelModel.VideoCustomConfig), &config) != nil || model.NormalizeAndValidateCustomVideoConfig(&config) != nil {
				return "", errors.New("自定义视频配置无效")
			}
			payload["custom_config"] = config
		}
	case string(model.CapabilityText), string(model.CapabilityAudio):
	default:
		return "", errors.New("不支持的智能路由能力")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func channelModelCapabilities(item *model.ChannelModel) []string {
	capabilities := parseChannelCapabilities(item.Capabilities)
	if len(capabilities) == 0 {
		return defaultChannelModelCapabilities()
	}
	return capabilities
}

func validAutoCapability(value string) bool {
	return value == "image" || value == "video" || value == "text" || value == "audio"
}

func containsStringValue(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func successRatePercentage(total, success int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}
