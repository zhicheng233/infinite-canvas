package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
)

const (
	legacyAPIConfigMigrationSource  = "tenant_api_config"
	legacyAPIConfigMigrationVersion = 2
)

type LegacyAPIConfigService struct{ db *gorm.DB }

func NewLegacyAPIConfigService(db *gorm.DB) *LegacyAPIConfigService {
	return &LegacyAPIConfigService{db: db}
}

func (s *LegacyAPIConfigService) MigrateAll() error {
	var configs []model.TenantApiConfig
	if err := s.db.Order("id ASC").Find(&configs).Error; err != nil {
		return err
	}
	for index := range configs {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			completed, err := legacyAPIConfigMigrationCompleted(tx, configs[index].ID)
			if err != nil || completed {
				return err
			}
			return syncLegacyAPIConfig(tx, &configs[index], 0)
		}); err != nil {
			if recordErr := s.recordMigrationFailure(&configs[index], err); recordErr != nil {
				return recordErr
			}
		}
	}
	return nil
}

func legacyAPIConfigMigrationCompleted(tx *gorm.DB, sourceID uint) (bool, error) {
	var migration model.ModelConfigMigration
	err := tx.Where("source = ? AND source_id = ? AND version = ?", legacyAPIConfigMigrationSource, sourceID, legacyAPIConfigMigrationVersion).First(&migration).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil || migration.Status != "completed" || migration.TargetID == 0 {
		return false, err
	}
	var count int64
	if err := tx.Unscoped().Model(&model.Channel{}).Where("id = ?", migration.TargetID).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 1, nil
}

func (s *LegacyAPIConfigService) Save(config *model.TenantApiConfig, actorUserID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.TenantApiConfig
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", config.TenantID).First(&existing).Error
		if err == nil {
			config.ID = existing.ID
			config.CreatedAt = existing.CreatedAt
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Save(config).Error; err != nil {
			return err
		}
		return syncLegacyAPIConfig(tx, config, actorUserID)
	})
}

func (s *LegacyAPIConfigService) recordMigrationFailure(config *model.TenantApiConfig, cause error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		migration := model.ModelConfigMigration{Source: legacyAPIConfigMigrationSource, SourceID: config.ID, Version: legacyAPIConfigMigrationVersion}
		if err := tx.Where("source = ? AND source_id = ? AND version = ?", migration.Source, migration.SourceID, migration.Version).FirstOrCreate(&migration).Error; err != nil {
			return err
		}
		if err := tx.Model(&migration).Updates(map[string]any{"status": "failed", "detail": cause.Error(), "completed_at": &now}).Error; err != nil {
			return err
		}
		issue := model.ModelConfigMigrationIssue{MigrationID: migration.ID, Resource: "tenant_api_config", Identifier: fmt.Sprintf("tenant:%d", config.TenantID), Reason: cause.Error()}
		return tx.Where("migration_id = ? AND resource = ? AND identifier = ?", issue.MigrationID, issue.Resource, issue.Identifier).Assign(map[string]any{"reason": issue.Reason, "resolved": false}).FirstOrCreate(&issue).Error
	})
}

