package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"infinite-canvas-server/crypto"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

const (
	apiConfigTransferCipher      = "aes-256-gcm"
	apiConfigTransferKDF         = "argon2id"
	apiConfigTransferArgonTime   = uint32(3)
	apiConfigTransferArgonMemory = uint32(64 * 1024)
	apiConfigTransferArgonThread = uint8(1)
	apiConfigTransferMaxBytes    = 10 << 20
)

type apiConfigTransferRepository interface {
	Load(tenantID uint) (*repository.APIConfigTransferData, error)
	Apply(plan *repository.APIConfigTransferApplyPlan) error
}

type APIConfigTransferService struct {
	repo       apiConfigTransferRepository
	encryptKey string
}

func NewAPIConfigTransferService(repo apiConfigTransferRepository, encryptKey string) *APIConfigTransferService {
	return &APIConfigTransferService{repo: repo, encryptKey: encryptKey}
}

func (s *APIConfigTransferService) Export(tenantID uint, password string) (*model.APIConfigTransferExportResult, error) {
	if err := validateTransferPassword(password); err != nil {
		return nil, err
	}
	data, err := s.repo.Load(tenantID)
	if err != nil {
		return nil, err
	}
	snapshot, summary, warnings, err := s.buildSnapshot(data)
	if err != nil {
		return nil, err
	}
	envelope, err := encryptAPIConfigSnapshot(snapshot, password)
	if err != nil {
		return nil, err
	}
	return &model.APIConfigTransferExportResult{
		FileName: fmt.Sprintf("infinite-canvas-model-api-config-%s.json", time.Now().Format("20060102-150405")),
		Envelope: envelope,
		Summary:  summary,
		Warnings: warnings,
	}, nil
}

func (s *APIConfigTransferService) Preview(tenantID uint, input model.APIConfigTransferImportInput) (*model.APIConfigTransferResult, error) {
	_, result, err := s.prepareImport(tenantID, input)
	return result, err
}

