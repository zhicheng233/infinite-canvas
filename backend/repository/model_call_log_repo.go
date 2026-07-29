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
	base := r.db.Model(&model.ModelCallLog{}).Where("tenant_id = ?", tenantID)
	base = applyModelCallLogStatusFilter(base, query.Status)
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
		base = base.Where("error_message LIKE ? OR error_body LIKE ? OR path LIKE ? OR username LIKE ? OR upstream_url LIKE ? OR request_content_type LIKE ? OR request_body LIKE ?", keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	base.Count(&total)

	q := r.db.Select("model_call_logs.*, channels.name as channel_name").
		Where("tenant_id = ?", tenantID).
		Joins("LEFT JOIN channels ON channels.id = model_call_logs.channel_id")
	q = applyModelCallLogStatusFilter(q, query.Status)
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
		q = q.Where("error_message LIKE ? OR error_body LIKE ? OR path LIKE ? OR username LIKE ? OR upstream_url LIKE ? OR request_content_type LIKE ? OR request_body LIKE ?", keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	err := q.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Order("model_call_logs.id DESC").Find(&items).Error
	return items, total, err
}

func applyModelCallLogStatusFilter(q *gorm.DB, status string) *gorm.DB {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return q.Where("is_success = ?", true)
	case "all":
		return q
	default:
		return q.Where("is_success = ?", false)
	}
}

func (r *ModelCallLogRepo) ListSince(tenantID uint, since time.Time, limit int) ([]model.ModelCallLog, error) {
	var items []model.ModelCallLog
	if limit <= 0 {
		limit = 500
	}
	err := r.db.Select("model_call_logs.id, model_call_logs.created_at, model_call_logs.tenant_id, model_call_logs.user_id, model_call_logs.username, model_call_logs.display_name, model_call_logs.generation, model_call_logs.model, model_call_logs.method, model_call_logs.path, model_call_logs.status_code, model_call_logs.error_message, model_call_logs.is_success, model_call_logs.response_time, model_call_logs.channel_id, model_call_logs.channel_model_id, channels.name as channel_name").
		Joins("LEFT JOIN channels ON channels.id = model_call_logs.channel_id").
		Where("model_call_logs.tenant_id = ? AND model_call_logs.created_at >= ?", tenantID, since).
		Order("model_call_logs.id DESC").
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
	Status     string
}
