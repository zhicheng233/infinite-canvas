package service

import (
	"errors"
	"reflect"
	"testing"

	"infinite-canvas-server/model"
)

type channelModelPersistenceRepoStub struct {
	item      model.ChannelModel
	saveCalls int
}

func (repo *channelModelPersistenceRepoStub) FindByID(id uint) (*model.ChannelModel, error) {
	if repo.item.ID != id {
		return nil, errors.New("not found")
	}
	item := repo.item
	return &item, nil
}

func (*channelModelPersistenceRepoStub) FindByChannelAndName(uint, string) (*model.ChannelModel, error) {
	return nil, errors.New("not implemented")
}

func (repo *channelModelPersistenceRepoStub) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	if repo.item.ChannelID != channelID || (enabledOnly && !repo.item.Enabled) {
		return []model.ChannelModel{}, nil
	}
	return []model.ChannelModel{repo.item}, nil
}

func (repo *channelModelPersistenceRepoStub) Save(item *model.ChannelModel) error {
	repo.saveCalls++
	repo.item = *item
	return nil
}

func (*channelModelPersistenceRepoStub) Upsert(*model.ChannelModel) error {
	return errors.New("not implemented")
}

func (*channelModelPersistenceRepoStub) DeleteStaleModels(uint, []string) error {
	return errors.New("not implemented")
}

func TestChannelModelServicePersistsAboveFormerCapMediaCountsAcrossReopen(t *testing.T) {
	const channelModelID uint = 101
	const channelID uint = 7
	repo := &channelModelPersistenceRepoStub{item: model.ChannelModel{
		BaseModel:      model.BaseModel{ID: channelModelID},
		ChannelID:      channelID,
		ModelName:      "persisted-video",
		Capabilities:   `[]`,
		VideoRoute:     "auto",
		VideoDurations: `[]`,
	}}
	service := NewChannelModelService(nil, nil, repo, nil)
	route := "custom"
	config := serviceTestAboveFormerCapCustomVideoConfig()
	wantCounts := serviceTestExpectedAboveFormerCapMediaCounts()

	updated, err := service.Update(channelModelID, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: config})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("save calls=%d, want 1", repo.saveCalls)
	}
	if updated.VideoCustomConfig == nil {
		t.Fatal("updated custom config is nil")
	}
	if got := serviceTestMediaMaxCounts(updated.VideoCustomConfig); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("updated counts=%v, want %v", got, wantCounts)
	}

	config.Dimensions.Options[0] = "input mutation"
	updated.VideoCustomConfig.Dimensions.Options[0] = "response mutation"
	listed, err := service.List(channelID, false)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(listed) != 1 || listed[0].VideoCustomConfig == nil {
		t.Fatalf("list=%#v, want one custom model", listed)
	}
	if got := serviceTestMediaMaxCounts(listed[0].VideoCustomConfig); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("listed counts=%v, want %v", got, wantCounts)
	}
	if got := listed[0].VideoCustomConfig.Dimensions.Options[0]; got != "1280x720" {
		t.Fatalf("listed dimensions option=%q, want persisted value", got)
	}
	listed[0].VideoCustomConfig.Dimensions.Options[0] = "list mutation"

	reopened := NewChannelModelService(nil, nil, repo, nil)
	reopenedList, err := reopened.List(channelID, false)
	if err != nil {
		t.Fatalf("reopened list failed: %v", err)
	}
	if len(reopenedList) != 1 || reopenedList[0].VideoCustomConfig == nil {
		t.Fatalf("reopened list=%#v, want one custom model", reopenedList)
	}
	if got := serviceTestMediaMaxCounts(reopenedList[0].VideoCustomConfig); !reflect.DeepEqual(got, wantCounts) {
		t.Fatalf("reopened counts=%v, want %v", got, wantCounts)
	}
}

func TestChannelModelServiceRejectsInvalidEnabledMediaMaxCountsBeforeSave(t *testing.T) {
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
				repo := &channelModelPersistenceRepoStub{item: model.ChannelModel{
					BaseModel:      model.BaseModel{ID: 102},
					ChannelID:      7,
					ModelName:      "invalid-video",
					Capabilities:   `[]`,
					VideoRoute:     "auto",
					VideoDurations: `[]`,
				}}
				service := NewChannelModelService(nil, nil, repo, nil)
				route := "custom"
				config := serviceTestAboveFormerCapCustomVideoConfig()
				media.selectRole(config).MaxCount = invalid.maxCount

				if _, err := service.Update(102, model.UpdateChannelModelInput{VideoRoute: &route, VideoCustomConfig: config}); err == nil {
					t.Fatalf("max_count=%d accepted", invalid.maxCount)
				}
				if repo.saveCalls != 0 {
					t.Fatalf("save calls=%d, want 0", repo.saveCalls)
				}
				if repo.item.VideoRoute != "auto" {
					t.Fatalf("repository route=%q, want unchanged auto", repo.item.VideoRoute)
				}
			})
		}
	}
}

func TestChannelModelServiceRejectsUnknownRoutesBeforeSave(t *testing.T) {
	for name, input := range map[string]model.UpdateChannelModelInput{
		"image generation": {ImageGenerateRoute: serviceTestStringPointer("unknown")},
		"image edit":       {ImageEditRoute: serviceTestStringPointer("unknown")},
		"video":            {VideoRoute: serviceTestStringPointer("unknown")},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &channelModelPersistenceRepoStub{item: model.ChannelModel{
				BaseModel: model.BaseModel{ID: 103}, ChannelID: 7, ModelName: "route-model",
				Capabilities: `["image","video"]`, ImageGenerateRoute: "auto", ImageEditRoute: "auto", VideoRoute: "auto", VideoDurations: `[]`,
			}}
			if _, err := NewChannelModelService(nil, nil, repo, nil).Update(103, input); err == nil {
				t.Fatal("unknown route was accepted")
			}
			if repo.saveCalls != 0 {
				t.Fatalf("save calls = %d, want 0", repo.saveCalls)
			}
		})
	}
}

func serviceTestStringPointer(value string) *string { return &value }
