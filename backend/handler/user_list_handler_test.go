package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type userListPageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items []struct {
			ID       uint   `json:"id"`
			Username string `json:"username"`
			Balance  int    `json:"balance"`
		} `json:"items"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	} `json:"data"`
}

type userListRepoStub struct {
	users []model.User
}

func (repo userListRepoStub) List(repository.UserListQuery) ([]model.User, int64, error) {
	return repo.users, int64(len(repo.users)), nil
}

type balanceRepoErrorStub struct {
	err error
}

func (repo balanceRepoErrorStub) GetBalancesByUserIDs([]uint) (map[uint]int, error) {
	return nil, repo.err
}

func TestAdminHandler_GetUsersWithBalance_returns_structured_500_when_balance_repository_fails(t *testing.T) {
	// Given
	adminHandler := &AdminHandler{adminUserSvc: service.NewAdminUserService(
		userListRepoStub{users: []model.User{{BaseModel: model.BaseModel{ID: 7}, TenantID: 11, Username: "alice"}}},
		balanceRepoErrorStub{err: errors.New("balance query failed")},
	)}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/users-with-balance", func(context *gin.Context) {
		context.Set("claims", &service.Claims{TenantID: 11, Role: model.RoleTenantAdmin})
		adminHandler.GetUsersWithBalance(context)
	})
	recorder := httptest.NewRecorder()

	// When
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users-with-balance", nil))

	// Then
	var response struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusOK || response.Code != http.StatusInternalServerError || response.Data != nil {
		t.Fatalf("unexpected balance failure response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantUserListEndpoints_filterByKeywordWithoutCrossTenantResults(t *testing.T) {
	dsn := os.Getenv("USER_LIST_TEST_DSN")
	if dsn == "" {
		t.Skip("USER_LIST_TEST_DSN is required for the MySQL request-contract test")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.Exec("SET SESSION sql_mode = 'NO_BACKSLASH_ESCAPES'").Error; err != nil {
		t.Fatalf("enable NO_BACKSLASH_ESCAPES: %v", err)
	}
	var sqlMode string
	if err := tx.Raw("SELECT @@SESSION.sql_mode").Scan(&sqlMode).Error; err != nil {
		t.Fatalf("read SQL mode: %v", err)
	}
	if !strings.Contains(sqlMode, "NO_BACKSLASH_ESCAPES") {
		t.Fatalf("NO_BACKSLASH_ESCAPES not enabled: %q", sqlMode)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	tenantID := uint(time.Now().UnixNano()%1_000_000_000 + 1_000_000_000)
	foreignTenantID := tenantID + 1
	users := []*model.User{
		{TenantID: tenantID, Username: "task2alice_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID, Username: "task2alicia_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID, Username: "task2bob_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: foreignTenantID, Username: "task2alice_foreign_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
	}
	for _, user := range users {
		if err := tx.Create(user).Error; err != nil {
			t.Fatalf("seed user %q: %v", user.Username, err)
		}
	}
	accounts := []*model.CreditAccount{
		{TenantID: tenantID, UserID: users[0].ID, Balance: 25},
		{TenantID: tenantID, UserID: users[1].ID},
		{TenantID: tenantID, UserID: users[2].ID},
	}
	if err := tx.Create(accounts).Error; err != nil {
		t.Fatalf("seed credit accounts: %v", err)
	}

	userRepo := repository.NewUserRepo(tx)
	creditRepo := repository.NewCreditRepo(tx)
	userHandler := NewUserHandler(service.NewUserService(userRepo))
	adminHandler := &AdminHandler{adminUserSvc: service.NewAdminUserService(userRepo, creditRepo)}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &service.Claims{TenantID: tenantID, UserID: users[2].ID, Role: model.RoleTenantAdmin})
		c.Next()
	})
	router.GET("/users", userHandler.List)
	router.GET("/users-with-balance", adminHandler.GetUsersWithBalance)

	partial := requestUserListPage(t, router, "/users?keyword="+url.QueryEscape("task2ali")+"&page=2&page_size=1")
	if partial.Data.Total != 2 || partial.Data.Page != 2 || partial.Data.PageSize != 1 || len(partial.Data.Items) != 1 {
		t.Fatalf("partial username pagination mismatch: %+v", partial.Data)
	}

	exact := requestUserListPage(t, router, fmt.Sprintf("/users-with-balance?keyword=%d", users[0].ID))
	if exact.Data.Total != 1 || len(exact.Data.Items) != 1 || exact.Data.Items[0].ID != users[0].ID || exact.Data.Items[0].Balance != 25 {
		t.Fatalf("exact tenant ID search mismatch: %+v", exact.Data)
	}

	for _, endpoint := range []string{"/users", "/users-with-balance"} {
		foreign := requestUserListPage(t, router, fmt.Sprintf("%s?keyword=%d", endpoint, users[3].ID))
		if foreign.Data.Total != 0 || len(foreign.Data.Items) != 0 {
			t.Fatalf("foreign tenant user leaked from %s: %+v", endpoint, foreign.Data)
		}
	}

	blank := requestUserListPage(t, router, "/users-with-balance?keyword=%20%20%20&page=1&page_size=2")
	if blank.Data.Total != 3 || len(blank.Data.Items) != 2 || blank.Data.Items[0].ID != users[2].ID || blank.Data.Items[1].ID != users[1].ID {
		t.Fatalf("blank search changed tenant ordering or pagination: %+v", blank.Data)
	}

	literalUsers := []*model.User{
		{TenantID: tenantID, Username: "task2under_score_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID, Username: "task2underXscore_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID, Username: "task2percent%marker_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
		{TenantID: tenantID, Username: "task2percentXmarker_" + suffix, PasswordHash: "test", Role: model.RoleUser, Status: model.UserActive},
	}
	for _, user := range literalUsers {
		if err := tx.Create(user).Error; err != nil {
			t.Fatalf("seed literal user %q: %v", user.Username, err)
		}
	}
	literalAccounts := make([]*model.CreditAccount, len(literalUsers))
	for index, user := range literalUsers {
		literalAccounts[index] = &model.CreditAccount{TenantID: tenantID, UserID: user.ID}
	}
	if err := tx.Create(literalAccounts).Error; err != nil {
		t.Fatalf("seed literal credit accounts: %v", err)
	}
	for _, test := range []struct {
		keyword      string
		wantUsername string
	}{
		{keyword: "under_score", wantUsername: literalUsers[0].Username},
		{keyword: "percent%marker", wantUsername: literalUsers[2].Username},
	} {
		for _, endpoint := range []string{"/users", "/users-with-balance"} {
			result := requestUserListPage(t, router, endpoint+"?keyword="+url.QueryEscape(test.keyword))
			if result.Data.Total != 1 || len(result.Data.Items) != 1 || result.Data.Items[0].Username != test.wantUsername {
				t.Fatalf("literal keyword %q matched wildcards from %s: %+v", test.keyword, endpoint, result.Data)
			}
		}
	}

	balanceErr := errors.New("balance query failed")
	callbackName := "test:fail_balance_query"
	if err := tx.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		if db.Statement.Table == "credit_accounts" {
			db.AddError(balanceErr)
		}
	}); err != nil {
		t.Fatalf("register balance failure callback: %v", err)
	}
	t.Cleanup(func() { _ = tx.Callback().Query().Remove(callbackName) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users-with-balance", nil))
	var failure struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode balance failure response: %v", err)
	}
	if failure.Code != http.StatusInternalServerError || failure.Data != nil {
		t.Fatalf("balance failure response fabricated items: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func requestUserListPage(t *testing.T, router http.Handler, target string) userListPageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", target, recorder.Code, recorder.Body.String())
	}
	var response userListPageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode GET %s: %v", target, err)
	}
	if response.Code != 0 {
		t.Fatalf("GET %s failed: %s", target, recorder.Body.String())
	}
	return response
}
