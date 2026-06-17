package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"infinite-canvas-server/config"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type AuthService struct {
	cfg        *config.Config
	userRepo   *repository.UserRepo
	tenantRepo *repository.TenantRepo
	creditRepo *repository.CreditRepo
}

func NewAuthService(cfg *config.Config, userRepo *repository.UserRepo, tenantRepo *repository.TenantRepo, creditRepo *repository.CreditRepo) *AuthService {
	return &AuthService{cfg: cfg, userRepo: userRepo, tenantRepo: tenantRepo, creditRepo: creditRepo}
}

type Claims struct {
	UserID   uint           `json:"user_id"`
	TenantID uint           `json:"tenant_id"`
	Role     model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type RegisterInput struct {
	TenantName string `json:"tenant_name"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type RegisterResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthService) Register(input RegisterInput) (*RegisterResult, error) {
	if input.Username == "" || input.Password == "" {
		return nil, errors.New("请输入用户名和密码")
	}
	if len(input.Password) < 6 {
		return nil, errors.New("密码至少需要6个字符")
	}

	tenantName := input.TenantName
	if tenantName == "" {
		tenantName = input.Username
	}

	tenant := &model.Tenant{
		Name:   tenantName,
		Domain: tenantName,
		Plan:   model.PlanFree,
		Status: model.TenantActive,
	}
	if err := s.tenantRepo.Create(tenant); err != nil {
		return nil, errors.New("创建租户失败")
	}

	result, err := s.createUserAndToken(tenant.ID, input.Username, input.Password, model.RoleTenantAdmin)
	if err != nil {
		return nil, err
	}

	// Give 1000 free credits to new tenant admin
	account := &model.CreditAccount{
		TenantID:    tenant.ID,
		UserID:      result.User.ID,
		Balance:     1000,
		TotalEarned: 1000,
	}
	if err := s.creditRepo.CreateAccount(account); err == nil {
		s.creditRepo.CreateTransaction(&model.CreditTransaction{
			AccountID:    account.ID,
			Type:         model.TxTypeEarn,
			Amount:       1000,
			BalanceAfter: 1000,
			RefType:      "welcome",
			Note:         "注册赠送 1000 积分",
		})
	}

	return result, nil
}

func (s *AuthService) createUserAndToken(tenantID uint, username, password string, role model.UserRole) (*RegisterResult, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		TenantID:     tenantID,
		Username:     username,
		PasswordHash: hash,
		DisplayName:  username,
		Role:         role,
		Status:       model.UserActive,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, errors.New("用户名已存在")
	}
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}
	return &RegisterResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(input LoginInput) (*LoginResult, error) {
	user, err := s.userRepo.FindByUsernameGlobal(input.Username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if !CheckPassword(user.PasswordHash, input.Password) {
		return nil, errors.New("用户名或密码错误")
	}
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}

func (s *AuthService) generateToken(user *model.User) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTKey))
}

func (s *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTKey), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("无效的令牌")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("无效的令牌声明")
	}
	return claims, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
