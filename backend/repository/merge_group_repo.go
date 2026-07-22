package repository

import (
	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type MergeGroupRepo struct {
	db *gorm.DB
}

func NewMergeGroupRepo(db *gorm.DB) *MergeGroupRepo {
	return &MergeGroupRepo{db: db}
}

func (r *MergeGroupRepo) ListByChannel(channelID uint) ([]model.ModelMergeGroup, error) {
	var items []model.ModelMergeGroup
	err := r.db.Where("channel_id = ?", channelID).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *MergeGroupRepo) Create(group *model.ModelMergeGroup) error {
	return r.db.Create(group).Error
}

func (r *MergeGroupRepo) Delete(id uint) error {
	return r.db.Delete(&model.ModelMergeGroup{}, id).Error
}

func (r *MergeGroupRepo) FindByID(id uint) (*model.ModelMergeGroup, error) {
	var group model.ModelMergeGroup
	if err := r.db.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *MergeGroupRepo) DeleteByChannel(channelID uint) error {
	return r.db.Where("channel_id = ?", channelID).Delete(&model.ModelMergeGroup{}).Error
}

func (r *MergeGroupRepo) ListModelNames(channelID uint) ([]string, error) {
	var names []string
	err := r.db.Model(&model.ChannelModel{}).Where("channel_id = ?", channelID).Distinct("model_name").Pluck("model_name", &names).Error
	return names, err
}
