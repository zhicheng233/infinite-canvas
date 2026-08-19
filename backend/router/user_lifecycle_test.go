package router

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

func TestUserLifecycleRoutes_resetPasswordAllowsNewLogin(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	path := fmt.Sprintf("/backend-api/users/%d/password", fixture.target.ID)

	// When
	recorder, response := fixture.request(t, lifecycleRequest{method: http.MethodPut, path: path, token: fixture.actorToken, body: map[string]string{"new_password": "ResetPass2"}})

	// Then
	if recorder.Code != http.StatusOK || response.Code != 0 {
		t.Fatalf("reset password status=%d code=%d msg=%q", recorder.Code, response.Code, response.Msg)
	}
	if body := recorder.Body.String(); body == "" || strings.Contains(body, "ResetPass2") {
		t.Fatalf("reset response exposed password: %s", body)
	}
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "TargetPass1", wantCode: http.StatusBadRequest})
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "ResetPass2"})
}

func TestUserLifecycleRoutes_resetPasswordRejectsWeakSelfForeignAndMissingTargets(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	tests := []struct {
		name     string
		targetID uint
		password string
	}{
		{name: "weak password", targetID: fixture.target.ID, password: "weak"},
		{name: "self target", targetID: fixture.actor.ID, password: "ResetPass2"},
		{name: "foreign tenant target", targetID: fixture.foreign.ID, password: "ResetPass2"},
		{name: "missing target", targetID: fixture.foreign.ID + 1_000_000, password: "ResetPass2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, response := fixture.request(t, lifecycleRequest{method: http.MethodPut, path: fmt.Sprintf("/backend-api/users/%d/password", test.targetID), token: fixture.actorToken, body: map[string]string{"new_password": test.password}})

			// Then
			if response.Code == 0 {
				t.Fatalf("reset target %d unexpectedly succeeded", test.targetID)
			}
		})
	}
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "TargetPass1"})
}

func TestUserLifecycleRoutes_deleteRemovesEveryOwnedRecordInOrder(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	targetAccountID := fixture.seedOwnedRecords(t, fixture.target)
	foreignAccountID := fixture.seedOwnedRecords(t, fixture.foreign)
	deletedSchemas := fixture.observeDeletes(t, "")

	// When
	recorder, response := fixture.request(t, lifecycleRequest{method: http.MethodDelete, path: fmt.Sprintf("/backend-api/users/%d", fixture.target.ID), token: fixture.actorToken})

	// Then
	if recorder.Code != http.StatusOK || response.Code != 0 {
		t.Fatalf("delete account status=%d code=%d msg=%q", recorder.Code, response.Code, response.Msg)
	}
	wantOrder := []string{"CreditTransaction", "CreditAccount", "CanvasProject", "GenerationRecord", "RechargeOrder", "ModelCallLog", "User"}
	if !reflect.DeepEqual(*deletedSchemas, wantOrder) {
		t.Fatalf("delete order=%v, want %v", *deletedSchemas, wantOrder)
	}
	if got := fixture.countOwnedRecords(t, fixture.target.ID, targetAccountID); got != (lifecycleOwnedCounts{}) {
		t.Fatalf("target owned rows remain after hard delete: %+v", got)
	}
	if got := fixture.countOwnedRecords(t, fixture.foreign.ID, foreignAccountID); got != (lifecycleOwnedCounts{transactions: 1, accounts: 1, projects: 1, generations: 1, recharges: 1, modelLogs: 1, users: 1}) {
		t.Fatalf("foreign tenant rows changed: %+v", got)
	}
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "TargetPass1", wantCode: http.StatusBadRequest})
	meRecorder, _ := fixture.request(t, lifecycleRequest{method: http.MethodGet, path: "/backend-api/auth/me", token: fixture.targetToken})
	if meRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("deleted target token status=%d, want %d", meRecorder.Code, http.StatusUnauthorized)
	}
}

