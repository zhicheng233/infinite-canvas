package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
)

var ErrModelConfigRevisionConflict = errors.New("model config revision conflict")

type ModelConfigData struct {
	Channels   []model.Channel
	Catalogs   []model.CatalogModel
	Models     []model.ChannelModel
	Operations []model.ChannelModelOperation
	Defaults   []model.ChannelProtocolDefault
	Pricing    []model.ModelPricingRule
}

type SaveModelConfigParams struct {
	TenantID          uint
	ActorUserID       uint
	ModelID           uint
	ExpectedRevision  uint
	PublicKey         string
	DisplayName       string
	UpstreamModelID   string
	Status            string
	SortOrder         int
	LegacyUnreviewed  bool
	Capabilities      string
	ImageGenerate     string
	ImageEdit         string
	VideoRoute        string
	VideoDurations    string
	VideoCustomizable bool
	VideoCustomConfig string
	Operations        []model.ChannelModelOperation
	Pricing           []model.ModelPricingRule
	BeforeJSON        string
	AfterJSON         string
}

type ModelConfigRepo struct{ db *gorm.DB }

func NewModelConfigRepo(db *gorm.DB) *ModelConfigRepo { return &ModelConfigRepo{db: db} }

func (r *ModelConfigRepo) RecordAudit(item *model.ModelConfigAuditLog) error {
	return r.db.Create(item).Error
}

func (r *ModelConfigRepo) Load(tenantID uint, includeArchived bool) (*ModelConfigData, error) {
	data := &ModelConfigData{}
	channelQuery := r.db
	modelQuery := r.db
	if includeArchived {
		channelQuery = channelQuery.Unscoped()
		modelQuery = modelQuery.Unscoped()
	}
	if err := channelQuery.Order("id ASC").Find(&data.Channels).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("public_key ASC, id ASC").Find(&data.Catalogs).Error; err != nil {
		return nil, err
	}
	if err := modelQuery.Order("channel_id ASC, sort_order ASC, id ASC").Find(&data.Models).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("channel_model_id ASC, capability ASC, operation ASC").Find(&data.Operations).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("channel_id ASC, capability ASC, operation ASC").Find(&data.Defaults).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("tenant_id = ?", tenantID).Order("catalog_model_id ASC, capability ASC, scope ASC, scope_id ASC").Find(&data.Pricing).Error; err != nil {
		return nil, err
	}
	return data, nil
}

func (r *ModelConfigRepo) FindModel(id uint) (*model.ChannelModel, error) {
	var item model.ChannelModel
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ModelConfigRepo) SaveModelConfig(input SaveModelConfigParams) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current model.ChannelModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, input.ModelID).Error; err != nil {
			return err
		}
		if current.ConfigRevision != input.ExpectedRevision {
			return ErrModelConfigRevisionConflict
		}
		var catalog model.CatalogModel
		err := tx.Where("public_key = ?", input.PublicKey).First(&catalog).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			catalog = model.CatalogModel{PublicKey: input.PublicKey, DisplayName: input.DisplayName}
			if err := tx.Create(&catalog).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if catalog.DisplayName != input.DisplayName {
			if err := tx.Model(&catalog).Update("display_name", input.DisplayName).Error; err != nil {
				return err
			}
		}

		updates := map[string]any{
			"catalog_model_id": catalog.ID, "model_name": input.UpstreamModelID, "upstream_model_id": input.UpstreamModelID,
			"status": input.Status, "enabled": input.Status == model.ModelStatusActive,
			"sort_order": input.SortOrder, "capabilities": input.Capabilities,
			"image_generate_route": input.ImageGenerate, "image_edit_route": input.ImageEdit,
			"video_route": input.VideoRoute, "video_durations": input.VideoDurations,
			"video_customizable": input.VideoCustomizable, "video_custom_config": input.VideoCustomConfig,
			"legacy_unreviewed": input.LegacyUnreviewed, "config_revision": current.ConfigRevision + 1,
		}
		result := tx.Model(&model.ChannelModel{}).Where("id = ? AND config_revision = ?", current.ID, current.ConfigRevision).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrModelConfigRevisionConflict
		}

		if err := tx.Unscoped().Where("channel_model_id = ?", current.ID).Delete(&model.ChannelModelOperation{}).Error; err != nil {
			return err
		}
		for index := range input.Operations {
			input.Operations[index].ChannelModelID = current.ID
			if err := tx.Create(&input.Operations[index]).Error; err != nil {
				return err
			}
		}

		if err := tx.Unscoped().Where("tenant_id = ? AND scope = ? AND scope_id = ?", input.TenantID, model.PricingScopeImplementation, current.ID).Delete(&model.ModelPricingRule{}).Error; err != nil {
			return err
		}
		oldNames := uniqueModelNames([]string{current.ModelName, current.UpstreamModelID})
		if len(oldNames) > 0 {
			if err := tx.Unscoped().Where("tenant_id = ? AND channel_id = ? AND model IN ? AND model NOT IN ?", input.TenantID, current.ChannelID, oldNames, []string{input.UpstreamModelID}).Delete(&model.CreditPricing{}).Error; err != nil {
				return err
			}
		}
		for index := range input.Pricing {
			input.Pricing[index].TenantID = input.TenantID
			input.Pricing[index].CatalogModelID = catalog.ID
			input.Pricing[index].Scope = model.PricingScopeImplementation
			input.Pricing[index].ScopeID = current.ID
			input.Pricing[index].ConfigRevision = current.ConfigRevision + 1
			if err := tx.Create(&input.Pricing[index]).Error; err != nil {
				return err
			}
		}
		if len(input.Pricing) > 0 {
			pricing := input.Pricing[0]
			shadow := model.CreditPricing{TenantID: input.TenantID, ChannelID: current.ChannelID, Model: input.UpstreamModelID, CreditsPerUnit: pricing.CreditsPerUnit, UnitType: pricing.UnitType, PricingMode: pricing.PricingMode, PricingRule: pricing.PricingRule}
			if err := upsertLegacyPricingProjection(tx, shadow); err != nil {
				return err
			}
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: input.TenantID, ActorUserID: input.ActorUserID, Resource: "channel_model", ResourceID: current.ID, Action: "update", BeforeJSON: input.BeforeJSON, AfterJSON: input.AfterJSON}).Error
	})
}

