package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

var (
	ErrUserLifecycleAdminRequired = errors.New("需要管理员权限")
	ErrUserLifecycleSelfTarget    = errors.New("不能操作当前账号")
	ErrUserLifecycleNotFound      = errors.New("用户不存在")
	ErrUserLifecyclePersistence   = errors.New("账号操作失败")
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

func (s *UserService) ListUsers(query repository.UserListQuery) ([]model.User, int64, error) {
	return s.userRepo.List(query)
}

func (s *UserService) ResetPassword(actor *Claims, targetUserID uint, password string) error {
	target, err := s.lifecycleTarget(actor, targetUserID)
	if err != nil {
		return err
	}
	if err := validatePasswordStrength(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("%w: hash password: %w", ErrUserLifecyclePersistence, err)
	}
	if err := s.userRepo.UpdatePassword(target.ID, hash); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserLifecycleNotFound
		}
		return fmt.Errorf("%w: update password: %w", ErrUserLifecyclePersistence, err)
	}
	return nil
}

func (s *UserService) DeleteAccount(actor *Claims, targetUserID uint) error {
	target, err := s.lifecycleTarget(actor, targetUserID)
	if err != nil {
		return err
	}
	if err := s.userRepo.DeleteAccount(target.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserLifecycleNotFound
		}
		return fmt.Errorf("%w: delete account: %w", ErrUserLifecyclePersistence, err)
	}
	return nil
}

func (s *UserService) lifecycleTarget(actor *Claims, targetUserID uint) (*model.User, error) {
	if actor.Role != model.RoleSuperAdmin && actor.Role != model.RoleTenantAdmin {
		return nil, ErrUserLifecycleAdminRequired
	}
	if actor.UserID == targetUserID {
		return nil, ErrUserLifecycleSelfTarget
	}
	actingUser, err := s.userRepo.FindByTenantAndID(actor.TenantID, actor.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserLifecycleAdminRequired
	}
	if err != nil {
		return nil, fmt.Errorf("%w: find acting administrator: %w", ErrUserLifecyclePersistence, err)
	}
	if !actingUser.IsAdmin() {
		return nil, ErrUserLifecycleAdminRequired
	}
	target, err := s.userRepo.FindByTenantAndID(actor.TenantID, targetUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserLifecycleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: find target user: %w", ErrUserLifecyclePersistence, err)
	}
	return target, nil
}
