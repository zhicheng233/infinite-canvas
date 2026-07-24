package repository

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type ModelCallLogRepo struct{ db *gorm.DB }

func NewModelCallLogRepo(db *gorm.DB) *ModelCallLogRepo { return &ModelCallLogRepo{db: db} }

func (r *ModelCallLogRepo) Create(log *model.ModelCallLog) error {
	return r.db.Create(log).Error
}

func (r *ModelCallLogRepo) List(tenantID uint, query ModelCallLogQuery) ([]model.ModelCallLog, int64, error) {
	var items []model.ModelCallLog
	var total int64
	base := r.db.Model(&model.ModelCallLog{}).Where("tenant_id = ? AND is_success = ?", tenantID, false)
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if query.Model != "" {
		base = base.Where("model LIKE ?", "%"+query.Model+"%")
	}
	if query.Generation != "" {
		base = base.Where("generation = ?", query.Generation)
	}
	if query.Keyword != "" {
		keyword := "%" + strings.TrimSpace(query.Keyword) + "%"
		base = base.Where("error_message LIKE ? OR error_body LIKE ? OR path LIKE ? OR username LIKE ?", keyword, keyword, keyword, keyword)
	}
	base.Count(&total)

	q := r.db.Select("model_call_logs.*, channels.name as channel_name").
		Where("tenant_id = ? AND is_success = ?", tenantID, false).
		Joins("LEFT JOIN channels ON channels.id = model_call_logs.channel_id")
	if query.UserID > 0 {
		q = q.Where("user_id = ?", query.UserID)
	}
	if query.Model != "" {
		q = q.Where("model LIKE ?", "%"+query.Model+"%")
	}
	if query.Generation != "" {
		q = q.Where("generation = ?", query.Generation)
	}
	if query.Keyword != "" {
		keyword := "%" + strings.TrimSpace(query.Keyword) + "%"
		q = q.Where("error_message LIKE ? OR error_body LIKE ? OR path LIKE ? OR username LIKE ?", keyword, keyword, keyword, keyword)
	}
	err := q.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Order("id DESC").Find(&items).Error
	return items, total, err
}

func (r *ModelCallLogRepo) ListSince(tenantID uint, since time.Time, limit int) ([]model.ModelCallLog, error) {
	var items []model.ModelCallLog
	if limit <= 0 {
		limit = 500
	}
	err := r.db.Where("tenant_id = ? AND created_at >= ?", tenantID, since).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

type ModelCallLogQuery struct {
	Page       int
	PageSize   int
	UserID     uint
	Model      string
	Generation string
	Keyword    string
}