func syncLegacyAPIConfig(tx *gorm.DB, config *model.TenantApiConfig, actorUserID uint) error {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseUrl), "/")
	if baseURL == "" || strings.TrimSpace(config.ApiKey) == "" {
		return errors.New("旧版 API 配置缺少 Base URL 或 API Key")
	}
	lists, routes, durations, customizable, err := decodeLegacyAPIConfig(config)
	if err != nil {
		return err
	}

	migration := model.ModelConfigMigration{Source: legacyAPIConfigMigrationSource, SourceID: config.ID, Version: legacyAPIConfigMigrationVersion}
	if err := tx.Where("source = ? AND source_id = ? AND version = ?", migration.Source, migration.SourceID, migration.Version).FirstOrCreate(&migration).Error; err != nil {
		return err
	}
	channel, err := resolveLegacyCompatibilityChannel(tx, config, &migration, baseURL)
	if err != nil {
		return err
	}
	if err := ensureLegacyProtocolDefaults(tx, channel.ID); err != nil {
		return err
	}

	allModels := append([]string{}, lists["all"]...)
	for _, capability := range []string{"image", "video", "text", "audio"} {
		allModels = append(allModels, lists[capability]...)
	}
	allModels = uniqueLegacyStrings(allModels)
	activeNames := make(map[string]struct{}, len(allModels))
	for index, name := range allModels {
		activeNames[name] = struct{}{}
		catalogID, err := ensureLegacyCatalogModel(tx, name)
		if err != nil {
			return err
		}
		capabilities := legacyCapabilities(name, lists)
		capabilitiesJSON, _ := json.Marshal(capabilities)
		videoDurationsJSON, _ := json.Marshal(durations[name])
		imageGenerateRoute := legacyRoute(routes, "image_generate", name)
		imageEditRoute := legacyRoute(routes, "image_edit", name)
		videoRoute := legacyRoute(routes, "video", name)
		item := model.ChannelModel{}
		err = tx.Unscoped().Where("channel_id = ? AND model_name = ?", channel.ID, name).First(&item).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		item.ChannelID, item.ModelName, item.CatalogModelID, item.UpstreamModelID = channel.ID, name, catalogID, name
		item.Status, item.DiscoveryStatus, item.Enabled, item.LegacyUnreviewed = model.ModelStatusActive, model.DiscoveryStatusPresent, true, true
		item.ConfigRevision = normalizedLegacyRevision(item.ConfigRevision) + boolToUint(item.ID > 0)
		item.Capabilities, item.ImageGenerateRoute, item.ImageEditRoute, item.VideoRoute = string(capabilitiesJSON), imageGenerateRoute, imageEditRoute, videoRoute
		item.VideoDurations, item.VideoCustomizable, item.SortOrder = string(videoDurationsJSON), customizable[name], index
		item.DeletedAt = gorm.DeletedAt{}
		if err := tx.Unscoped().Save(&item).Error; err != nil {
			return err
		}
		if err := replaceLegacyOperations(tx, &item, capabilities); err != nil {
			return err
		}
	}
	var stale []model.ChannelModel
	if err := tx.Where("channel_id = ?", channel.ID).Find(&stale).Error; err != nil {
		return err
	}
	for index := range stale {
		if _, ok := activeNames[stale[index].ModelName]; ok {
			continue
		}
		if err := tx.Model(&stale[index]).Updates(map[string]any{"enabled": false, "status": model.ModelStatusDisabled, "config_revision": normalizedLegacyRevision(stale[index].ConfigRevision) + 1}).Error; err != nil {
			return err
		}
	}

	now := time.Now()
	if err := tx.Where("migration_id = ?", migration.ID).Delete(&model.ModelConfigMigrationIssue{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&migration).Updates(map[string]any{"status": "completed", "target_id": channel.ID, "detail": "已迁移到兼容渠道", "completed_at": &now}).Error; err != nil {
		return err
	}
	if actorUserID > 0 {
		return tx.Create(&model.ModelConfigAuditLog{TenantID: config.TenantID, ActorUserID: actorUserID, Resource: "tenant_api_config", ResourceID: config.ID, Action: "legacy_dual_write", AfterJSON: fmt.Sprintf(`{"channel_id":%d}`, channel.ID)}).Error
	}
	return nil
}

func resolveLegacyCompatibilityChannel(tx *gorm.DB, config *model.TenantApiConfig, migration *model.ModelConfigMigration, baseURL string) (*model.Channel, error) {
	var channel model.Channel
	if migration.TargetID > 0 {
		if err := tx.Unscoped().First(&channel, migration.TargetID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	name := fmt.Sprintf("旧版 API（租户 %d）", config.TenantID)
	if channel.ID == 0 {
		if err := tx.Unscoped().Where("name = ? AND base_url = ?", name, baseURL).First(&channel).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	channel.Name, channel.BaseUrl, channel.ApiKey, channel.Enabled = name, baseURL, config.ApiKey, true
	channel.VideoAPIStandard, channel.ConfigRevision, channel.DeletedAt = model.VideoAPIStandardDefault, normalizedLegacyRevision(channel.ConfigRevision)+boolToUint(channel.ID > 0), gorm.DeletedAt{}
	channel.Remark = "由旧版 API 配置兼容同步，请在模型服务中复核"
	if err := tx.Unscoped().Save(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func ensureLegacyProtocolDefaults(tx *gorm.DB, channelID uint) error {
	defaults := []model.ChannelProtocolDefault{
		{ChannelID: channelID, Capability: "image", Operation: "generate", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{ChannelID: channelID, Capability: "image", Operation: "edit", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{ChannelID: channelID, Capability: "video", Operation: "generate", Adapter: "auto", ConfigJSON: "{}", ConfigVersion: 1},
		{ChannelID: channelID, Capability: "text", Operation: "generate", Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1},
		{ChannelID: channelID, Capability: "audio", Operation: "generate", Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1},
	}
	for index := range defaults {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults[index]).Error; err != nil {
			return err
		}
	}
	return nil
}

func replaceLegacyOperations(tx *gorm.DB, item *model.ChannelModel, capabilities []string) error {
	if err := tx.Unscoped().Where("channel_model_id = ?", item.ID).Delete(&model.ChannelModelOperation{}).Error; err != nil {
		return err
	}
	operations := make([]model.ChannelModelOperation, 0, len(capabilities)+1)
	for _, capability := range capabilities {
		operation := "generate"
		adapter := ""
		mode := model.ProtocolModeInherit
		configJSON := "{}"
		switch capability {
		case "image":
			for _, route := range []struct{ operation, adapter string }{{"generate", item.ImageGenerateRoute}, {"edit", item.ImageEditRoute}} {
				entry := model.ChannelModelOperation{ChannelModelID: item.ID, Capability: capability, Operation: route.operation, Enabled: true, ProtocolMode: mode, ConfigJSON: configJSON, ConfigVersion: 1}
				if route.adapter != "auto" {
					entry.ProtocolMode, entry.Adapter = model.ProtocolModeOverride, route.adapter
				}
				operations = append(operations, entry)
			}
			continue
		case "video":
			adapter = item.VideoRoute
			videoConfig := map[string]any{"durations": decodeLegacyIntSlice(item.VideoDurations), "customizable": item.VideoCustomizable}
			encoded, _ := json.Marshal(videoConfig)
			configJSON = string(encoded)
		case "text", "audio":
			adapter = ""
		}
		entry := model.ChannelModelOperation{ChannelModelID: item.ID, Capability: capability, Operation: operation, Enabled: true, ProtocolMode: mode, ConfigJSON: configJSON, ConfigVersion: 1}
		if adapter != "" && adapter != "auto" {
			entry.ProtocolMode, entry.Adapter = model.ProtocolModeOverride, adapter
		}
		operations = append(operations, entry)
	}
	for index := range operations {
		if err := tx.Create(&operations[index]).Error; err != nil {
			return err
		}
	}
	return nil
}

func decodeLegacyAPIConfig(config *model.TenantApiConfig) (map[string][]string, map[string]string, map[string][]int, map[string]bool, error) {
	lists := map[string][]string{}
	for key, raw := range map[string]string{"all": config.Models, "image": config.ImageModels, "video": config.VideoModels, "text": config.TextModels, "audio": config.AudioModels} {
		var values []string
		if err := decodeLegacyJSON(raw, &values); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("旧版 %s 模型列表无效: %w", key, err)
		}
		lists[key] = uniqueLegacyStrings(values)
	}
	routes := map[string]string{}
	durations := map[string][]int{}
	customizable := map[string]bool{}
	if err := decodeLegacyJSON(config.ModelRoutes, &routes); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("旧版模型路由无效: %w", err)
	}
	if err := decodeLegacyJSON(config.ModelVideoDurations, &durations); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("旧版视频时长无效: %w", err)
	}
	if err := decodeLegacyJSON(config.ModelVideoCustomizable, &customizable); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("旧版视频配置无效: %w", err)
	}
	for key, values := range durations {
		clean := make([]int, 0, len(values))
		seen := map[int]struct{}{}
		for _, value := range values {
			if value <= 0 {
				continue
			}
			if _, ok := seen[value]; !ok {
				seen[value], clean = struct{}{}, append(clean, value)
			}
		}
		sort.Ints(clean)
		durations[strings.TrimSpace(key)] = clean
	}
	return lists, routes, durations, customizable, nil
}

func decodeLegacyJSON(raw string, output any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), output)
}

