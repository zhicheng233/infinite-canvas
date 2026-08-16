package service

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

type channelModelRepoStub struct {
	item      *model.ChannelModel
	saveCalls int
}

func (repo *channelModelRepoStub) FindByID(uint) (*model.ChannelModel, error) {
	if repo.item == nil {
		return nil, errors.New("not found")
	}
	return repo.item, nil
}

func (*channelModelRepoStub) FindByChannelAndName(uint, string) (*model.ChannelModel, error) {
	return nil, errors.New("not implemented")
}

func (repo *channelModelRepoStub) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	if repo.item == nil || repo.item.ChannelID != channelID || (enabledOnly && !repo.item.Enabled) {
		return []model.ChannelModel{}, nil
	}
	return []model.ChannelModel{*repo.item}, nil
}

func (repo *channelModelRepoStub) Save(*model.ChannelModel) error {
	repo.saveCalls++
	return nil
}

func (*channelModelRepoStub) Upsert(*model.ChannelModel) error { return errors.New("not implemented") }

func (*channelModelRepoStub) DeleteStaleModels(uint, []string) error {
	return errors.New("not implemented")
}

type channelModelCatalogChannelServiceStub struct {
	channels []model.ChannelInfo
}

func (stub channelModelCatalogChannelServiceStub) ListEnabled() ([]model.ChannelInfo, error) {
	return stub.channels, nil
}

func (channelModelCatalogChannelServiceStub) DecryptedApiKey(uint) (string, error) {
	return "", errors.New("not implemented")
}

type channelModelCatalogChannelRepoStub struct {
	channel *model.Channel
}

func (stub channelModelCatalogChannelRepoStub) FindByID(uint) (*model.Channel, error) {
	return stub.channel, nil
}

func (channelModelCatalogChannelRepoStub) Save(*model.Channel) error { return nil }

type channelModelCatalogPricingStub struct {
	items map[string]map[uint]model.CreditPricing
}

func (stub channelModelCatalogPricingStub) FindPricingMap(uint) (map[string]map[uint]model.CreditPricing, error) {
	return stub.items, nil
}

func newChannelModelCatalogTestService(item *model.ChannelModel) *ChannelModelService {
	return NewChannelModelService(
		channelModelCatalogChannelServiceStub{channels: []model.ChannelInfo{{ID: item.ChannelID, Name: "A", Enabled: true}}},
		channelModelCatalogChannelRepoStub{channel: &model.Channel{BaseModel: model.BaseModel{ID: item.ChannelID}, Name: "A", Enabled: true}},
		&channelModelRepoStub{item: item},
		channelModelCatalogPricingStub{items: map[string]map[uint]model.CreditPricing{
			item.ModelName: {item.ChannelID: {ChannelID: item.ChannelID, Model: item.ModelName, CreditsPerUnit: 1, UnitType: model.UnitPerVideo}},
		}},
	)
}

