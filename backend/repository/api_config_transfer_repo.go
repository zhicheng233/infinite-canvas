package repository

import (
	"errors"
	"fmt"

	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type APIConfigTransferData struct {
	Channels     []model.Channel
	Models       []model.ChannelModel
	Pricing      []model.CreditPricing
	MergeGroups  []model.ModelMergeGroup
	VideoPresets []model.VideoConfigPreset
}

type APIConfigTransferChannelOperation struct {
	Ref        string
	ExistingID uint
	Item       model.Channel
}

type APIConfigTransferModelOperation struct {
	ChannelRef string
	ExistingID uint
	Item       model.ChannelModel
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

type APIConfigTransferApplyPlan struct {
	Channels     []APIConfigTransferChannelOperation
	Models       []APIConfigTransferModelOperation
	Pricing      []APIConfigTransferPricingOperation
	MergeGroups  []APIConfigTransferMergeGroupOperation
	VideoPresets []APIConfigTransferPresetOperation
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
	if err := r.db.Unscoped().Order("channel_id ASC, sort_order ASC, id ASC").Find(&data.Models).Error; err != nil {
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
	return data, nil
}

func (r *APIConfigTransferRepo) Apply(plan *APIConfigTransferApplyPlan) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		channelIDs := make(map[string]uint, len(plan.Channels))
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
		}

		for _, operation := range plan.Models {
			item := operation.Item
			channelID, ok := channelIDs[operation.ChannelRef]
			if !ok {
				return fmt.Errorf("missing channel reference %q", operation.ChannelRef)
			}
			item.ChannelID = channelID
			item.DeletedAt = gorm.DeletedAt{}
			if operation.ExistingID > 0 {
				item.ID = operation.ExistingID
				if err := tx.Unscoped().Save(&item).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&item).Error; err != nil {
				return err
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
			var existing model.CreditPricing
			err := tx.Where("tenant_id = ? AND model = ? AND channel_id = ?", item.TenantID, item.Model, item.ChannelID).First(&existing).Error
			if err == nil {
				if err := tx.Model(&existing).Updates(map[string]interface{}{
					"credits_per_unit": item.CreditsPerUnit,
					"unit_type":        item.UnitType,
					"pricing_mode":     item.PricingMode,
					"pricing_rule":     item.PricingRule,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Create(&item).Error; err != nil {
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
		return nil
	})
}
