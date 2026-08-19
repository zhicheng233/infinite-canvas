package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"infinite-canvas-server/model"
)

func TestAdministratorCreditAdjustment_add_deduct_and_set_write_one_adjustment(t *testing.T) {
	tests := []struct {
		name            string
		body            func(adminCreditFixture) string
		wantDelta       int
		wantBalance     int
		wantTotalEarned int
		wantTotalSpent  int
		wantMode        string
	}{
		{name: "add", body: func(fixture adminCreditFixture) string {
			return fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":25}`, fixture.targetUserID)
		}, wantDelta: 25, wantBalance: 125, wantTotalEarned: 125, wantMode: "add"},
		{name: "deduct", body: func(fixture adminCreditFixture) string {
			return fmt.Sprintf(`{"user_id":%d,"mode":"deduct","amount":40}`, fixture.targetUserID)
		}, wantDelta: -40, wantBalance: 60, wantTotalEarned: 100, wantTotalSpent: 40, wantMode: "deduct"},
		{name: "set", body: func(fixture adminCreditFixture) string {
			return fmt.Sprintf(`{"user_id":%d,"mode":"set","target_balance":60}`, fixture.targetUserID)
		}, wantDelta: -40, wantBalance: 60, wantTotalEarned: 100, wantTotalSpent: 40, wantMode: "set"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAdminCreditFixture(t, 100)

			response := fixture.request(t, fixture.handler.AdjustTenantCredits, test.body(fixture))

			if response.Code != 0 || response.Data.UserID != fixture.targetUserID || response.Data.Amount != test.wantDelta || response.Data.Balance != test.wantBalance {
				t.Fatalf("unexpected response: %+v", response)
			}
			var account model.CreditAccount
			if err := fixture.db.Where("user_id = ?", fixture.targetUserID).First(&account).Error; err != nil {
				t.Fatalf("read account: %v", err)
			}
			if account.Balance != test.wantBalance || account.TotalEarned != test.wantTotalEarned || account.TotalSpent != test.wantTotalSpent {
				t.Fatalf("unexpected account: %+v", account)
			}
			var transactions []model.CreditTransaction
			if err := fixture.db.Where("account_id = ?", account.ID).Find(&transactions).Error; err != nil {
				t.Fatalf("read transactions: %v", err)
			}
			if len(transactions) != 1 {
				t.Fatalf("transaction count = %d, want 1", len(transactions))
			}
			transaction := transactions[0]
			if transaction.Type != model.TxTypeAdjust || transaction.Amount != test.wantDelta || transaction.BalanceBefore == nil || *transaction.BalanceBefore != 100 || transaction.BalanceAfter != test.wantBalance {
				t.Fatalf("unexpected transaction: %+v", transaction)
			}
			var metadata creditAdjustmentMetadata
			if err := json.Unmarshal([]byte(transaction.Metadata), &metadata); err != nil {
				t.Fatalf("decode metadata: %v", err)
			}
			if metadata.Scene != "后台调整" || metadata.OperatorUserID != fixture.operatorUserID || metadata.TargetUserID != fixture.targetUserID || metadata.Mode != test.wantMode || metadata.Amount != test.wantDelta || metadata.BalanceBefore != 100 || metadata.BalanceAfter != test.wantBalance || metadata.TargetBalance != test.wantBalance {
				t.Fatalf("unexpected metadata: %+v", metadata)
			}
		})
	}
}