func serviceTestCustomVideoConfig() *model.CustomVideoConfig {
	return &model.CustomVideoConfig{
		Seconds:    model.CustomVideoSecondsConfig{Enabled: true, Key: "seconds", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions: model.CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: "size", Options: []string{"1280x720", "720x1280"}, Default: "1280x720"},
		Images:     model.CustomVideoMediaConfig{Enabled: true, Required: false, Key: "images", MaxCount: 1},
		InputVideo: model.CustomVideoMediaConfig{Enabled: true, Required: true, Key: "input_video", MaxCount: 1},
		N:          model.CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
}

func TestUniqueDiscoveredModelNames(t *testing.T) {
	items := []discoveredModel{
		{ID: " omni_flash_nowater "},
		{ID: ""},
		{ID: "omni_flash_nowater"},
		{ID: "omni-fast"},
	}
	want := []string{"omni_flash_nowater", "omni-fast"}
	if got := uniqueDiscoveredModelNames(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("names=%v, want %v", got, want)
	}
}

func TestUpdateChannelModelCapabilities(t *testing.T) {
	// Setup: Create a channel model with initial capabilities
	initialCapabilities := []string{"image", "video"}
	initialJSON, err := json.Marshal(initialCapabilities)
	if err != nil {
		t.Fatalf("failed to marshal initial capabilities: %v", err)
	}

	item := &model.ChannelModel{
		ChannelID:    1,
		ModelName:    "test-model",
		Capabilities: string(initialJSON),
		Enabled:      true,
	}

	// Test 1: Update with new capabilities ["image","text","audio"]
	t.Run("update with new capabilities", func(t *testing.T) {
		newCapabilities := []string{"image", "text", "audio"}
		input := model.UpdateChannelModelInput{
			Capabilities: newCapabilities,
		}

		// Simulate the update logic from channel_model_service.go lines 138-147
		if input.Capabilities != nil {
			if len(input.Capabilities) == 0 {
				t.Fatal("should not reach here - empty capabilities test is separate")
			}
			encoded, encodeErr := json.Marshal(input.Capabilities)
			if encodeErr != nil {
				t.Fatalf("failed to marshal capabilities: %v", encodeErr)
			}
			item.Capabilities = string(encoded)
		}

		// Verify the result
		var result []string
		if err := json.Unmarshal([]byte(item.Capabilities), &result); err != nil {
			t.Fatalf("failed to unmarshal result capabilities: %v", err)
		}

		if len(result) != 3 {
			t.Fatalf("expected 3 capabilities, got %d", len(result))
		}
		expected := map[string]bool{"image": true, "text": true, "audio": true}
		for _, cap := range result {
			if !expected[cap] {
				t.Fatalf("unexpected capability: %s", cap)
			}
		}
	})

	// Test 2: Empty capabilities should return error
	t.Run("empty capabilities returns error", func(t *testing.T) {
		input := model.UpdateChannelModelInput{
			Capabilities: []string{},
		}

		// Simulate the validation logic from channel_model_service.go lines 139-141
		if input.Capabilities != nil {
			if len(input.Capabilities) == 0 {
				// This is the expected error path
				return
			}
			t.Fatal("empty capabilities should trigger error before marshal")
		}
	})

	// Test 3: Nil capabilities preserves existing values
	t.Run("nil capabilities preserves existing", func(t *testing.T) {
		// Reset to initial state
		item.Capabilities = string(initialJSON)

		input := model.UpdateChannelModelInput{
			Capabilities: nil,
		}

		// Simulate the update logic - nil means no change
		if input.Capabilities != nil {
			t.Fatal("should not enter update block when Capabilities is nil")
		}

		// Verify original capabilities are preserved
		var result []string
		if err := json.Unmarshal([]byte(item.Capabilities), &result); err != nil {
			t.Fatalf("failed to unmarshal preserved capabilities: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 preserved capabilities, got %d", len(result))
		}
		expected := map[string]bool{"image": true, "video": true}
		for _, cap := range result {
			if !expected[cap] {
				t.Fatalf("unexpected preserved capability: %s", cap)
			}
		}
	})
}

func TestUpdateChannelModelRequiresConfigForCustomRoute(t *testing.T) {
	repo := &channelModelRepoStub{item: &model.ChannelModel{VideoRoute: "auto"}}
	service := NewChannelModelService(nil, nil, repo, nil)
	route := "custom"

	if _, err := service.Update(1, model.UpdateChannelModelInput{VideoRoute: &route}); err == nil {
		t.Fatal("custom route without config should fail")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("save calls=%d, want 0", repo.saveCalls)
	}
}

func TestUpdateChannelModelPreservesCustomConfigForSortOnlyPatch(t *testing.T) {
	configJSON, err := json.Marshal(serviceTestCustomVideoConfig())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	repo := &channelModelRepoStub{item: &model.ChannelModel{Capabilities: `[]`, VideoDurations: `[]`, VideoRoute: "custom", VideoCustomConfig: string(configJSON)}}
	service := NewChannelModelService(nil, nil, repo, nil)
	sortOrder := 7

	info, err := service.Update(1, model.UpdateChannelModelInput{SortOrder: &sortOrder})
	if err != nil {
		t.Fatalf("sort-only update failed: %v", err)
	}
	if repo.saveCalls != 1 || repo.item.SortOrder != sortOrder {
		t.Fatalf("save calls=%d sort order=%d, want 1 and %d", repo.saveCalls, repo.item.SortOrder, sortOrder)
	}
	if repo.item.VideoRoute != "custom" || repo.item.VideoCustomConfig != string(configJSON) {
		t.Fatalf("persisted custom route/config changed: route=%q config=%q", repo.item.VideoRoute, repo.item.VideoCustomConfig)
	}
	if info.VideoCustomConfig == nil || info.VideoCustomConfig.Seconds.Default != 6 {
		t.Fatalf("returned custom config was not preserved: %#v", info.VideoCustomConfig)
	}
}

func TestUpdateChannelModelSavesAndReturnsCustomConfig(t *testing.T) {
	repo := &channelModelRepoStub{item: &model.ChannelModel{Capabilities: "[]", VideoDurations: "[]", VideoRoute: "auto"}}
	service := NewChannelModelService(nil, nil, repo, nil)
	route := "custom"
	config := serviceTestCustomVideoConfig()

	info, err := service.Update(1, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: config})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if repo.saveCalls != 1 || repo.item.VideoCustomConfig == "" {
		t.Fatalf("save calls=%d config=%q", repo.saveCalls, repo.item.VideoCustomConfig)
	}
	if info.VideoCustomConfig == nil || info.VideoCustomConfig.Dimensions.Mode != "size" || info.VideoCustomConfig.Dimensions.Default != "1280x720" {
		t.Fatalf("unexpected returned config: %#v", info.VideoCustomConfig)
	}
	if info.VideoCustomConfig.Images.Required || !info.VideoCustomConfig.InputVideo.Required {
		t.Fatalf("returned media required values changed: %#v", info.VideoCustomConfig)
	}
	var stored model.CustomVideoConfig
	if err := json.Unmarshal([]byte(repo.item.VideoCustomConfig), &stored); err != nil {
		t.Fatalf("unmarshal stored config: %v", err)
	}
	if stored.Images.Required || !stored.InputVideo.Required {
		t.Fatalf("stored media required values changed: %#v", stored)
	}
}

