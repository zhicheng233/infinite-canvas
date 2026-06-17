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
	return r.db.Save(cfg).Error
}
