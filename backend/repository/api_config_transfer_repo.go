package repository

import (
	"errors"
	"fmt"
	"strings"

	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type APIConfigTransferData struct {
	Channels         []model.Channel
	Catalogs         []model.CatalogModel
	Models           []model.ChannelModel
	Operations       []model.ChannelModelOperation
	ProtocolDefaults []model.ChannelProtocolDefault
	PricingRules     []model.ModelPricingRule
	Pricing          []model.CreditPricing
	MergeGroups      []model.ModelMergeGroup
	VideoPresets     []model.VideoConfigPreset
	AutoRoutingPools []model.AutoRoutingPool
}

type APIConfigTransferChannelOperation struct {
	Ref        string
	ExistingID uint
	Item       model.Channel
	Defaults   []model.ChannelProtocolDefault
}

type APIConfigTransferModelOperation struct {
	ChannelRef  string
	ExistingID  uint
	Item        model.ChannelModel
	PublicKey   string
	DisplayName string
	Operations  []model.ChannelModelOperation
}

type APIConfigTransferPricingRuleOperation struct {
	ChannelRef      string
	UpstreamModelID string
	PublicKey       string
	Item            model.ModelPricingRule
}

type APIConfigTransferPricingOperation struct {
	ChannelRef string
	Item       model.CreditPricing
}

type APIConfigTransferMergeGroupOperation struct {
	ChannelRef string
	ExistingID uint
	Item       model.ModelMergeGroup
}

type APIConfigTransferPresetOperation struct {
	ExistingID uint
	Item       model.VideoConfigPreset
}

type APIConfigTransferAutoRoutingMemberOperation struct {
	ChannelRef string
	Model      string
	Priority   int
	Enabled    bool
}

type APIConfigTransferAutoRoutingPoolOperation struct {
	ExistingID uint
	Item       model.AutoRoutingPool
	Members    []APIConfigTransferAutoRoutingMemberOperation
}

type APIConfigTransferApplyPlan struct {
	SchemaVersion    int
	Channels         []APIConfigTransferChannelOperation
	Models           []APIConfigTransferModelOperation
	PricingRules     []APIConfigTransferPricingRuleOperation
	Pricing          []APIConfigTransferPricingOperation
	MergeGroups      []APIConfigTransferMergeGroupOperation
	VideoPresets     []APIConfigTransferPresetOperation
	AutoRoutingPools []APIConfigTransferAutoRoutingPoolOperation
}

type APIConfigTransferRepo struct {
	db *gorm.DB
}

func NewAPIConfigTransferRepo(db *gorm.DB) *APIConfigTransferRepo {
	return &APIConfigTransferRepo{db: db}
}

func (r *APIConfigTransferRepo) Load(tenantID uint) (*APIConfigTransferData, error) {
	data := &APIConfigTransferData{}
	if err := r.db.Order("id ASC").Find(&data.Channels).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("id ASC").Find(&data.Catalogs).Error; err != nil {
		return nil, err
	}
	if err := r.db.Unscoped().Order("channel_id ASC, sort_order ASC, id ASC").Find(&data.Models).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("channel_model_id ASC, capability ASC, operation ASC").Find(&data.Operations).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("channel_id ASC, capability ASC, operation ASC").Find(&data.ProtocolDefaults).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("tenant_id = ?", tenantID).Order("catalog_model_id ASC, capability ASC, scope ASC, scope_id ASC").Find(&data.PricingRules).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("tenant_id = ?", tenantID).Order("model ASC, channel_id ASC").Find(&data.Pricing).Error; err != nil {
		return nil, err
	}
	if err := r.db.Order("channel_id ASC, id ASC").Find(&data.MergeGroups).Error; err != nil {
		return nil, err
	}
	if err := r.db.Where("tenant_id = ?", tenantID).Order("normalized_name ASC, id ASC").Find(&data.VideoPresets).Error; err != nil {
		return nil, err
	}
	if err := r.db.Preload("Members", func(db *gorm.DB) *gorm.DB { return db.Order("priority DESC, id ASC") }).Order("public_model_name ASC, capability ASC").Find(&data.AutoRoutingPools).Error; err != nil {
		return nil, err
	}
	return data, nil
}

func (r *APIConfigTransferRepo) Apply(plan *APIConfigTransferApplyPlan) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		channelIDs := make(map[string]uint, len(plan.Channels))
		channelModelIDs := make(map[string]uint, len(plan.Models))
		for _, operation := range plan.Channels {
			item := operation.Item
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
			}
			channelIDs[operation.Ref] = item.ID
			if len(operation.Defaults) > 0 {
				if err := tx.Unscoped().Where("channel_id = ?", item.ID).Delete(&model.ChannelProtocolDefault{}).Error; err != nil {
					return err
				}
				for index := range operation.Defaults {
					entry := operation.Defaults[index]
					entry.ChannelID = item.ID
					if err := tx.Create(&entry).Error; err != nil {
						return err
					}
				}
			}
		}

		for _, operation := range plan.Models {
			item := operation.Item
			channelID, ok := channelIDs[operation.ChannelRef]
			if !ok {
				return fmt.Errorf("missing channel reference %q", operation.ChannelRef)
			}
			item.ChannelID = channelID
			publicKey := operation.PublicKey
			if publicKey == "" {
				publicKey = item.ModelName
			}
			catalogID, err := ensureTransferCatalog(tx, publicKey, operation.DisplayName)
			if err != nil {
				return err
			}
			item.CatalogModelID = catalogID
			if item.UpstreamModelID == "" {
				item.UpstreamModelID = item.ModelName
			}
			if item.Status == "" {
				if item.Enabled {
					item.Status = model.ModelStatusActive
				} else {
					item.Status = model.ModelStatusDisabled
				}
			}
			if item.DiscoveryStatus == "" {
				item.DiscoveryStatus = model.DiscoveryStatusPresent
			}
			if item.ConfigRevision == 0 {
				item.ConfigRevision = 1
			}
			item.DeletedAt = gorm.DeletedAt{}
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Unscoped().Save(&item).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
			}
			channelModelIDs[transferImportedModelKey(operation.ChannelRef, item.UpstreamModelID)] = item.ID
			if len(operation.Operations) > 0 {
				if err := tx.Unscoped().Where("channel_model_id = ?", item.ID).Delete(&model.ChannelModelOperation{}).Error; err != nil {
					return err
				}
				for index := range operation.Operations {
					entry := operation.Operations[index]
					entry.ChannelModelID = item.ID
					if err := tx.Create(&entry).Error; err != nil {
						return err
					}
				}
			}
		}

		for _, operation := range plan.Pricing {
			item := operation.Item
			if operation.ChannelRef != "" {
				channelID, ok := channelIDs[operation.ChannelRef]
				if !ok {
					return fmt.Errorf("missing channel reference %q", operation.ChannelRef)
				}
				item.ChannelID = channelID
			}
			if err := upsertLegacyPricingProjection(tx, item); err != nil {
				return err
			}
			if err := upsertLegacyTransferPricingRule(tx, item); err != nil {
				return err
			}
		}

		for _, operation := range plan.PricingRules {
			item := operation.Item
			catalogID, err := ensureTransferCatalog(tx, operation.PublicKey, operation.PublicKey)
			if err != nil {
				return err
			}
			item.CatalogModelID = catalogID
			if item.Scope == model.PricingScopeImplementation {
				modelID := channelModelIDs[transferImportedModelKey(operation.ChannelRef, operation.UpstreamModelID)]
				if modelID == 0 {
					return fmt.Errorf("missing channel model reference %q/%q", operation.ChannelRef, operation.UpstreamModelID)
				}
				item.ScopeID = modelID
			} else {
				item.Scope, item.ScopeID = model.PricingScopeDefault, 0
			}
			var existing model.ModelPricingRule
			err = tx.Where("tenant_id = ? AND catalog_model_id = ? AND capability = ? AND scope = ? AND scope_id = ?", item.TenantID, item.CatalogModelID, item.Capability, item.Scope, item.ScopeID).First(&existing).Error
			if err == nil {
				item.ID, item.CreatedAt = existing.ID, existing.CreatedAt
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if item.ConfigRevision == 0 {
				item.ConfigRevision = 1
			}
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
			shadowModel, shadowChannelID := operation.PublicKey, uint(0)
			if item.Scope == model.PricingScopeImplementation {
				shadowModel = operation.UpstreamModelID
				shadowChannelID = channelIDs[operation.ChannelRef]
			}
			shadow := model.CreditPricing{TenantID: item.TenantID, ChannelID: shadowChannelID, Model: shadowModel, CreditsPerUnit: item.CreditsPerUnit, UnitType: item.UnitType, PricingMode: item.PricingMode, PricingRule: item.PricingRule}
			if err := upsertLegacyPricingProjection(tx, shadow); err != nil {
				return err
			}
		}

		for _, operation := range plan.MergeGroups {
			item := operation.Item
			channelID, ok := channelIDs[operation.ChannelRef]
			if !ok {
				return fmt.Errorf("missing channel reference %q", operation.ChannelRef)
			}
			item.ChannelID = channelID
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		for _, operation := range plan.VideoPresets {
			item := operation.Item
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		for _, operation := range plan.AutoRoutingPools {
			item := operation.Item
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Save(&item).Error; err != nil {
					return err
				}
				if err := tx.Unscoped().Where("pool_id = ?", item.ID).Delete(&model.AutoRoutingPoolMember{}).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
			}
			for _, member := range operation.Members {
				channelID, ok := channelIDs[member.ChannelRef]
				if !ok {
					return fmt.Errorf("missing channel reference %q", member.ChannelRef)
				}
				var channelModel model.ChannelModel
				if err := tx.Where("channel_id = ? AND model_name = ?", channelID, member.Model).First(&channelModel).Error; err != nil {
					return err
				}
				if err := tx.Create(&model.AutoRoutingPoolMember{PoolID: item.ID, ChannelModelID: channelModel.ID, Priority: member.Priority, Enabled: member.Enabled}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func ensureTransferCatalog(tx *gorm.DB, publicKey, displayName string) (uint, error) {
	publicKey = strings.TrimSpace(publicKey)
	if displayName = strings.TrimSpace(displayName); displayName == "" {
		displayName = publicKey
	}
	var item model.CatalogModel
	err := tx.Where("public_key = ?", publicKey).First(&item).Error
	if err == nil {
		if item.DisplayName != displayName {
			err = tx.Model(&item).Update("display_name", displayName).Error
		}
		return item.ID, err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	item = model.CatalogModel{PublicKey: publicKey, DisplayName: displayName}
	if err := tx.Create(&item).Error; err != nil {
		return 0, err
	}
	return item.ID, nil
}

func upsertLegacyTransferPricingRule(tx *gorm.DB, pricing model.CreditPricing) error {
	var catalog model.CatalogModel
	if err := tx.Where("public_key = ?", pricing.Model).First(&catalog).Error; err != nil {
		return nil
	}
	capability := "text"
	if pricing.PricingMode == model.PricingModeVideoDynamic || pricing.UnitType == model.UnitPerVideo || pricing.UnitType == model.UnitPerVideoSecond {
		capability = "video"
	} else if pricing.UnitType == model.UnitPerImage {
		capability = "image"
	}
	scope, scopeID := model.PricingScopeDefault, uint(0)
	if pricing.ChannelID > 0 {
		var channelModel model.ChannelModel
		if err := tx.Where("channel_id = ? AND catalog_model_id = ?", pricing.ChannelID, catalog.ID).First(&channelModel).Error; err != nil {
			return nil
		}
		scope, scopeID = model.PricingScopeImplementation, channelModel.ID
	}
	item := model.ModelPricingRule{TenantID: pricing.TenantID, CatalogModelID: catalog.ID, Capability: capability, Scope: scope, ScopeID: scopeID, CreditsPerUnit: pricing.CreditsPerUnit, UnitType: pricing.UnitType, PricingMode: pricing.PricingMode, PricingRule: pricing.PricingRule, ConfigRevision: 1}
	var existing model.ModelPricingRule
	err := tx.Unscoped().Where("tenant_id = ? AND catalog_model_id = ? AND capability = ? AND scope = ? AND scope_id = ?", item.TenantID, item.CatalogModelID, item.Capability, item.Scope, item.ScopeID).First(&existing).Error
	if err == nil {
		item.ID, item.CreatedAt = existing.ID, existing.CreatedAt
		item.DeletedAt = gorm.DeletedAt{}
		return tx.Unscoped().Save(&item).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&item).Error
}

func transferImportedModelKey(channelRef, upstreamModelID string) string {
	return channelRef + "\x00" + strings.TrimSpace(upstreamModelID)
}