func (s *APIConfigTransferService) Import(tenantID uint, input model.APIConfigTransferImportInput) (*model.APIConfigTransferResult, error) {
	plan, result, err := s.prepareImport(tenantID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Apply(plan); err != nil {
		return nil, err
	}
	result.Applied = true
	return result, nil
}

func (s *APIConfigTransferService) prepareImport(tenantID uint, input model.APIConfigTransferImportInput) (*repository.APIConfigTransferApplyPlan, *model.APIConfigTransferResult, error) {
	if err := validateTransferPassword(input.Password); err != nil {
		return nil, nil, err
	}
	snapshot, err := decryptAPIConfigSnapshot(input.Envelope, input.Password)
	if err != nil {
		return nil, nil, err
	}
	data, err := s.repo.Load(tenantID)
	if err != nil {
		return nil, nil, err
	}
	plan, result := s.buildImportPlan(tenantID, snapshot, data)
	return plan, result, nil
}

func (s *APIConfigTransferService) buildSnapshot(data *repository.APIConfigTransferData) (*model.APIConfigTransferSnapshot, model.APIConfigTransferStats, []model.APIConfigTransferConflict, error) {
	snapshot := &model.APIConfigTransferSnapshot{SchemaVersion: 2, ExportedAt: time.Now().UTC(), Channels: make([]model.APIConfigTransferChannel, 0, len(data.Channels)), Pricing: []model.APIConfigTransferPricing{}, PricingRules: []model.APIConfigTransferPricingRule{}, VideoPresets: []model.APIConfigTransferVideoPreset{}, AutoRoutingPools: []model.APIConfigTransferAutoRoutingPool{}}
	stats := model.APIConfigTransferStats{}
	warnings := make([]model.APIConfigTransferConflict, 0)
	refs := make(map[uint]string, len(data.Channels))
	channelsByID := make(map[uint]*model.Channel, len(data.Channels))
	channelIndexes := make(map[uint]int, len(data.Channels))
	defaultsByChannel := make(map[uint][]model.ChannelProtocolDefault)
	for index := range data.ProtocolDefaults {
		item := data.ProtocolDefaults[index]
		defaultsByChannel[item.ChannelID] = append(defaultsByChannel[item.ChannelID], item)
	}
	for index := range data.Channels {
		item := &data.Channels[index]
		ref := fmt.Sprintf("channel_%d", index+1)
		apiKey := ""
		if item.ApiKey != "" {
			decrypted, err := crypto.Decrypt(s.encryptKey, item.ApiKey)
			if err != nil {
				return nil, stats, nil, fmt.Errorf("解密渠道 %q 的 API Key 失败", item.Name)
			}
			apiKey = decrypted
		}
		refs[item.ID] = ref
		channelsByID[item.ID] = item
		channelIndexes[item.ID] = len(snapshot.Channels)
		snapshot.Channels = append(snapshot.Channels, model.APIConfigTransferChannel{
			Ref: ref, Name: item.Name, BaseURL: item.BaseUrl, APIKey: apiKey, Enabled: item.Enabled,
			VideoAPIStandard: normalizeChannelVideoAPIStandard(item.VideoAPIStandard), NewAPIChannelID: item.NewApiChannelID,
			MetricsBaseURL: item.MetricsBaseUrl, Remark: item.Remark, ConfigRevision: item.ConfigRevision,
			ProtocolDefaults: transferProtocolDefaults(defaultsByChannel[item.ID]), Models: []model.APIConfigTransferModel{}, MergeGroups: []model.APIConfigTransferMergeGroup{},
		})
		stats.Channels.Create++
	}
	modelsByID := make(map[uint]*model.ChannelModel, len(data.Models))
	catalogsByID := make(map[uint]*model.CatalogModel, len(data.Catalogs))
	for index := range data.Catalogs {
		catalogsByID[data.Catalogs[index].ID] = &data.Catalogs[index]
	}
	operationsByModel := make(map[uint][]model.ChannelModelOperation)
	for index := range data.Operations {
		item := data.Operations[index]
		operationsByModel[item.ChannelModelID] = append(operationsByModel[item.ChannelModelID], item)
	}
	for index := range data.Models {
		item := &data.Models[index]
		if item.DeletedAt.Valid {
			continue
		}
		modelsByID[item.ID] = item
		channelIndex, ok := channelIndexes[item.ChannelID]
		if !ok {
			stats.Models.Skip++
			warnings = append(warnings, transferConflict("model", item.ModelName, "所属渠道不存在，已跳过"))
			continue
		}
		info, err := channelModelToInfo(item)
		if err != nil {
			return nil, stats, nil, fmt.Errorf("导出模型 %q 失败: %w", item.ModelName, err)
		}
		catalog := catalogsByID[item.CatalogModelID]
		publicKey, displayName := item.ModelName, item.ModelName
		if catalog != nil {
			publicKey, displayName = catalog.PublicKey, catalog.DisplayName
		}
		upstreamModelID := item.UpstreamModelID
		if upstreamModelID == "" {
			upstreamModelID = item.ModelName
		}
		snapshot.Channels[channelIndex].Models = append(snapshot.Channels[channelIndex].Models, model.APIConfigTransferModel{
			ModelName: info.ModelName, Capabilities: info.Capabilities, Enabled: info.Enabled,
			PublicKey: publicKey, DisplayName: displayName, UpstreamModelID: upstreamModelID, Status: item.Status,
			DiscoveryStatus: item.DiscoveryStatus, ConfigRevision: item.ConfigRevision, LegacyUnreviewed: item.LegacyUnreviewed,
			Operations:         transferModelOperations(operationsByModel[item.ID]),
			ImageGenerateRoute: info.ImageGenerateRoute, ImageEditRoute: info.ImageEditRoute, VideoRoute: info.VideoRoute,
			VideoDurations: info.VideoDurations, VideoCustomizable: info.VideoCustomizable, VideoCustomConfig: info.VideoCustomConfig, SortOrder: info.SortOrder,
		})
		stats.Models.Create++
	}
	for index := range data.MergeGroups {
		item := &data.MergeGroups[index]
		channelIndex, ok := channelIndexes[item.ChannelID]
		if !ok {
			stats.MergeGroups.Skip++
			warnings = append(warnings, transferConflict("merge_group", item.GroupName, "所属渠道不存在，已跳过"))
			continue
		}
		snapshot.Channels[channelIndex].MergeGroups = append(snapshot.Channels[channelIndex].MergeGroups, model.APIConfigTransferMergeGroup{GroupName: item.GroupName, Pattern: item.Pattern, Enabled: item.Enabled})
		stats.MergeGroups.Create++
	}
	for index := range data.Pricing {
		item := &data.Pricing[index]
		channelRef := ""
		if item.ChannelID > 0 {
			var ok bool
			channelRef, ok = refs[item.ChannelID]
			if !ok {
				stats.Pricing.Skip++
				warnings = append(warnings, transferConflict("pricing", item.Model, "引用的渠道不存在，已跳过"))
				continue
			}
		}
		snapshot.Pricing = append(snapshot.Pricing, model.APIConfigTransferPricing{Model: item.Model, ChannelRef: channelRef, CreditsPerUnit: item.CreditsPerUnit, UnitType: item.UnitType, PricingMode: item.PricingMode, PricingRule: item.PricingRule})
		if len(data.PricingRules) == 0 {
			stats.Pricing.Create++
		}
	}
	for index := range data.PricingRules {
		item := &data.PricingRules[index]
		catalog := catalogsByID[item.CatalogModelID]
		if catalog == nil {
			stats.Pricing.Skip++
			warnings = append(warnings, transferConflict("pricing", fmt.Sprintf("catalog:%d/%s", item.CatalogModelID, item.Capability), "公开模型不存在，已跳过"))
			continue
		}
		output := model.APIConfigTransferPricingRule{PublicKey: catalog.PublicKey, Capability: item.Capability, Scope: item.Scope, CreditsPerUnit: item.CreditsPerUnit, UnitType: item.UnitType, PricingMode: item.PricingMode, PricingRule: item.PricingRule, ConfigRevision: item.ConfigRevision}
		if item.Scope == model.PricingScopeImplementation {
			channelModel := modelsByID[item.ScopeID]
			if channelModel == nil {
				stats.Pricing.Skip++
				warnings = append(warnings, transferConflict("pricing", catalog.PublicKey+"/"+item.Capability, "渠道模型覆盖引用无效，已跳过"))
				continue
			}
			output.ChannelRef = refs[channelModel.ChannelID]
			output.UpstreamModelID = channelModel.UpstreamModelID
			if output.UpstreamModelID == "" {
				output.UpstreamModelID = channelModel.ModelName
			}
			if output.ChannelRef == "" {
				stats.Pricing.Skip++
				warnings = append(warnings, transferConflict("pricing", catalog.PublicKey+"/"+item.Capability, "渠道模型覆盖所属渠道无效，已跳过"))
				continue
			}
		}
		snapshot.PricingRules = append(snapshot.PricingRules, output)
		stats.Pricing.Create++
	}
	for index := range data.VideoPresets {
		info, err := videoConfigPresetToInfo(&data.VideoPresets[index])
		if err != nil {
			return nil, stats, nil, fmt.Errorf("导出视频预设 %q 失败: %w", data.VideoPresets[index].Name, err)
		}
		snapshot.VideoPresets = append(snapshot.VideoPresets, model.APIConfigTransferVideoPreset{Name: info.Name, Config: info.Config})
		stats.VideoPresets.Create++
	}
	for index := range data.AutoRoutingPools {
		pool := &data.AutoRoutingPools[index]
		output := model.APIConfigTransferAutoRoutingPool{Model: pool.PublicModelName, Capability: pool.Capability, ContractKey: pool.ContractKey, Enabled: pool.Enabled, MaxAttempts: pool.MaxAttempts, Members: []model.APIConfigTransferAutoRoutingMember{}}
		valid := true
		for memberIndex := range pool.Members {
			member := &pool.Members[memberIndex]
			channelModel := modelsByID[member.ChannelModelID]
			if channelModel == nil {
				valid = false
				break
			}
			channelRef, ok := refs[channelModel.ChannelID]
			channel := channelsByID[channelModel.ChannelID]
			contract, contractErr := autoRoutingContract(channel, channelModel, pool.Capability)
			if !ok || contractErr != nil || contract != pool.ContractKey || channelModel.ModelName != pool.PublicModelName {
				valid = false
				break
			}
			output.Members = append(output.Members, model.APIConfigTransferAutoRoutingMember{ChannelRef: channelRef, Model: channelModel.ModelName, Priority: member.Priority, Enabled: member.Enabled})
		}
		if !valid || len(output.Members) < 2 {
			stats.AutoRoutingPools.Skip++
			warnings = append(warnings, transferConflict("auto_routing_pool", pool.PublicModelName+"/"+pool.Capability, "路由池合同或候选已失效，已跳过"))
			continue
		}
		snapshot.AutoRoutingPools = append(snapshot.AutoRoutingPools, output)
		stats.AutoRoutingPools.Create++
	}
	return snapshot, stats, warnings, nil
}

type transferChannelState struct {
	conflict bool
	targetID uint
}

func (s *APIConfigTransferService) buildImportPlan(tenantID uint, snapshot *model.APIConfigTransferSnapshot, data *repository.APIConfigTransferData) (*repository.APIConfigTransferApplyPlan, *model.APIConfigTransferResult) {
	plan := &repository.APIConfigTransferApplyPlan{SchemaVersion: snapshot.SchemaVersion}
	result := &model.APIConfigTransferResult{Conflicts: []model.APIConfigTransferConflict{}}
	states := make(map[string]transferChannelState, len(snapshot.Channels))
	targetByKey, targetByName := indexTransferChannels(data.Channels)
	refCounts, sourceNameCounts, sourceKeyCounts := countSnapshotChannels(snapshot.Channels)

	for index := range snapshot.Channels {
		input := &snapshot.Channels[index]
		identifier := transferChannelIdentifier(input)
		name, baseURL, err := validateChannelInput(input.Name, input.BaseURL)
		nameKey, urlKey := normalizeTransferName(input.Name), canonicalTransferURL(input.BaseURL)
		key := nameKey + "\x00" + urlKey
		if err != nil || input.Ref == "" || refCounts[input.Ref] > 1 || sourceNameCounts[nameKey] > 1 || sourceKeyCounts[key] > 1 {
			reason := "渠道字段无效或导入文件内存在重复名称、组合键、引用"
			result.Stats.Channels.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, reason))
			states[input.Ref] = transferChannelState{conflict: true}
			continue
		}
		if len([]rune(baseURL)) > 500 || len([]rune(input.Remark)) > 500 || (input.MetricsBaseURL != nil && len([]rune(strings.TrimSpace(*input.MetricsBaseURL))) > 500) {
			result.Stats.Channels.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, "渠道地址、指标地址或备注超过长度限制"))
			states[input.Ref] = transferChannelState{conflict: true}
			continue
		}
		standard, err := validateVideoAPIStandard(&input.VideoAPIStandard)
		if err != nil {
			result.Stats.Channels.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, err.Error()))
			states[input.Ref] = transferChannelState{conflict: true}
			continue
		}
		exact := targetByKey[key]
		if len(exact) > 1 || len(targetByName[nameKey]) > 1 || (len(exact) == 0 && len(targetByName[nameKey]) > 0) {
			result.Stats.Channels.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, "目标环境存在同名不同地址或重复渠道"))
			states[input.Ref] = transferChannelState{conflict: true}
			continue
		}
		var item model.Channel
		existingID := uint(0)
		updating := len(exact) == 1
		if len(exact) == 1 {
			item = *exact[0]
			existingID = item.ID
		} else {
			if strings.TrimSpace(input.APIKey) == "" {
				result.Stats.Channels.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, "新渠道缺少 API Key"))
				states[input.Ref] = transferChannelState{conflict: true}
				continue
			}
		}
		item.Name, item.BaseUrl, item.Enabled, item.VideoAPIStandard = name, baseURL, input.Enabled, standard
		item.NewApiChannelID, item.MetricsBaseUrl, item.Remark = input.NewAPIChannelID, input.MetricsBaseURL, input.Remark
		if !updating && input.ConfigRevision > 0 {
			item.ConfigRevision = input.ConfigRevision
		}
		if item.ConfigRevision == 0 {
			item.ConfigRevision = 1
		}
		if strings.TrimSpace(input.APIKey) != "" {
			encrypted, encryptErr := crypto.Encrypt(s.encryptKey, strings.TrimSpace(input.APIKey))
			if encryptErr != nil || len(encrypted) > 500 {
				result.Stats.Channels.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, "API Key 加密失败或超过长度限制"))
				states[input.Ref] = transferChannelState{conflict: true}
				continue
			}
			item.ApiKey = encrypted
		}
		defaults, defaultsErr := importProtocolDefaults(input.ProtocolDefaults)
		if defaultsErr != nil {
			result.Stats.Channels.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("channel", identifier, defaultsErr.Error()))
			states[input.Ref] = transferChannelState{conflict: true}
			continue
		}
		if len(defaults) == 0 {
			defaults = defaultTransferProtocols()
		}
		operation := repository.APIConfigTransferChannelOperation{Ref: input.Ref, ExistingID: existingID, Item: item, Defaults: defaults}
		plan.Channels = append(plan.Channels, operation)
		states[input.Ref] = transferChannelState{targetID: existingID}
		if updating {
			result.Stats.Channels.Update++
		} else {
			result.Stats.Channels.Create++
		}
	}

	currentModels := indexTransferModels(data.Models)
	currentGroups := indexTransferGroups(data.MergeGroups)
	for channelIndex := range snapshot.Channels {
		channel := &snapshot.Channels[channelIndex]
		state, usable := states[channel.Ref]
		modelCounts := make(map[string]int, len(channel.Models))
		groupCounts := make(map[string]int, len(channel.MergeGroups))
		for _, item := range channel.Models {
			modelCounts[strings.TrimSpace(item.ModelName)]++
		}
		for _, item := range channel.MergeGroups {
			groupCounts[normalizeTransferName(item.GroupName)]++
		}
		for modelIndex := range channel.Models {
			input := &channel.Models[modelIndex]
			identifier := channel.Name + "/" + input.ModelName
			if !usable || state.conflict {
				result.Stats.Models.Skip++
				continue
			}
			if modelCounts[strings.TrimSpace(input.ModelName)] > 1 {
				result.Stats.Models.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("model", identifier, "导入文件内模型名称重复"))
				continue
			}
			item, err := transferModelToRecord(input)
			if err != nil {
				result.Stats.Models.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("model", identifier, err.Error()))
				continue
			}
			var existing []*model.ChannelModel
			if state.targetID > 0 {
				existing = currentModels[transferModelKey(state.targetID, item.ModelName)]
			}
			publicKey := strings.TrimSpace(input.PublicKey)
			if publicKey == "" {
				publicKey = item.ModelName
			}
			displayName := strings.TrimSpace(input.DisplayName)
			if displayName == "" {
				displayName = publicKey
			}
			if len([]rune(publicKey)) > 191 || len([]rune(displayName)) > 200 {
				result.Stats.Models.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("model", identifier, "公开调用键或显示名称超过长度限制"))
				continue
			}
			operations, operationErr := importModelOperations(input.Operations, item)
			if operationErr != nil {
				result.Stats.Models.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("model", identifier, operationErr.Error()))
				continue
			}
			operation := repository.APIConfigTransferModelOperation{ChannelRef: channel.Ref, Item: item, PublicKey: publicKey, DisplayName: displayName, Operations: operations}
			if len(existing) > 1 {
				result.Stats.Models.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("model", identifier, "目标环境存在重复模型"))
				continue
			}
			if len(existing) == 1 {
				base := *existing[0]
				item.BaseModel = base.BaseModel
				item.ChannelID = base.ChannelID
				operation.ExistingID, operation.Item = base.ID, item
				result.Stats.Models.Update++
			} else {
				result.Stats.Models.Create++
			}
			plan.Models = append(plan.Models, operation)
		}
		for groupIndex := range channel.MergeGroups {
			input := &channel.MergeGroups[groupIndex]
			identifier := channel.Name + "/" + input.GroupName
			if !usable || state.conflict {
				result.Stats.MergeGroups.Skip++
				continue
			}
			name, pattern := strings.TrimSpace(input.GroupName), strings.TrimSpace(input.Pattern)
			nameKey := normalizeTransferName(name)
			if name == "" || pattern == "" || len([]rune(name)) > 200 || len([]rune(pattern)) > 200 || groupCounts[nameKey] > 1 {
				result.Stats.MergeGroups.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("merge_group", identifier, "合并组名称或匹配规则无效，或名称重复"))
				continue
			}
			var existing []*model.ModelMergeGroup
			if state.targetID > 0 {
				existing = currentGroups[transferGroupKey(state.targetID, nameKey)]
			}
			operation := repository.APIConfigTransferMergeGroupOperation{ChannelRef: channel.Ref, Item: model.ModelMergeGroup{GroupName: name, Pattern: pattern, Enabled: input.Enabled}}
			if len(existing) > 1 {
				result.Stats.MergeGroups.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("merge_group", identifier, "目标环境存在重复合并组"))
				continue
			}
			if len(existing) == 1 {
				item := *existing[0]
				item.GroupName, item.Pattern, item.Enabled = name, pattern, input.Enabled
				operation.ExistingID, operation.Item = item.ID, item
				result.Stats.MergeGroups.Update++
			} else {
				result.Stats.MergeGroups.Create++
			}
			plan.MergeGroups = append(plan.MergeGroups, operation)
		}
	}

	if snapshot.SchemaVersion == 1 || len(snapshot.PricingRules) == 0 {
		s.addPricingPlan(tenantID, snapshot, data, states, plan, result)
	} else {
		s.addPricingRulePlan(tenantID, snapshot, data, states, plan, result)
	}
	s.addPresetPlan(tenantID, snapshot, data, plan, result)
	s.addAutoRoutingPlan(snapshot, data, states, plan, result)
	return plan, result
}

