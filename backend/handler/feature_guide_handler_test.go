package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type handlerFeatureGuideRepo struct {
	mu      sync.Mutex
	items   map[model.FeatureGuideSurface]model.FeatureGuide
	getErr  error
	listErr error
	saveErr error
}

func (repo *handlerFeatureGuideRepo) GetBySurface(surface model.FeatureGuideSurface) (*model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.getErr != nil {
		return nil, repo.getErr
	}
	item, ok := repo.items[surface]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (repo *handlerFeatureGuideRepo) List() ([]model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listErr != nil {
		return nil, repo.listErr
	}
	items := make([]model.FeatureGuide, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *handlerFeatureGuideRepo) UpdateLocked(surface model.FeatureGuideSurface, update func(*model.FeatureGuide) (*model.FeatureGuide, error)) (*model.FeatureGuide, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.saveErr != nil {
		return nil, repo.saveErr
	}
	var existing *model.FeatureGuide
	if item, ok := repo.items[surface]; ok {
		existing = &item
	}
	next, err := update(existing)
	if err != nil {
		return nil, err
	}
	repo.items[surface] = *next
	return next, nil
}

func TestFeatureGuideHandlerResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &handlerFeatureGuideRepo{items: make(map[model.FeatureGuideSurface]model.FeatureGuide)}
	handler := NewFeatureGuideHandler(service.NewFeatureGuideService(repo))
	router := gin.New()
	router.GET("/feature-guides/:surface", handler.Get)
	router.GET("/admin/feature-guides", handler.AdminList)
	router.PUT("/admin/feature-guides/:surface", handler.AdminSave)

	missing := performFeatureGuideRequest(t, router, http.MethodGet, "/feature-guides/canvas", nil)
	if missing.Code != 0 || string(missing.Data) != "null" {
		t.Fatalf("missing response=%#v", missing)
	}
	defaults := performFeatureGuideRequest(t, router, http.MethodGet, "/admin/feature-guides", nil)
	var defaultItems []model.FeatureGuidePayload
	if defaults.Code != 0 || json.Unmarshal(defaults.Data, &defaultItems) != nil || len(defaultItems) != 3 {
		t.Fatalf("defaults response=%#v items=%#v", defaults, defaultItems)
	}

	saved := performFeatureGuideRequest(t, router, http.MethodPut, "/admin/feature-guides/canvas", map[string]any{
		"surface": "video", "enabled": false, "title": " 画布 ", "pages": []string{"正文", " "}, "version": 99,
	})
	var item model.FeatureGuidePayload
	if saved.Code != 0 || json.Unmarshal(saved.Data, &item) != nil || item.Surface != model.FeatureGuideSurfaceCanvas || item.Version != 1 || len(item.Pages) != 1 {
		t.Fatalf("saved response=%#v item=%#v", saved, item)
	}
	invalid := performFeatureGuideRequest(t, router, http.MethodPut, "/admin/feature-guides/audio", map[string]any{})
	if invalid.Code != 400 {
		t.Fatalf("invalid response=%#v", invalid)
	}
	invalidContent := performFeatureGuideRequest(t, router, http.MethodPut, "/admin/feature-guides/canvas", map[string]any{"enabled": true, "pages": []string{" "}})
	if invalidContent.Code != 400 {
		t.Fatalf("invalid content response=%#v", invalidContent)
	}
	invalidJSON := performRawFeatureGuideRequest(t, router, http.MethodPut, "/admin/feature-guides/canvas", `{`)
	if invalidJSON.Code != 400 {
		t.Fatalf("invalid JSON response=%#v", invalidJSON)
	}
}

func TestFeatureGuideHandlerRepositoryErrorsReturnInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	internalErr := errors.New("database password leaked")
	tests := []struct {
		name       string
		repo       *handlerFeatureGuideRepo
		method     string
		target     string
		body       any
		message    string
	}{
		{name: "get", repo: &handlerFeatureGuideRepo{items: make(map[model.FeatureGuideSurface]model.FeatureGuide), getErr: internalErr}, method: http.MethodGet, target: "/feature-guides/canvas", message: "读取功能引导失败"},
		{name: "list", repo: &handlerFeatureGuideRepo{items: make(map[model.FeatureGuideSurface]model.FeatureGuide), listErr: internalErr}, method: http.MethodGet, target: "/admin/feature-guides", message: "读取功能引导配置失败"},
		{name: "save", repo: &handlerFeatureGuideRepo{items: make(map[model.FeatureGuideSurface]model.FeatureGuide), saveErr: internalErr}, method: http.MethodPut, target: "/admin/feature-guides/canvas", body: map[string]any{"title": "画布", "pages": []string{}}, message: "保存功能引导配置失败"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewFeatureGuideHandler(service.NewFeatureGuideService(test.repo))
			router := gin.New()
			router.GET("/feature-guides/:surface", handler.Get)
			router.GET("/admin/feature-guides", handler.AdminList)
			router.PUT("/admin/feature-guides/:surface", handler.AdminSave)
			response := performFeatureGuideRequest(t, router, test.method, test.target, test.body)
			if response.Code != 500 || response.Msg != test.message || response.Msg == internalErr.Error() {
				t.Fatalf("response=%#v", response)
			}
		})
	}
}

