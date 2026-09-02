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
	snapshot := &model.APIConfigTransferSnapshot{SchemaVersion: 1, ExportedAt: time.Now().UTC(), Channels: make([]model.APIConfigTransferChannel, 0, len(data.Channels)), Pricing: []model.APIConfigTransferPricing{}, VideoPresets: []model.APIConfigTransferVideoPreset{}, AutoRoutingPools: []model.APIConfigTransferAutoRoutingPool{}}
	stats := model.APIConfigTransferStats{}
	warnings := make([]model.APIConfigTransferConflict, 0)
	refs := make(map[uint]string, len(data.Channels))
	channelsByID := make(map[uint]*model.Channel, len(data.Channels))
	channelIndexes := make(map[uint]int, len(data.Channels))
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
			MetricsBaseURL: item.MetricsBaseUrl, Remark: item.Remark, Models: []model.APIConfigTransferModel{}, MergeGroups: []model.APIConfigTransferMergeGroup{},
		})
		stats.Channels.Create++
	}
	modelsByID := make(map[uint]*model.ChannelModel, len(data.Models))
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
		snapshot.Channels[channelIndex].Models = append(snapshot.Channels[channelIndex].Models, model.APIConfigTransferModel{
			ModelName: info.ModelName, Capabilities: info.Capabilities, Enabled: info.Enabled,
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
	plan := &repository.APIConfigTransferApplyPlan{}
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
		operation := repository.APIConfigTransferChannelOperation{Ref: input.Ref, ExistingID: existingID, Item: item}
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
			operation := repository.APIConfigTransferModelOperation{ChannelRef: channel.Ref, Item: item}
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

	s.addPricingPlan(tenantID, snapshot, data, states, plan, result)
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
	name := strings.TrimSpace(input.ModelName)
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
	item := model.ChannelModel{ModelName: name, Capabilities: string(capabilities), Enabled: input.Enabled, ImageGenerateRoute: imageGenerateRoute, ImageEditRoute: imageEditRoute, VideoRoute: videoRoute, VideoDurations: string(durations), VideoCustomizable: input.VideoCustomizable, SortOrder: input.SortOrder}
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
	if snapshot.SchemaVersion != 1 {
		return nil, errors.New("不支持的配置内容版本")
	}
	return &snapshot, nil
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
