package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"infinite-canvas-server/model"
)

var ErrCreditTargetTenantMismatch = errors.New("credit target tenant mismatch")

var (
	ErrCreditAccountTenantMismatch = errors.New("credit account tenant mismatch")
	ErrCreditMutationMissingLedger = errors.New("credit mutation missing ledger")
)

type CreditMutationTarget struct {
	UserID             uint
	TenantID           uint
	CrossTenantAllowed bool
}

type CreditAccountMutation func(account *model.CreditAccount) (*model.CreditTransaction, error)

func (r *CreditRepo) MutateAccount(target CreditMutationTarget, mutation CreditAccountMutation) (*model.CreditAccount, error) {
	var result model.CreditAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, target.UserID).Error; err != nil {
			return err
		}
		if !target.CrossTenantAllowed && user.TenantID != target.TenantID {
			return ErrCreditTargetTenantMismatch
		}

		var account model.CreditAccount
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			seed := model.CreditAccount{TenantID: user.TenantID, UserID: user.ID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&account).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if account.TenantID != user.TenantID {
			return ErrCreditAccountTenantMismatch
		}

		ledger, err := mutation(&account)
		if err != nil {
			return err
		}
		if ledger == nil {
			return ErrCreditMutationMissingLedger
		}
		if err := tx.Model(&account).Updates(map[string]interface{}{
			"balance":      account.Balance,
			"total_earned": account.TotalEarned,
			"total_spent":  account.TotalSpent,
		}).Error; err != nil {
			return err
		}
		ledger.AccountID = account.ID
		if err := tx.Create(ledger).Error; err != nil {
			return err
		}
		result = account
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
