package service

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type featureGuideRepoStub struct {
	mu    sync.Mutex
	items map[model.FeatureGuideSurface]model.FeatureGuide
}

func newFeatureGuideRepoStub() *featureGuideRepoStub {
	return &featureGuideRepoStub{items: make(map[model.FeatureGuideSurface]model.FeatureGuide)}
}

func (repo *featureGuideRepoStub) GetBySurface(surface model.FeatureGuideSurface) (*model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	item, ok := repo.items[surface]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (repo *featureGuideRepoStub) List() ([]model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	items := make([]model.FeatureGuide, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *featureGuideRepoStub) UpdateLocked(surface model.FeatureGuideSurface, update func(*model.FeatureGuide) (*model.FeatureGuide, error)) (*model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var existing *model.FeatureGuide
	if item, ok := repo.items[surface]; ok {
		existing = &item
	}
	next, err := update(existing)
	if err != nil {
		return nil, err
	}
	repo.items[surface] = *next
	copy := *next
	return &copy, nil
}

func TestFeatureGuideServiceReturnsThreeDefaults(t *testing.T) {
	items, err := NewFeatureGuideService(newFeatureGuideRepoStub()).ListAdmin()
	if err != nil {
		t.Fatal(err)
	}
	wantSurfaces := []model.FeatureGuideSurface{model.FeatureGuideSurfaceCanvas, model.FeatureGuideSurfaceImage, model.FeatureGuideSurfaceVideo}
	if len(items) != 3 {
		t.Fatalf("items=%#v, want three defaults", items)
	}
	for index, item := range items {
		if item.Surface != wantSurfaces[index] || item.Enabled || item.Version != 1 || item.Pages == nil || len(item.Pages) != 0 || item.Title == "" {
			t.Fatalf("unexpected default at %d: %#v", index, item)
		}
	}
}

func TestFeatureGuideServiceVersionOnlyTracksContent(t *testing.T) {
	svc := NewFeatureGuideService(newFeatureGuideRepoStub())
	draft, err := svc.Save(model.FeatureGuideSurfaceCanvas, model.FeatureGuidePayload{
		Title: " 画布更新 ", Pages: []string{"第一页", "第二页", "   "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Version != 1 || draft.Title != "画布更新" || !reflect.DeepEqual(draft.Pages, []string{"第一页", "第二页"}) {
		t.Fatalf("unexpected normalized draft: %#v", draft)
	}

	enabled, err := svc.Save(model.FeatureGuideSurfaceCanvas, model.FeatureGuidePayload{Enabled: true, Title: draft.Title, Pages: draft.Pages})
	if err != nil || enabled.Version != 1 {
		t.Fatalf("enabled=%#v err=%v, want unchanged version", enabled, err)
	}
	changed, err := svc.Save(model.FeatureGuideSurfaceCanvas, model.FeatureGuidePayload{Enabled: true, Title: draft.Title, Pages: []string{"第二页", "第一页"}})
	if err != nil || changed.Version != 2 {
		t.Fatalf("changed=%#v err=%v, want version 2", changed, err)
	}
	repeated, err := svc.Save(model.FeatureGuideSurfaceCanvas, *changed)
	if err != nil || repeated.Version != 2 {
		t.Fatalf("repeated=%#v err=%v, want version 2", repeated, err)
	}
	renamed, err := svc.Save(model.FeatureGuideSurfaceCanvas, model.FeatureGuidePayload{Enabled: true, Title: "新标题", Pages: changed.Pages})
	if err != nil || renamed.Version != 3 {
		t.Fatalf("renamed=%#v err=%v, want version 3", renamed, err)
	}
}

func TestFeatureGuideServiceConcurrentContentUpdatesKeepVersions(t *testing.T) {
	svc := NewFeatureGuideService(newFeatureGuideRepoStub())
	if _, err := svc.Save(model.FeatureGuideSurfaceVideo, model.FeatureGuidePayload{Title: "视频", Pages: []string{"初始"}}); err != nil {
		t.Fatal(err)
	}
	versions := make(chan int, 2)
	errorsBySave := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, content := range []string{"更新一", "更新二"} {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			item, err := svc.Save(model.FeatureGuideSurfaceVideo, model.FeatureGuidePayload{Title: "视频", Pages: []string{content}})
			if err == nil {
				versions <- item.Version
			}
			errorsBySave <- err
		}()
	}
	waitGroup.Wait()
	close(versions)
	close(errorsBySave)
	for err := range errorsBySave {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int]bool{}
	for version := range versions {
		seen[version] = true
	}
	if !seen[2] || !seen[3] || len(seen) != 2 {
		t.Fatalf("versions=%v, want one version 2 and one version 3", seen)
	}
}

func TestFeatureGuideServicePublicOnlyReturnsEnabledValidContent(t *testing.T) {
	svc := NewFeatureGuideService(newFeatureGuideRepoStub())
	missing, err := svc.GetPublic(model.FeatureGuideSurfaceImage)
	if err != nil || missing != nil {
		t.Fatalf("missing=%#v err=%v", missing, err)
	}
	if _, err := svc.Save(model.FeatureGuideSurfaceImage, model.FeatureGuidePayload{Title: "图片", Pages: []string{"内容"}}); err != nil {
		t.Fatal(err)
	}
	disabled, err := svc.GetPublic(model.FeatureGuideSurfaceImage)
	if err != nil || disabled != nil {
		t.Fatalf("disabled=%#v err=%v", disabled, err)
	}
	if _, err := svc.Save(model.FeatureGuideSurfaceImage, model.FeatureGuidePayload{Enabled: true, Title: "图片", Pages: []string{"内容"}}); err != nil {
		t.Fatal(err)
	}
	enabled, err := svc.GetPublic(model.FeatureGuideSurfaceImage)
	if err != nil || enabled == nil || enabled.Surface != model.FeatureGuideSurfaceImage || len(enabled.Pages) != 1 {
		t.Fatalf("enabled=%#v err=%v", enabled, err)
	}
}

func TestFeatureGuideServiceValidationLimits(t *testing.T) {
	svc := NewFeatureGuideService(newFeatureGuideRepoStub())
	tests := []struct {
		name    string
		surface model.FeatureGuideSurface
		input   model.FeatureGuidePayload
	}{
		{name: "surface", surface: "audio", input: model.FeatureGuidePayload{}},
		{name: "title", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Title: strings.Repeat("中", 101)}},
		{name: "page count", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Pages: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21"}}},
		{name: "page length", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Pages: []string{strings.Repeat("中", 20001)}}},
		{name: "total length", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Pages: []string{strings.Repeat("中", 18000), strings.Repeat("中", 18000), strings.Repeat("中", 18000), strings.Repeat("中", 18000), strings.Repeat("中", 18000), strings.Repeat("中", 18000)}}},
		{name: "enabled without pages", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Enabled: true}},
		{name: "enabled blank page", surface: model.FeatureGuideSurfaceCanvas, input: model.FeatureGuidePayload{Enabled: true, Pages: []string{"内容", " \n "}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := svc.Save(test.surface, test.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestFeatureGuideServiceInvalidStoredContentIsHidden(t *testing.T) {
	repo := newFeatureGuideRepoStub()
	repo.items[model.FeatureGuideSurfaceVideo] = model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceVideo, Enabled: true, Title: "视频", Pages: `["有效","  "]`, Version: 1,
	}
	item, err := NewFeatureGuideService(repo).GetPublic(model.FeatureGuideSurfaceVideo)
	if err != nil || item != nil {
		t.Fatalf("item=%#v err=%v, want hidden invalid content", item, err)
	}
	repo.items[model.FeatureGuideSurfaceVideo] = model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceVideo, Enabled: true, Title: "视频", Pages: `{`, Version: 1,
	}
	item, err = NewFeatureGuideService(repo).GetPublic(model.FeatureGuideSurfaceVideo)
	if err != nil || item != nil {
		t.Fatalf("malformed item=%#v err=%v, want hidden content", item, err)
	}
	if _, err := NewFeatureGuideService(repo).GetPublic("audio"); err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unexpected invalid surface error: %v", err)
	}
}

func TestFeatureGuideServiceAdminListRecoversMalformedAndNullPages(t *testing.T) {
	repo := newFeatureGuideRepoStub()
	repo.items[model.FeatureGuideSurfaceCanvas] = model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceCanvas, Enabled: true, Title: "保留标题", Pages: `{`, Version: 7,
	}
	repo.items[model.FeatureGuideSurfaceImage] = model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceImage, Enabled: true, Title: "空页面", Pages: `null`, Version: 4,
	}
	items, err := NewFeatureGuideService(repo).ListAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Surface != model.FeatureGuideSurfaceCanvas || items[0].Enabled || items[0].Title != "保留标题" || items[0].Version != 7 || items[0].Pages == nil || len(items[0].Pages) != 0 {
		t.Fatalf("malformed draft=%#v", items[0])
	}
	if items[1].Surface != model.FeatureGuideSurfaceImage || !items[1].Enabled || items[1].Pages == nil || len(items[1].Pages) != 0 {
		t.Fatalf("null pages=%#v", items[1])
	}
}

func TestFeatureGuideServiceSaveRepairsMalformedPagesAndIncrementsVersion(t *testing.T) {
	repo := newFeatureGuideRepoStub()
	repo.items[model.FeatureGuideSurfaceVideo] = model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceVideo, Enabled: true, Title: "旧标题", Pages: `{`, Version: 8,
	}
	saved, err := NewFeatureGuideService(repo).Save(model.FeatureGuideSurfaceVideo, model.FeatureGuidePayload{
		Enabled: true, Title: "新标题", Pages: []string{"可恢复内容"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 9 || saved.Title != "新标题" || !reflect.DeepEqual(saved.Pages, []string{"可恢复内容"}) {
		t.Fatalf("saved=%#v", saved)
	}
	stored := repo.items[model.FeatureGuideSurfaceVideo]
	if stored.Version != 9 || stored.Pages != `["可恢复内容"]` {
		t.Fatalf("stored=%#v", stored)
	}
}
