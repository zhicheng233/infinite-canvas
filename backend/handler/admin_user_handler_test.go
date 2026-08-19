package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type adminUserHandlerBalanceRepoStub struct {
	balances map[uint]int
}

func (repo adminUserHandlerBalanceRepoStub) GetBalancesByUserIDs([]uint) (map[uint]int, error) {
	return repo.balances, nil
}

func newAdminUserListRouter(balanceRepo adminUserHandlerBalanceRepoStub) *gin.Engine {
	adminHandler := &AdminHandler{adminUserSvc: service.NewAdminUserService(
		userListRepoStub{users: []model.User{{BaseModel: model.BaseModel{ID: 7}, TenantID: 11, Username: "alice"}}},
		balanceRepo,
	)}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/users-with-balance", func(context *gin.Context) {
		context.Set("claims", &service.Claims{TenantID: 11, Role: model.RoleTenantAdmin})
		adminHandler.GetUsersWithBalance(context)
	})
	return router
}

func TestAdminHandler_GetUsersWithBalance_returns_structured_500_when_credit_account_is_missing(t *testing.T) {
	// Given
	router := newAdminUserListRouter(adminUserHandlerBalanceRepoStub{balances: map[uint]int{}})
	recorder := httptest.NewRecorder()

	// When
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users-with-balance", nil))

	// Then
	var response struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusOK || response.Code != http.StatusInternalServerError || response.Data != nil || response.Msg == "" {
		t.Fatalf("missing account response fabricated balance data: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminHandler_GetUsersWithBalance_returns_paginated_balance_response_when_account_exists(t *testing.T) {
	// Given
	router := newAdminUserListRouter(adminUserHandlerBalanceRepoStub{balances: map[uint]int{7: 25}})
	recorder := httptest.NewRecorder()

	// When
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users-with-balance?page=2&page_size=5", nil))

	// Then
	var response userListPageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 0 || response.Msg != "ok" || response.Data.Total != 1 || response.Data.Page != 2 || response.Data.PageSize != 5 || len(response.Data.Items) != 1 || response.Data.Items[0].Balance != 25 {
		t.Fatalf("unexpected success response shape: %s", recorder.Body.String())
	}
}
