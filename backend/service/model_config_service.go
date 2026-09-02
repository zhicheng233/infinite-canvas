package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

var ErrModelConfigRevisionConflict = errors.New("配置已被其他管理员修改，请刷新后重试")

type ModelConfigFilter struct {
	ChannelID       uint
	Capability      string
	Status          string
	Search          string
	IncludeArchived bool
}

type ModelConfigService struct {
	repo            *repository.ModelConfigRepo
	channelService  *ChannelService
	generateService *GenerateService
	httpClient      *http.Client
}

func NewModelConfigService(repo *repository.ModelConfigRepo, channelService *ChannelService, generateService *GenerateService) *ModelConfigService {
	return &ModelConfigService{repo: repo, channelService: channelService, generateService: generateService, httpClient: &http.Client{Timeout: 2 * time.Minute}}
}

func (s *ModelConfigService) CreateChannel(tenantID, actorUserID uint, input model.SaveChannelInput) (*model.ChannelAdminInfo, error) {
	item, err := s.channelService.Create(input)
	if err != nil {
		return nil, err
	}
	afterJSON, _ := json.Marshal(item)
	if err := s.repo.RecordAudit(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel", ResourceID: item.ID, Action: "create", AfterJSON: string(afterJSON)}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ModelConfigService) UpdateChannel(tenantID, actorUserID, id uint, input model.SaveChannelInput) (*model.ChannelAdminInfo, error) {
	before, err := s.channelService.Get(id)
	if err != nil {
		return nil, err
	}
	item, err := s.channelService.Update(id, input)
	if err != nil {
		return nil, err
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(item)
	if err := s.repo.RecordAudit(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel", ResourceID: id, Action: "update", BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON)}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ModelConfigService) ListChannels(tenantID uint) ([]model.ModelServiceChannelInfo, error) {
	data, err := s.repo.Load(tenantID, true)
	if err != nil {
		return nil, err
	}
	defaults := make(map[uint][]model.SaveChannelProtocolDefaultInput)
	for _, item := range data.Defaults {
		defaults[item.ChannelID] = append(defaults[item.ChannelID], model.SaveChannelProtocolDefaultInput{Capability: item.Capability, Operation: item.Operation, Adapter: item.Adapter, Config: decodeConfigMap(item.ConfigJSON)})
	}
	counts := make(map[uint]int)
	readyCounts := make(map[uint]int)
	for index := range data.Models {
		counts[data.Models[index].ChannelID]++
		if info, buildErr := buildModelConfigInfo(data, &data.Models[index]); buildErr == nil && info.Ready {
			readyCounts[data.Models[index].ChannelID]++
		}
	}
	result := make([]model.ModelServiceChannelInfo, 0, len(data.Channels))
	for _, channel := range data.Channels {
		result = append(result, model.ModelServiceChannelInfo{
			ChannelAdminInfo: model.ChannelAdminInfo{ChannelInfo: model.ChannelInfo{ID: channel.ID, Name: channel.Name, Enabled: channel.Enabled, VideoAPIStandard: normalizeChannelVideoAPIStandard(channel.VideoAPIStandard), NewApiChannelID: channel.NewApiChannelID, MetricsBaseUrl: channel.MetricsBaseUrl, SyncStatus: channel.SyncStatus, SyncError: channel.SyncError, SyncedAt: channel.SyncedAt}, BaseUrl: channel.BaseUrl, HasKey: strings.TrimSpace(channel.ApiKey) != "", Remark: channel.Remark},
			Archived:         channel.DeletedAt.Valid, ConfigRevision: normalizedRevision(channel.ConfigRevision), ProtocolDefaults: defaults[channel.ID], ModelCount: counts[channel.ID], ReadyModelCount: readyCounts[channel.ID],
		})
	}
	return result, nil
}

func (s *ModelConfigService) ListModels(tenantID uint, filter ModelConfigFilter) ([]model.ModelConfigInfo, error) {
	data, err := s.repo.Load(tenantID, filter.IncludeArchived)
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	result := make([]model.ModelConfigInfo, 0, len(data.Models))
	for index := range data.Models {
		item := &data.Models[index]
		if filter.ChannelID > 0 && item.ChannelID != filter.ChannelID {
			continue
		}
		info, err := buildModelConfigInfo(data, item)
		if err != nil {
			return nil, err
		}
		if filter.Status != "" && info.Status != filter.Status {
			continue
		}
		if filter.Capability != "" && !hasOperationCapability(info.Operations, filter.Capability) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(info.ChannelName+" "+info.PublicKey+" "+info.DisplayName+" "+info.UpstreamModelID), search) {
			continue
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *ModelConfigService) GetModel(tenantID, id uint) (*model.ModelConfigInfo, error) {
	data, err := s.repo.Load(tenantID, false)
	if err != nil {
		return nil, err
	}
	for index := range data.Models {
		if data.Models[index].ID == id {
			info, err := buildModelConfigInfo(data, &data.Models[index])
			return &info, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *ModelConfigService) UpdateModel(tenantID, actorUserID, id uint, input model.UpdateModelConfigInput) (*model.ModelConfigInfo, error) {
	data, err := s.repo.Load(tenantID, false)
	if err != nil {
		return nil, err
	}
	current := findModel(data.Models, id)
	if current == nil {
		return nil, gorm.ErrRecordNotFound
	}
	current.ConfigRevision = normalizedRevision(current.ConfigRevision)
	if input.ExpectedRevision != current.ConfigRevision {
		return nil, ErrModelConfigRevisionConflict
	}
	currentCatalog := findCatalog(data.Catalogs, current.CatalogModelID)
	publicKey := strings.TrimSpace(input.PublicKey)
	if publicKey == "" || len([]rune(publicKey)) > 191 {
		return nil, errors.New("公开调用键不能为空且不能超过 191 个字符")
	}
	if strings.ContainsAny(publicKey, "\r\n\t") {
		return nil, errors.New("公开调用键不能包含控制字符")
	}
	if currentCatalog != nil && current.Status != model.ModelStatusDraft && currentCatalog.PublicKey != publicKey {
		return nil, errors.New("已启用或已配置的模型不能直接修改公开调用键")
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = publicKey
	}
	if len([]rune(displayName)) > 200 {
		return nil, errors.New("显示名称不能超过 200 个字符")
	}
	upstreamModelID := strings.TrimSpace(input.UpstreamModelID)
	if upstreamModelID == "" {
		upstreamModelID = strings.TrimSpace(current.UpstreamModelID)
	}
	if upstreamModelID == "" {
		upstreamModelID = strings.TrimSpace(current.ModelName)
	}
	if upstreamModelID == "" || len([]rune(upstreamModelID)) > 191 || strings.ContainsAny(upstreamModelID, "\r\n\t") {
		return nil, errors.New("上游模型 ID 不能为空、不能包含控制字符且不能超过 191 个字符")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = current.Status
	}
	if status != model.ModelStatusDraft && status != model.ModelStatusActive && status != model.ModelStatusDisabled {
		return nil, errors.New("模型状态无效")
	}

	operations, effective, err := normalizeModelOperations(current.ChannelID, input.Operations, data.Defaults)
	if err != nil {
		return nil, err
	}
	targetCatalogID := current.CatalogModelID
	if catalog := findCatalogByKey(data.Catalogs, publicKey); catalog != nil {
		targetCatalogID = catalog.ID
	} else if currentCatalog == nil || currentCatalog.PublicKey != publicKey {
		targetCatalogID = 0
	}
	pricing, err := normalizeModelPricing(input.PricingOverrides)
	if err != nil {
		return nil, err
	}
	issues := proposedReadinessIssues(current, targetCatalogID, operations, effective, pricing, data.Pricing)
	if status == model.ModelStatusActive && current.Status != model.ModelStatusActive && len(issues) > 0 {
		return nil, fmt.Errorf("模型尚未就绪：%s", issues[0].Message)
	}

	legacy, err := legacyProjection(operations, effective)
	if err != nil {
		return nil, err
	}
	beforeInfo, _ := buildModelConfigInfo(data, current)
	beforeJSON, _ := json.Marshal(beforeInfo)
	afterJSON, _ := json.Marshal(input)
	params := repository.SaveModelConfigParams{
		TenantID: tenantID, ActorUserID: actorUserID, ModelID: id, ExpectedRevision: input.ExpectedRevision,
		PublicKey: publicKey, DisplayName: displayName, UpstreamModelID: upstreamModelID, Status: status, SortOrder: input.SortOrder,
		LegacyUnreviewed: len(issues) > 0,
		Capabilities:     legacy.Capabilities, ImageGenerate: legacy.ImageGenerate, ImageEdit: legacy.ImageEdit,
		VideoRoute: legacy.VideoRoute, VideoDurations: legacy.VideoDurations, VideoCustomizable: legacy.VideoCustomizable,
		VideoCustomConfig: legacy.VideoCustomConfig, Operations: operations, Pricing: pricing,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON),
	}
	if err := s.repo.SaveModelConfig(params); err != nil {
		if errors.Is(err, repository.ErrModelConfigRevisionConflict) {
			return nil, ErrModelConfigRevisionConflict
		}
		return nil, err
	}
	return s.GetModel(tenantID, id)
}

func (s *ModelConfigService) PreviewChannelDefaults(tenantID, channelID uint, input model.UpdateChannelDefaultsInput) (*model.ChannelDefaultsImpact, error) {
	data, err := s.repo.Load(tenantID, false)
	if err != nil {
		return nil, err
	}
	channel := findChannel(data.Channels, channelID)
	if channel == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if input.ExpectedRevision != normalizedRevision(channel.ConfigRevision) {
		return nil, ErrModelConfigRevisionConflict
	}
	defaults, err := normalizeChannelDefaults(channelID, input.Defaults)
	if err != nil {
		return nil, err
	}
	impact := &model.ChannelDefaultsImpact{AffectedModelIDs: []uint{}, Issues: []model.ModelReadinessIssue{}}
	for index := range data.Models {
		item := &data.Models[index]
		if item.ChannelID != channelID {
			continue
		}
		for _, operation := range operationsForModel(data.Operations, item.ID) {
			if operation.ProtocolMode != model.ProtocolModeInherit {
				continue
			}
			impact.AffectedModelIDs = appendUniqueUint(impact.AffectedModelIDs, item.ID)
			if _, err := effectiveProtocol(operation, defaults); err != nil {
				impact.Issues = append(impact.Issues, model.ModelReadinessIssue{Code: "protocol_default_missing", Capability: operation.Capability, Operation: operation.Operation, Message: err.Error()})
			}
		}
	}
	return impact, nil
}

func (s *ModelConfigService) UpdateChannelDefaults(tenantID, actorUserID, channelID uint, input model.UpdateChannelDefaultsInput) error {
	if _, err := s.PreviewChannelDefaults(tenantID, channelID, input); err != nil {
		return err
	}
	defaults, err := normalizeChannelDefaults(channelID, input.Defaults)
	if err != nil {
		return err
	}
	if err := s.repo.SaveChannelDefaults(tenantID, actorUserID, channelID, input.ExpectedRevision, defaults); err != nil {
		if errors.Is(err, repository.ErrModelConfigRevisionConflict) {
			return ErrModelConfigRevisionConflict
		}
		return err
	}
	return nil
}

func (s *ModelConfigService) ArchiveChannel(tenantID, actorUserID, channelID uint, archived bool) error {
	return s.repo.SetChannelArchived(tenantID, actorUserID, channelID, archived)
}

func (s *ModelConfigService) ArchiveModel(tenantID, actorUserID, modelID uint, archived bool) error {
	return s.repo.SetModelArchived(tenantID, actorUserID, modelID, archived)
}

func (s *ModelConfigService) SyncChannel(tenantID, actorUserID, channelID uint) (*model.ChannelSyncReport, error) {
	if err := s.repo.SetChannelSyncState(channelID, "syncing", "", nil); err != nil {
		return nil, err
	}
	channels, err := s.ListChannels(tenantID)
	if err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	var channel *model.ModelServiceChannelInfo
	for index := range channels {
		if channels[index].ID == channelID && !channels[index].Archived {
			channel = &channels[index]
			break
		}
	}
	if channel == nil {
		return nil, s.failChannelSync(channelID, gorm.ErrRecordNotFound)
	}
	apiKey, err := s.channelService.DecryptedApiKey(channelID)
	if err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	request, err := http.NewRequest(http.MethodGet, buildChannelModelsURL(channel.BaseUrl), nil)
	if err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, s.failChannelSync(channelID, fmt.Errorf("上游模型接口返回 HTTP %d", response.StatusCode))
	}
	var payload discoveredModelsResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	names := uniqueDiscoveredModelNames(payload.Data)
	report, err := s.repo.ApplyDiscovery(tenantID, actorUserID, channelID, names, time.Now())
	if err != nil {
		return nil, s.failChannelSync(channelID, err)
	}
	return report, nil
}

func (s *ModelConfigService) failChannelSync(channelID uint, cause error) error {
	message := cause.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	_ = s.repo.SetChannelSyncState(channelID, "failed", message, nil)
	return cause
}

func (s *ModelConfigService) SaveDefaultPricing(tenantID, actorUserID, catalogModelID uint, input model.SaveModelPricingInput) error {
	data, err := s.repo.Load(tenantID, false)
	if err != nil {
		return err
	}
	if findCatalog(data.Catalogs, catalogModelID) == nil {
		return gorm.ErrRecordNotFound
	}
	capability := strings.TrimSpace(input.Capability)
	pricing, err := normalizePricingRule(capability, input.CreditsPerUnit, input.UnitType, input.PricingMode, input.PricingRule)
	if err != nil {
		return err
	}
	pricing.CatalogModelID = catalogModelID
	return s.repo.SaveDefaultPricing(tenantID, actorUserID, catalogModelID, capability, pricing)
}

func (s *ModelConfigService) TestModel(tenantID, userID, id uint, input model.ModelTestDraftInput) (*ModelTestResult, error) {
	info, err := s.GetModel(tenantID, id)
	if err != nil {
		return nil, err
	}
	operation := findOperationInfo(info.Operations, input.Capability, input.Operation)
	if input.Draft != nil {
		data, loadErr := s.repo.Load(tenantID, false)
		if loadErr != nil {
			return nil, loadErr
		}
		current := findModel(data.Models, id)
		if current == nil {
			return nil, gorm.ErrRecordNotFound
		}
		_, effective, normalizeErr := normalizeModelOperations(current.ChannelID, input.Draft.Operations, data.Defaults)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		key := operationKey(input.Capability, input.Operation)
		if value, ok := effective[key]; ok {
			operation = &model.ModelOperationInfo{Capability: input.Capability, Operation: input.Operation, Enabled: true, Effective: value}
		}
		if value := strings.TrimSpace(input.Draft.UpstreamModelID); value != "" {
			info.UpstreamModelID = value
		}
	}
	if operation == nil || !operation.Enabled {
		return nil, errors.New("请选择已启用的模型操作")
	}
	generation := operation.Capability
	operationName := generation + "_generate"
	if generation == "image" && operation.Operation == "edit" {
		operationName = "image_edit"
	}
	return s.generateService.TestModel(tenantID, userID, ModelTestInput{Model: info.UpstreamModelID, ChannelID: info.ChannelID, ChannelModelID: info.ID, Generation: generation, Operation: operationName, Route: operation.Effective.Adapter, Prompt: input.Prompt, Size: input.Size, AspectRatio: input.AspectRatio, Seconds: input.Seconds, HasReferences: input.ReferenceCount > 0, ReferenceCount: input.ReferenceCount})
}

func buildModelConfigInfo(data *repository.ModelConfigData, item *model.ChannelModel) (model.ModelConfigInfo, error) {
	channel := findChannel(data.Channels, item.ChannelID)
	catalog := findCatalog(data.Catalogs, item.CatalogModelID)
	info := model.ModelConfigInfo{ID: item.ID, ChannelID: item.ChannelID, CatalogModelID: item.CatalogModelID, UpstreamModelID: item.UpstreamModelID, Status: item.Status, DiscoveryStatus: item.DiscoveryStatus, LastDiscoveredAt: item.LastDiscoveredAt, ConfigRevision: normalizedRevision(item.ConfigRevision), LegacyUnreviewed: item.LegacyUnreviewed, SortOrder: item.SortOrder, Operations: []model.ModelOperationInfo{}, Pricing: []model.ModelPricingRuleInfo{}, ReadinessIssues: []model.ModelReadinessIssue{}}
	if info.UpstreamModelID == "" {
		info.UpstreamModelID = item.ModelName
	}
	if channel != nil {
		info.ChannelName = channel.Name
	}
	if catalog != nil {
		info.PublicKey, info.DisplayName = catalog.PublicKey, catalog.DisplayName
	} else {
		info.PublicKey, info.DisplayName = item.ModelName, item.ModelName
	}
	operations := operationsForModel(data.Operations, item.ID)
	for _, operation := range operations {
		effective, err := effectiveProtocol(operation, defaultsForChannel(data.Defaults, item.ChannelID))
		if err != nil {
			info.ReadinessIssues = append(info.ReadinessIssues, model.ModelReadinessIssue{Code: "protocol_missing", Capability: operation.Capability, Operation: operation.Operation, Message: err.Error()})
			effective = model.EffectiveProtocolInfo{}
		}
		info.Operations = append(info.Operations, model.ModelOperationInfo{Capability: operation.Capability, Operation: operation.Operation, Enabled: operation.Enabled, Mode: operation.ProtocolMode, Adapter: operation.Adapter, Config: decodeConfigMap(operation.ConfigJSON), Effective: effective})
	}
	pricingByCapability := effectivePricingByCapability(data.Pricing, item.CatalogModelID, item.ID)
	for capability, pricing := range pricingByCapability {
		info.Pricing = append(info.Pricing, pricing)
		_ = capability
	}
	sort.Slice(info.Pricing, func(i, j int) bool { return info.Pricing[i].Capability < info.Pricing[j].Capability })
	info.ReadinessIssues = append(info.ReadinessIssues, readinessIssues(item, info.Operations, pricingByCapability)...)
	info.Ready = len(info.ReadinessIssues) == 0
	return info, nil
}

func readinessIssues(item *model.ChannelModel, operations []model.ModelOperationInfo, pricing map[string]model.ModelPricingRuleInfo) []model.ModelReadinessIssue {
	issues := make([]model.ModelReadinessIssue, 0)
	enabledCapabilities := map[string]struct{}{}
	for _, operation := range operations {
		if !operation.Enabled {
			continue
		}
		enabledCapabilities[operation.Capability] = struct{}{}
		if operation.Effective.Adapter == "" {
			issues = append(issues, model.ModelReadinessIssue{Code: "protocol_missing", Capability: operation.Capability, Operation: operation.Operation, Message: "操作缺少有效协议"})
		}
	}
	if len(enabledCapabilities) == 0 {
		issues = append(issues, model.ModelReadinessIssue{Code: "operation_missing", Message: "至少配置一个模型操作"})
	}
	for capability := range enabledCapabilities {
		value, ok := pricing[capability]
		if !ok || !pricingInfoValid(value) {
			issues = append(issues, model.ModelReadinessIssue{Code: "pricing_missing", Capability: capability, Message: "该能力缺少有效定价"})
		}
	}
	if item.DiscoveryStatus == model.DiscoveryStatusMissing {
		issues = append(issues, model.ModelReadinessIssue{Code: "upstream_missing", Message: "最近一次同步未发现该模型"})
	}
	return dedupeReadinessIssues(issues)
}

func proposedReadinessIssues(item *model.ChannelModel, catalogID uint, operations []model.ChannelModelOperation, effective map[string]model.EffectiveProtocolInfo, proposed []model.ModelPricingRule, stored []model.ModelPricingRule) []model.ModelReadinessIssue {
	operationInfos := make([]model.ModelOperationInfo, 0, len(operations))
	for _, operation := range operations {
		operationInfos = append(operationInfos, model.ModelOperationInfo{Capability: operation.Capability, Operation: operation.Operation, Enabled: operation.Enabled, Effective: effective[operationKey(operation.Capability, operation.Operation)]})
	}
	pricing := effectivePricingByCapability(stored, catalogID, item.ID)
	for _, rule := range proposed {
		pricing[rule.Capability] = pricingRuleInfo(rule, "implementation")
	}
	return readinessIssues(item, operationInfos, pricing)
}

func normalizeModelOperations(channelID uint, inputs []model.SaveModelOperationInput, defaults []model.ChannelProtocolDefault) ([]model.ChannelModelOperation, map[string]model.EffectiveProtocolInfo, error) {
	result := make([]model.ChannelModelOperation, 0, len(inputs))
	effective := make(map[string]model.EffectiveProtocolInfo, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		capability, operation := strings.TrimSpace(input.Capability), strings.TrimSpace(input.Operation)
		key := operationKey(capability, operation)
		if !validCapabilityOperation(capability, operation) {
			return nil, nil, fmt.Errorf("不支持的模型操作 %s", key)
		}
		if _, ok := seen[key]; ok {
			return nil, nil, fmt.Errorf("模型操作重复：%s", key)
		}
		seen[key] = struct{}{}
		mode := strings.TrimSpace(input.Mode)
		if mode != model.ProtocolModeInherit && mode != model.ProtocolModeOverride {
			return nil, nil, fmt.Errorf("%s 的协议模式无效", key)
		}
		adapter := strings.TrimSpace(input.Adapter)
		if mode == model.ProtocolModeOverride {
			var err error
			adapter, err = normalizeOperationAdapter(capability, operation, adapter)
			if err != nil {
				return nil, nil, err
			}
		} else {
			adapter = ""
		}
		configJSON, err := encodeConfigMap(input.Config)
		if err != nil {
			return nil, nil, fmt.Errorf("%s 配置无效: %w", key, err)
		}
		item := model.ChannelModelOperation{Capability: capability, Operation: operation, Enabled: input.Enabled, ProtocolMode: mode, Adapter: adapter, ConfigJSON: configJSON, ConfigVersion: 1}
		resolved, err := effectiveProtocol(item, defaultsForChannel(defaults, channelID))
		if err != nil && input.Enabled {
			return nil, nil, err
		}
		item.ContractKey = resolved.ContractKey
		result = append(result, item)
		effective[key] = resolved
	}
	return result, effective, nil
}

func normalizeChannelDefaults(channelID uint, inputs []model.SaveChannelProtocolDefaultInput) ([]model.ChannelProtocolDefault, error) {
	result := make([]model.ChannelProtocolDefault, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		capability, operation := strings.TrimSpace(input.Capability), strings.TrimSpace(input.Operation)
		key := operationKey(capability, operation)
		if !validCapabilityOperation(capability, operation) {
			return nil, fmt.Errorf("不支持的渠道默认操作 %s", key)
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("渠道默认操作重复：%s", key)
		}
		seen[key] = struct{}{}
		adapter, err := normalizeOperationAdapter(capability, operation, input.Adapter)
		if err != nil {
			return nil, err
		}
		configJSON, err := encodeConfigMap(input.Config)
		if err != nil {
			return nil, err
		}
		result = append(result, model.ChannelProtocolDefault{ChannelID: channelID, Capability: capability, Operation: operation, Adapter: adapter, ConfigJSON: configJSON, ConfigVersion: 1})
	}
	return result, nil
}

func normalizeModelPricing(inputs []model.SaveModelPricingInput) ([]model.ModelPricingRule, error) {
	result := make([]model.ModelPricingRule, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		capability := strings.TrimSpace(input.Capability)
		if _, ok := seen[capability]; ok {
			return nil, fmt.Errorf("%s 定价重复", capability)
		}
		seen[capability] = struct{}{}
		pricing, err := normalizePricingRule(capability, input.CreditsPerUnit, input.UnitType, input.PricingMode, input.PricingRule)
		if err != nil {
			return nil, err
		}
		result = append(result, pricing)
	}
	return result, nil
}

func normalizePricingRule(capability string, credits int, unit model.CreditPricingUnit, mode model.CreditPricingMode, rule string) (model.ModelPricingRule, error) {
	if capability != "image" && capability != "video" && capability != "text" && capability != "audio" {
		return model.ModelPricingRule{}, errors.New("计费能力无效")
	}
	if unit != model.UnitPerImage && unit != model.UnitPerVideo && unit != model.UnitPerVideoSecond && unit != model.UnitPerToken {
		return model.ModelPricingRule{}, errors.New("计费单位无效")
	}
	if mode != model.PricingModePerUnit && mode != model.PricingModeVideoDynamic {
		return model.ModelPricingRule{}, errors.New("计费模式无效")
	}
	result := model.ModelPricingRule{Capability: capability, CreditsPerUnit: credits, UnitType: unit, PricingMode: mode, PricingRule: strings.TrimSpace(rule)}
	if !result.HasValidPricingRule() {
		return model.ModelPricingRule{}, errors.New("计费规则必须包含有效的正数价格")
	}
	return result, nil
}

func effectiveProtocol(operation model.ChannelModelOperation, defaults []model.ChannelProtocolDefault) (model.EffectiveProtocolInfo, error) {
	adapter, configJSON, version, source := operation.Adapter, operation.ConfigJSON, operation.ConfigVersion, "model"
	if operation.ProtocolMode == model.ProtocolModeInherit {
		source = "channel"
		matched := false
		for _, item := range defaults {
			if item.Capability == operation.Capability && item.Operation == operation.Operation {
				adapter, configJSON, version, matched = item.Adapter, item.ConfigJSON, item.ConfigVersion, true
				break
			}
		}
		if !matched {
			return model.EffectiveProtocolInfo{}, fmt.Errorf("%s 缺少渠道默认协议", operationKey(operation.Capability, operation.Operation))
		}
	}
	normalizedAdapter, err := normalizeOperationAdapter(operation.Capability, operation.Operation, adapter)
	if err != nil {
		return model.EffectiveProtocolInfo{}, err
	}
	config := decodeConfigMap(configJSON)
	contract := contractKey(operation.Capability, operation.Operation, normalizedAdapter, config, version)
	return model.EffectiveProtocolInfo{Source: source, Adapter: normalizedAdapter, Config: config, ConfigVersion: version, ContractKey: contract}, nil
}

func normalizeOperationAdapter(capability, operation, adapter string) (string, error) {
	adapter = strings.ToLower(strings.TrimSpace(adapter))
	switch capability {
	case "image":
		if operation == "edit" {
			return model.NormalizeImageEditRoute(adapter)
		}
		return model.NormalizeImageGenerateRoute(adapter)
	case "video":
		return model.NormalizeVideoRoute(adapter)
	case "text", "audio":
		if adapter == "" || adapter == "auto" {
			return "openai", nil
		}
		if adapter != "openai" {
			return "", fmt.Errorf("%s 暂不支持协议 %s", capability, adapter)
		}
		return adapter, nil
	default:
		return "", errors.New("模型能力无效")
	}
}

type legacyModelProjection struct {
	Capabilities      string
	ImageGenerate     string
	ImageEdit         string
	VideoRoute        string
	VideoDurations    string
	VideoCustomizable bool
	VideoCustomConfig string
}

func legacyProjection(operations []model.ChannelModelOperation, effective map[string]model.EffectiveProtocolInfo) (legacyModelProjection, error) {
	result := legacyModelProjection{ImageGenerate: model.ImageRouteAuto, ImageEdit: model.ImageRouteAuto, VideoRoute: "auto", VideoDurations: "[]"}
	capabilities := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, operation := range operations {
		if !operation.Enabled {
			continue
		}
		if _, ok := seen[operation.Capability]; !ok {
			seen[operation.Capability] = struct{}{}
			capabilities = append(capabilities, operation.Capability)
		}
		resolved := effective[operationKey(operation.Capability, operation.Operation)]
		switch operationKey(operation.Capability, operation.Operation) {
		case "image:generate":
			result.ImageGenerate = resolved.Adapter
		case "image:edit":
			result.ImageEdit = resolved.Adapter
		case "video:generate":
			result.VideoRoute = resolved.Adapter
			if durations, ok := resolved.Config["durations"]; ok {
				encoded, _ := json.Marshal(durations)
				result.VideoDurations = string(encoded)
			}
			result.VideoCustomizable, _ = resolved.Config["customizable"].(bool)
			if custom, ok := resolved.Config["custom_config"]; ok && resolved.Adapter == "custom" {
				encoded, err := json.Marshal(custom)
				if err != nil {
					return result, err
				}
				var config model.CustomVideoConfig
				if err := json.Unmarshal(encoded, &config); err != nil {
					return result, errors.New("自定义视频配置格式无效")
				}
				if err := model.NormalizeAndValidateCustomVideoConfig(&config); err != nil {
					return result, err
				}
				normalized, _ := json.Marshal(config)
				result.VideoCustomConfig = string(normalized)
			}
		}
	}
	sort.Strings(capabilities)
	encoded, _ := json.Marshal(capabilities)
	result.Capabilities = string(encoded)
	return result, nil
}

func effectivePricingByCapability(items []model.ModelPricingRule, catalogID, modelID uint) map[string]model.ModelPricingRuleInfo {
	result := make(map[string]model.ModelPricingRuleInfo)
	for _, item := range items {
		if item.CatalogModelID == catalogID && item.Scope == model.PricingScopeDefault && item.ScopeID == 0 {
			result[item.Capability] = pricingRuleInfo(item, "default")
		}
	}
	for _, item := range items {
		if item.Scope == model.PricingScopeImplementation && item.ScopeID == modelID {
			result[item.Capability] = pricingRuleInfo(item, "implementation")
		}
	}
	return result
}

func pricingRuleInfo(item model.ModelPricingRule, source string) model.ModelPricingRuleInfo {
	return model.ModelPricingRuleInfo{ID: item.ID, Capability: item.Capability, Scope: item.Scope, ScopeID: item.ScopeID, CreditsPerUnit: item.CreditsPerUnit, UnitType: item.UnitType, PricingMode: item.PricingMode, PricingRule: item.PricingRule, ConfigRevision: normalizedRevision(item.ConfigRevision), EffectiveSource: source}
}

func pricingInfoValid(item model.ModelPricingRuleInfo) bool {
	return model.ModelPricingRule{CreditsPerUnit: item.CreditsPerUnit, UnitType: item.UnitType, PricingMode: item.PricingMode, PricingRule: item.PricingRule}.HasValidPricingRule()
}

func validCapabilityOperation(capability, operation string) bool {
	if capability == "image" {
		return operation == "generate" || operation == "edit"
	}
	return (capability == "video" || capability == "text" || capability == "audio") && operation == "generate"
}

func operationKey(capability, operation string) string { return capability + ":" + operation }

func contractKey(capability, operation, adapter string, config map[string]any, version int) string {
	payload, _ := json.Marshal(struct {
		Capability string         `json:"capability"`
		Operation  string         `json:"operation"`
		Adapter    string         `json:"adapter"`
		Config     map[string]any `json:"config"`
		Version    int            `json:"version"`
	}{capability, operation, adapter, config, version})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func encodeConfigMap(value map[string]any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return "", errors.New("配置必须是 JSON 对象")
	}
	return string(encoded), nil
}

func decodeConfigMap(raw string) map[string]any {
	result := map[string]any{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &result)
	return result
}

func defaultsForChannel(items []model.ChannelProtocolDefault, channelID uint) []model.ChannelProtocolDefault {
	result := make([]model.ChannelProtocolDefault, 0)
	for _, item := range items {
		if item.ChannelID == channelID {
			result = append(result, item)
		}
	}
	return result
}

func operationsForModel(items []model.ChannelModelOperation, modelID uint) []model.ChannelModelOperation {
	result := make([]model.ChannelModelOperation, 0)
	for _, item := range items {
		if item.ChannelModelID == modelID {
			result = append(result, item)
		}
	}
	return result
}

func findModel(items []model.ChannelModel, id uint) *model.ChannelModel {
	for index := range items {
		if items[index].ID == id {
			return &items[index]
		}
	}
	return nil
}

func findChannel(items []model.Channel, id uint) *model.Channel {
	for index := range items {
		if items[index].ID == id {
			return &items[index]
		}
	}
	return nil
}

func findCatalog(items []model.CatalogModel, id uint) *model.CatalogModel {
	for index := range items {
		if items[index].ID == id {
			return &items[index]
		}
	}
	return nil
}

func findCatalogByKey(items []model.CatalogModel, key string) *model.CatalogModel {
	for index := range items {
		if items[index].PublicKey == key {
			return &items[index]
		}
	}
	return nil
}

func findOperationInfo(items []model.ModelOperationInfo, capability, operation string) *model.ModelOperationInfo {
	for index := range items {
		if items[index].Capability == capability && items[index].Operation == operation {
			return &items[index]
		}
	}
	return nil
}

func hasOperationCapability(items []model.ModelOperationInfo, capability string) bool {
	for _, item := range items {
		if item.Capability == capability && item.Enabled {
			return true
		}
	}
	return false
}

func appendUniqueUint(items []uint, value uint) []uint {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func dedupeReadinessIssues(items []model.ModelReadinessIssue) []model.ModelReadinessIssue {
	result := make([]model.ModelReadinessIssue, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		key := item.Code + ":" + item.Capability + ":" + item.Operation
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizedRevision(value uint) uint {
	if value == 0 {
		return 1
	}
	return value
}
