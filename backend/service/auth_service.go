package service

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"infinite-canvas-server/config"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type AuthService struct {
	cfg            *config.Config
	userRepo       *repository.UserRepo
	tenantRepo     *repository.TenantRepo
	creditRepo     *repository.CreditRepo
	captchaService *CaptchaService
}

func NewAuthService(cfg *config.Config, userRepo *repository.UserRepo, tenantRepo *repository.TenantRepo, creditRepo *repository.CreditRepo, captchaService *CaptchaService) *AuthService {
	return &AuthService{cfg: cfg, userRepo: userRepo, tenantRepo: tenantRepo, creditRepo: creditRepo, captchaService: captchaService}
}

type Claims struct {
	UserID   uint           `json:"user_id"`
	TenantID uint           `json:"tenant_id"`
	Role     model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type RegisterInput struct {
	TenantName    string `json:"tenant_name"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer string `json:"captcha_answer"`
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

type ChangePasswordInput struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type UpdateProfileInput struct {
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("密码至少需要8个字符")
	}
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	if !hasLetter || !hasDigit {
		return errors.New("密码需包含字母和数字")
	}
	return nil
}

func shouldBootstrapInitialAdmin(userCount int64, username, password string) (bool, error) {
	if userCount > 0 {
		return false, nil
	}
	if username == "" && password == "" {
		return false, nil
	}
	if username == "" || password == "" {
		return false, errors.New("INIT_ADMIN_USERNAME 和 INIT_ADMIN_PASSWORD 需要同时配置")
	}
	if err := validatePasswordStrength(password); err != nil {
		return false, err
	}
	return true, nil
}

func (s *AuthService) Register(input RegisterInput) (*RegisterResult, error) {
	if input.Username == "" || input.Password == "" {
		return nil, errors.New("请输入用户名和密码")
	}
	if err := validatePasswordStrength(input.Password); err != nil {
		return nil, err
	}

	if s.captchaService != nil && !s.captchaService.Validate(input.CaptchaID, input.CaptchaAnswer) {
		return nil, errors.New("验证码不正确")
	}

	result, err := s.createUserAndToken(0, input.Username, input.Password, model.RoleUser)
	if err != nil {
		return nil, err
	}

	credits := s.cfg.RegistrationCredits
	account := &model.CreditAccount{
		TenantID:    0,
		UserID:      result.User.ID,
		Balance:     credits,
		TotalEarned: credits,
	}
	if err := s.creditRepo.CreateAccount(account); err == nil {
		s.creditRepo.CreateTransaction(&model.CreditTransaction{
			AccountID:     account.ID,
			Type:          model.TxTypeEarn,
			Amount:        credits,
			BalanceBefore: intPtr(0),
			BalanceAfter:  credits,
			RefType:       "welcome",
			Note:          fmt.Sprintf("注册赠送 %d 积分", credits),
		})
	}

	return result, nil
}

func (s *AuthService) EnsureInitialAdmin() error {
	userCount, err := s.userRepo.CountAll()
	if err != nil {
		return err
	}

	username := strings.TrimSpace(s.cfg.InitAdminUsername)
	password := strings.TrimSpace(s.cfg.InitAdminPassword)
	displayName := strings.TrimSpace(s.cfg.InitAdminDisplayName)

	shouldCreate, err := shouldBootstrapInitialAdmin(userCount, username, password)
	if err != nil {
		return fmt.Errorf("初始化管理员配置无效: %w", err)
	}
	if !shouldCreate {
		if userCount == 0 {
			log.Printf("skip bootstrap initial admin: INIT_ADMIN_USERNAME or INIT_ADMIN_PASSWORD not configured")
		}
		return nil
	}

	if displayName == "" {
		displayName = username
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	user := &model.User{
		TenantID:     0,
		Username:     username,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         model.RoleSuperAdmin,
		Status:       model.UserActive,
	}
	if err := s.userRepo.Create(user); err != nil {
		return fmt.Errorf("创建初始管理员失败: %w", err)
	}

	credits := s.cfg.RegistrationCredits
	account := &model.CreditAccount{
		TenantID:    0,
		UserID:      user.ID,
		Balance:     credits,
		TotalEarned: credits,
	}
	if err := s.creditRepo.CreateAccount(account); err != nil {
		return fmt.Errorf("创建初始管理员积分账户失败: %w", err)
	}
	if credits > 0 {
		if err := s.creditRepo.CreateTransaction(&model.CreditTransaction{
			AccountID:     account.ID,
			Type:          model.TxTypeEarn,
			Amount:        credits,
			BalanceBefore: intPtr(0),
			BalanceAfter:  credits,
			RefType:       "bootstrap_admin",
			Note:          fmt.Sprintf("初始化管理员赠送 %d 积分", credits),
		}); err != nil {
			return fmt.Errorf("写入初始管理员积分流水失败: %w", err)
		}
	}

	log.Printf("initial admin created: username=%s role=%s", user.Username, user.Role)
	return nil
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

func (s *AuthService) ChangePassword(userID uint, input ChangePasswordInput) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if !CheckPassword(user.PasswordHash, input.OldPassword) {
		return errors.New("原密码不正确")
	}
	if err := validatePasswordStrength(input.NewPassword); err != nil {
		return err
	}
	hash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return s.userRepo.Update(user)
}

func (s *AuthService) UpdateProfile(userID uint, input UpdateProfileInput) (*model.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	if input.DisplayName != "" {
		user.DisplayName = input.DisplayName
	}
	if input.AvatarURL != "" {
		user.AvatarURL = input.AvatarURL
	}
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
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
