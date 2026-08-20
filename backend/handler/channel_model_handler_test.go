package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestChannelModelHandlerUpdateRejectsFractionalCustomVideoMaxCountAtJSONBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &ChannelModelHandler{}
	router.PUT("/admin/channels/:id/models/:modelId", handler.Update)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/admin/channels/1/models/2", strings.NewReader(`{"video_route":"custom","video_custom_config":{"images":{"enabled":true,"max_count":1.5}}}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusOK || response.Code != http.StatusBadRequest || response.Msg != "无效的请求参数" {
		t.Fatalf("unexpected binding response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
