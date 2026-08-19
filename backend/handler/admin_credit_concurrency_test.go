package handler

import (
	"errors"
	"sync"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

func TestAdministratorCreditAdjustment_serializes_concurrent_mutations(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	requests := []service.AdministratorCreditAdjustment{
		{
			OperatorUserID:   fixture.operatorUserID,
			OperatorTenantID: fixture.tenantID,
			TargetUserID:     fixture.targetUserID,
			Mode:             model.CreditAdjustmentAdd,
			Amount:           20,
		},
		{
			OperatorUserID:   fixture.operatorUserID,
			OperatorTenantID: fixture.tenantID,
			TargetUserID:     fixture.targetUserID,
			Mode:             model.CreditAdjustmentAdd,
			Amount:           30,
		},
	}
	start := make(chan struct{})
	errors := make(chan error, len(requests))
	var waitGroup sync.WaitGroup
	for _, request := range requests {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := fixture.creditService.AdjustAdministratorCredits(request)
			errors <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent adjustment: %v", err)
		}
	}

	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", fixture.targetUserID).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	if account.Balance != 150 || account.TotalEarned != 150 || account.TotalSpent != 0 {
		t.Fatalf("unexpected account: %+v", account)
	}
	assertTransactionCount(t, fixture.db, 2)
}

func TestAdministratorCreditAdjustment_serializes_first_account_creation(t *testing.T) {
	fixture := newAdminCreditFixture(t, 100)
	if err := fixture.db.Unscoped().Where("user_id = ?", fixture.targetUserID).Delete(&model.CreditAccount{}).Error; err != nil {
		t.Fatalf("remove seeded account: %v", err)
	}
	request := service.AdministratorCreditAdjustment{
		OperatorUserID:   fixture.operatorUserID,
		OperatorTenantID: fixture.tenantID,
		TargetUserID:     fixture.targetUserID,
		Mode:             model.CreditAdjustmentAdd,
		Amount:           10,
	}
	start := make(chan struct{})
	errors := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := fixture.creditService.AdjustAdministratorCredits(request)
			errors <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent adjustment: %v", err)
		}
	}

	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", fixture.targetUserID).First(&account).Error; err != nil {
		t.Fatalf("read account: %v", err)
	}
	if account.Balance != 20 || account.TotalEarned != 20 || account.TotalSpent != 0 {
		t.Fatalf("unexpected account: %+v", account)
	}
	assertTransactionCount(t, fixture.db, 2)
}

func TestAccountDeletion_serializes_with_first_credit_account_adjustment(t *testing.T) {
	// Given
	fixture := newAdminCreditFixture(t, 100)
	if err := fixture.db.Unscoped().Where("user_id = ?", fixture.targetUserID).Delete(&model.CreditAccount{}).Error; err != nil {
		t.Fatalf("remove seeded account: %v", err)
	}
	deletePaused := make(chan struct{})
	resumeDelete := make(chan struct{})
	var pauseOnce sync.Once
	const deleteCallback = "test_pause_delete_after_credit_account"
	if err := fixture.db.Callback().Delete().After("gorm:delete").Register(deleteCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "CreditAccount" {
			pauseOnce.Do(func() {
				close(deletePaused)
				<-resumeDelete
			})
		}
	}); err != nil {
		t.Fatalf("register delete pause: %v", err)
	}
	t.Cleanup(func() { fixture.db.Callback().Delete().Remove(deleteCallback) })

	deleteErrors := make(chan error, 1)
	go func() {
		deleteErrors <- repository.NewUserRepo(fixture.db).DeleteAccount(fixture.targetUserID)
	}()
	select {
	case <-deletePaused:
	case <-time.After(5 * time.Second):
		close(resumeDelete)
		t.Fatal("delete did not reach the credit-account step")
	}

	probe := fixture.db.Begin()
	if probe.Error != nil {
		close(resumeDelete)
		t.Fatalf("begin target-user lock probe: %v", probe.Error)
	}
	var target model.User
	probeErr := probe.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).First(&target, fixture.targetUserID).Error
	if err := probe.Rollback().Error; err != nil {
		close(resumeDelete)
		t.Fatalf("rollback target-user lock probe: %v", err)
	}
	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(probeErr, &mysqlErr) || mysqlErr.Number != 3572 {
		close(resumeDelete)
		t.Fatalf("delete must hold target-user lock, probe error=%v", probeErr)
	}

	adjustmentStarted := make(chan struct{})
	var adjustmentStartOnce sync.Once
	const queryCallback = "test_observe_adjustment_user_lock"
	if err := fixture.db.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" {
			adjustmentStartOnce.Do(func() { close(adjustmentStarted) })
		}
	}); err != nil {
		close(resumeDelete)
		t.Fatalf("register adjustment observer: %v", err)
	}
	t.Cleanup(func() { fixture.db.Callback().Query().Remove(queryCallback) })
	adjustmentErrors := make(chan error, 1)
	go func() {
		_, err := fixture.creditService.AdjustAdministratorCredits(service.AdministratorCreditAdjustment{
			OperatorUserID:   fixture.operatorUserID,
			OperatorTenantID: fixture.tenantID,
			TargetUserID:     fixture.targetUserID,
			Mode:             model.CreditAdjustmentAdd,
			Amount:           10,
		})
		adjustmentErrors <- err
	}()
	select {
	case <-adjustmentStarted:
	case <-time.After(5 * time.Second):
		close(resumeDelete)
		t.Fatal("adjustment did not attempt the target-user lock")
	}

	close(resumeDelete)

	var deleteErr error
	select {
	case deleteErr = <-deleteErrors:
	case <-time.After(5 * time.Second):
		t.Fatal("account deletion did not complete")
	}
	var adjustmentErr error
	select {
	case adjustmentErr = <-adjustmentErrors:
	case <-time.After(5 * time.Second):
		t.Fatal("credit adjustment did not complete")
	}

	// Then
	var userCount, accountCount, transactionCount int64
	if err := fixture.db.Unscoped().Model(&model.User{}).Where("id = ?", fixture.targetUserID).Count(&userCount).Error; err != nil {
		t.Fatalf("count target users: %v", err)
	}
	if err := fixture.db.Unscoped().Model(&model.CreditAccount{}).Where("user_id = ?", fixture.targetUserID).Count(&accountCount).Error; err != nil {
		t.Fatalf("count target credit accounts: %v", err)
	}
	if err := fixture.db.Unscoped().Model(&model.CreditTransaction{}).Count(&transactionCount).Error; err != nil {
		t.Fatalf("count target credit transactions: %v", err)
	}
	if userCount == 0 {
		if deleteErr != nil || accountCount != 0 || transactionCount != 0 {
			t.Fatalf("deleted target left owned rows: delete=%v adjust=%v users=%d accounts=%d transactions=%d", deleteErr, adjustmentErr, userCount, accountCount, transactionCount)
		}
		return
	}
	if userCount != 1 || adjustmentErr != nil || accountCount != 1 || transactionCount != 1 {
		t.Fatalf("existing target lacks completed adjustment: delete=%v adjust=%v users=%d accounts=%d transactions=%d", deleteErr, adjustmentErr, userCount, accountCount, transactionCount)
	}
}
