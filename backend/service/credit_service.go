package service

import (
	"encoding/json"
	"errors"
	"strings"

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
	return s.SpendWithIdempotencyMetadata(accountID, userID, amount, refType, refID, note, metadata, "")
}

func (s *CreditService) SpendWithIdempotencyMetadata(accountID, userID uint, amount int, refType, refID, note, metadata, idempotencyKey string) error {
	return s.creditRepo.ApplyBalanceChange(repository.BalanceChangeInput{
		UserID: userID, Type: model.TxTypeSpend, Amount: amount, RefType: refType, RefID: refID, Note: note, Metadata: metadata, IdempotencyKey: strings.TrimSpace(idempotencyKey),
	})
}

func (s *CreditService) Earn(userID uint, amount int, refType, refID, note string) error {
	return s.EarnWithMetadata(userID, amount, refType, refID, note, "")
}

func (s *CreditService) EarnWithMetadata(userID uint, amount int, refType, refID, note, metadata string) error {
	return s.creditRepo.ApplyBalanceChange(repository.BalanceChangeInput{UserID: userID, Type: model.TxTypeEarn, Amount: amount, RefType: refType, RefID: refID, Note: note, Metadata: metadata})
}

func (s *CreditService) Refund(userID uint, amount int, refType, refID, note string) error {
	return s.RefundWithMetadata(userID, amount, refType, refID, note, "")
}

func (s *CreditService) RefundWithMetadata(userID uint, amount int, refType, refID, note, metadata string) error {
	return s.creditRepo.ApplyBalanceChange(repository.BalanceChangeInput{UserID: userID, Type: model.TxTypeRefund, Amount: amount, RefType: refType, RefID: refID, Note: note, Metadata: metadata})
}

func (s *CreditService) RefundAsyncSpendOnce(userID uint, refType, refID, idempotencyKey, note, metadata string) (*repository.AsyncRefundResult, error) {
	if s == nil || s.creditRepo == nil {
		return nil, errors.New("credit service is not configured")
	}
	if userID == 0 || strings.TrimSpace(refType) == "" || strings.TrimSpace(refID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return nil, errors.New("invalid async refund reference")
	}
	return s.creditRepo.RefundSpendOnce(userID, refType, refID, idempotencyKey, note, metadata)
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
