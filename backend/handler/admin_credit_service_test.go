package handler

import (
	"errors"
	"testing"

	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

func TestCreditService_AdjustAdministratorCredits_rejects_omitted_mode_without_mutation(t *testing.T) {
	tests := []struct {
		name   string
		amount int
	}{
		{name: "positive amount", amount: 25},
		{name: "negative amount", amount: -25},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAdminCreditFixture(t, 100)

			_, err := fixture.creditService.AdjustAdministratorCredits(service.AdministratorCreditAdjustment{
				OperatorUserID:   fixture.operatorUserID,
				OperatorTenantID: fixture.tenantID,
				TargetUserID:     fixture.targetUserID,
				Amount:           test.amount,
			})

			if !errors.Is(err, service.ErrInvalidCreditAdjustment) {
				t.Fatalf("AdjustAdministratorCredits error = %v, want ErrInvalidCreditAdjustment", err)
			}
			var account model.CreditAccount
			if err := fixture.db.Where("user_id = ?", fixture.targetUserID).First(&account).Error; err != nil {
				t.Fatalf("read account: %v", err)
			}
			if account.Balance != 100 || account.TotalEarned != 100 || account.TotalSpent != 0 {
				t.Fatalf("account changed after omitted mode: %+v", account)
			}
			assertTransactionCount(t, fixture.db, 0)
		})
	}
}
