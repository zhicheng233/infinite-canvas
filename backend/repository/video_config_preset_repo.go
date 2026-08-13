package repository

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

var ErrVideoConfigPresetNameConflict = errors.New("video config preset name conflict")

type VideoConfigPresetRepo struct {
	db *gorm.DB
}

func NewVideoConfigPresetRepo(db *gorm.DB) *VideoConfigPresetRepo {
	return &VideoConfigPresetRepo{db: db}
}

func (r *VideoConfigPresetRepo) ListByTenant(tenantID uint) ([]model.VideoConfigPreset, error) {
	items := make([]model.VideoConfigPreset, 0)
	err := r.db.Where("tenant_id = ?", tenantID).Order("normalized_name ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *VideoConfigPresetRepo) Create(item *model.VideoConfigPreset) error {
	return normalizeVideoConfigPresetCreateError(r.db.Create(item).Error)
}

func normalizeVideoConfigPresetCreateError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrVideoConfigPresetNameConflict
	}
	return err
}

func (r *VideoConfigPresetRepo) DeleteByTenantAndID(tenantID, presetID uint) error {
	result := r.db.Where("tenant_id = ? AND id = ?", tenantID, presetID).Delete(&model.VideoConfigPreset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
