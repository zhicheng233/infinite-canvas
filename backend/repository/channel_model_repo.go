package repository

import (
	"strings"

	"infinite-canvas-server/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelModelRepo struct {
	db *gorm.DB
}

func NewChannelModelRepo(db *gorm.DB) *ChannelModelRepo {
	return &ChannelModelRepo{db: db}
}

func (r *ChannelModelRepo) FindByID(id uint) (*model.ChannelModel, error) {
	var item model.ChannelModel
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ChannelModelRepo) FindByChannelAndName(channelID uint, modelName string) (*model.ChannelModel, error) {
	var item model.ChannelModel
	if err := r.db.Where("channel_id = ? AND model_name = ?", channelID, modelName).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ChannelModelRepo) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	items := make([]model.ChannelModel, 0)
	query := r.db.Where("channel_id = ?", channelID)
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ChannelModelRepo) Save(item *model.ChannelModel) error {
	return r.db.Save(item).Error
}

func (r *ChannelModelRepo) Upsert(item *model.ChannelModel) error {
	var existing model.ChannelModel
	result := r.db.Unscoped().Where("channel_id = ? AND model_name = ?", item.ChannelID, item.ModelName).First(&existing)
	if result.Error == nil {
		mergeChannelModelForUpsert(&existing, item)
		if err := r.db.Unscoped().Save(&existing).Error; err != nil {
			return err
		}
		*item = existing
		return nil
	}
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	item.DeletedAt = gorm.DeletedAt{}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}, {Name: "model_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"deleted_at"}),
	}).Create(item).Error
}

func mergeChannelModelForUpsert(existing, incoming *model.ChannelModel) {
	existing.DeletedAt = gorm.DeletedAt{}
	if strings.TrimSpace(existing.Capabilities) == "" {
		existing.Capabilities = incoming.Capabilities
	}
}

func (r *ChannelModelRepo) SetEnabled(id uint, enabled bool) error {
	return r.db.Model(&model.ChannelModel{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (r *ChannelModelRepo) FindByModelName(modelName string) ([]model.ChannelModel, error) {
	var items []model.ChannelModel
	err := r.db.Where("model_name = ?", modelName).Order("channel_id ASC").Find(&items).Error
	return items, err
}

// DeleteStaleModels removes channel_models for the given channel that are NOT in keepNames.
func (r *ChannelModelRepo) DeleteStaleModels(channelID uint, keepNames []string) error {
	if len(keepNames) == 0 {
		return nil
	}
	return r.db.Where("channel_id = ? AND model_name NOT IN ?", channelID, keepNames).Delete(&model.ChannelModel{}).Error
}