func legacyCapabilities(name string, lists map[string][]string) []string {
	capabilities := make([]string, 0, 4)
	for _, capability := range []string{"image", "video", "text", "audio"} {
		for _, item := range lists[capability] {
			if item == name {
				capabilities = append(capabilities, capability)
				break
			}
		}
	}
	if len(capabilities) == 0 {
		capabilities = append(capabilities, inferLegacyCapability(name))
	}
	return capabilities
}

func legacyRoute(routes map[string]string, capability, name string) string {
	keys := []string{capability + ":" + name}
	if strings.HasPrefix(capability, "image_") {
		keys = append(keys, "image:"+name)
	}
	if capability == "video" {
		keys = append(keys, name)
	}
	for _, key := range keys {
		value := strings.TrimSpace(routes[key])
		if value == "" || value == "auto" {
			continue
		}
		if capability == "video" {
			if normalized, err := model.NormalizeVideoRoute(value); err == nil {
				return normalized
			}
		} else if capability == "image_generate" {
			if normalized, err := model.NormalizeImageGenerateRoute(value); err == nil {
				return normalized
			}
		} else if normalized, err := model.NormalizeImageEditRoute(value); err == nil {
			return normalized
		}
	}
	return "auto"
}

func ensureLegacyCatalogModel(tx *gorm.DB, publicKey string) (uint, error) {
	var catalog model.CatalogModel
	err := tx.Where("public_key = ?", publicKey).First(&catalog).Error
	if err == nil {
		return catalog.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	catalog = model.CatalogModel{PublicKey: publicKey, DisplayName: publicKey}
	if err := tx.Create(&catalog).Error; err != nil {
		return 0, err
	}
	return catalog.ID, nil
}

func uniqueLegacyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > 191 {
			continue
		}
		if _, ok := seen[value]; !ok {
			seen[value], result = struct{}{}, append(result, value)
		}
	}
	return result
}

func inferLegacyCapability(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "video") || strings.Contains(lower, "veo") || strings.Contains(lower, "seedance") || strings.Contains(lower, "sora") {
		return "video"
	}
	if strings.Contains(lower, "image") || strings.Contains(lower, "banana") || strings.Contains(lower, "flux") {
		return "image"
	}
	if strings.Contains(lower, "tts") || strings.Contains(lower, "audio") || strings.Contains(lower, "speech") {
		return "audio"
	}
	return "text"
}

func decodeLegacyIntSlice(raw string) []int {
	var values []int
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func normalizedLegacyRevision(value uint) uint {
	if value == 0 {
		return 1
	}
	return value
}

func boolToUint(value bool) uint {
	if value {
		return 1
	}
	return 0
}
