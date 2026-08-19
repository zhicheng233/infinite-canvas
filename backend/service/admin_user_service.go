package service

import (
	"fmt"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type adminUserListRepository interface {
	List(repository.UserListQuery) ([]model.User, int64, error)
}

type adminBalanceRepository interface {
	GetBalancesByUserIDs([]uint) (map[uint]int, error)
}

type AdminUserService struct {
	userRepo    adminUserListRepository
	balanceRepo adminBalanceRepository
}

type UserWithBalance struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Balance     int    `json:"balance"`
}

type MissingCreditAccountError struct {
	UserID uint
}

func (err *MissingCreditAccountError) Error() string {
	return fmt.Sprintf("credit account missing for user %d", err.UserID)
}

func NewAdminUserService(userRepo adminUserListRepository, balanceRepo adminBalanceRepository) *AdminUserService {
	return &AdminUserService{userRepo: userRepo, balanceRepo: balanceRepo}
}

func (service *AdminUserService) ListUsersWithBalance(query repository.UserListQuery) ([]UserWithBalance, int64, error) {
	users, total, err := service.userRepo.List(query)
	if err != nil {
		return nil, 0, fmt.Errorf("list tenant users: %w", err)
	}
	userIDs := make([]uint, len(users))
	for index, user := range users {
		userIDs[index] = user.ID
	}
	balances, err := service.balanceRepo.GetBalancesByUserIDs(userIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("list user balances: %w", err)
	}
	items := make([]UserWithBalance, len(users))
	for index, user := range users {
		balance, exists := balances[user.ID]
		if !exists {
			return nil, 0, &MissingCreditAccountError{UserID: user.ID}
		}
		items[index] = UserWithBalance{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Role:        string(user.Role),
			Status:      string(user.Status),
			Balance:     balance,
		}
	}
	return items, total, nil
}
