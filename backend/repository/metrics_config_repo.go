package repository

import (
	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type MetricsConfigRepo struct{ db *gorm.DB }

func NewMetricsConfigRepo(db *gorm.DB) *MetricsConfigRepo { return &MetricsConfigRepo{db: db} }

func (r *MetricsConfigRepo) Get() (*model.MetricsConfig, error) {
	var cfg model.MetricsConfig
	err := r.db.Order("id ASC").First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *MetricsConfigRepo) Save(cfg *model.MetricsConfig) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.MetricsConfig
		err := tx.Order("id ASC").First(&existing).Error
		if err == nil {
			existing.MetricsBaseURL = cfg.MetricsBaseURL
			return tx.Save(&existing).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(cfg).Error
	})
}
