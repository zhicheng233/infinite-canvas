package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type apiConfigRepoCatalogStub struct {
	config *model.TenantApiConfig
}

func (stub apiConfigRepoCatalogStub) FindByTenant(uint) (*model.TenantApiConfig, error) {
	if stub.config == nil {
		return nil, errors.New("not found")
	}
	return stub.config, nil
}

func (apiConfigRepoCatalogStub) Save(*model.TenantApiConfig) error { return nil }

type apiConfigPricingCatalogStub struct {
	items map[string]map[uint]model.CreditPricing
}

func (stub apiConfigPricingCatalogStub) FindPricingMap(uint) (map[string]map[uint]model.CreditPricing, error) {
	return stub.items, nil
}

type apiConfigChannelCatalogStub struct {
	items []model.ChannelCatalogItem
}

func (stub apiConfigChannelCatalogStub) ListTenantCatalog(uint) ([]model.ChannelCatalogItem, error) {
	return stub.items, nil
}

func TestEncodeDecodeIntListMap(t *testing.T) {
	input := map[string][]int{
		"veo-omni-flash":         {10},
		"grok-imagine-video-1.5": {5, 10, 10, 0, -1},
		"":                       {6},
	}

	encoded, err := encodeIntListMap(input)
	if err != nil {
		t.Fatalf("encodeIntListMap returned error: %v", err)
	}

	decoded, err := decodeIntListMap(encoded)
	if err != nil {
		t.Fatalf("decodeIntListMap returned error: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("expected 2 models after cleanup, got %d: %#v", len(decoded), decoded)
	}
	if got := decoded["veo-omni-flash"]; len(got) != 1 || got[0] != 10 {
		t.Fatalf("unexpected veo durations: %#v", got)
	}
	if got := decoded["grok-imagine-video-1.5"]; len(got) != 2 || got[0] != 5 || got[1] != 10 {
		t.Fatalf("unexpected grok durations: %#v", got)
	}
}

func TestDecodeIntListMapEmpty(t *testing.T) {
	decoded, err := decodeIntListMap("")
	if err != nil {
		t.Fatalf("decodeIntListMap returned error: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("expected empty map, got %#v", decoded)
	}
}

func TestEncodeDecodeBoolMap(t *testing.T) {
	input := map[string]bool{
		"veo-omni-flash":         true,
		"grok-imagine-video-1.5": false,
		"":                       true,
	}

	encoded, err := encodeBoolMap(input)
	if err != nil {
		t.Fatalf("encodeBoolMap returned error: %v", err)
	}

	decoded, err := decodeBoolMap(encoded)
	if err != nil {
		t.Fatalf("decodeBoolMap returned error: %v", err)
	}

	if len(decoded) != 1 || !decoded["veo-omni-flash"] {
		t.Fatalf("unexpected decoded bool map: %#v", decoded)
	}
}

func TestApiConfigCatalogReturnsChannelModelCustomConfigWithoutTenantMap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	customConfig := &model.CustomVideoConfig{
		Seconds:    model.CustomVideoSecondsConfig{Enabled: true, Key: "seconds", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions: model.CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: "size", Options: []string{"1280x720"}, Default: "1280x720"},
		N:          model.CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
	handler := &ApiConfigHandler{
		apiConfigRepo: apiConfigRepoCatalogStub{config: &model.TenantApiConfig{Models: `["catalog-video"]`, VideoModels: `["catalog-video"]`}},
		creditRepo: apiConfigPricingCatalogStub{items: map[string]map[uint]model.CreditPricing{
			"catalog-video": {1: {ChannelID: 1, Model: "catalog-video", CreditsPerUnit: 1, UnitType: model.UnitPerVideo}},
		}},
		channelCatalog: apiConfigChannelCatalogStub{items: []model.ChannelCatalogItem{{
			ChannelID:   1,
			ChannelName: "A",
			Models: []model.ChannelModelInfo{
				{ID: 91, ChannelID: 1, ModelName: "catalog-video", Capabilities: []string{"video"}, Enabled: true, VideoRoute: "custom", VideoDurations: []int{}, VideoCustomConfig: customConfig},
				{ID: 92, ChannelID: 1, ModelName: "openai-video", Capabilities: []string{"video"}, Enabled: true, VideoRoute: "openai", VideoDurations: []int{}},
			},
		}}},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", &service.Claims{TenantID: 7})
		c.Next()
	})
	router.GET("/api-config/catalog", handler.Catalog)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api-config/catalog", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Channels []model.ChannelCatalogItem `json:"channels"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 0 || len(response.Data.Channels) != 1 || len(response.Data.Channels[0].Models) != 2 {
		t.Fatalf("unexpected catalog response: %#v", response)
	}
	if response.Data.Channels[0].Models[0].VideoCustomConfig == nil || response.Data.Channels[0].Models[0].VideoCustomConfig.Seconds.Default != 6 {
		t.Fatalf("custom config missing from catalog: %#v", response.Data.Channels[0].Models[0])
	}
	if response.Data.Channels[0].Models[1].VideoCustomConfig != nil {
		t.Fatalf("non-custom model retained config: %#v", response.Data.Channels[0].Models[1])
	}
	if strings.Contains(recorder.Body.String(), "model_custom_video_configs") || strings.Count(recorder.Body.String(), `"video_custom_config"`) != 1 {
		t.Fatalf("unexpected custom config JSON shape: %s", recorder.Body.String())
	}
}
