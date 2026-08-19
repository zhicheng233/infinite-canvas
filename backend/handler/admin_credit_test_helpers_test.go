package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type adminCreditFixture struct {
	db             *gorm.DB
	handler        *AdminHandler
	creditService  *service.CreditService
	tenantID       uint
	operatorUserID uint
	targetUserID   uint
	foreignUserID  uint
}

type creditAdjustmentHTTPResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UserID  uint `json:"user_id"`
		Amount  int  `json:"amount"`
		Balance int  `json:"balance"`
	} `json:"data"`
}

type creditAdjustmentMetadata struct {
	Scene          string `json:"scene"`
	OperatorUserID uint   `json:"operator_user_id"`
	TargetUserID   uint   `json:"target_user_id"`
	Mode           string `json:"mode"`
	Amount         int    `json:"amount"`
	BalanceBefore  int    `json:"balance_before"`
	BalanceAfter   int    `json:"balance_after"`
	TargetBalance  int    `json:"target_balance"`
}

func newAdminCreditFixture(t *testing.T, balance int) adminCreditFixture {
	t.Helper()
	db := openAdminCreditTestDB(t)
	const tenantID = 101
	users := []model.User{
		{TenantID: tenantID, Username: "operator", PasswordHash: "test", Role: model.RoleTenantAdmin, Status: model.UserActive},
		{TenantID: tenantID, Username: "target", PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID + 1, Username: "foreign", PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	account := model.CreditAccount{TenantID: tenantID, UserID: users[1].ID, Balance: balance, TotalEarned: balance}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("seed credit account: %v", err)
	}
	creditRepo := repository.NewCreditRepo(db)
	creditService := service.NewCreditService(creditRepo)
	return adminCreditFixture{
		db:             db,
		handler:        NewAdminHandler(nil, repository.NewUserRepo(db), creditService, creditRepo, nil, nil, nil),
		creditService:  creditService,
		tenantID:       tenantID,
		operatorUserID: users[0].ID,
		targetUserID:   users[1].ID,
		foreignUserID:  users[2].ID,
	}
}

func (fixture adminCreditFixture) request(t *testing.T, endpoint gin.HandlerFunc, body string) creditAdjustmentHTTPResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/adjust", func(context *gin.Context) {
		context.Set("claims", &service.Claims{UserID: fixture.operatorUserID, TenantID: fixture.tenantID, Role: model.RoleTenantAdmin})
		endpoint(context)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/adjust", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response creditAdjustmentHTTPResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func openAdminCreditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is required for database-backed credit tests")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *config
	adminConfig.DBName = ""
	adminDB, err := gorm.Open(gormmysql.Open(adminConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("credit_adjustment_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatalf("create test database: %v", err)
	}
	adminSQL, err := adminDB.DB()
	if err != nil {
		t.Fatalf("get admin sql.DB: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Exec("DROP DATABASE `" + databaseName + "`").Error
		_ = adminSQL.Close()
	})
	testConfig := *config
	testConfig.DBName = databaseName
	db, err := gorm.Open(gormmysql.Open(testConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	testSQL, err := db.DB()
	if err != nil {
		t.Fatalf("get test sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = testSQL.Close() })
	if err := db.AutoMigrate(
		&model.User{},
		&model.CreditAccount{},
		&model.CreditTransaction{},
		&model.CanvasProject{},
		&model.GenerationRecord{},
		&model.RechargeOrder{},
		&model.ModelCallLog{},
	); err != nil {
		t.Fatalf("migrate credit test tables: %v", err)
	}
	return db
}