func TestFeatureGuideHandlerRecoversMalformedPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &handlerFeatureGuideRepo{items: map[model.FeatureGuideSurface]model.FeatureGuide{
		model.FeatureGuideSurfaceCanvas: {
			Surface: model.FeatureGuideSurfaceCanvas, Enabled: true, Title: "损坏配置", Pages: `{`, Version: 7,
		},
	}}
	handler := NewFeatureGuideHandler(service.NewFeatureGuideService(repo))
	router := gin.New()
	router.GET("/feature-guides/:surface", handler.Get)
	router.GET("/admin/feature-guides", handler.AdminList)
	router.PUT("/admin/feature-guides/:surface", handler.AdminSave)

	public := performFeatureGuideRequest(t, router, http.MethodGet, "/feature-guides/canvas", nil)
	if public.Code != 0 || string(public.Data) != "null" {
		t.Fatalf("public response=%#v", public)
	}
	admin := performFeatureGuideRequest(t, router, http.MethodGet, "/admin/feature-guides", nil)
	var items []model.FeatureGuidePayload
	if admin.Code != 0 || json.Unmarshal(admin.Data, &items) != nil || len(items) != 3 {
		t.Fatalf("admin response=%#v items=%#v", admin, items)
	}
	if items[0].Enabled || items[0].Title != "损坏配置" || items[0].Version != 7 || items[0].Pages == nil || len(items[0].Pages) != 0 {
		t.Fatalf("repair draft=%#v", items[0])
	}
	repaired := performFeatureGuideRequest(t, router, http.MethodPut, "/admin/feature-guides/canvas", map[string]any{
		"enabled": true, "title": "已修复", "pages": []string{"正文"},
	})
	var saved model.FeatureGuidePayload
	if repaired.Code != 0 || json.Unmarshal(repaired.Data, &saved) != nil || saved.Version != 8 || len(saved.Pages) != 1 {
		t.Fatalf("repair response=%#v saved=%#v", repaired, saved)
	}
}

type featureGuideHandlerResponse struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

func performFeatureGuideRequest(t *testing.T, router http.Handler, method, target string, body any) featureGuideHandlerResponse {
	t.Helper()
	bodyJSON := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		bodyJSON = string(encoded)
	}
	return performRawFeatureGuideRequest(t, router, method, target, bodyJSON)
}

func performRawFeatureGuideRequest(t *testing.T, router http.Handler, method, target, body string) featureGuideHandlerResponse {
	t.Helper()
	requestBody := bytes.NewReader([]byte(body))
	request := httptest.NewRequest(method, target, requestBody)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var response featureGuideHandlerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return response
}