func (r *ModelConfigRepo) SaveDefaultPricing(tenantID, actorUserID uint, catalogModelID uint, capability string, pricing model.ModelPricingRule) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current model.ModelPricingRule
		err := tx.Where("tenant_id = ? AND catalog_model_id = ? AND capability = ? AND scope = ? AND scope_id = 0", tenantID, catalogModelID, capability, model.PricingScopeDefault).First(&current).Error
		if err == nil {
			pricing.ID = current.ID
			pricing.ConfigRevision = current.ConfigRevision + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			pricing.ConfigRevision = 1
		}
		pricing.TenantID, pricing.CatalogModelID, pricing.Capability = tenantID, catalogModelID, capability
		pricing.Scope, pricing.ScopeID = model.PricingScopeDefault, 0
		if err := tx.Save(&pricing).Error; err != nil {
			return err
		}
		var catalog model.CatalogModel
		if err := tx.First(&catalog, catalogModelID).Error; err != nil {
			return err
		}
		shadowNames := []string{catalog.PublicKey}
		var implementations []model.ChannelModel
		if err := tx.Where("catalog_model_id = ?", catalogModelID).Find(&implementations).Error; err != nil {
			return err
		}
		for _, implementation := range implementations {
			name := implementation.UpstreamModelID
			if name == "" {
				name = implementation.ModelName
			}
			shadowNames = append(shadowNames, name)
		}
		for _, name := range uniqueModelNames(shadowNames) {
			shadow := model.CreditPricing{TenantID: tenantID, ChannelID: 0, Model: name, CreditsPerUnit: pricing.CreditsPerUnit, UnitType: pricing.UnitType, PricingMode: pricing.PricingMode, PricingRule: pricing.PricingRule}
			if err := upsertLegacyPricingProjection(tx, shadow); err != nil {
				return err
			}
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "model_pricing", ResourceID: pricing.ID, Action: "save_default"}).Error
	})
}

func (r *ModelConfigRepo) SaveChannelDefaults(tenantID, actorUserID, channelID, expectedRevision uint, defaults []model.ChannelProtocolDefault) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Channel{}).Where("id = ? AND config_revision = ?", channelID, expectedRevision).Update("config_revision", expectedRevision+1)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrModelConfigRevisionConflict
		}
		if err := tx.Unscoped().Where("channel_id = ?", channelID).Delete(&model.ChannelProtocolDefault{}).Error; err != nil {
			return err
		}
		for index := range defaults {
			defaults[index].ChannelID = channelID
			if err := tx.Create(&defaults[index]).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel", ResourceID: channelID, Action: "update_protocol_defaults"}).Error
	})
}

func (r *ModelConfigRepo) SetChannelArchived(tenantID, actorUserID, channelID uint, archived bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var channel model.Channel
		if err := tx.Unscoped().First(&channel, channelID).Error; err != nil {
			return err
		}
		if archived {
			if err := tx.Delete(&channel).Error; err != nil {
				return err
			}
		} else if err := tx.Unscoped().Model(&channel).Updates(map[string]any{"deleted_at": nil, "enabled": false, "config_revision": channel.ConfigRevision + 1}).Error; err != nil {
			return err
		}
		action := "restore"
		if archived {
			action = "archive"
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel", ResourceID: channelID, Action: action}).Error
	})
}