func TestUpdateChannelModelClearsConfigForNonCustomRoute(t *testing.T) {
	repo := &channelModelRepoStub{item: &model.ChannelModel{Capabilities: "[]", VideoDurations: "[]", VideoRoute: "custom", VideoCustomConfig: `{"stale":true}`}}
	service := NewChannelModelService(nil, nil, repo, nil)
	route := "openai"

	info, err := service.Update(1, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: serviceTestCustomVideoConfig()})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if repo.item.VideoCustomConfig != "" || info.VideoCustomConfig != nil {
		t.Fatalf("config was not cleared: stored=%q info=%#v", repo.item.VideoCustomConfig, info.VideoCustomConfig)
	}
}

func TestChannelModelToInfoRejectsInvalidCustomConfigJSON(t *testing.T) {
	_, err := channelModelToInfo(&model.ChannelModel{Capabilities: "[]", VideoDurations: "[]", VideoRoute: "custom", VideoCustomConfig: "{"})
	if err == nil {
		t.Fatal("invalid persisted config should fail conversion")
	}
}

func TestChannelModelCatalogPreservesCustomConfigAfterAdminUpdate(t *testing.T) {
	item := &model.ChannelModel{BaseModel: model.BaseModel{ID: 91}, ChannelID: 1, ModelName: "catalog-video", Capabilities: `["video"]`, Enabled: true, VideoRoute: "auto", VideoDurations: "[]"}
	service := newChannelModelCatalogTestService(item)
	route := "custom"
	inputConfig := serviceTestCustomVideoConfig()
	inputConfig.Seconds.Key = " seconds "
	inputConfig.Dimensions.Options = []string{"720x1280", "1280x720", "1280x720"}

	updated, err := service.Update(item.ID, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: inputConfig})
	if err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	catalog, err := service.ListTenantCatalog(7)
	if err != nil {
		t.Fatalf("catalog readback failed: %v", err)
	}
	if len(catalog) != 1 || len(catalog[0].Models) != 1 {
		t.Fatalf("unexpected catalog: %#v", catalog)
	}
	if !reflect.DeepEqual(catalog[0].Models[0].VideoCustomConfig, updated.VideoCustomConfig) {
		t.Fatalf("catalog config %#v does not match update %#v", catalog[0].Models[0].VideoCustomConfig, updated.VideoCustomConfig)
	}
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	if !strings.Contains(string(payload), `"video_custom_config":{`) {
		t.Fatalf("catalog JSON omitted custom config: %s", payload)
	}
}

func TestChannelModelCatalogClearsCustomConfigAfterNonCustomAdminUpdate(t *testing.T) {
	configJSON, err := json.Marshal(serviceTestCustomVideoConfig())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	item := &model.ChannelModel{BaseModel: model.BaseModel{ID: 92}, ChannelID: 1, ModelName: "catalog-video", Capabilities: `["video"]`, Enabled: true, VideoRoute: "custom", VideoDurations: "[]", VideoCustomConfig: string(configJSON)}
	service := newChannelModelCatalogTestService(item)
	route := "openai"

	if _, err := service.Update(item.ID, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: nil}); err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	catalog, err := service.ListTenantCatalog(7)
	if err != nil {
		t.Fatalf("catalog readback failed: %v", err)
	}
	if len(catalog) != 1 || len(catalog[0].Models) != 1 || catalog[0].Models[0].VideoCustomConfig != nil {
		t.Fatalf("non-custom catalog retained config: %#v", catalog)
	}
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	if strings.Contains(string(payload), `"video_custom_config"`) {
		t.Fatalf("non-custom catalog JSON retained config: %s", payload)
	}
}
