package service

import (
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type UserService struct {
	userRepo *repository.UserRepo
}

func NewUserService(userRepo *repository.UserRepo) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetUser(id uint) (*model.User, error) {
	return s.userRepo.FindByID(id)
}

func (s *UserService) ListUsers(tenantID uint, page, pageSize int) ([]model.User, int64, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	return s.userRepo.List(tenantID, page, pageSize)
}
