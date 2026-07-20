package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type WebhookRepo struct{ db *gorm.DB }

func NewWebhookRepo(db *gorm.DB) *WebhookRepo { return &WebhookRepo{db: db} }

// Save upserts a webhook config by (tenant_id, platform).
func (r *WebhookRepo) Save(cfg *model.WebhookConfig) error {
	var existing model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND platform = ?", cfg.TenantID, cfg.Platform).First(&existing).Error
	if err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"webhook_url":      cfg.WebhookURL,
			"enabled":          cfg.Enabled,
			"template_down":    cfg.TemplateDown,
			"template_up":      cfg.TemplateUp,
			"interval_seconds": cfg.IntervalSeconds,
			"cooldown_minutes": cfg.CooldownMinutes,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.Create(cfg).Error
}

// ListEnabled returns all enabled webhook configs for a tenant.
func (r *WebhookRepo) ListEnabled(tenantID uint) ([]model.WebhookConfig, error) {
	var items []model.WebhookConfig
	err := r.db.Where("tenant_id = ? AND enabled = ?", tenantID, true).Find(&items).Error
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

// LastLogForModel returns the most recent log for a given tenant, model, and status.
func (r *WebhookRepo) LastLogForModel(tenantID uint, modelName string, status string) (*model.WebhookLog, error) {
	var log model.WebhookLog
	err := r.db.Where("tenant_id = ? AND model_name = ? AND status = ?", tenantID, modelName, status).
		Order("id DESC").Limit(1).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}
