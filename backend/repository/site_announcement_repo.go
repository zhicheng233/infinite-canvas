package repository

import (
	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type SiteAnnouncementRepo struct{ db *gorm.DB }

func NewSiteAnnouncementRepo(db *gorm.DB) *SiteAnnouncementRepo {
	return &SiteAnnouncementRepo{db: db}
}

func (r *SiteAnnouncementRepo) Get() (*model.SiteAnnouncement, error) {
	var item model.SiteAnnouncement
	if err := r.db.Order("id ASC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *SiteAnnouncementRepo) Save(item *model.SiteAnnouncement) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.SiteAnnouncement
		err := tx.Order("id ASC").First(&existing).Error
		if err == nil {
			existing.Enabled = item.Enabled
			existing.Title = item.Title
			existing.Content = item.Content
			existing.Version = item.Version
			return tx.Save(&existing).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(item).Error
	})
}