func (r *ModelConfigRepo) SetModelArchived(tenantID, actorUserID, modelID uint, archived bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var item model.ChannelModel
		if err := tx.Unscoped().First(&item, modelID).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"enabled":         false,
			"status":          model.ModelStatusDisabled,
			"config_revision": normalizedModelConfigRevision(item.ConfigRevision) + 1,
		}
		if archived {
			updates["deleted_at"] = time.Now()
		} else {
			updates["deleted_at"] = nil
		}
		if err := tx.Unscoped().Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		action := "restore"
		if archived {
			action = "archive"
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel_model", ResourceID: modelID, Action: action}).Error
	})
}

func (r *ModelConfigRepo) SetChannelSyncState(channelID uint, status, message string, syncedAt *time.Time) error {
	updates := map[string]any{"sync_status": status, "sync_error": message}
	if syncedAt != nil {
		updates["synced_at"] = syncedAt
	}
	return r.db.Model(&model.Channel{}).Where("id = ?", channelID).Updates(updates).Error
}

func (r *ModelConfigRepo) ApplyDiscovery(tenantID, actorUserID, channelID uint, names []string, discoveredAt time.Time) (*model.ChannelSyncReport, error) {
	report := &model.ChannelSyncReport{Discovered: len(names), ModelIDs: make([]uint, 0, len(names))}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing []model.ChannelModel
		if err := tx.Unscoped().Where("channel_id = ?", channelID).Find(&existing).Error; err != nil {
			return err
		}
		byName := make(map[string]*model.ChannelModel, len(existing))
		for index := range existing {
			byName[existing[index].ModelName] = &existing[index]
		}
		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			seen[name] = struct{}{}
			item := byName[name]
			if item != nil {
				restored := item.DeletedAt.Valid
				updates := map[string]any{"deleted_at": nil, "discovery_status": model.DiscoveryStatusPresent, "last_discovered_at": discoveredAt}
				if item.UpstreamModelID == "" {
					updates["upstream_model_id"] = name
				}
				if item.CatalogModelID == 0 {
					catalogID, err := ensureCatalogModel(tx, name)
					if err != nil {
						return err
					}
					updates["catalog_model_id"] = catalogID
				}
				if err := tx.Unscoped().Model(item).Updates(updates).Error; err != nil {
					return err
				}
				if restored {
					report.Restored++
				} else {
					report.Unchanged++
				}
				report.ModelIDs = append(report.ModelIDs, item.ID)
				continue
			}
			catalogID, err := ensureCatalogModel(tx, name)
			if err != nil {
				return err
			}
			created := model.ChannelModel{ChannelID: channelID, ModelName: name, CatalogModelID: catalogID, UpstreamModelID: name, Status: model.ModelStatusDraft, DiscoveryStatus: model.DiscoveryStatusPresent, LastDiscoveredAt: &discoveredAt, ConfigRevision: 1, Capabilities: "[]", Enabled: false, ImageGenerateRoute: model.ImageRouteAuto, ImageEditRoute: model.ImageRouteAuto, VideoRoute: "auto", VideoDurations: "[]"}
			if err := tx.Select("*").Create(&created).Error; err != nil {
				return err
			}
			if err := tx.Model(&created).Update("enabled", false).Error; err != nil {
				return err
			}
			report.Created++
			report.ModelIDs = append(report.ModelIDs, created.ID)
		}
		for index := range existing {
			if _, ok := seen[existing[index].ModelName]; ok || existing[index].DeletedAt.Valid {
				continue
			}
			if existing[index].DiscoveryStatus != model.DiscoveryStatusMissing {
				report.Missing++
			}
			if err := tx.Model(&existing[index]).Update("discovery_status", model.DiscoveryStatusMissing).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Channel{}).Where("id = ?", channelID).Updates(map[string]any{"sync_status": "success", "sync_error": "", "synced_at": discoveredAt}).Error; err != nil {
			return err
		}
		return tx.Create(&model.ModelConfigAuditLog{TenantID: tenantID, ActorUserID: actorUserID, Resource: "channel", ResourceID: channelID, Action: "sync_models"}).Error
	})
	return report, err
}

func ensureCatalogModel(tx *gorm.DB, publicKey string) (uint, error) {
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

func normalizedModelConfigRevision(value uint) uint {
	if value == 0 {
		return 1
	}
	return value
}

func uniqueModelNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
