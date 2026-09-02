package repository

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
)

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByTenantAndID(tenantID, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByUsername(tenantID uint, username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("tenant_id = ? AND username = ?", tenantID, username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByUsernameGlobal(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type UserListQuery struct {
	TenantID uint
	Page     int
	PageSize int
	Keyword  string
}

func (query UserListQuery) Normalize() UserListQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	return query
}

func (r *UserRepo) List(rawQuery UserListQuery) ([]model.User, int64, error) {
	query := rawQuery.Normalize()
	var users []model.User
	var total int64
	q := r.db.Model(&model.User{}).Where("tenant_id = ?", query.TenantID)
	if query.Keyword != "" {
		isNumeric := true
		for _, char := range query.Keyword {
			if char < '0' || char > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			userID, err := strconv.ParseUint(query.Keyword, 10, strconv.IntSize)
			if err != nil {
				q = q.Where("id = 0")
			} else {
				q = q.Where("id = ?", userID)
			}
		} else {
			keyword := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(query.Keyword)
			q = q.Where("username LIKE ? ESCAPE '!'", "%"+keyword+"%")
		}
	}
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Order("id DESC").Find(&users).Error
	return users, total, err
}

func (r *UserRepo) ListAll(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	q := r.db.Model(&model.User{})
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&users).Error
	return users, total, err
}

func (r *UserRepo) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepo) UpdatePassword(userID uint, passwordHash string) error {
	result := r.db.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", passwordHash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepo) DeleteAccount(userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}

		var accountIDs []uint
		if err := tx.Unscoped().Model(&model.CreditAccount{}).Where("user_id = ?", userID).Pluck("id", &accountIDs).Error; err != nil {
			return fmt.Errorf("find user credit accounts: %w", err)
		}
		if err := tx.Unscoped().Where("account_id IN ?", accountIDs).Delete(&model.CreditTransaction{}).Error; err != nil {
			return fmt.Errorf("delete user credit transactions: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.CreditAccount{}).Error; err != nil {
			return fmt.Errorf("delete user credit accounts: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.CanvasProject{}).Error; err != nil {
			return fmt.Errorf("delete user canvas projects: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.GenerationRecord{}).Error; err != nil {
			return fmt.Errorf("delete user generation records: %w", err)
		}
		var generationRequestIDs []string
		if err := tx.Unscoped().Model(&model.GenerationJob{}).Where("user_id = ?", userID).Pluck("request_id", &generationRequestIDs).Error; err != nil {
			return fmt.Errorf("find user generation jobs: %w", err)
		}
		if len(generationRequestIDs) > 0 {
			if err := tx.Unscoped().Where("request_id IN ?", generationRequestIDs).Delete(&model.GenerationAttempt{}).Error; err != nil {
				return fmt.Errorf("delete user generation attempts: %w", err)
			}
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.GenerationJob{}).Error; err != nil {
			return fmt.Errorf("delete user generation jobs: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.RechargeOrder{}).Error; err != nil {
			return fmt.Errorf("delete user recharge orders: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&model.ModelCallLog{}).Error; err != nil {
			return fmt.Errorf("delete user model call logs: %w", err)
		}
		result := tx.Unscoped().Where("id = ?", userID).Delete(&model.User{})
		if result.Error != nil {
			return fmt.Errorf("delete user: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("delete user: %w", gorm.ErrRecordNotFound)
		}
		return nil
	})
}

func (r *UserRepo) CountAll() (int64, error) {
	var total int64
	err := r.db.Model(&model.User{}).Count(&total).Error
	return total, err
}
