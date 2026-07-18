package service

import (
	"encoding/json"
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
	return s.SpendWithMetadata(accountID, userID, amount, refType, refID, note, "")
}

func (s *CreditService) SpendWithMetadata(accountID, userID uint, amount int, refType, refID, note, metadata string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	if account.Balance < amount {
		return errors.New("积分不足")
	}
	balanceBefore := account.Balance
	account.Balance -= amount
	account.TotalSpent += amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:     account.ID,
		Type:          model.TxTypeSpend,
		Amount:        amount,
		BalanceBefore: intPtr(balanceBefore),
		BalanceAfter:  account.Balance,
		RefType:       refType,
		RefID:         refID,
		Note:          note,
		Metadata:      metadata,
	}
	return s.creditRepo.CreateTransaction(tx)
}

func (s *CreditService) Earn(userID uint, amount int, refType, refID, note string) error {
	return s.EarnWithMetadata(userID, amount, refType, refID, note, "")
}

func (s *CreditService) EarnWithMetadata(userID uint, amount int, refType, refID, note, metadata string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	balanceBefore := account.Balance
	account.Balance += amount
	account.TotalEarned += amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:     account.ID,
		Type:          model.TxTypeEarn,
		Amount:        amount,
		BalanceBefore: intPtr(balanceBefore),
		BalanceAfter:  account.Balance,
		RefType:       refType,
		RefID:         refID,
		Note:          note,
		Metadata:      metadata,
	}
	return s.creditRepo.CreateTransaction(tx)
}

func (s *CreditService) Refund(userID uint, amount int, refType, refID, note string) error {
	return s.RefundWithMetadata(userID, amount, refType, refID, note, "")
}

func (s *CreditService) RefundWithMetadata(userID uint, amount int, refType, refID, note, metadata string) error {
	account, err := s.creditRepo.FindAccountByUser(userID)
	if err != nil {
		return err
	}
	balanceBefore := account.Balance
	account.Balance += amount
	account.TotalSpent -= amount
	if err := s.creditRepo.UpdateAccountBalance(account); err != nil {
		return err
	}
	tx := &model.CreditTransaction{
		AccountID:     account.ID,
		Type:          model.TxTypeRefund,
		Amount:        amount,
		BalanceBefore: intPtr(balanceBefore),
		BalanceAfter:  account.Balance,
		RefType:       refType,
		RefID:         refID,
		Note:          note,
		Metadata:      metadata,
	}
	return s.creditRepo.CreateTransaction(tx)
}

func BuildCreditMetadata(values map[string]interface{}) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

func intPtr(value int) *int {
	return &value
}
