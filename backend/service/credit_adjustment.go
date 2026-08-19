package service

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

var (
	ErrInvalidCreditAdjustment = errors.New("无效的积分调整")
	ErrCreditAdjustmentNoop    = errors.New("目标余额不能与当前余额相同")
	ErrInsufficientCredits     = errors.New("积分不足")
	ErrCreditTargetForbidden   = errors.New("不能调整其它租户用户积分")
)

type AdministratorCreditAdjustment struct {
	OperatorUserID     uint
	OperatorTenantID   uint
	TargetUserID       uint
	Mode               model.CreditAdjustmentMode
	Amount             int
	TargetBalance      *int
	Note               string
	CrossTenantAllowed bool
}

type AdministratorCreditAdjustmentResult struct {
	UserID  uint
	Amount  int
	Balance int
}

type administratorCreditMetadata struct {
	Scene          string                     `json:"scene"`
	OperatorUserID uint                       `json:"operator_user_id"`
	TargetUserID   uint                       `json:"target_user_id"`
	Mode           model.CreditAdjustmentMode `json:"mode"`
	Amount         int                        `json:"amount"`
	BalanceBefore  int                        `json:"balance_before"`
	BalanceAfter   int                        `json:"balance_after"`
	TargetBalance  int                        `json:"target_balance"`
}

func (s *CreditService) AdjustAdministratorCredits(input AdministratorCreditAdjustment) (*AdministratorCreditAdjustmentResult, error) {
	if input.Mode == "" {
		return nil, ErrInvalidCreditAdjustment
	}
	if input.OperatorUserID == 0 || input.TargetUserID == 0 {
		return nil, ErrInvalidCreditAdjustment
	}
	mode := input.Mode
	amount := input.Amount
	var signedDelta int
	account, err := s.creditRepo.MutateAccount(repository.CreditMutationTarget{
		UserID:             input.TargetUserID,
		TenantID:           input.OperatorTenantID,
		CrossTenantAllowed: input.CrossTenantAllowed,
	}, func(account *model.CreditAccount) (*model.CreditTransaction, error) {
		balanceBefore := account.Balance
		if balanceBefore < 0 {
			return nil, ErrInvalidCreditAdjustment
		}
		switch mode {
		case model.CreditAdjustmentAdd:
			if amount <= 0 || input.TargetBalance != nil || amount > math.MaxInt-balanceBefore || amount > math.MaxInt-account.TotalEarned {
				return nil, ErrInvalidCreditAdjustment
			}
			signedDelta = amount
		case model.CreditAdjustmentDeduct:
			if amount <= 0 || input.TargetBalance != nil || amount > balanceBefore || amount > math.MaxInt-account.TotalSpent {
				if amount > balanceBefore {
					return nil, ErrInsufficientCredits
				}
				return nil, ErrInvalidCreditAdjustment
			}
			signedDelta = -amount
		case model.CreditAdjustmentSet:
			if amount != 0 || input.TargetBalance == nil || *input.TargetBalance < 0 {
				return nil, ErrInvalidCreditAdjustment
			}
			signedDelta = *input.TargetBalance - balanceBefore
			if signedDelta == 0 {
				return nil, ErrCreditAdjustmentNoop
			}
			if signedDelta > 0 && signedDelta > math.MaxInt-account.TotalEarned {
				return nil, ErrInvalidCreditAdjustment
			}
			if signedDelta < 0 && -signedDelta > math.MaxInt-account.TotalSpent {
				return nil, ErrInvalidCreditAdjustment
			}
		default:
			return nil, ErrInvalidCreditAdjustment
		}

		balanceAfter := balanceBefore + signedDelta
		if balanceAfter < 0 {
			return nil, ErrInsufficientCredits
		}
		account.Balance = balanceAfter
		if signedDelta > 0 {
			account.TotalEarned += signedDelta
		} else {
			account.TotalSpent -= signedDelta
		}
		metadata, err := json.Marshal(administratorCreditMetadata{
			Scene:          "后台调整",
			OperatorUserID: input.OperatorUserID,
			TargetUserID:   input.TargetUserID,
			Mode:           mode,
			Amount:         signedDelta,
			BalanceBefore:  balanceBefore,
			BalanceAfter:   balanceAfter,
			TargetBalance:  balanceAfter,
		})
		if err != nil {
			return nil, err
		}
		note := strings.TrimSpace(input.Note)
		if note == "" {
			note = "管理员调整积分"
		}
		if utf8.RuneCountInString(note) > 500 {
			return nil, ErrInvalidCreditAdjustment
		}
		return &model.CreditTransaction{
			Type:          model.TxTypeAdjust,
			Amount:        signedDelta,
			BalanceBefore: intPtr(balanceBefore),
			BalanceAfter:  balanceAfter,
			RefType:       "adjust",
			Note:          note,
			Metadata:      string(metadata),
		}, nil
	})
	if errors.Is(err, repository.ErrCreditTargetTenantMismatch) || errors.Is(err, repository.ErrCreditAccountTenantMismatch) {
		return nil, ErrCreditTargetForbidden
	}
	if err != nil {
		return nil, err
	}
	return &AdministratorCreditAdjustmentResult{UserID: input.TargetUserID, Amount: signedDelta, Balance: account.Balance}, nil
}
