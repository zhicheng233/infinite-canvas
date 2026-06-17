package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type RechargeRepo struct{ db *gorm.DB }

func NewRechargeRepo(db *gorm.DB) *RechargeRepo { return &RechargeRepo{db: db} }

func (r *RechargeRepo) Create(order *model.RechargeOrder) error {
	return r.db.Create(order).Error
}

func (r *RechargeRepo) FindByID(id uint) (*model.RechargeOrder, error) {
	var order model.RechargeOrder
	err := r.db.First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *RechargeRepo) UpdateStatus(id uint, status string) error {
	return r.db.Model(&model.RechargeOrder{}).Where("id = ?", id).Update("status", status).Error
}

func (r *RechargeRepo) ListByTenant(tenantID uint, page, pageSize int) ([]model.RechargeOrder, int64, error) {
	var orders []model.RechargeOrder
	var total int64
	q := r.db.Model(&model.RechargeOrder{}).Where("tenant_id = ?", tenantID)
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&orders).Error
	return orders, total, err
}

func (r *RechargeRepo) ListAll(page, pageSize int) ([]model.RechargeOrder, int64, error) {
	var orders []model.RechargeOrder
	var total int64
	q := r.db.Model(&model.RechargeOrder{})
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&orders).Error
	return orders, total, err
}

func (r *RechargeRepo) ListByUser(userID uint, page, pageSize int) ([]model.RechargeOrder, int64, error) {
	var orders []model.RechargeOrder
	var total int64
	q := r.db.Model(&model.RechargeOrder{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&orders).Error
	return orders, total, err
}

func (r *RechargeRepo) UpdatePaymentRef(id uint, paymentRef string) error {
	return r.db.Model(&model.RechargeOrder{}).Where("id = ?", id).Update("payment_ref", paymentRef).Error
}

func (r *RechargeRepo) SumCompletedByTenant(tenantID uint) (int64, error) {
	var total int64
	err := r.db.Model(&model.RechargeOrder{}).
		Where("tenant_id = ? AND status = ?", tenantID, "completed").
		Select("COALESCE(SUM(credits), 0)").
		Scan(&total).Error
	return total, err
}
