package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type handlerVideoConfigPresetRepo struct {
	items []model.VideoConfigPreset
}

func (repo *handlerVideoConfigPresetRepo) ListByTenant(tenantID uint) ([]model.VideoConfigPreset, error) {
	items := make([]model.VideoConfigPreset, 0)
	for _, item := range repo.items {
		if item.TenantID == tenantID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *handlerVideoConfigPresetRepo) Create(item *model.VideoConfigPreset) error {
	for _, existing := range repo.items {
		if existing.TenantID == item.TenantID && existing.NormalizedName == item.NormalizedName {
			return repository.ErrVideoConfigPresetNameConflict
		}
	}
	item.ID = uint(len(repo.items) + 1)
	repo.items = append(repo.items, *item)
	return nil
}

func TestVideoConfigPresetHandlerNormalizedDuplicateReturnsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &handlerVideoConfigPresetRepo{}
	handler := NewVideoConfigPresetHandler(service.NewVideoConfigPresetService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &service.Claims{TenantID: 31})
		c.Next()
	})
	router.POST("/backend-api/api-config/video-presets", handler.Create)

	first := performPresetRequest(t, router, http.MethodPost, "/backend-api/api-config/video-presets", map[string]any{"name": "Omni", "config": handlerPresetConfig()})
	duplicate := performPresetRequest(t, router, http.MethodPost, "/backend-api/api-config/video-presets", map[string]any{"name": " omni ", "config": handlerPresetConfig()})
	if first.Code != 0 || duplicate.Code != http.StatusConflict || len(repo.items) != 1 {
		t.Fatalf("first=%#v duplicate=%#v stored=%#v", first, duplicate, repo.items)
	}
}

func (repo *handlerVideoConfigPresetRepo) DeleteByTenantAndID(tenantID, presetID uint) error {
	for index, item := range repo.items {
		if item.TenantID == tenantID && item.ID == presetID {
			repo.items = append(repo.items[:index], repo.items[index+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func TestVideoConfigPresetHandlerCreateListDeleteUsesClaimsTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &handlerVideoConfigPresetRepo{}
	handler := NewVideoConfigPresetHandler(service.NewVideoConfigPresetService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &service.Claims{TenantID: 31})
		c.Next()
	})
	router.GET("/backend-api/api-config/video-presets", handler.List)
	router.POST("/backend-api/api-config/video-presets", handler.Create)
	router.DELETE("/backend-api/api-config/video-presets/:presetId", handler.Delete)

	body := map[string]any{"name": " Omni ", "tenant_id": 99, "config": handlerPresetConfig()}
	createResponse := performPresetRequest(t, router, http.MethodPost, "/backend-api/api-config/video-presets", body)
	if createResponse.Code != 0 || len(repo.items) != 1 || repo.items[0].TenantID != 31 {
		t.Fatalf("create response=%#v stored=%#v", createResponse, repo.items)
	}
	listResponse := performPresetRequest(t, router, http.MethodGet, "/backend-api/api-config/video-presets", nil)
	if listResponse.Code != 0 {
		t.Fatalf("list response=%#v", listResponse)
	}
	deleteResponse := performPresetRequest(t, router, http.MethodDelete, "/backend-api/api-config/video-presets/1", nil)
	if deleteResponse.Code != 0 || len(repo.items) != 0 {
		t.Fatalf("delete response=%#v stored=%#v", deleteResponse, repo.items)
	}
}

func TestVideoConfigPresetHandlerCrossTenantDeleteReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configBytes, err := json.Marshal(handlerPresetConfig())
	if err != nil {
		t.Fatal(err)
	}
	repo := &handlerVideoConfigPresetRepo{items: []model.VideoConfigPreset{{ID: 5, TenantID: 41, Name: "Other", NormalizedName: "other", Config: string(configBytes)}}}
	handler := NewVideoConfigPresetHandler(service.NewVideoConfigPresetService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &service.Claims{TenantID: 42})
		c.Next()
	})
	router.DELETE("/backend-api/api-config/video-presets/:presetId", handler.Delete)

	response := performPresetRequest(t, router, http.MethodDelete, "/backend-api/api-config/video-presets/5", nil)
	if response.Code != http.StatusNotFound || len(repo.items) != 1 {
		t.Fatalf("response=%#v stored=%#v", response, repo.items)
	}
}

func handlerPresetConfig() *model.CustomVideoConfig {
	return &model.CustomVideoConfig{
		Seconds:    model.CustomVideoSecondsConfig{Enabled: true, Key: "seconds", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions: model.CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: "size", Options: []string{"1280x720", "720x1280"}, Default: "1280x720"},
		N:          model.CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
}

func performPresetRequest(t *testing.T, router http.Handler, method, target string, body any) model.Response {
	t.Helper()
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, target, requestBody)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var response model.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return response
}