func (s *APIConfigTransferService) addAutoRoutingPlan(snapshot *model.APIConfigTransferSnapshot, data *repository.APIConfigTransferData, states map[string]transferChannelState, plan *repository.APIConfigTransferApplyPlan, result *model.APIConfigTransferResult) {
	current := make(map[string][]*model.AutoRoutingPool, len(data.AutoRoutingPools))
	for index := range data.AutoRoutingPools {
		item := &data.AutoRoutingPools[index]
		current[autoRoutingPoolKey(item.PublicModelName, item.Capability)] = append(current[autoRoutingPoolKey(item.PublicModelName, item.Capability)], item)
	}
	counts := make(map[string]int, len(snapshot.AutoRoutingPools))
	for _, item := range snapshot.AutoRoutingPools {
		counts[autoRoutingPoolKey(item.Model, item.Capability)]++
	}
	channels := make(map[string]*model.APIConfigTransferChannel, len(snapshot.Channels))
	models := make(map[string][]model.ChannelModel)
	for channelIndex := range snapshot.Channels {
		channel := &snapshot.Channels[channelIndex]
		channels[channel.Ref] = channel
		for modelIndex := range channel.Models {
			if item, err := transferModelToRecord(&channel.Models[modelIndex]); err == nil {
				models[channel.Ref+"\x00"+item.ModelName] = append(models[channel.Ref+"\x00"+item.ModelName], item)
			}
		}
	}
	plannedModels := make(map[string]bool, len(plan.Models))
	for _, operation := range plan.Models {
		plannedModels[operation.ChannelRef+"\x00"+operation.Item.ModelName] = true
	}

	for poolIndex := range snapshot.AutoRoutingPools {
		input := &snapshot.AutoRoutingPools[poolIndex]
		modelName, capability := strings.TrimSpace(input.Model), strings.TrimSpace(input.Capability)
		identifier := modelName + "/" + capability
		key := autoRoutingPoolKey(modelName, capability)
		if modelName == "" || !validAutoCapability(capability) || counts[key] > 1 || input.MaxAttempts < 1 || input.MaxAttempts > autoDefaultMaxAttempts || len(input.Members) < 2 {
			result.Stats.AutoRoutingPools.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("auto_routing_pool", identifier, "路由池字段无效、重复或候选不足"))
			continue
		}
		memberKeys := make(map[string]bool, len(input.Members))
		members := make([]repository.APIConfigTransferAutoRoutingMemberOperation, 0, len(input.Members))
		contractKey := ""
		enabledMembers := 0
		invalidReason := ""
		for _, member := range input.Members {
			memberModel := strings.TrimSpace(member.Model)
			memberKey := member.ChannelRef + "\x00" + memberModel
			state, stateExists := states[member.ChannelRef]
			channel := channels[member.ChannelRef]
			modelItems := models[memberKey]
			if member.ChannelRef == "" || memberModel != modelName || memberKeys[memberKey] || !stateExists || state.conflict || channel == nil || len(modelItems) != 1 || !plannedModels[memberKey] {
				invalidReason = "候选引用不存在、重复或关联配置已冲突"
				break
			}
			memberKeys[memberKey] = true
			channelRecord := &model.Channel{Enabled: channel.Enabled, VideoAPIStandard: normalizeChannelVideoAPIStandard(channel.VideoAPIStandard)}
			modelRecord := modelItems[0]
			contract, err := autoRoutingContract(channelRecord, &modelRecord, capability)
			if err != nil || contractKey != "" && contract != contractKey || strings.TrimSpace(input.ContractKey) != "" && contract != strings.TrimSpace(input.ContractKey) {
				invalidReason = "候选协议合同不一致"
				break
			}
			contractKey = contract
			if member.Enabled {
				enabledMembers++
			}
			members = append(members, repository.APIConfigTransferAutoRoutingMemberOperation{ChannelRef: member.ChannelRef, Model: memberModel, Priority: member.Priority, Enabled: member.Enabled})
		}
		if invalidReason == "" && input.Enabled && enabledMembers < 2 {
			invalidReason = "启用的路由池至少需要两个启用候选"
		}
		if invalidReason != "" || len(members) < 2 {
			result.Stats.AutoRoutingPools.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("auto_routing_pool", identifier, invalidReason))
			continue
		}
		existing := current[key]
		if len(existing) > 1 {
			result.Stats.AutoRoutingPools.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("auto_routing_pool", identifier, "目标环境存在重复路由池"))
			continue
		}
		item := model.AutoRoutingPool{PublicModelName: modelName, Capability: capability, ContractKey: contractKey, Enabled: input.Enabled, MaxAttempts: input.MaxAttempts}
		operation := repository.APIConfigTransferAutoRoutingPoolOperation{Item: item, Members: members}
		if len(existing) == 1 {
			item.BaseModel = existing[0].BaseModel
			operation.ExistingID, operation.Item = existing[0].ID, item
			result.Stats.AutoRoutingPools.Update++
		} else {
			result.Stats.AutoRoutingPools.Create++
		}
		plan.AutoRoutingPools = append(plan.AutoRoutingPools, operation)
	}
}

