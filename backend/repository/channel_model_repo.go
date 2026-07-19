package repository

import (
	"infinite-canvas-server/model"

	"gorm.io/gorm"
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
	result := r.db.Where("channel_id = ? AND model_name = ?", item.ChannelID, item.ModelName).First(&existing)
	if result.Error == nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		if item.Capabilities == "" {
			item.Capabilities = existing.Capabilities
		}
		return r.db.Save(item).Error
	}
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	return r.db.Create(item).Error
}

func (r *ChannelModelRepo) SetEnabled(id uint, enabled bool) error {
	return r.db.Model(&model.ChannelModel{}).Where("id = ?", id).Update("enabled", enabled).Error
}