func TestUserLifecycleRoutes_deleteRejectsSelfForeignAndMissingTargets(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	tests := []struct {
		name     string
		targetID uint
	}{
		{name: "self target", targetID: fixture.actor.ID},
		{name: "foreign tenant target", targetID: fixture.foreign.ID},
		{name: "missing target", targetID: fixture.foreign.ID + 1_000_000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, response := fixture.request(t, lifecycleRequest{method: http.MethodDelete, path: fmt.Sprintf("/backend-api/users/%d", test.targetID), token: fixture.actorToken})

			// Then
			if response.Code == 0 {
				t.Fatalf("delete target %d unexpectedly succeeded", test.targetID)
			}
		})
	}
	fixture.login(t, lifecycleLogin{username: fixture.actor.Username, password: "ActorPass1"})
	fixture.login(t, lifecycleLogin{username: fixture.foreign.Username, password: "ForeignPass1"})
}

func TestUserLifecycleRoutes_rejectsAlreadyDeletedTarget(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	if err := fixture.db.Delete(fixture.target).Error; err != nil {
		t.Fatalf("soft-delete target: %v", err)
	}

	// When
	_, resetResponse := fixture.request(t, lifecycleRequest{method: http.MethodPut, path: fmt.Sprintf("/backend-api/users/%d/password", fixture.target.ID), token: fixture.actorToken, body: map[string]string{"new_password": "ResetPass2"}})
	_, deleteResponse := fixture.request(t, lifecycleRequest{method: http.MethodDelete, path: fmt.Sprintf("/backend-api/users/%d", fixture.target.ID), token: fixture.actorToken})

	// Then
	if resetResponse.Code != http.StatusNotFound || deleteResponse.Code != http.StatusNotFound {
		t.Fatalf("deleted target reset code=%d delete code=%d, want 404", resetResponse.Code, deleteResponse.Code)
	}
	var count int64
	if err := fixture.db.Unscoped().Model(&model.User{}).Where("id = ?", fixture.target.ID).Count(&count).Error; err != nil {
		t.Fatalf("count soft-deleted target: %v", err)
	}
	if count != 1 {
		t.Fatalf("soft-deleted target count=%d, want 1", count)
	}
}

func TestUserLifecycleRoutes_rejectsDeletedActingAdministrator(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	if err := fixture.db.Delete(fixture.actor).Error; err != nil {
		t.Fatalf("soft-delete acting administrator: %v", err)
	}

	// When
	_, response := fixture.request(t, lifecycleRequest{method: http.MethodPut, path: fmt.Sprintf("/backend-api/users/%d/password", fixture.target.ID), token: fixture.actorToken, body: map[string]string{"new_password": "ResetPass2"}})

	// Then
	if response.Code != http.StatusForbidden {
		t.Fatalf("deleted acting administrator reset code=%d, want 403", response.Code)
	}
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "TargetPass1"})
}

func TestUserLifecycleRoutes_deleteRollsBackWhenLateStepFails(t *testing.T) {
	// Given
	fixture := newLifecycleFixture(t)
	accountID := fixture.seedOwnedRecords(t, fixture.target)
	wantCounts := fixture.countOwnedRecords(t, fixture.target.ID, accountID)
	deletedSchemas := fixture.observeDeletes(t, "ModelCallLog")

	// When
	_, response := fixture.request(t, lifecycleRequest{method: http.MethodDelete, path: fmt.Sprintf("/backend-api/users/%d", fixture.target.ID), token: fixture.actorToken})

	// Then
	if response.Code == 0 {
		t.Fatal("delete unexpectedly succeeded after forced late failure")
	}
	wantAttemptedOrder := []string{"CreditTransaction", "CreditAccount", "CanvasProject", "GenerationRecord", "RechargeOrder", "ModelCallLog"}
	if !reflect.DeepEqual(*deletedSchemas, wantAttemptedOrder) {
		t.Fatalf("attempted delete order=%v, want %v", *deletedSchemas, wantAttemptedOrder)
	}
	if got := fixture.countOwnedRecords(t, fixture.target.ID, accountID); got != wantCounts {
		t.Fatalf("late delete failure did not roll back: got %+v want %+v", got, wantCounts)
	}
	fixture.login(t, lifecycleLogin{username: fixture.target.Username, password: "TargetPass1"})
}
