package handler

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

func TestGenerationReservationIsConcurrentAndRefundIdempotent(t *testing.T) {
	db := openAdminCreditTestDB(t)
	const (
		tenantID = 21
		userID   = 34
		balance  = 100
		cost     = 10
		attempts = 20
	)
	if err := db.Create(&model.CreditAccount{TenantID: tenantID, UserID: userID, Balance: balance, TotalEarned: balance}).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
	repo := repository.NewGenerationJobRepo(db)

	type reserveResult struct {
		job *model.GenerationJob
		err error
	}
	results := make(chan reserveResult, attempts)
	var wait sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			job := &model.GenerationJob{
				RequestID: fmt.Sprintf("concurrent-%d", index), TenantID: tenantID, UserID: userID,
				Capability: "image", ModelName: "test", ChannelID: 1, ChannelModelID: 2, AuthorizedAmount: cost, BillingAmount: cost,
			}
			_, err := repo.Reserve(job, "test", "{}")
			results <- reserveResult{job: job, err: err}
		}(i)
	}
	wait.Wait()
	close(results)

	var succeeded []*model.GenerationJob
	for result := range results {
		if result.err == nil {
			succeeded = append(succeeded, result.job)
			continue
		}
		if result.err.Error() != "积分不足" {
			t.Fatalf("reserve error = %v", result.err)
		}
	}
	if len(succeeded) != balance/cost {
		t.Fatalf("successful reservations = %d, want %d", len(succeeded), balance/cost)
	}
	var account model.CreditAccount
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Balance != 0 || account.TotalSpent != balance {
		t.Fatalf("account after reserve = %+v", account)
	}

	refunds := make(chan *repository.GenerationRefundResult, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := repo.RefundByRequest(succeeded[0].RequestID, "failed")
			refunds <- result
			errs <- err
		}()
	}
	wait.Wait()
	close(refunds)
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, repository.ErrGenerationJobNotFound) {
			t.Fatalf("refund error = %v", err)
		}
	}
	refunded := 0
	for result := range refunds {
		if result != nil && result.Refunded {
			refunded++
		}
	}
	if refunded != 1 {
		t.Fatalf("applied refunds = %d, want 1", refunded)
	}
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&account).Error; err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if account.Balance != cost || account.TotalSpent != balance-cost {
		t.Fatalf("account after refund = %+v", account)
	}
}

func TestAutoGenerationSettlesActualCostAndRefundsAsyncFailureOnce(t *testing.T) {
	db := openAdminCreditTestDB(t)
	const tenantID, userID = 51, 52
	if err := db.Create(&model.CreditAccount{TenantID: tenantID, UserID: userID, Balance: 100, TotalEarned: 100}).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
	repo := repository.NewGenerationJobRepo(db)
	job := &model.GenerationJob{RequestID: "auto-settlement", TenantID: tenantID, UserID: userID, Capability: "video", ModelName: "video-model", AutoRoutingPoolID: 3, AuthorizedAmount: 20, BillingAmount: 20}
	if _, err := repo.Reserve(job, "智能路由预留", "{}"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	settlement, err := repo.Settle(job.RequestID, repository.GenerationSettlementInput{Amount: 7, ChannelID: 2, ChannelModelID: 22, ChannelName: "Channel B", ChannelBaseURL: "https://b.example.com", VideoRoute: "openai", UpstreamTaskID: "task-1"})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if settlement.Amount != 7 || settlement.Refund != 13 || settlement.Balance != 93 {
		t.Fatalf("unexpected settlement: %#v", settlement)
	}
	var stored model.GenerationJob
	if err := db.Where("request_id = ?", job.RequestID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.GenerationJobPending || stored.AuthorizedAmount != 20 || stored.BillingAmount != 7 || stored.ChannelModelID != 22 || stored.UpstreamTaskID != "task-1" {
		t.Fatalf("unexpected settled job: %#v", stored)
	}
	first, err := repo.RefundByTask(tenantID, userID, 22, "task-1", "failed")
	if err != nil || !first.Refunded || first.Amount != 7 || first.Balance != 100 {
		t.Fatalf("first async refund: %#v, %v", first, err)
	}
	second, err := repo.RefundByTask(tenantID, userID, 22, "task-1", "failed again")
	if err != nil || !second.AlreadyClosed || second.Refunded || second.Balance != 100 {
		t.Fatalf("duplicate async refund: %#v, %v", second, err)
	}
}
