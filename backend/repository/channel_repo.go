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

func (r *ChannelRepo) SaveWithRevision(channel *model.Channel, expectedRevision uint) (bool, error) {
	result := r.db.Model(&model.Channel{}).Where("id = ? AND config_revision = ?", channel.ID, expectedRevision).Updates(map[string]any{
		"name": channel.Name, "base_url": channel.BaseUrl, "api_key": channel.ApiKey,
		"enabled": channel.Enabled, "video_api_standard": channel.VideoAPIStandard,
		"new_api_channel_id": channel.NewApiChannelID, "metrics_base_url": channel.MetricsBaseUrl,
		"remark": channel.Remark, "config_revision": channel.ConfigRevision,
	})
	return result.RowsAffected == 1, result.Error
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
