package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type ChannelRepo struct{ db *gorm.DB }

func NewChannelRepo(db *gorm.DB) *ChannelRepo { return &ChannelRepo{db: db} }

func (r *ChannelRepo) Create(channel *model.Channel) error {
	return r.db.Create(channel).Error
}

func (r *ChannelRepo) Save(channel *model.Channel) error {
	return r.db.Save(channel).Error
}

func (r *ChannelRepo) FindByID(id uint) (*model.Channel, error) {
	var channel model.Channel
	err := r.db.First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *ChannelRepo) ListAll() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.db.Order("id ASC").Find(&channels).Error
	return channels, err
}

func (r *ChannelRepo) ListEnabled() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.db.Where("enabled = ?", true).Order("id ASC").Find(&channels).Error
	return channels, err
}

func (r *ChannelRepo) Disable(id uint) error {
	return r.db.Model(&model.Channel{}).Where("id = ?", id).Update("enabled", false).Error
}

func (r *ChannelRepo) Enable(id uint) error {
	return r.db.Model(&model.Channel{}).Where("id = ?", id).Update("enabled", true).Error
}

func (r *ChannelRepo) Delete(id uint) error {
	var channel model.Channel
	if err := r.db.First(&channel, id).Error; err != nil {
		return err
	}

	// Cascade delete related records
	r.db.Where("channel_id = ?", id).Delete(&model.ModelMergeGroup{})
	r.db.Where("channel_id = ?", id).Delete(&model.ChannelModel{})

	return r.db.Delete(&channel).Error
}