func autoRoutingPoolKey(modelName, capability string) string {
	return strings.TrimSpace(modelName) + "\x00" + strings.TrimSpace(capability)
}

func (s *APIConfigTransferService) addPricingPlan(tenantID uint, snapshot *model.APIConfigTransferSnapshot, data *repository.APIConfigTransferData, states map[string]transferChannelState, plan *repository.APIConfigTransferApplyPlan, result *model.APIConfigTransferResult) {
	current := make(map[string]*model.CreditPricing, len(data.Pricing))
	for index := range data.Pricing {
		item := &data.Pricing[index]
		current[transferPricingKey(item.Model, item.ChannelID)] = item
	}
	sourceCounts := make(map[string]int, len(snapshot.Pricing))
	for _, item := range snapshot.Pricing {
		sourceCounts[strings.TrimSpace(item.Model)+"\x00"+item.ChannelRef]++
	}
	for index := range snapshot.Pricing {
		input := &snapshot.Pricing[index]
		identifier := input.Model
		state := transferChannelState{}
		if input.ChannelRef != "" {
			var ok bool
			state, ok = states[input.ChannelRef]
			if !ok || state.conflict {
				result.Stats.Pricing.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, "引用的渠道不存在或已冲突"))
				continue
			}
		}
		if sourceCounts[strings.TrimSpace(input.Model)+"\x00"+input.ChannelRef] > 1 {
			result.Stats.Pricing.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, "导入文件内定价重复"))
			continue
		}
		item, err := transferPricingToRecord(tenantID, input)
		if err != nil {
			result.Stats.Pricing.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, err.Error()))
			continue
		}
		var existing *model.CreditPricing
		if input.ChannelRef == "" || state.targetID > 0 {
			existing = current[transferPricingKey(item.Model, state.targetID)]
		}
		if existing != nil {
			item.BaseModel = existing.BaseModel
			item.ChannelID = existing.ChannelID
			result.Stats.Pricing.Update++
		} else {
			result.Stats.Pricing.Create++
		}
		plan.Pricing = append(plan.Pricing, repository.APIConfigTransferPricingOperation{ChannelRef: input.ChannelRef, Item: item})
	}
}

