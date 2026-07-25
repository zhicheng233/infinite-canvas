package repository

import (
	"errors"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type WebhookRepo struct{ db *gorm.DB }

func NewWebhookRepo(db *gorm.DB) *WebhookRepo { return &WebhookRepo{db: db} }

func (r *WebhookRepo) SavePatch(tenantID uint, platform string, updates map[string]interface{}) (*model.WebhookConfig, error) {
	if platform == "" {
		return nil, errors.New("platform 不能为空")
	}
	var existing model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND platform = ?", tenantID, platform).First(&existing).Error
	if err == nil {
		if len(updates) > 0 {
			if err := r.db.Model(&existing).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
		if err := r.db.Where("tenant_id = ? AND platform = ?", tenantID, platform).First(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	item := model.WebhookConfig{
		TenantID:        tenantID,
		Platform:        platform,
		Enabled:         false,
		CooldownMinutes: 10,
	}
	if value, ok := updates["webhook_url"].(string); ok {
		item.WebhookURL = value
	}
	if value, ok := updates["enabled"].(bool); ok {
		item.Enabled = value
	}
	if value, ok := updates["cooldown_minutes"].(int); ok {
		item.CooldownMinutes = value
	}
	enabled := item.Enabled
	cooldownMinutes := item.CooldownMinutes
	if err := r.db.Create(&item).Error; err != nil {
		return nil, err
	}
	if !enabled {
		if err := r.db.Model(&item).Update("enabled", false).Error; err != nil {
			return nil, err
		}
		item.Enabled = false
	}
	if cooldownMinutes == 0 {
		if err := r.db.Model(&item).Update("cooldown_minutes", 0).Error; err != nil {
			return nil, err
		}
		item.CooldownMinutes = 0
	}
	return &item, nil
}

// ListByTenant returns all webhook configs for a tenant, including disabled platforms.
func (r *WebhookRepo) ListByTenant(tenantID uint) ([]model.WebhookConfig, error) {
	var items []model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND platform <> ?", tenantID, "").Order("id ASC").Find(&items).Error
	return items, err
}

// ListEnabled returns all enabled webhook configs for a tenant.
func (r *WebhookRepo) ListEnabled(tenantID uint) ([]model.WebhookConfig, error) {
	var items []model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND enabled = ? AND platform <> ? AND webhook_url <> ?", tenantID, true, "", "").Find(&items).Error
	return items, err
}

// GetByPlatform returns a single webhook config by tenant and platform.
func (r *WebhookRepo) GetByPlatform(tenantID uint, platform string) (*model.WebhookConfig, error) {
	var cfg model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND platform = ?", tenantID, platform).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// InsertLog creates a new webhook log record.
func (r *WebhookRepo) InsertLog(log *model.WebhookLog) error {
	return r.db.Create(log).Error
}

// ListLogs returns the most recent webhook logs for a tenant, ordered by id DESC.
func (r *WebhookRepo) ListLogs(tenantID uint, limit int) ([]model.WebhookLog, error) {
	var logs []model.WebhookLog
	err := r.db.Where("tenant_id = ?", tenantID).Order("id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *WebhookRepo) LastAlert(tenantID uint, platform string, channelID uint, modelName string, status string) (*model.WebhookLog, error) {
	var log model.WebhookLog
	err := r.db.Where("tenant_id = ? AND platform = ? AND channel_id = ? AND model_name = ? AND status = ? AND success = ? AND cooldown_skipped = ?", tenantID, platform, channelID, modelName, status, true, false).
		Order("id DESC").Limit(1).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}