func TestAdministratorCreditAdjustment_rejects_no_op_set(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	body := fmt.Sprintf(`{"user_id":%d,"mode":"set","target_balance":100}`, fixture.targetUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	assertCreditAdjustmentUnchanged(t, fixture, response, 400, fixture.targetUserID, 100)
	if response.Msg != "目标余额不能与当前余额相同" {
		t.Fatalf("response message = %q, want no-op target error", response.Msg)
	}
}

func TestAdministratorCreditAdjustment_no_op_set_rolls_back_new_account(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	if err := fixture.db.Unscoped().Where("user_id = ?", fixture.targetUserID).Delete(&model.CreditAccount{}).Error; err != nil {
		t.Fatalf("remove seeded account: %v", err)
	}
	body := fmt.Sprintf(`{"user_id":%d,"mode":"set","target_balance":0}`, fixture.targetUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	if response.Code != 400 {
		t.Fatalf("response code = %d, want 400: %+v", response.Code, response)
	}
	var accountCount int64
	if err := fixture.db.Model(&model.CreditAccount{}).Where("user_id = ?", fixture.targetUserID).Count(&accountCount).Error; err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accountCount != 0 {
		t.Fatalf("account count = %d, want 0", accountCount)
	}
	assertTransactionCount(t, fixture.db, 0)
}

func TestAdministratorCreditAdjustment_rejects_insufficient_balance(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	body := fmt.Sprintf(`{"user_id":%d,"mode":"deduct","amount":101}`, fixture.targetUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	assertCreditAdjustmentUnchanged(t, fixture, response, 400, fixture.targetUserID, 100)
}

func TestAdministratorCreditAdjustment_rejects_omitted_mode(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	body := fmt.Sprintf(`{"user_id":%d,"amount":-25}`, fixture.targetUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	assertCreditAdjustmentUnchanged(t, fixture, response, 400, fixture.targetUserID, 100)
	if response.Msg != "无效的积分调整" {
		t.Fatalf("response message = %q, want explicit-mode error", response.Msg)
	}
}

func TestAdministratorCreditAdjustment_rejects_foreign_tenant_before_account_creation(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	body := fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":25}`, fixture.foreignUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	if response.Code != 403 {
		t.Fatalf("response code = %d, want 403: %+v", response.Code, response)
	}
	var accountCount int64
	if err := fixture.db.Model(&model.CreditAccount{}).Where("user_id = ?", fixture.foreignUserID).Count(&accountCount).Error; err != nil {
		t.Fatalf("count foreign account: %v", err)
	}
	if accountCount != 0 {
		t.Fatalf("foreign account count = %d, want 0", accountCount)
	}
	assertTransactionCount(t, fixture.db, 0)
}

func TestSuperAdministratorCreditAdjustment_allows_explicit_cross_tenant_target(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	body := fmt.Sprintf(`{"user_id":%d,"mode":"set","target_balance":30}`, fixture.foreignUserID)

	response := fixture.request(t, fixture.handler.AdjustCredits, body)

	if response.Code != 0 || response.Data.Amount != 30 || response.Data.Balance != 30 {
		t.Fatalf("unexpected response: %+v", response)
	}
	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", fixture.foreignUserID).First(&account).Error; err != nil {
		t.Fatalf("read foreign account: %v", err)
	}
	if account.TenantID != fixture.tenantID+1 || account.Balance != 30 || account.TotalEarned != 30 {
		t.Fatalf("unexpected foreign account: %+v", account)
	}
	assertTransactionCount(t, fixture.db, 1)
}

func TestAdministratorCreditAdjustment_rolls_back_when_ledger_insert_fails(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	forcedError := errors.New("forced transaction insert failure")
	if err := fixture.db.Callback().Create().Before("gorm:create").Register("force_credit_transaction_failure", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "CreditTransaction" {
			tx.AddError(forcedError)
		}
	}); err != nil {
		t.Fatalf("register create failure callback: %v", err)
	}
	body := fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":25}`, fixture.targetUserID)

	response := fixture.request(t, fixture.handler.AdjustTenantCredits, body)

	assertCreditAdjustmentUnchanged(t, fixture, response, 500, fixture.targetUserID, 100)
}

func TestAdministratorCreditAdjustment_rejects_invalid_requests(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	tests := []struct {
		name string
		body string
		code int
	}{
		{name: "unknown mode", body: fmt.Sprintf(`{"user_id":%d,"mode":"replace","amount":10}`, fixture.targetUserID), code: 400},
		{name: "zero add", body: fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":0}`, fixture.targetUserID), code: 400},
		{name: "negative target", body: fmt.Sprintf(`{"user_id":%d,"mode":"set","target_balance":-1}`, fixture.targetUserID), code: 400},
		{name: "missing target balance", body: fmt.Sprintf(`{"user_id":%d,"mode":"set"}`, fixture.targetUserID), code: 400},
		{name: "target with add", body: fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":10,"target_balance":110}`, fixture.targetUserID), code: 400},
		{name: "overlong note", body: fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":10,"note":"%s"}`, fixture.targetUserID, strings.Repeat("a", 501)), code: 400},
		{name: "missing target user", body: `{"user_id":0,"mode":"add","amount":10}`, code: 400},
		{name: "fractional amount", body: fmt.Sprintf(`{"user_id":%d,"mode":"add","amount":1.5}`, fixture.targetUserID), code: 400},
		{name: "missing user", body: `{"user_id":999999,"mode":"add","amount":10}`, code: 404},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := fixture.request(t, fixture.handler.AdjustTenantCredits, test.body)
			if response.Code != test.code {
				t.Fatalf("response code = %d, want %d: %+v", response.Code, test.code, response)
			}
		})
	}
	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", fixture.targetUserID).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	if account.Balance != 100 {
		t.Fatalf("balance = %d, want 100", account.Balance)
	}
	assertTransactionCount(t, fixture.db, 0)
}

func assertCreditAdjustmentUnchanged(t *testing.T, fixture adminCreditFixture, response creditAdjustmentHTTPResponse, wantCode int, userID uint, wantBalance int) {
	t.Helper()
	if response.Code != wantCode {
		t.Fatalf("response code = %d, want %d: %+v", response.Code, wantCode, response)
	}
	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	if account.Balance != wantBalance || account.TotalEarned != wantBalance || account.TotalSpent != 0 {
		t.Fatalf("account changed: %+v", account)
	}
	assertTransactionCount(t, fixture.db, 0)
}

func assertTransactionCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var transactionCount int64
	if err := db.Model(&model.CreditTransaction{}).Count(&transactionCount).Error; err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if transactionCount != want {
		t.Fatalf("transaction count = %d, want %d", transactionCount, want)
	}
}