func (s *APIConfigTransferService) addPricingRulePlan(tenantID uint, snapshot *model.APIConfigTransferSnapshot, data *repository.APIConfigTransferData, states map[string]transferChannelState, plan *repository.APIConfigTransferApplyPlan, result *model.APIConfigTransferResult) {
	counts := make(map[string]int, len(snapshot.PricingRules))
	for _, item := range snapshot.PricingRules {
		counts[transferPricingRuleKey(item)]++
	}
	currentCatalogs := make(map[string]uint, len(data.Catalogs))
	for _, item := range data.Catalogs {
		currentCatalogs[item.PublicKey] = item.ID
	}
	currentRules := make(map[string]model.ModelPricingRule, len(data.PricingRules))
	for _, item := range data.PricingRules {
		currentRules[fmt.Sprintf("%d\x00%s\x00%s\x00%d", item.CatalogModelID, item.Capability, item.Scope, item.ScopeID)] = item
	}
	plannedModels := make(map[string]struct{}, len(plan.Models))
	for _, item := range plan.Models {
		upstream := item.Item.UpstreamModelID
		if upstream == "" {
			upstream = item.Item.ModelName
		}
		plannedModels[item.ChannelRef+"\x00"+upstream] = struct{}{}
	}
	for index := range snapshot.PricingRules {
		input := &snapshot.PricingRules[index]
		identifier := strings.TrimSpace(input.PublicKey) + "/" + strings.TrimSpace(input.Capability)
		if counts[transferPricingRuleKey(*input)] > 1 {
			result.Stats.Pricing.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, "导入文件内规范化定价重复"))
			continue
		}
		publicKey, capability, scope := strings.TrimSpace(input.PublicKey), strings.TrimSpace(input.Capability), strings.TrimSpace(input.Scope)
		if publicKey == "" || len([]rune(publicKey)) > 191 || !validAutoCapability(capability) || scope != model.PricingScopeDefault && scope != model.PricingScopeImplementation {
			result.Stats.Pricing.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, "公开模型、能力或定价范围无效"))
			continue
		}
		state := transferChannelState{}
		if scope == model.PricingScopeImplementation {
			var ok bool
			state, ok = states[input.ChannelRef]
			_, modelPlanned := plannedModels[input.ChannelRef+"\x00"+strings.TrimSpace(input.UpstreamModelID)]
			if !ok || state.conflict || strings.TrimSpace(input.UpstreamModelID) == "" || !modelPlanned {
				result.Stats.Pricing.Skip++
				result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, "渠道模型覆盖引用不存在或已冲突"))
				continue
			}
		}
		legacy, err := transferPricingToRecord(tenantID, &model.APIConfigTransferPricing{Model: publicKey, CreditsPerUnit: input.CreditsPerUnit, UnitType: input.UnitType, PricingMode: input.PricingMode, PricingRule: input.PricingRule})
		if err != nil {
			result.Stats.Pricing.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("pricing", identifier, err.Error()))
			continue
		}
		item := model.ModelPricingRule{TenantID: tenantID, Capability: capability, Scope: scope, CreditsPerUnit: legacy.CreditsPerUnit, UnitType: legacy.UnitType, PricingMode: legacy.PricingMode, PricingRule: legacy.PricingRule, ConfigRevision: input.ConfigRevision}
		if item.ConfigRevision == 0 {
			item.ConfigRevision = 1
		}
		existing := false
		catalogID := currentCatalogs[publicKey]
		if catalogID > 0 {
			scopeID := uint(0)
			if scope == model.PricingScopeImplementation && state.targetID > 0 {
				for _, candidate := range data.Models {
					upstream := candidate.UpstreamModelID
					if upstream == "" {
						upstream = candidate.ModelName
					}
					if candidate.ChannelID == state.targetID && upstream == strings.TrimSpace(input.UpstreamModelID) {
						scopeID = candidate.ID
						break
					}
				}
			}
			_, existing = currentRules[fmt.Sprintf("%d\x00%s\x00%s\x00%d", catalogID, capability, scope, scopeID)]
		}
		if existing {
			result.Stats.Pricing.Update++
		} else {
			result.Stats.Pricing.Create++
		}
		plan.PricingRules = append(plan.PricingRules, repository.APIConfigTransferPricingRuleOperation{ChannelRef: input.ChannelRef, UpstreamModelID: strings.TrimSpace(input.UpstreamModelID), PublicKey: publicKey, Item: item})
	}
}

