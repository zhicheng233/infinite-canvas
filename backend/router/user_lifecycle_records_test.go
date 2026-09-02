package router

import (
	"fmt"
	"strconv"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type lifecycleOwnedCounts struct {
	transactions int64
	accounts     int64
	projects     int64
	generations  int64
	jobs         int64
	attempts     int64
	recharges    int64
	modelLogs    int64
	users        int64
}

func (fixture *lifecycleFixture) seedOwnedRecords(t *testing.T, user *model.User) (uint, string) {
	t.Helper()
	account := &model.CreditAccount{TenantID: user.TenantID, UserID: user.ID, Balance: 10, TotalEarned: 10}
	if err := fixture.db.Create(account).Error; err != nil {
		t.Fatalf("seed credit account: %v", err)
	}
	transaction := &model.CreditTransaction{AccountID: account.ID, Type: model.TxTypeEarn, Amount: 10, BalanceAfter: 10}
	project := &model.CanvasProject{TenantID: user.TenantID, UserID: user.ID, ProjectID: "task4-project-" + fixture.suffix + "-" + strconv.FormatUint(uint64(user.ID), 10), Title: "task4"}
	generation := &model.GenerationRecord{TenantID: user.TenantID, UserID: user.ID, RecordID: "task4-record-" + fixture.suffix, Type: "image", Payload: "{}"}
	requestID := "task4-request-" + fixture.suffix + "-" + strconv.FormatUint(uint64(user.ID), 10)
	job := &model.GenerationJob{RequestID: requestID, TenantID: user.TenantID, UserID: user.ID, Capability: "image", ModelName: "task4", Status: model.GenerationJobSucceeded}
	attempt := &model.GenerationAttempt{RequestID: requestID, AttemptNo: 1, Success: true, CountsForHealth: true}
	recharge := &model.RechargeOrder{TenantID: user.TenantID, UserID: user.ID, Amount: 1, Credits: 10, Status: "pending"}
	modelLog := &model.ModelCallLog{TenantID: user.TenantID, UserID: user.ID, Username: user.Username, Generation: "image", Model: "task4"}
	for name, record := range map[string]any{
		"credit transaction": transaction,
		"canvas project":     project,
		"generation record":  generation,
		"generation job":     job,
		"generation attempt": attempt,
		"recharge order":     recharge,
		"model call log":     modelLog,
	} {
		if err := fixture.db.Create(record).Error; err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	if err := fixture.db.Delete(transaction).Error; err != nil {
		t.Fatalf("soft-delete credit transaction fixture: %v", err)
	}
	return account.ID, requestID
}

func (fixture *lifecycleFixture) countOwnedRecords(t *testing.T, userID, accountID uint, requestID string) lifecycleOwnedCounts {
	t.Helper()
	counts := lifecycleOwnedCounts{}
	queries := []struct {
		model any
		where string
		value any
		count *int64
	}{
		{&model.CreditTransaction{}, "account_id = ?", accountID, &counts.transactions},
		{&model.CreditAccount{}, "user_id = ?", userID, &counts.accounts},
		{&model.CanvasProject{}, "user_id = ?", userID, &counts.projects},
		{&model.GenerationRecord{}, "user_id = ?", userID, &counts.generations},
		{&model.GenerationJob{}, "user_id = ?", userID, &counts.jobs},
		{&model.GenerationAttempt{}, "request_id = ?", requestID, &counts.attempts},
		{&model.RechargeOrder{}, "user_id = ?", userID, &counts.recharges},
		{&model.ModelCallLog{}, "user_id = ?", userID, &counts.modelLogs},
		{&model.User{}, "id = ?", userID, &counts.users},
	}
	for _, query := range queries {
		if err := fixture.db.Unscoped().Model(query.model).Where(query.where, query.value).Count(query.count).Error; err != nil {
			t.Fatalf("count lifecycle-owned records: %v", err)
		}
	}
	return counts
}

func (fixture *lifecycleFixture) observeDeletes(t *testing.T, failSchema string) *[]string {
	t.Helper()
	deletedSchemas := []string{}
	callbackName := fmt.Sprintf("task4_delete_observer_%s", fixture.suffix)
	if err := fixture.db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		deletedSchemas = append(deletedSchemas, tx.Statement.Schema.Name)
		if tx.Statement.Schema.Name == failSchema {
			tx.AddError(fmt.Errorf("forced %s delete failure", failSchema))
		}
	}); err != nil {
		t.Fatalf("register delete observer: %v", err)
	}
	t.Cleanup(func() { fixture.db.Callback().Delete().Remove(callbackName) })
	return &deletedSchemas
}
