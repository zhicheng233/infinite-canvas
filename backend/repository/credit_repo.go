package repository

import (
	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type CreditRepo struct{ db *gorm.DB }

func NewCreditRepo(db *gorm.DB) *CreditRepo { return &CreditRepo{db: db} }

func (r *CreditRepo) FindAccountByUser(userID uint) (*model.CreditAccount, error) {
	var account model.CreditAccount
	err := r.db.Where("user_id = ?", userID).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *CreditRepo) CreateAccount(account *model.CreditAccount) error {
	return r.db.Create(account).Error
}

func (r *CreditRepo) UpdateAccountBalance(account *model.CreditAccount) error {
	return r.db.Model(account).Updates(map[string]interface{}{
		"balance":      account.Balance,
		"total_earned": account.TotalEarned,
		"total_spent":  account.TotalSpent,
	}).Error
}

func (r *CreditRepo) CreateTransaction(tx *model.CreditTransaction) error {
	return r.db.Create(tx).Error
}

func (r *CreditRepo) ListTransactions(accountID uint, page, pageSize int) ([]model.CreditTransaction, int64, error) {
	var txs []model.CreditTransaction
	var total int64
	q := r.db.Model(&model.CreditTransaction{}).Where("account_id = ?", accountID)
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&txs).Error
	return txs, total, err
}

func (r *CreditRepo) FindPricing(tenantID uint, modelName string) (*model.CreditPricing, error) {
	var pricing model.CreditPricing
	err := r.db.Where("tenant_id = ? AND model = ?", tenantID, modelName).First(&pricing).Error
	if err != nil {
		return nil, err
	}
	return &pricing, nil
}

func (r *CreditRepo) SavePricing(pricing *model.CreditPricing) error {
	return r.db.Save(pricing).Error
}

func (r *CreditRepo) ListPricing(tenantID uint) ([]model.CreditPricing, error) {
	var items []model.CreditPricing
	err := r.db.Where("tenant_id = ?", tenantID).Order("model ASC").Find(&items).Error
	return items, err
}

func (r *CreditRepo) DeletePricing(id uint) error {
	return r.db.Delete(&model.CreditPricing{}, id).Error
}

func (r *CreditRepo) SumEarnedByTenant(tenantID uint) (int64, error) {
	var total int64
	err := r.db.Model(&model.CreditTransaction{}).
		Joins("JOIN credit_accounts ON credit_accounts.id = credit_transactions.account_id").
		Where("credit_accounts.tenant_id = ? AND credit_transactions.type = ?", tenantID, model.TxTypeEarn).
		Select("COALESCE(SUM(credit_transactions.amount), 0)").
		Scan(&total).Error
	return total, err
}

func (r *CreditRepo) SumSpentByTenant(tenantID uint) (int64, error) {
	var total int64
	err := r.db.Model(&model.CreditTransaction{}).
		Joins("JOIN credit_accounts ON credit_accounts.id = credit_transactions.account_id").
		Where("credit_accounts.tenant_id = ? AND credit_transactions.type = ?", tenantID, model.TxTypeSpend).
		Select("COALESCE(SUM(credit_transactions.amount), 0)").
		Scan(&total).Error
	return total, err
}

func (r *CreditRepo) GetBalancesByUserIDs(userIDs []uint) (map[uint]int, error) {
	if len(userIDs) == 0 {
		return map[uint]int{}, nil
	}
	type row struct {
		UserID  uint
		Balance int
	}
	var rows []row
	err := r.db.Model(&model.CreditAccount{}).
		Where("user_id IN ?", userIDs).
		Select("user_id, balance").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]int, len(rows))
	for _, r := range rows {
		result[r.UserID] = r.Balance
	}
	return result, nil
}

func (r *CreditRepo) ListTransactionsByTenant(tenantID uint, page, pageSize int) ([]model.CreditTransaction, int64, error) {
	var txs []model.CreditTransaction
	var total int64
	q := r.db.Model(&model.CreditTransaction{}).
		Joins("JOIN credit_accounts ON credit_accounts.id = credit_transactions.account_id").
		Where("credit_accounts.tenant_id = ?", tenantID)
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("credit_transactions.id DESC").Find(&txs).Error
	return txs, total, err
}
