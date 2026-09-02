package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type AutoRoutingRepo struct{ db *gorm.DB }

func NewAutoRoutingRepo(db *gorm.DB) *AutoRoutingRepo { return &AutoRoutingRepo{db: db} }

func (r *AutoRoutingRepo) ListPools() ([]model.AutoRoutingPool, error) {
	var items []model.AutoRoutingPool
	err := r.db.Preload("Members", func(db *gorm.DB) *gorm.DB { return db.Order("priority DESC, id ASC") }).Order("public_model_name ASC, capability ASC").Find(&items).Error
	return items, err
}

func (r *AutoRoutingRepo) FindPool(id uint) (*model.AutoRoutingPool, error) {
	var item model.AutoRoutingPool
	if err := r.db.Preload("Members", func(db *gorm.DB) *gorm.DB { return db.Order("priority DESC, id ASC") }).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AutoRoutingRepo) SavePool(pool *model.AutoRoutingPool, memberIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Members").Save(pool).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("pool_id = ?", pool.ID).Delete(&model.AutoRoutingPoolMember{}).Error; err != nil {
			return err
		}
		for _, channelModelID := range memberIDs {
			if err := tx.Create(&model.AutoRoutingPoolMember{PoolID: pool.ID, ChannelModelID: channelModelID, Enabled: true}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AutoRoutingRepo) UpdatePool(pool *model.AutoRoutingPool) error {
	return r.db.Omit("Members").Save(pool).Error
}

func (r *AutoRoutingRepo) UpdateMember(member *model.AutoRoutingPoolMember) error {
	return r.db.Model(member).Updates(map[string]any{"priority": member.Priority, "enabled": member.Enabled}).Error
}

func (r *AutoRoutingRepo) ReplaceMembers(pool *model.AutoRoutingPool, channelModelIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current []model.AutoRoutingPoolMember
		if err := tx.Where("pool_id = ?", pool.ID).Find(&current).Error; err != nil {
			return err
		}
		settings := make(map[uint]model.AutoRoutingPoolMember, len(current))
		for _, member := range current {
			settings[member.ChannelModelID] = member
		}
		if err := tx.Unscoped().Where("pool_id = ?", pool.ID).Delete(&model.AutoRoutingPoolMember{}).Error; err != nil {
			return err
		}
		for _, channelModelID := range channelModelIDs {
			member := model.AutoRoutingPoolMember{PoolID: pool.ID, ChannelModelID: channelModelID, Enabled: true}
			if previous, ok := settings[channelModelID]; ok {
				member.Priority, member.Enabled = previous.Priority, previous.Enabled
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		return tx.Omit("Members").Save(pool).Error
	})
}

func (r *AutoRoutingRepo) DeletePool(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("pool_id = ?", id).Delete(&model.AutoRoutingPoolMember{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.AutoRoutingPool{}, id).Error
	})
}

func (r *AutoRoutingRepo) CreateAttempt(item *model.GenerationAttempt) error {
	return r.db.Create(item).Error
}

func (r *AutoRoutingRepo) ListHealthAttempts(channelModelID uint, since time.Time) ([]model.GenerationAttempt, error) {
	var items []model.GenerationAttempt
	err := r.db.Where("channel_model_id = ? AND counts_for_health = ? AND created_at >= ?", channelModelID, true, since).Order("id DESC").Find(&items).Error
	return items, err
}

func (r *AutoRoutingRepo) FindMember(poolID, memberID uint) (*model.AutoRoutingPoolMember, error) {
	var item model.AutoRoutingPoolMember
	if err := r.db.Where("pool_id = ? AND id = ?", poolID, memberID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AutoRoutingRepo) DeleteMember(poolID, memberID uint) error {
	result := r.db.Where("pool_id = ? AND id = ?", poolID, memberID).Delete(&model.AutoRoutingPoolMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *AutoRoutingRepo) DB() (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("auto routing repository is not configured")
	}
	return r.db, nil
}