func (s *APIConfigTransferService) addPresetPlan(tenantID uint, snapshot *model.APIConfigTransferSnapshot, data *repository.APIConfigTransferData, plan *repository.APIConfigTransferApplyPlan, result *model.APIConfigTransferResult) {
	current := make(map[string][]*model.VideoConfigPreset, len(data.VideoPresets))
	for index := range data.VideoPresets {
		item := &data.VideoPresets[index]
		current[item.NormalizedName] = append(current[item.NormalizedName], item)
	}
	counts := make(map[string]int, len(snapshot.VideoPresets))
	for _, item := range snapshot.VideoPresets {
		counts[normalizeTransferName(item.Name)]++
	}
	for index := range snapshot.VideoPresets {
		input := &snapshot.VideoPresets[index]
		name, normalized := strings.TrimSpace(input.Name), normalizeTransferName(input.Name)
		config := input.Config
		if name == "" || len([]rune(name)) > 200 || counts[normalized] > 1 || model.NormalizeAndValidateCustomVideoConfig(&config) != nil {
			result.Stats.VideoPresets.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("video_preset", input.Name, "预设名称、配置无效或名称重复"))
			continue
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			result.Stats.VideoPresets.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("video_preset", input.Name, "预设配置无法编码"))
			continue
		}
		operation := repository.APIConfigTransferPresetOperation{Item: model.VideoConfigPreset{TenantID: tenantID, Name: name, NormalizedName: normalized, Config: string(encoded)}}
		existing := current[normalized]
		if len(existing) > 1 {
			result.Stats.VideoPresets.Skip++
			result.Conflicts = append(result.Conflicts, transferConflict("video_preset", input.Name, "目标环境存在重复预设"))
			continue
		}
		if len(existing) == 1 {
			item := *existing[0]
			item.Name, item.Config = name, string(encoded)
			operation.ExistingID, operation.Item = item.ID, item
			result.Stats.VideoPresets.Update++
		} else {
			result.Stats.VideoPresets.Create++
		}
		plan.VideoPresets = append(plan.VideoPresets, operation)
	}
}

