package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type ApiConfigRepo struct{ db *gorm.DB }

func NewApiConfigRepo(db *gorm.DB) *ApiConfigRepo { return &ApiConfigRepo{db: db} }

func (r *ApiConfigRepo) FindByTenant(tenantID uint) (*model.TenantApiConfig, error) {
	var cfg model.TenantApiConfig
	err := r.db.Where("tenant_id = ?", tenantID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ApiConfigRepo) Save(cfg *model.TenantApiConfig) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.TenantApiConfig
		err := tx.Where("tenant_id = ?", cfg.TenantID).First(&existing).Error
		if err == nil {
			existing.BaseUrl = cfg.BaseUrl
			existing.ApiKey = cfg.ApiKey
			return tx.Save(&existing).Error
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(cfg).Error
	})
}
