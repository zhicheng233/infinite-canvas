package service

import (
	"errors"
	"testing"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type adminUserListRepoStub struct {
	users     []model.User
	lastQuery repository.UserListQuery
}

func (repo *adminUserListRepoStub) List(query repository.UserListQuery) ([]model.User, int64, error) {
	repo.lastQuery = query
	return repo.users, int64(len(repo.users)), nil
}

type adminBalanceRepoStub struct {
	err error
}

func (repo adminBalanceRepoStub) GetBalancesByUserIDs([]uint) (map[uint]int, error) {
	return nil, repo.err
}

func TestAdminUserService_ListUsersWithBalance_returns_error_when_balance_repository_fails(t *testing.T) {
	// Given
	balanceErr := errors.New("balance query failed")
	service := NewAdminUserService(
		&adminUserListRepoStub{users: []model.User{{BaseModel: model.BaseModel{ID: 7}, TenantID: 11, Username: "alice"}}},
		adminBalanceRepoStub{err: balanceErr},
	)

	// When
	items, total, err := service.ListUsersWithBalance(repository.UserListQuery{TenantID: 11, Page: 1, PageSize: 20})

	// Then
	if !errors.Is(err, balanceErr) {
		t.Fatalf("error = %v, want wrapped balance repository error", err)
	}
	if items != nil || total != 0 {
		t.Fatalf("failure returned fabricated result: items=%v total=%d", items, total)
	}
}

func TestAdminUserService_ListUsersWithBalance_returns_error_when_credit_account_is_missing(t *testing.T) {
	// Given
	service := NewAdminUserService(
		&adminUserListRepoStub{users: []model.User{{BaseModel: model.BaseModel{ID: 7}, TenantID: 11, Username: "alice"}}},
		adminBalanceRepoStubSuccess{balances: map[uint]int{}},
	)

	// When
	items, total, err := service.ListUsersWithBalance(repository.UserListQuery{TenantID: 11, Page: 1, PageSize: 20})

	// Then
	var missingAccountErr *MissingCreditAccountError
	if !errors.As(err, &missingAccountErr) || missingAccountErr.UserID != 7 {
		t.Fatalf("error = %v, want missing credit account error for user 7", err)
	}
	if items != nil || total != 0 {
		t.Fatalf("missing credit account returned fabricated result: items=%v total=%d", items, total)
	}
}

func TestAdminUserService_ListUsersWithBalance_preserves_query_and_response_when_repositories_succeed(t *testing.T) {
	// Given
	userRepo := &adminUserListRepoStub{users: []model.User{{
		BaseModel:   model.BaseModel{ID: 7},
		TenantID:    11,
		Username:    "alice",
		DisplayName: "Alice",
		Role:        model.RoleUser,
		Status:      model.UserActive,
	}}}
	service := NewAdminUserService(userRepo, adminBalanceRepoStubSuccess{balances: map[uint]int{7: 25}})
	query := repository.UserListQuery{TenantID: 11, Page: 2, PageSize: 5, Keyword: "ali"}

	// When
	items, total, err := service.ListUsersWithBalance(query)

	// Then
	if err != nil {
		t.Fatalf("list users with balance: %v", err)
	}
	if userRepo.lastQuery != query {
		t.Fatalf("query changed: got %+v want %+v", userRepo.lastQuery, query)
	}
	if total != 1 || len(items) != 1 || items[0].ID != 7 || items[0].Username != "alice" || items[0].Balance != 25 {
		t.Fatalf("unexpected result: items=%+v total=%d", items, total)
	}
}

type adminBalanceRepoStubSuccess struct {
	balances map[uint]int
}

func (repo adminBalanceRepoStubSuccess) GetBalancesByUserIDs([]uint) (map[uint]int, error) {
	return repo.balances, nil
}
