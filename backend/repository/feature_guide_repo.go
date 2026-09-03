package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
)

type FeatureGuideRepo struct{ db *gorm.DB }

func NewFeatureGuideRepo(db *gorm.DB) *FeatureGuideRepo {
	return &FeatureGuideRepo{db: db}
}

func (r *FeatureGuideRepo) GetBySurface(surface model.FeatureGuideSurface) (*model.FeatureGuide, error) {
	var item model.FeatureGuide
	if err := r.db.Where("surface = ?", surface).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FeatureGuideRepo) List() ([]model.FeatureGuide, error) {
	items := make([]model.FeatureGuide, 0, 3)
	err := r.db.Order("FIELD(surface, 'canvas', 'image', 'video')").Find(&items).Error
	return items, err
}

func (r *FeatureGuideRepo) UpdateLocked(surface model.FeatureGuideSurface, update func(*model.FeatureGuide) (*model.FeatureGuide, error)) (*model.FeatureGuide, error) {
	var saved *model.FeatureGuide
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var current model.FeatureGuide
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("surface = ?", surface).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing *model.FeatureGuide
		if err == nil {
			existing = &current
		}
		next, err := update(existing)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := tx.Create(next).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(existing).Updates(map[string]any{
				"enabled": next.Enabled,
				"title":   next.Title,
				"pages":   next.Pages,
				"version": next.Version,
			}).Error; err != nil {
				return err
			}
			next.ID = existing.ID
			next.CreatedAt = existing.CreatedAt
		}
		saved = next
		return nil
	})
	return saved, err
}
