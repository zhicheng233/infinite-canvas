package service

import (
	"errors"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type CreditService struct {
	creditRepo *repository.CreditRepo
}

func NewCreditService(creditRepo *repository.CreditRepo) *CreditService {
	return &CreditService{creditRepo: creditRepo}
}

func (s *CreditService) GetOrCreateAccount(tenantID, userID uint) (*model.CreditAccount, error) {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err == nil {
		return account, nil
	}
	account = &model.CreditAccount{
		TenantID: tenantID,
		UserID:   userID,
		Balance:  0,
	}
	if err := s.creditRepo.CreateAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *CreditService) Spend(accountID, userID uint, amount int, refType, refID, note string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	if account.Balance < amount {
		return errors.New("积分不足")
	}
	account.Balance -= amount
	account.TotalSpent += amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:    account.ID,
		Type:         model.TxTypeSpend,
		Amount:       amount,
		BalanceAfter: account.Balance,
		RefType:      refType,
		RefID:        refID,
		Note:         note,
	}
	return s.creditRepo.CreateTransaction(tx)
}

func (s *CreditService) Earn(userID uint, amount int, refType, refID, note string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	account.Balance += amount
	account.TotalEarned += amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:    account.ID,
		Type:         model.TxTypeEarn,
		Amount:       amount,
		BalanceAfter: account.Balance,
		RefType:      refType,
		RefID:        refID,
		Note:         note,
	}
	return s.creditRepo.CreateTransaction(tx)
}

func (s *CreditService) Refund(userID uint, amount int, refType, refID, note string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	account.Balance += amount
	account.TotalSpent -= amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:    account.ID,
		Type:         model.TxTypeRefund,
		Amount:       amount,
		BalanceAfter: account.Balance,
		RefType:      refType,
		RefID:        refID,
		Note:         note,
	}
	return s.creditRepo.CreateTransaction(tx)
}
