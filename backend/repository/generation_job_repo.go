package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"infinite-canvas-server/model"
)

var ErrGenerationJobNotFound = errors.New("generation job not found")

type GenerationJobRepo struct {
	db *gorm.DB
}

func NewGenerationJobRepo(db *gorm.DB) *GenerationJobRepo {
	return &GenerationJobRepo{db: db}
}

func (r *GenerationJobRepo) Reserve(job *model.GenerationJob, note, metadata string) (*model.CreditAccount, error) {
	if r == nil || r.db == nil || job == nil || job.UserID == 0 || strings.TrimSpace(job.RequestID) == "" || job.AuthorizedAmount < 0 {
		return nil, errors.New("invalid generation reservation")
	}
	var account model.CreditAccount
	err := r.db.Transaction(func(tx *gorm.DB) error {
		accountQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ?", job.TenantID, job.UserID)
		if err := accountQuery.First(&account).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CreditAccount{TenantID: job.TenantID, UserID: job.UserID}).Error; err != nil {
				return err
			}
			if err := accountQuery.First(&account).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if account.Balance < job.AuthorizedAmount {
			return errors.New("积分不足")
		}

		balanceBefore := account.Balance
		account.Balance -= job.AuthorizedAmount
		account.TotalSpent += job.AuthorizedAmount
		if err := tx.Model(&account).Updates(map[string]any{"balance": account.Balance, "total_spent": account.TotalSpent}).Error; err != nil {
			return err
		}

		job.Status = model.GenerationJobReserved
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		key := "spend:generation:" + job.RequestID
		entry := &model.CreditTransaction{
			AccountID: account.ID, Type: model.TxTypeSpend, Amount: job.AuthorizedAmount,
			BalanceBefore: creditIntPtr(balanceBefore), BalanceAfter: account.Balance,
			RefType: job.Capability, RefID: generationJobRefID(job.RequestID), IdempotencyKey: &key,
			Note: note, Metadata: metadata,
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		job.SpendTransactionID = entry.ID
		return tx.Model(job).Update("spend_transaction_id", entry.ID).Error
	})
	return &account, err
}

type GenerationSettlementInput struct {
	Amount         int
	ChannelID      uint
	ChannelModelID uint
	ChannelName    string
	ChannelBaseURL string
	VideoRoute     string
	UpstreamTaskID string
}

type GenerationSettlementResult struct {
	Amount  int
	Refund  int
	Balance int
}

func (r *GenerationJobRepo) Settle(requestID string, input GenerationSettlementInput) (*GenerationSettlementResult, error) {
	result := &GenerationSettlementResult{Amount: input.Amount}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var job model.GenerationJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_id = ?", requestID).First(&job).Error; err != nil {
			return err
		}
		if job.Status != model.GenerationJobReserved {
			return ErrGenerationJobNotFound
		}
		if input.Amount < 0 || input.Amount > job.AuthorizedAmount || input.ChannelID == 0 || input.ChannelModelID == 0 {
			return errors.New("invalid generation settlement")
		}
		result.Refund = job.AuthorizedAmount - input.Amount
		if result.Refund > 0 {
			var account model.CreditAccount
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ?", job.TenantID, job.UserID).First(&account).Error; err != nil {
				return err
			}
			balanceBefore := account.Balance
			account.Balance += result.Refund
			account.TotalSpent -= result.Refund
			if err := tx.Model(&account).Updates(map[string]any{"balance": account.Balance, "total_spent": account.TotalSpent}).Error; err != nil {
				return err
			}
			key := "settle:generation:" + job.RequestID
			entry := &model.CreditTransaction{AccountID: account.ID, Type: model.TxTypeRefund, Amount: result.Refund, BalanceBefore: creditIntPtr(balanceBefore), BalanceAfter: account.Balance, RefType: job.Capability, RefID: generationJobRefID(job.RequestID), IdempotencyKey: &key, Note: "智能路由费用差额退回", Metadata: mergeRefundMetadata("", job.SpendTransactionID)}
			if err := tx.Create(entry).Error; err != nil {
				return err
			}
			job.SettlementTransactionID = entry.ID
			result.Balance = account.Balance
		} else {
			var account model.CreditAccount
			if err := tx.Where("tenant_id = ? AND user_id = ?", job.TenantID, job.UserID).First(&account).Error; err != nil {
				return err
			}
			result.Balance = account.Balance
		}
		status := model.GenerationJobSucceeded
		if strings.TrimSpace(input.UpstreamTaskID) != "" {
			status = model.GenerationJobPending
		}
		return tx.Model(&job).Updates(map[string]any{
			"channel_id": input.ChannelID, "channel_model_id": input.ChannelModelID, "channel_name": input.ChannelName,
			"channel_base_url": input.ChannelBaseURL, "video_route": input.VideoRoute, "billing_amount": input.Amount,
			"settlement_transaction_id": job.SettlementTransactionID, "upstream_task_id": strings.TrimSpace(input.UpstreamTaskID),
			"status": status, "failure_reason": "",
		}).Error
	})
	return result, err
}

