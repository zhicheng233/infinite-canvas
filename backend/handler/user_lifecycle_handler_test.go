package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

func TestWriteUserLifecycleError_mapsRepositoryNotFoundTo404(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	// When
	writeUserLifecycleError(context, gorm.ErrRecordNotFound)

	// Then
	var response model.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode lifecycle error response: %v", err)
	}
	if response.Code != 404 || response.Msg != "用户不存在" {
		t.Fatalf("response=%+v, want 404 用户不存在", response)
	}
}

func TestWriteUserLifecycleError_hidesPersistenceDetails(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	err := errors.Join(service.ErrUserLifecyclePersistence, errors.New("database connection secret"))

	// When
	writeUserLifecycleError(context, err)

	// Then
	var response model.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode lifecycle error response: %v", err)
	}
	if response.Code != 500 || response.Msg != "账号操作失败" {
		t.Fatalf("response=%+v, want 500 账号操作失败", response)
	}
}
