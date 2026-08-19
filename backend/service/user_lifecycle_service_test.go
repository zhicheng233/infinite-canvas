package service

import (
	"errors"
	"testing"

	"infinite-canvas-server/model"
)

func TestUserServiceDeleteAccount_rejectsNonAdminActorBeforeRepositoryAccess(t *testing.T) {
	// Given
	userService := NewUserService(nil)
	actor := &Claims{UserID: 1, TenantID: 1, Role: model.RoleUser}

	// When
	err := userService.DeleteAccount(actor, 2)

	// Then
	if !errors.Is(err, ErrUserLifecycleAdminRequired) {
		t.Fatalf("DeleteAccount error=%v, want admin required", err)
	}
}
