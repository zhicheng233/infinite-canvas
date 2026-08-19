package service

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"infinite-canvas-server/config"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

func TestShouldBootstrapInitialAdmin(t *testing.T) {
	tests := []struct {
		name      string
		userCount int64
		username  string
		password  string
		want      bool
		wantErr   bool
	}{
		{name: "skip when users already exist", userCount: 1, username: "admin", password: "Admin1234", want: false, wantErr: false},
		{name: "skip when init admin not configured", userCount: 0, username: "", password: "", want: false, wantErr: false},
		{name: "error when username missing", userCount: 0, username: "", password: "Admin1234", want: false, wantErr: true},
		{name: "error when password missing", userCount: 0, username: "admin", password: "", want: false, wantErr: true},
		{name: "error when username contains spaces", userCount: 0, username: "bad name", password: "Admin1234", want: false, wantErr: true},
		{name: "error when username is too long", userCount: 0, username: strings.Repeat("a", 65), password: "Admin1234", want: false, wantErr: true},
		{name: "error when password too weak", userCount: 0, username: "admin", password: "12345678", want: false, wantErr: true},
		{name: "create when empty database and config valid", userCount: 0, username: "admin", password: "Admin1234", want: true, wantErr: false},
	}

	for _, tt := range tests {
		got, err := shouldBootstrapInitialAdmin(tt.userCount, tt.username, tt.password)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRegisterTrimsUsername(t *testing.T) {
	service := newDryRunAuthService(t, nil)

	result, err := service.Register(RegisterInput{Username: "  User_01-name  ", Password: "Password1"})

	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if result.User.Username != "User_01-name" {
		t.Fatalf("stored username = %q, want %q", result.User.Username, "User_01-name")
	}
}

func TestRegisterRejectsInvalidUsername(t *testing.T) {
	service := newDryRunAuthService(t, nil)
	tests := []struct {
		name     string
		username string
	}{
		{name: "empty", username: "   "},
		{name: "space", username: "bad name"},
		{name: "punctuation", username: "x!"},
		{name: "too long", username: strings.Repeat("a", 65)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Register(RegisterInput{Username: test.username, Password: "Password1"})
			if err == nil {
				t.Fatal("Register returned nil error")
			}
		})
	}
}

func TestRegisterMapsDuplicateUsernameCreateError(t *testing.T) {
	service := newDryRunAuthService(t, &mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry"})

	_, err := service.Register(RegisterInput{Username: "existing", Password: "Password1"})

	if err == nil || err.Error() != "用户名已存在" {
		t.Fatalf("Register error = %v, want 用户名已存在", err)
	}
}

func TestCreateUserAndTokenMapsRealMySQLDuplicateUsernameAcrossTenants(t *testing.T) {
	dsn := os.Getenv("AUTH_POLICY_TEST_DSN")
	if dsn == "" {
		t.Skip("AUTH_POLICY_TEST_DSN is required for auth policy MySQL tests")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open auth policy test database: %v", err)
	}
	tableName := "auth_service_users_" + time.Now().Format("20060102150405.000000000")
	tableName = strings.ReplaceAll(tableName, ".", "")
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(tableName); err != nil {
			t.Errorf("drop auth service user table: %v", err)
		}
	})
	tableDB := db.Table(tableName)
	if err := tableDB.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate auth service user table: %v", err)
	}
	first := &model.User{TenantID: 101, Username: "global_name", PasswordHash: "seed-hash", Status: model.UserActive}
	if err := tableDB.Create(first).Error; err != nil {
		t.Fatalf("insert first tenant user: %v", err)
	}
	service := NewAuthService(
		&config.Config{JWTKey: "test-jwt-key"},
		repository.NewUserRepo(tableDB),
		nil,
		repository.NewCreditRepo(tableDB),
		nil,
	)

	_, err = service.createUserAndToken(202, "global_name", "Password1", model.RoleUser)

	if err == nil || err.Error() != "用户名已存在" {
		t.Fatalf("createUserAndToken error = %v, want 用户名已存在", err)
	}
}

func TestRegisterPreservesNonDuplicateCreateError(t *testing.T) {
	createErr := errors.New("database unavailable")
	service := newDryRunAuthService(t, createErr)

	_, err := service.Register(RegisterInput{Username: "new-user", Password: "Password1"})

	if !errors.Is(err, createErr) {
		t.Fatalf("Register error = %v, want original create error", err)
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "accepts letters and digits", password: "Password1", wantErr: false},
		{name: "rejects seven ASCII characters", password: "Passwo1", wantErr: true},
		{name: "accepts eight ASCII characters", password: "Passwor1", wantErr: false},
		{name: "rejects four Unicode code points", password: "a1密码", wantErr: true},
		{name: "accepts eight Unicode code points", password: "a1密码密码密码", wantErr: false},
		{name: "rejects five code points containing astral symbols", password: "a1😀😀😀", wantErr: true},
		{name: "rejects missing letter", password: "12345678", wantErr: true},
		{name: "rejects missing digit", password: "Password", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePasswordStrength(test.password)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePasswordStrength error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func newDryRunAuthService(t *testing.T, userCreateErr error) *AuthService {
	t.Helper()
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{
		DSN:                       "unused:unused@tcp(localhost:3306)/unused?parseTime=true",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	if userCreateErr != nil {
		err := db.Callback().Create().Before("gorm:create").Register("auth_test_user_create_error", func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" {
				tx.AddError(userCreateErr)
			}
		})
		if err != nil {
			t.Fatalf("register create callback: %v", err)
		}
	}
	return NewAuthService(
		&config.Config{JWTKey: "test-jwt-key"},
		repository.NewUserRepo(db),
		nil,
		repository.NewCreditRepo(db),
		nil,
	)
}
