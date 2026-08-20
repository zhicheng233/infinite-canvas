package service

import (
	"reflect"
	"testing"

	"infinite-canvas-server/model"
)

func TestVideoConfigPresetServicePersistsAboveFormerCapMediaCountsAcrossListAndReopen(t *testing.T) {
	repo := &videoConfigPresetRepoStub{}
	service := NewVideoConfigPresetService(repo)
	config := serviceTestAboveFormerCapCustomVideoConfig()
	wantCounts := serviceTestExpectedAboveFormerCapMediaCounts()

	created, err := service.Create(31, model.CreateVideoConfigPresetInput{Name: "Large media", Config: config})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if got := serviceTestMediaMaxCounts(&created.Config); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("created counts=%v, want %v", got, wantCounts)
	}

	config.Dimensions.Options[0] = "input mutation"
	created.Config.Dimensions.Options[0] = "created mutation"
	items, err := service.List(31)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("list length=%d, want 1", len(items))
	}
	if got := serviceTestMediaMaxCounts(&items[0].Config); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("listed counts=%v, want %v", got, wantCounts)
	}
	if got := items[0].Config.Dimensions.Options[0]; got != "1280x720" {
		t.Fatalf("listed dimensions option=%q, want persisted value", got)
	}

	items[0].Config.Dimensions.Options[0] = "list mutation"
	reopened := NewVideoConfigPresetService(repo)
	reopenedItems, err := reopened.List(31)
	if err != nil {
		t.Fatalf("reopened list failed: %v", err)
	}
	if len(reopenedItems) != 1 {
		t.Fatalf("reopened list length=%d, want 1", len(reopenedItems))
	}
	if got := serviceTestMediaMaxCounts(&reopenedItems[0].Config); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("reopened counts=%v, want %v", got, wantCounts)
	}
	if got := reopenedItems[0].Config.Dimensions.Options[0]; got != "1280x720" {
		t.Fatalf("reopened dimensions option=%q, want persisted value", got)
	}
}

func TestVideoConfigPresetServiceRejectsInvalidEnabledMediaMaxCountsBeforeCreate(t *testing.T) {
	invalidCounts := []struct {
		name     string
		maxCount int64
	}{
		{name: "enabled zero", maxCount: 0},
		{name: "above safe JSON integer", maxCount: serviceTestMaxSafeJSONInteger + 1},
	}
	for _, media := range serviceTestMediaRoles {
		for _, invalid := range invalidCounts {
			t.Run(media.name+"/"+invalid.name, func(t *testing.T) {
				repo := &videoConfigPresetRepoStub{}
				service := NewVideoConfigPresetService(repo)
				config := serviceTestAboveFormerCapCustomVideoConfig()
				media.selectRole(config).MaxCount = invalid.maxCount

				if _, err := service.Create(31, model.CreateVideoConfigPresetInput{Name: "invalid", Config: config}); err == nil {
					t.Fatalf("max_count=%d accepted", invalid.maxCount)
				}
				if repo.createCalls != 0 || len(repo.items) != 0 {
					t.Fatalf("create calls=%d items=%d, want no repository write", repo.createCalls, len(repo.items))
				}
			})
		}
	}
}
