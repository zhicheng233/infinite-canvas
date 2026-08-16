package service

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type videoConfigPresetRepoStub struct {
	mu     sync.Mutex
	nextID uint
	items  []model.VideoConfigPreset
}

func (repo *videoConfigPresetRepoStub) ListByTenant(tenantID uint) ([]model.VideoConfigPreset, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	items := make([]model.VideoConfigPreset, 0)
	for _, item := range repo.items {
		if item.TenantID == tenantID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repo *videoConfigPresetRepoStub) Create(item *model.VideoConfigPreset) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, existing := range repo.items {
		if existing.TenantID == item.TenantID && existing.NormalizedName == item.NormalizedName {
			return repository.ErrVideoConfigPresetNameConflict
		}
	}
	repo.nextID++
	item.ID = repo.nextID
	repo.items = append(repo.items, *item)
	return nil
}

func (repo *videoConfigPresetRepoStub) DeleteByTenantAndID(tenantID, presetID uint) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for index, item := range repo.items {
		if item.TenantID == tenantID && item.ID == presetID {
			repo.items = append(repo.items[:index], repo.items[index+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func presetTestConfig() *model.CustomVideoConfig {
	return &model.CustomVideoConfig{
		Seconds:    model.CustomVideoSecondsConfig{Enabled: true, Key: " seconds ", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions: model.CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: "size", Options: []string{"720x1280", "1280x720"}, Default: "1280x720"},
		Images:     model.CustomVideoMediaConfig{Enabled: true, Required: false, Key: "images", MaxCount: 1},
		InputVideo: model.CustomVideoMediaConfig{Enabled: true, Required: true, Key: "input_video", MaxCount: 1},
		N:          model.CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
}

func TestVideoConfigPresetServiceTenantIsolationAndDelete(t *testing.T) {
	repo := &videoConfigPresetRepoStub{}
	svc := NewVideoConfigPresetService(repo)
	created, err := svc.Create(11, model.CreateVideoConfigPresetInput{Name: " Omni 默认 ", Config: presetTestConfig()})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.Name != "Omni 默认" {
		t.Fatalf("name=%q, want trimmed display name", created.Name)
	}

	tenantA, err := svc.List(11)
	if err != nil || len(tenantA) != 1 {
		t.Fatalf("tenant A list=%#v err=%v", tenantA, err)
	}
	tenantB, err := svc.List(22)
	if err != nil || len(tenantB) != 0 {
		t.Fatalf("tenant B list=%#v err=%v", tenantB, err)
	}
	if err := svc.Delete(22, created.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant delete error=%v, want not found", err)
	}
	if err := svc.Delete(11, created.ID); err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
}

func TestVideoConfigPresetServiceRejectsEmptyAndInvalidInput(t *testing.T) {
	repo := &videoConfigPresetRepoStub{}
	svc := NewVideoConfigPresetService(repo)
	if _, err := svc.Create(1, model.CreateVideoConfigPresetInput{Name: "   ", Config: presetTestConfig()}); err == nil {
		t.Fatal("empty name should fail")
	}
	invalid := presetTestConfig()
	invalid.N.Value = 17
	if _, err := svc.Create(1, model.CreateVideoConfigPresetInput{Name: "invalid", Config: invalid}); err == nil {
		t.Fatal("invalid custom video config should fail")
	}
	if len(repo.items) != 0 {
		t.Fatalf("invalid creates wrote %d items", len(repo.items))
	}
}

func TestVideoConfigPresetServiceConcurrentNormalizedDuplicate(t *testing.T) {
	repo := &videoConfigPresetRepoStub{}
	svc := NewVideoConfigPresetService(repo)
	inputs := []string{"Omni", " omni "}
	errorsByRequest := make(chan error, len(inputs))
	var waitGroup sync.WaitGroup
	for _, name := range inputs {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := svc.Create(7, model.CreateVideoConfigPresetInput{Name: name, Config: presetTestConfig()})
			errorsByRequest <- err
		}()
	}
	waitGroup.Wait()
	close(errorsByRequest)

	successes := 0
	conflicts := 0
	for err := range errorsByRequest {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrVideoConfigPresetNameConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestVideoConfigPresetServiceSnapshotStability(t *testing.T) {
	repo := &videoConfigPresetRepoStub{}
	svc := NewVideoConfigPresetService(repo)
	config := presetTestConfig()
	created, err := svc.Create(3, model.CreateVideoConfigPresetInput{Name: "Snapshot", Config: config})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	config.Seconds.Default = 9
	config.Dimensions.Options[0] = "changed"

	items, err := svc.List(3)
	if err != nil || len(items) != 1 {
		t.Fatalf("list=%#v err=%v", items, err)
	}
	if items[0].Config.Seconds.Default != created.Config.Seconds.Default || items[0].Config.Dimensions.Options[0] != "1280x720" {
		t.Fatalf("snapshot changed after input mutation: %#v", items[0].Config)
	}
	if items[0].Config.Images.Required || !items[0].Config.InputVideo.Required {
		t.Fatalf("preset media required values changed: %#v", items[0].Config)
	}
	var stored model.CustomVideoConfig
	if err := json.Unmarshal([]byte(repo.items[0].Config), &stored); err != nil {
		t.Fatalf("unmarshal stored preset: %v", err)
	}
	if stored.Images.Required || !stored.InputVideo.Required {
		t.Fatalf("stored preset media required values changed: %#v", stored)
	}

	channelModel := model.ChannelModel{VideoCustomConfig: repo.items[0].Config}
	if err := svc.Delete(3, created.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if channelModel.VideoCustomConfig == "" || channelModel.VideoCustomConfig != createdConfigJSON(t, created.Config) {
		t.Fatalf("deleting preset mutated channel model snapshot: %q", channelModel.VideoCustomConfig)
	}
}

func createdConfigJSON(t *testing.T, config model.CustomVideoConfig) string {
	t.Helper()
	bytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return string(bytes)
}
