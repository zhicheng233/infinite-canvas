package router

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"infinite-canvas-server/model"
)

func TestAdminCreditMutationRoutes_writeOnlyAdjustmentLedgers(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		token      func(*adminCreditRouteFixture) string
		target     func(*adminCreditRouteFixture) uint
		body       func(uint) map[string]any
		wantTenant func(*adminCreditRouteFixture) uint
	}{
		{name: "legacy tenant recharge", path: "/backend-api/credits/recharge", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }, target: func(f *adminCreditRouteFixture) uint { return f.target.ID }, body: func(userID uint) map[string]any {
			return map[string]any{"user_id": userID, "amount": 25, "note": "legacy"}
		}, wantTenant: func(f *adminCreditRouteFixture) uint { return f.target.TenantID }},
		{name: "tenant adjustment", path: "/backend-api/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }, target: func(f *adminCreditRouteFixture) uint { return f.target.ID }, body: func(userID uint) map[string]any {
			return map[string]any{"user_id": userID, "mode": "add", "amount": 25}
		}, wantTenant: func(f *adminCreditRouteFixture) uint { return f.target.TenantID }},
		{name: "super administrator adjustment", path: "/backend-api/admin/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.superToken }, target: func(f *adminCreditRouteFixture) uint { return f.foreign.ID }, body: func(userID uint) map[string]any {
			return map[string]any{"user_id": userID, "mode": "add", "amount": 25}
		}, wantTenant: func(f *adminCreditRouteFixture) uint { return f.foreign.TenantID }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAdminCreditRouteFixture(t)
			userID := test.target(fixture)

			response := fixture.post(t, test.path, test.token(fixture), test.body(userID))

			if response.Code != 0 {
				t.Fatalf("POST %s code=%d msg=%q", test.path, response.Code, response.Msg)
			}
			var account model.CreditAccount
			if err := fixture.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
				t.Fatalf("read adjusted account: %v", err)
			}
			if account.TenantID != test.wantTenant(fixture) || account.Balance != 125 || account.TotalEarned != 125 || account.TotalSpent != 0 {
				t.Fatalf("unexpected adjusted account: %+v", account)
			}
			var transactions []model.CreditTransaction
			if err := fixture.db.Where("account_id = ?", account.ID).Find(&transactions).Error; err != nil {
				t.Fatalf("read adjustment ledgers: %v", err)
			}
			if len(transactions) != 1 || transactions[0].Type != model.TxTypeAdjust || transactions[0].Amount != 25 || transactions[0].BalanceBefore == nil || *transactions[0].BalanceBefore != 100 || transactions[0].BalanceAfter != 125 || transactions[0].RefType != "adjust" {
				t.Fatalf("unexpected administrator ledgers: %+v", transactions)
			}
			var metadata struct {
				Mode string `json:"mode"`
			}
			if err := json.Unmarshal([]byte(transactions[0].Metadata), &metadata); err != nil {
				t.Fatalf("decode adjustment metadata: %v", err)
			}
			if metadata.Mode != "add" {
				t.Fatalf("administrator adjustment mode=%q, want add", metadata.Mode)
			}
		})
	}
}

func TestAdminCreditMutationRoutes_rejectOmittedModeWithoutMutation(t *testing.T) {
	for _, route := range []struct {
		name  string
		path  string
		token func(*adminCreditRouteFixture) string
	}{
		{name: "tenant adjustment", path: "/backend-api/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }},
		{name: "super administrator adjustment", path: "/backend-api/admin/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.superToken }},
	} {
		t.Run(route.name, func(t *testing.T) {
			fixture := newAdminCreditRouteFixture(t)
			response := fixture.post(t, route.path, route.token(fixture), map[string]any{
				"user_id": fixture.target.ID,
				"amount":  -25,
			})

			if response.Code != http.StatusBadRequest || response.Msg != "无效的积分调整" {
				t.Fatalf("POST %s code=%d msg=%q, want business 400 explicit-mode error", route.path, response.Code, response.Msg)
			}
			fixture.assertAccountUnchanged(t, fixture.target.ID)
		})
	}
}

func TestTenantScopedCreditMutationRoutes_rejectForeignTargetForEveryAdministrator(t *testing.T) {
	for _, path := range []string{"/backend-api/credits/recharge", "/backend-api/credits/adjust"} {
		for _, actor := range []struct {
			name  string
			token func(*adminCreditRouteFixture) string
		}{
			{name: "tenant administrator", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }},
			{name: "super administrator", token: func(f *adminCreditRouteFixture) string { return f.superToken }},
		} {
			t.Run(path+" "+actor.name, func(t *testing.T) {
				fixture := newAdminCreditRouteFixture(t)
				body := map[string]any{"user_id": fixture.foreign.ID, "amount": 25}
				if path == "/backend-api/credits/adjust" {
					body["mode"] = "add"
				}

				response := fixture.post(t, path, actor.token(fixture), body)

				if response.Code != http.StatusForbidden {
					t.Fatalf("POST %s code=%d msg=%q, want 403", path, response.Code, response.Msg)
				}
				fixture.assertAccountUnchanged(t, fixture.foreign.ID)
			})
		}
	}
}

func TestSuperAdminCreditMutationRoute_rejectsTenantAdministrator(t *testing.T) {
	fixture := newAdminCreditRouteFixture(t)
	body := map[string]any{"user_id": fixture.foreign.ID, "mode": "add", "amount": 25}

	response := fixture.post(t, "/backend-api/admin/credits/adjust", fixture.tenantToken, body)

	if response.Code != http.StatusForbidden {
		t.Fatalf("tenant administrator super route code=%d msg=%q, want 403", response.Code, response.Msg)
	}
	fixture.assertAccountUnchanged(t, fixture.foreign.ID)
}

func TestAdminCreditMutationRoutes_rollBackWhenLedgerInsertFails(t *testing.T) {
	for _, route := range []struct {
		path  string
		token func(*adminCreditRouteFixture) string
	}{
		{path: "/backend-api/credits/recharge", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }},
		{path: "/backend-api/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.tenantToken }},
		{path: "/backend-api/admin/credits/adjust", token: func(f *adminCreditRouteFixture) string { return f.superToken }},
	} {
		t.Run(route.path, func(t *testing.T) {
			fixture := newAdminCreditRouteFixture(t)
			forcedError := errors.New("forced administrator ledger failure")
			if err := fixture.db.Callback().Create().Before("gorm:create").Register("force_admin_credit_ledger_failure", func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "CreditTransaction" {
					tx.AddError(forcedError)
				}
			}); err != nil {
				t.Fatalf("register ledger failure callback: %v", err)
			}
			body := map[string]any{"user_id": fixture.target.ID, "amount": 25}
			if route.path != "/backend-api/credits/recharge" {
				body["mode"] = "add"
			}

			response := fixture.post(t, route.path, route.token(fixture), body)

			if response.Code != http.StatusInternalServerError {
				t.Fatalf("POST %s code=%d msg=%q, want 500", route.path, response.Code, response.Msg)
			}
			fixture.assertAccountUnchanged(t, fixture.target.ID)
		})
	}
}