func transferModelToRecord(input *model.APIConfigTransferModel) (model.ChannelModel, error) {
	name := strings.TrimSpace(input.UpstreamModelID)
	if name == "" {
		name = strings.TrimSpace(input.ModelName)
	}
	if name == "" || len([]rune(name)) > 200 {
		return model.ChannelModel{}, errors.New("模型名称无效")
	}
	if len(input.Capabilities) == 0 {
		return model.ChannelModel{}, errors.New("至少选择一个能力")
	}
	capabilities, err := json.Marshal(input.Capabilities)
	if err != nil {
		return model.ChannelModel{}, err
	}
	if len(capabilities) > 100 {
		return model.ChannelModel{}, errors.New("模型能力配置超过长度限制")
	}
	durations, err := json.Marshal(input.VideoDurations)
	if err != nil {
		return model.ChannelModel{}, err
	}
	if len(durations) > 200 {
		return model.ChannelModel{}, errors.New("视频时长配置超过长度限制")
	}
	imageGenerateRoute, err := model.NormalizeImageGenerateRoute(input.ImageGenerateRoute)
	if err != nil {
		return model.ChannelModel{}, err
	}
	imageEditRoute, err := model.NormalizeImageEditRoute(input.ImageEditRoute)
	if err != nil {
		return model.ChannelModel{}, err
	}
	videoRoute, err := model.NormalizeVideoRoute(input.VideoRoute)
	if err != nil {
		return model.ChannelModel{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		if input.Enabled {
			status = model.ModelStatusActive
		} else {
			status = model.ModelStatusDisabled
		}
	}
	if status != model.ModelStatusDraft && status != model.ModelStatusActive && status != model.ModelStatusDisabled {
		return model.ChannelModel{}, errors.New("模型状态无效")
	}
	discoveryStatus := strings.TrimSpace(input.DiscoveryStatus)
	if discoveryStatus == "" {
		discoveryStatus = model.DiscoveryStatusPresent
	}
	if discoveryStatus != model.DiscoveryStatusPresent && discoveryStatus != model.DiscoveryStatusMissing {
		return model.ChannelModel{}, errors.New("模型同步状态无效")
	}
	revision := input.ConfigRevision
	if revision == 0 {
		revision = 1
	}
	item := model.ChannelModel{ModelName: name, UpstreamModelID: name, Status: status, DiscoveryStatus: discoveryStatus, ConfigRevision: revision, LegacyUnreviewed: input.LegacyUnreviewed, Capabilities: string(capabilities), Enabled: status == model.ModelStatusActive, ImageGenerateRoute: imageGenerateRoute, ImageEditRoute: imageEditRoute, VideoRoute: videoRoute, VideoDurations: string(durations), VideoCustomizable: input.VideoCustomizable, SortOrder: input.SortOrder}
	if item.VideoRoute == "custom" {
		if input.VideoCustomConfig == nil {
			return model.ChannelModel{}, errors.New("自定义视频路由必须提供配置")
		}
		config := *input.VideoCustomConfig
		if err := model.NormalizeAndValidateCustomVideoConfig(&config); err != nil {
			return model.ChannelModel{}, err
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			return model.ChannelModel{}, err
		}
		item.VideoCustomConfig = string(encoded)
	}
	return item, nil
}

func transferPricingToRecord(tenantID uint, input *model.APIConfigTransferPricing) (model.CreditPricing, error) {
	item := model.CreditPricing{TenantID: tenantID, Model: strings.TrimSpace(input.Model), CreditsPerUnit: input.CreditsPerUnit, UnitType: input.UnitType, PricingMode: input.PricingMode, PricingRule: input.PricingRule}
	if item.Model == "" || len([]rune(item.Model)) > 100 {
		return item, errors.New("模型名称不能为空")
	}
	if item.PricingMode == "" {
		item.PricingMode = model.PricingModePerUnit
	}
	if item.PricingMode == model.PricingModeVideoDynamic || item.UnitType == model.UnitPerVideoSecond {
		item.PricingMode, item.UnitType = model.PricingModeVideoDynamic, model.UnitPerVideoSecond
		var rule model.VideoPricingRule
		if json.Unmarshal([]byte(item.PricingRule), &rule) != nil || !hasPositiveVideoRateForTransfer(rule.ResolutionSecondRates) {
			return item, errors.New("视频动态计费规则无效")
		}
		return item, nil
	}
	if item.PricingMode != model.PricingModePerUnit || item.CreditsPerUnit <= 0 {
		return item, errors.New("模型定价无效")
	}
	switch item.UnitType {
	case model.UnitPerImage, model.UnitPerVideo, model.UnitPerToken:
	default:
		return item, errors.New("计价单位无效")
	}
	return item, nil
}

func encryptAPIConfigSnapshot(snapshot *model.APIConfigTransferSnapshot, password string) (model.APIConfigTransferEnvelope, error) {
	plaintext, err := json.Marshal(snapshot)
	if err != nil {
		return model.APIConfigTransferEnvelope{}, err
	}
	if len(plaintext) > apiConfigTransferMaxBytes {
		return model.APIConfigTransferEnvelope{}, errors.New("导出配置超过 10 MiB 限制")
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return model.APIConfigTransferEnvelope{}, err
	}
	key := argon2.IDKey([]byte(password), salt, apiConfigTransferArgonTime, apiConfigTransferArgonMemory, apiConfigTransferArgonThread, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return model.APIConfigTransferEnvelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return model.APIConfigTransferEnvelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return model.APIConfigTransferEnvelope{}, err
	}
	aad := []byte(fmt.Sprintf("%s:%d", model.APIConfigTransferFormat, model.APIConfigTransferFormatVersion))
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	return model.APIConfigTransferEnvelope{
		Format: model.APIConfigTransferFormat, Version: model.APIConfigTransferFormatVersion, Cipher: apiConfigTransferCipher,
		KDF:  model.APIConfigTransferKDF{Name: apiConfigTransferKDF, Time: apiConfigTransferArgonTime, MemoryKiB: apiConfigTransferArgonMemory, Parallelism: apiConfigTransferArgonThread},
		Salt: base64.StdEncoding.EncodeToString(salt), Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func decryptAPIConfigSnapshot(envelope model.APIConfigTransferEnvelope, password string) (*model.APIConfigTransferSnapshot, error) {
	if envelope.Format != model.APIConfigTransferFormat || envelope.Version != model.APIConfigTransferFormatVersion || envelope.Cipher != apiConfigTransferCipher || envelope.KDF.Name != apiConfigTransferKDF || envelope.KDF.Time != apiConfigTransferArgonTime || envelope.KDF.MemoryKiB != apiConfigTransferArgonMemory || envelope.KDF.Parallelism != apiConfigTransferArgonThread {
		return nil, errors.New("不支持的配置文件格式或加密参数")
	}
	salt, err := base64.StdEncoding.DecodeString(envelope.Salt)
	if err != nil || len(salt) != 16 {
		return nil, errors.New("配置文件盐值无效")
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != 12 {
		return nil, errors.New("配置文件 nonce 无效")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) > apiConfigTransferMaxBytes+(1<<20) {
		return nil, errors.New("配置文件密文无效或超过 10 MiB 限制")
	}
	key := argon2.IDKey([]byte(password), salt, envelope.KDF.Time, envelope.KDF.MemoryKiB, envelope.KDF.Parallelism, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := []byte(fmt.Sprintf("%s:%d", envelope.Format, envelope.Version))
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("密码错误或配置文件已损坏")
	}
	if len(plaintext) > apiConfigTransferMaxBytes {
		return nil, errors.New("配置内容超过 10 MiB 限制")
	}
	var snapshot model.APIConfigTransferSnapshot
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, errors.New("配置内容格式无效")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("配置内容格式无效")
	}
	if snapshot.SchemaVersion != 1 && snapshot.SchemaVersion != 2 {
		return nil, errors.New("不支持的配置内容版本")
	}
	return &snapshot, nil
}

func transferProtocolDefaults(items []model.ChannelProtocolDefault) []model.APIConfigTransferProtocol {
	result := make([]model.APIConfigTransferProtocol, 0, len(items))
	for _, item := range items {
		result = append(result, model.APIConfigTransferProtocol{Capability: item.Capability, Operation: item.Operation, Adapter: item.Adapter, Config: decodeConfigMap(item.ConfigJSON), ConfigVersion: item.ConfigVersion})
	}
	return result
}

func transferModelOperations(items []model.ChannelModelOperation) []model.APIConfigTransferOperation {
	result := make([]model.APIConfigTransferOperation, 0, len(items))
	for _, item := range items {
		result = append(result, model.APIConfigTransferOperation{Capability: item.Capability, Operation: item.Operation, Enabled: item.Enabled, ProtocolMode: item.ProtocolMode, Adapter: item.Adapter, Config: decodeConfigMap(item.ConfigJSON), ConfigVersion: item.ConfigVersion, ContractKey: item.ContractKey})
	}
	return result
}

func importProtocolDefaults(items []model.APIConfigTransferProtocol) ([]model.ChannelProtocolDefault, error) {
	result := make([]model.ChannelProtocolDefault, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		capability, operation, adapter := strings.TrimSpace(item.Capability), strings.TrimSpace(item.Operation), strings.TrimSpace(item.Adapter)
		key := capability + "\x00" + operation
		if !validTransferOperation(capability, operation) || adapter == "" || len([]rune(adapter)) > 50 {
			return nil, errors.New("渠道协议默认值无效")
		}
		if _, ok := seen[key]; ok {
			return nil, errors.New("渠道协议默认值重复")
		}
		seen[key] = struct{}{}
		encoded, err := json.Marshal(item.Config)
		if err != nil {
			return nil, errors.New("渠道协议参数无效")
		}
		version := item.ConfigVersion
		if version <= 0 {
			version = 1
		}
		result = append(result, model.ChannelProtocolDefault{Capability: capability, Operation: operation, Adapter: adapter, ConfigJSON: string(encoded), ConfigVersion: version})
	}
	return result, nil
}

func importModelOperations(items []model.APIConfigTransferOperation, legacy model.ChannelModel) ([]model.ChannelModelOperation, error) {
	if len(items) == 0 {
		return legacyTransferOperations(legacy), nil
	}
	result := make([]model.ChannelModelOperation, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		capability, operation, mode := strings.TrimSpace(item.Capability), strings.TrimSpace(item.Operation), strings.TrimSpace(item.ProtocolMode)
		key := capability + "\x00" + operation
		if !validTransferOperation(capability, operation) || mode != model.ProtocolModeInherit && mode != model.ProtocolModeOverride {
			return nil, errors.New("模型操作合同无效")
		}
		if _, ok := seen[key]; ok {
			return nil, errors.New("模型操作合同重复")
		}
		seen[key] = struct{}{}
		adapter := strings.TrimSpace(item.Adapter)
		if mode == model.ProtocolModeOverride && adapter == "" || len([]rune(adapter)) > 50 {
			return nil, errors.New("模型覆盖协议无效")
		}
		encoded, err := json.Marshal(item.Config)
		if err != nil {
			return nil, errors.New("模型协议参数无效")
		}
		version := item.ConfigVersion
		if version <= 0 {
			version = 1
		}
		result = append(result, model.ChannelModelOperation{Capability: capability, Operation: operation, Enabled: item.Enabled, ProtocolMode: mode, Adapter: adapter, ConfigJSON: string(encoded), ConfigVersion: version, ContractKey: strings.TrimSpace(item.ContractKey)})
	}
	return result, nil
}

func defaultTransferProtocols() []model.ChannelProtocolDefault {
	return []model.ChannelProtocolDefault{
		{Capability: "image", Operation: "generate", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{Capability: "image", Operation: "edit", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{Capability: "video", Operation: "generate", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{Capability: "text", Operation: "generate", Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1},
		{Capability: "audio", Operation: "generate", Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1},
	}
}

func legacyTransferOperations(item model.ChannelModel) []model.ChannelModelOperation {
	capabilities := channelModelCapabilities(&item)
	result := make([]model.ChannelModelOperation, 0, len(capabilities)+1)
	for _, capability := range capabilities {
		adapters := []struct{ operation, adapter string }{{"generate", ""}}
		switch capability {
		case "image":
			adapters = []struct{ operation, adapter string }{{"generate", item.ImageGenerateRoute}, {"edit", item.ImageEditRoute}}
		case "video":
			adapters[0].adapter = item.VideoRoute
		}
		for _, adapter := range adapters {
			entry := model.ChannelModelOperation{Capability: capability, Operation: adapter.operation, Enabled: true, ProtocolMode: model.ProtocolModeInherit, ConfigJSON: "{}", ConfigVersion: 1}
			if adapter.adapter != "" && adapter.adapter != "auto" {
				entry.ProtocolMode, entry.Adapter = model.ProtocolModeOverride, adapter.adapter
			}
			result = append(result, entry)
		}
	}
	return result
}

func validTransferOperation(capability, operation string) bool {
	if capability == "image" {
		return operation == "generate" || operation == "edit"
	}
	return validAutoCapability(capability) && operation == "generate"
}

func transferPricingRuleKey(item model.APIConfigTransferPricingRule) string {
	return strings.Join([]string{strings.TrimSpace(item.PublicKey), strings.TrimSpace(item.Capability), strings.TrimSpace(item.Scope), item.ChannelRef, strings.TrimSpace(item.UpstreamModelID)}, "\x00")
}

func validateTransferPassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return errors.New("导入导出密码至少需要 8 个字符")
	}
	return nil
}

func indexTransferChannels(items []model.Channel) (map[string][]*model.Channel, map[string][]*model.Channel) {
	byKey := make(map[string][]*model.Channel)
	byName := make(map[string][]*model.Channel)
	for index := range items {
		item := &items[index]
		name, baseURL := normalizeTransferName(item.Name), canonicalTransferURL(item.BaseUrl)
		byKey[name+"\x00"+baseURL] = append(byKey[name+"\x00"+baseURL], item)
		byName[name] = append(byName[name], item)
	}
	return byKey, byName
}

func countSnapshotChannels(items []model.APIConfigTransferChannel) (map[string]int, map[string]int, map[string]int) {
	refs, names, keys := make(map[string]int), make(map[string]int), make(map[string]int)
	for _, item := range items {
		name, baseURL := normalizeTransferName(item.Name), canonicalTransferURL(item.BaseURL)
		refs[item.Ref]++
		names[name]++
		keys[name+"\x00"+baseURL]++
	}
	return refs, names, keys
}

func indexTransferModels(items []model.ChannelModel) map[string][]*model.ChannelModel {
	result := make(map[string][]*model.ChannelModel)
	for index := range items {
		item := &items[index]
		result[transferModelKey(item.ChannelID, item.ModelName)] = append(result[transferModelKey(item.ChannelID, item.ModelName)], item)
	}
	return result
}

func indexTransferGroups(items []model.ModelMergeGroup) map[string][]*model.ModelMergeGroup {
	result := make(map[string][]*model.ModelMergeGroup)
	for index := range items {
		item := &items[index]
		key := transferGroupKey(item.ChannelID, normalizeTransferName(item.GroupName))
		result[key] = append(result[key], item)
	}
	return result
}

func canonicalTransferURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

func normalizeTransferName(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func transferModelKey(channelID uint, name string) string {
	return fmt.Sprintf("%d\x00%s", channelID, strings.TrimSpace(name))
}
func transferGroupKey(channelID uint, name string) string {
	return fmt.Sprintf("%d\x00%s", channelID, name)
}
func transferPricingKey(name string, channelID uint) string {
	return fmt.Sprintf("%s\x00%d", strings.TrimSpace(name), channelID)
}
func transferChannelIdentifier(item *model.APIConfigTransferChannel) string {
	return strings.TrimSpace(item.Name) + " (" + strings.TrimSpace(item.BaseURL) + ")"
}
func transferConflict(resource, identifier, reason string) model.APIConfigTransferConflict {
	return model.APIConfigTransferConflict{Resource: resource, Identifier: identifier, Reason: reason}
}

func hasPositiveVideoRateForTransfer(items map[string]int) bool {
	for _, value := range items {
		if value > 0 {
			return true
		}
	}
	return false
}
