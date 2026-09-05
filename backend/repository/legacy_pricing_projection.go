package repository

import (
	"errors"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

// upsertLegacyPricingProjection keeps the old credit_pricing table usable as a
// compatibility projection even when an older row was soft-deleted.
func upsertLegacyPricingProjection(tx *gorm.DB, pricing model.CreditPricing) error {
	var existing model.CreditPricing
	err := tx.Unscoped().Where("tenant_id = ? AND model = ? AND channel_id = ?", pricing.TenantID, pricing.Model, pricing.ChannelID).First(&existing).Error
	if err == nil {
		return tx.Unscoped().Model(&existing).Updates(map[string]any{
			"deleted_at":       nil,
			"credits_per_unit": pricing.CreditsPerUnit,
			"unit_type":        pricing.UnitType,
			"pricing_mode":     pricing.PricingMode,
			"pricing_rule":     pricing.PricingRule,
		}).Error
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&pricing).Error
}
