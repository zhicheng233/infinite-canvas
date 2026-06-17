package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type TenantRepo struct{ db *gorm.DB }

func NewTenantRepo(db *gorm.DB) *TenantRepo { return &TenantRepo{db: db} }

func (r *TenantRepo) Create(tenant *model.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *TenantRepo) FindByID(id uint) (*model.Tenant, error) {
	var tenant model.Tenant
	err := r.db.First(&tenant, id).Error
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *TenantRepo) List(page, pageSize int) ([]model.Tenant, int64, error) {
	var tenants []model.Tenant
	var total int64
	q := r.db.Model(&model.Tenant{})
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&tenants).Error
	return tenants, total, err
}
