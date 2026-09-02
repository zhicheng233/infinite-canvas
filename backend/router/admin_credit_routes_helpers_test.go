package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"infinite-canvas-server/config"
	"infinite-canvas-server/handler"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type adminCreditRouteFixture struct {
	db          *gorm.DB
	server      *httptest.Server
	tenantAdmin *model.User
	superAdmin  *model.User
	target      *model.User
	foreign     *model.User
	tenantToken string
	superToken  string
}

type adminCreditRouteResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func newAdminCreditRouteFixture(t *testing.T) *adminCreditRouteFixture {
	t.Helper()
	db := openAdminCreditRouteDB(t)
	const tenantID = 301
	passwordHash := mustHashRoutePassword(t)
	users := []*model.User{
		{TenantID: tenantID, Username: "route_tenant_admin", PasswordHash: passwordHash, Role: model.RoleTenantAdmin, Status: model.UserActive},
		{TenantID: tenantID, Username: "route_super_admin", PasswordHash: passwordHash, Role: model.RoleSuperAdmin, Status: model.UserActive},
		{TenantID: tenantID, Username: "route_target", PasswordHash: passwordHash, Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID + 1, Username: "route_foreign", PasswordHash: passwordHash, Role: model.RoleUser, Status: model.UserActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed route users: %v", err)
	}
	for _, user := range users[2:] {
		if err := db.Create(&model.CreditAccount{TenantID: user.TenantID, UserID: user.ID, Balance: 100, TotalEarned: 100}).Error; err != nil {
			t.Fatalf("seed route account: %v", err)
		}
	}
	cfg := &config.Config{JWTKey: "admin-credit-route-test-key"}
	userRepo := repository.NewUserRepo(db)
	creditRepo := repository.NewCreditRepo(db)
	authService := service.NewAuthService(cfg, userRepo, nil, creditRepo, nil)
	creditService := service.NewCreditService(creditRepo)
	adminHandler := handler.NewAdminHandler(nil, userRepo, creditService, creditRepo, nil, nil, nil)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Setup(engine, Dependencies{AuthService: authService, AdminHandler: adminHandler})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	fixture := &adminCreditRouteFixture{db: db, server: server, tenantAdmin: users[0], superAdmin: users[1], target: users[2], foreign: users[3]}
	fixture.tenantToken = fixture.login(t, users[0].Username)
	fixture.superToken = fixture.login(t, users[1].Username)
	return fixture
}

func (fixture *adminCreditRouteFixture) login(t *testing.T, username string) string {
	t.Helper()
	authService := service.NewAuthService(&config.Config{JWTKey: "admin-credit-route-test-key"}, repository.NewUserRepo(fixture.db), nil, repository.NewCreditRepo(fixture.db), nil)
	result, err := authService.Login(service.LoginInput{Username: username, Password: "RoutePass1"})
	if err != nil {
		t.Fatalf("login %q: %v", username, err)
	}
	return result.Token
}

func (fixture *adminCreditRouteFixture) post(t *testing.T, path, token string, body map[string]any) adminCreditRouteResponse {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode POST %s body: %v", path, err)
	}
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create POST %s request: %v", path, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	httpResponse, err := fixture.server.Client().Do(request)
	if err != nil {
		t.Fatalf("send POST %s request: %v", path, err)
	}
	defer httpResponse.Body.Close()
	var response adminCreditRouteResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&response); err != nil {
		t.Fatalf("decode POST %s response: %v", path, err)
	}
	return response
}

func (fixture *adminCreditRouteFixture) assertAccountUnchanged(t *testing.T, userID uint) {
	t.Helper()
	var account model.CreditAccount
	if err := fixture.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("read unchanged account: %v", err)
	}
	if account.Balance != 100 || account.TotalEarned != 100 || account.TotalSpent != 0 {
		t.Fatalf("account changed after failed administrator mutation: %+v", account)
	}
	var transactionCount int64
	if err := fixture.db.Model(&model.CreditTransaction{}).Count(&transactionCount).Error; err != nil {
		t.Fatalf("count administrator transactions: %v", err)
	}
	if transactionCount != 0 {
		t.Fatalf("administrator transaction count=%d, want 0", transactionCount)
	}
}

func openAdminCreditRouteDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is required for route-level credit tests")
	}
	dsnConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *dsnConfig
	adminConfig.DBName = ""
	adminDB, err := gorm.Open(gormmysql.Open(adminConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open route MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("credit_route_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatalf("create route test database: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Exec("DROP DATABASE `" + databaseName + "`").Error })
	testConfig := *dsnConfig
	testConfig.DBName = databaseName
	db, err := gorm.Open(gormmysql.Open(testConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open route test database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}, &model.CreditTransaction{}); err != nil {
		t.Fatalf("migrate route credit tables: %v", err)
	}
	return db
}

func mustHashRoutePassword(t *testing.T) string {
	t.Helper()
	hash, err := service.HashPassword("RoutePass1")
	if err != nil {
		t.Fatalf("hash route password: %v", err)
	}
	return hash
}