func (r *GenerationJobRepo) MarkSucceeded(requestID, upstreamTaskID string) error {
	status := model.GenerationJobSucceeded
	if strings.TrimSpace(upstreamTaskID) != "" {
		status = model.GenerationJobPending
	}
	result := r.db.Model(&model.GenerationJob{}).
		Where("request_id = ? AND status = ?", requestID, model.GenerationJobReserved).
		Updates(map[string]any{"status": status, "upstream_task_id": strings.TrimSpace(upstreamTaskID), "failure_reason": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrGenerationJobNotFound
	}
	return nil
}

func (r *GenerationJobRepo) CompleteByTask(tenantID, userID, channelModelID uint, upstreamTaskID string) error {
	return r.db.Model(&model.GenerationJob{}).
		Where("tenant_id = ? AND user_id = ? AND channel_model_id = ? AND upstream_task_id = ? AND status = ?", tenantID, userID, channelModelID, strings.TrimSpace(upstreamTaskID), model.GenerationJobPending).
		Update("status", model.GenerationJobSucceeded).Error
}

type GenerationRefundResult struct {
	Amount        int
	Balance       int
	Refunded      bool
	AlreadyClosed bool
}

func (r *GenerationJobRepo) RefundByRequest(requestID, reason string) (*GenerationRefundResult, error) {
	return r.refund(func(tx *gorm.DB, job *model.GenerationJob) error {
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_id = ?", requestID).First(job).Error
	}, reason)
}

func (r *GenerationJobRepo) RefundByTask(tenantID, userID, channelModelID uint, upstreamTaskID, reason string) (*GenerationRefundResult, error) {
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if upstreamTaskID == "" {
		return nil, ErrGenerationJobNotFound
	}
	return r.refund(func(tx *gorm.DB, job *model.GenerationJob) error {
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND channel_model_id = ? AND upstream_task_id = ?", tenantID, userID, channelModelID, upstreamTaskID).
			Order("id DESC").First(job).Error
	}, reason)
}

func (r *GenerationJobRepo) refund(find func(*gorm.DB, *model.GenerationJob) error, reason string) (*GenerationRefundResult, error) {
	result := &GenerationRefundResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var job model.GenerationJob
		if err := find(tx, &job); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrGenerationJobNotFound
			}
			return err
		}
		var account model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ?", job.TenantID, job.UserID).First(&account).Error; err != nil {
			return err
		}
		result.Balance = account.Balance
		result.Amount = job.BillingAmount
		if job.Status == model.GenerationJobRefunded {
			result.AlreadyClosed = true
			return nil
		}

		balanceBefore := account.Balance
		account.Balance += job.BillingAmount
		account.TotalSpent -= job.BillingAmount
		if err := tx.Model(&account).Updates(map[string]any{"balance": account.Balance, "total_spent": account.TotalSpent}).Error; err != nil {
			return err
		}
		key := "refund:generation:" + job.RequestID
		entry := &model.CreditTransaction{
			AccountID: account.ID, Type: model.TxTypeRefund, Amount: job.BillingAmount,
			BalanceBefore: creditIntPtr(balanceBefore), BalanceAfter: account.Balance,
			RefType: job.Capability, RefID: generationJobRefID(job.RequestID), IdempotencyKey: &key,
			Note: "生成失败自动退款", Metadata: mergeRefundMetadata("", job.SpendTransactionID),
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		job.Status = model.GenerationJobRefunded
		job.RefundTransactionID = entry.ID
		job.FailureReason = strings.TrimSpace(reason)
		if len(job.FailureReason) > 500 {
			job.FailureReason = job.FailureReason[:500]
		}
		if err := tx.Model(&job).Updates(map[string]any{
			"status": model.GenerationJobRefunded, "refund_transaction_id": entry.ID, "failure_reason": job.FailureReason,
		}).Error; err != nil {
			return err
		}
		result.Balance = account.Balance
		result.Refunded = true
		return nil
	})
	return result, err
}

func generationJobRefID(requestID string) string {
	return "generation_job:" + requestID
}
