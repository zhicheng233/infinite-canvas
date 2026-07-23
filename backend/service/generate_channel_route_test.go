package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

type fakeChannelReader struct {
	items map[uint]*model.Channel
}

func (f fakeChannelReader) FindByID(id uint) (*model.Channel, error) {
	item := f.items[id]
	if item == nil {
		return nil, errors.New("not found")
	}
	return item, nil
}

type fakeChannelModelReader struct {
	items map[uint]*model.ChannelModel
}

func (f fakeChannelModelReader) FindByID(id uint) (*model.ChannelModel, error) {
	item := f.items[id]
	if item == nil {
		return nil, errors.New("not found")
	}
	return item, nil
}

type fakeChannelKeyReader struct {
	keys map[uint]string
}

func (f fakeChannelKeyReader) DecryptedApiKey(id uint) (string, error) {
	key := f.keys[id]
	if key == "" {
		return "", errors.New("not found")
	}
	return key, nil
}

func (f fakeChannelKeyReader) Disable(_ uint) error { return nil }

type fakePricingReader struct {
	items map[string]*model.CreditPricing
}

func (f fakePricingReader) FindPricing(_ uint, modelName string, _ uint) (*model.CreditPricing, error) {
	item := f.items[modelName]
	return item, nil
}

type countingPricingReader struct {
	calls int
}

func (f *countingPricingReader) FindPricing(_ uint, _ string, _ uint) (*model.CreditPricing, error) {
	f.calls++
	return nil, nil
}

type fakePricingMapReader struct {
	items map[string]map[uint]model.CreditPricing
}

type fakeAutoChannelModelAggregator struct {
	items []AggregatedModel
}

type fakeChannelPricingReader struct {
	items map[uint]*model.CreditPricing
}

type recordingPricingReader struct {
	items     map[string]map[uint]*model.CreditPricing
	modelName string
	channelID uint
}

func (f fakeChannelPricingReader) FindPricing(_ uint, _ string, channelID uint) (*model.CreditPricing, error) {
	return f.items[channelID], nil
}

func (f *recordingPricingReader) FindPricing(_ uint, modelName string, channelID uint) (*model.CreditPricing, error) {
	f.modelName = modelName
	f.channelID = channelID
	return f.items[modelName][channelID], nil
}

func (f fakeAutoChannelModelAggregator) AggregateModels() ([]AggregatedModel, error) {
	return f.items, nil
}

func (f fakePricingMapReader) FindPricingMap(_ uint) (map[string]map[uint]model.CreditPricing, error) {
	return f.items, nil
}

type fakeChannelModelServiceChannelRepo struct {
	items map[uint]*model.Channel
}

func (f fakeChannelModelServiceChannelRepo) FindByID(id uint) (*model.Channel, error) {
	item := f.items[id]
	if item == nil {
		return nil, errors.New("not found")
	}
	return item, nil
}

func (f fakeChannelModelServiceChannelRepo) Save(_ *model.Channel) error { return nil }

type fakeChannelModelServiceModelRepo struct {
	items []model.ChannelModel
}

func (f fakeChannelModelServiceModelRepo) FindByID(id uint) (*model.ChannelModel, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (f fakeChannelModelServiceModelRepo) FindByChannelAndName(channelID uint, modelName string) (*model.ChannelModel, error) {
	for i := range f.items {
		if f.items[i].ChannelID == channelID && f.items[i].ModelName == modelName {
			return &f.items[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (f fakeChannelModelServiceModelRepo) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	result := make([]model.ChannelModel, 0)
	for _, item := range f.items {
		if item.ChannelID != channelID {
			continue
		}
		if enabledOnly && !item.Enabled {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (f fakeChannelModelServiceModelRepo) Save(_ *model.ChannelModel) error           { return nil }
func (f fakeChannelModelServiceModelRepo) Upsert(_ *model.ChannelModel) error         { return nil }
func (f fakeChannelModelServiceModelRepo) DeleteStaleModels(_ uint, _ []string) error { return nil }

func newRouteTestGenerateService() *GenerateService {
	return &GenerateService{
		channelSvc: fakeChannelKeyReader{keys: map[uint]string{1: "key-a", 2: "key-b", 4: "key-model-test"}},
		channelRepo: fakeChannelReader{items: map[uint]*model.Channel{
			1: {BaseModel: model.BaseModel{ID: 1}, Name: "A", BaseUrl: "https://a.example", Enabled: true},
			2: {BaseModel: model.BaseModel{ID: 2}, Name: "B", BaseUrl: "https://b.example", Enabled: true},
			3: {BaseModel: model.BaseModel{ID: 3}, Name: "Disabled", BaseUrl: "https://disabled.example", Enabled: false},
			4: {BaseModel: model.BaseModel{ID: 4}, Name: "ModelTest", BaseUrl: "https://model-test.example", Enabled: true},
		}},
		modelRepo: fakeChannelModelReader{items: map[uint]*model.ChannelModel{
			11: {BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "same-model", Capabilities: `["image","text"]`, Enabled: true},
			22: {BaseModel: model.BaseModel{ID: 22}, ChannelID: 2, ModelName: "same-model", Capabilities: `["image"]`, Enabled: true},
			33: {BaseModel: model.BaseModel{ID: 33}, ChannelID: 1, ModelName: "same-model", Capabilities: `["image"]`, Enabled: false},
			44: {BaseModel: model.BaseModel{ID: 44}, ChannelID: 4, ModelName: "text-model", Capabilities: `["text"]`, Enabled: true},
			55: {BaseModel: model.BaseModel{ID: 55}, ChannelID: 1, ModelName: "synced-model", Capabilities: ``, Enabled: true},
		}},
		creditRepo: fakePricingReader{items: map[string]*model.CreditPricing{
			"same-model":   {Model: "same-model", CreditsPerUnit: 1, UnitType: model.UnitPerImage},
			"text-model":   {Model: "text-model", CreditsPerUnit: 1, UnitType: model.UnitPerToken},
			"synced-model": {Model: "synced-model", CreditsPerUnit: 1, UnitType: model.UnitPerImage},
		}},
		httpClient: http.DefaultClient,
	}
}

func TestResolveChannelRouteSameModelDifferentChannels(t *testing.T) {
	svc := newRouteTestGenerateService()

	first, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 1, ChannelModelID: 11}, "image", "same-model")
	if err != nil {
		t.Fatalf("resolve first channel failed: %v", err)
	}
	second, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 2, ChannelModelID: 22}, "image", "same-model")
	if err != nil {
		t.Fatalf("resolve second channel failed: %v", err)
	}

	if first.Channel.BaseUrl != "https://a.example" || first.ApiKey != "key-a" || *first.ChannelModelID != 11 {
		t.Fatalf("unexpected first route: %#v", first)
	}
	if second.Channel.BaseUrl != "https://b.example" || second.ApiKey != "key-b" || *second.ChannelModelID != 22 {
		t.Fatalf("unexpected second route: %#v", second)
	}
}

func TestResolveChannelRouteForEstimateUsesExactNormalSelection(t *testing.T) {
	svc := newRouteTestGenerateService()

	resolved, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{ChannelID: 2, ChannelModelID: 22}, "image", "same-model", "")
	if err != nil {
		t.Fatalf("resolve estimate route failed: %v", err)
	}
	if resolved.Selection.ChannelID != 2 || resolved.Selection.ChannelModelID != 22 || resolved.PricingModel != "same-model" {
		t.Fatalf("unexpected resolved route: %#v", resolved)
	}
	if _, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{ChannelID: 2, ChannelModelID: 11}, "image", "same-model", ""); err == nil {
		t.Fatal("expected mismatched exact selection to fail")
	}
}

func TestResolveChannelRouteForEstimateMatchesAutoCandidatePolicy(t *testing.T) {
	svc := newRouteTestGenerateService()
	svc.autoChannelService = fakeAutoChannelModelAggregator{items: []AggregatedModel{{Model: "same-model", Channels: []AggregatedChannelRef{
		{ChannelID: 1, ChannelModelID: 11, SuccessRate: 0.4},
		{ChannelID: 2, ChannelModelID: 22, SuccessRate: 0.9},
	}}}}
	svc.creditRepo = fakeChannelPricingReader{items: map[uint]*model.CreditPricing{
		2: {Model: "same-model", ChannelID: 2, CreditsPerUnit: 7, UnitType: model.UnitPerImage},
	}}

	resolved, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{}, "image", "same-model", "")
	if err != nil {
		t.Fatalf("resolve Auto estimate route failed: %v", err)
	}
	if resolved.Selection.ChannelID != 2 || resolved.Selection.ChannelModelID != 22 || resolved.PricingModel != "same-model" {
		t.Fatalf("expected highest-success Auto candidate, got %#v", resolved)
	}
}

func TestResolveChannelRouteForEstimateSkipsUnpricedAutoCandidate(t *testing.T) {
	svc := newRouteTestGenerateService()
	svc.autoChannelService = fakeAutoChannelModelAggregator{items: []AggregatedModel{{Model: "same-model", Channels: []AggregatedChannelRef{
		{ChannelID: 1, ChannelModelID: 11, SuccessRate: 0.4},
		{ChannelID: 2, ChannelModelID: 22, SuccessRate: 0.9},
	}}}}
	svc.creditRepo = fakeChannelPricingReader{items: map[uint]*model.CreditPricing{
		1: {Model: "same-model", ChannelID: 1, CreditsPerUnit: 3, UnitType: model.UnitPerImage},
	}}

	resolved, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{}, "image", "same-model", "")
	if err != nil {
		t.Fatalf("resolve Auto estimate route failed: %v", err)
	}
	if resolved.Selection.ChannelID != 1 || resolved.Selection.ChannelModelID != 11 || resolved.PricingModel != "same-model" {
		t.Fatalf("expected first priced Auto candidate, got %#v", resolved)
	}
}

func TestResolveChannelRouteForEstimateRejectsAllUnpricedAutoCandidates(t *testing.T) {
	svc := newRouteTestGenerateService()
	svc.autoChannelService = fakeAutoChannelModelAggregator{items: []AggregatedModel{{Model: "same-model", Channels: []AggregatedChannelRef{
		{ChannelID: 1, ChannelModelID: 11, SuccessRate: 40},
		{ChannelID: 2, ChannelModelID: 22, SuccessRate: 90},
	}}}}
	svc.creditRepo = fakeChannelPricingReader{items: map[uint]*model.CreditPricing{}}

	_, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{}, "image", "same-model", "")
	if err == nil || !strings.Contains(err.Error(), "无已配置计费的可用候选") {
		t.Fatalf("expected dedicated no-priced-candidate error, got %v", err)
	}
}

func TestResolveChannelRouteForEstimateUsesGenerationMergeResolver(t *testing.T) {
	svc := newRouteTestGenerateService()
	svc.estimateFuzzyRoute = func(channelID uint, fuzzyGroupName, capability string) (*channelRouteContext, error) {
		if channelID != 1 || fuzzyGroupName != "gpt-4o" || capability != "text" {
			t.Fatalf("unexpected merge request: channel=%d group=%s capability=%s", channelID, fuzzyGroupName, capability)
		}
		return &channelRouteContext{
			ChannelModel:   &model.ChannelModel{BaseModel: model.BaseModel{ID: 44}, ChannelID: 1, ModelName: "text-model"},
			ChannelID:      uintPtr(1),
			ChannelModelID: uintPtr(44),
		}, nil
	}

	resolved, err := svc.ResolveChannelRouteForEstimate(1, ChannelSelection{ChannelID: 1}, "text", "gpt-4o", "gpt-4o")
	if err != nil {
		t.Fatalf("resolve merge estimate route failed: %v", err)
	}
	if resolved.Selection.ChannelID != 1 || resolved.Selection.ChannelModelID != 44 || resolved.PricingModel != "text-model" {
		t.Fatalf("unexpected merge route: %#v", resolved)
	}
}

func TestMergePricingIdentityUsesResolvedConcreteModel(t *testing.T) {
	pricing := &recordingPricingReader{items: map[string]map[uint]*model.CreditPricing{
		"text-model": {1: {Model: "text-model", ChannelID: 1, CreditsPerUnit: 4, UnitType: model.UnitPerToken}},
	}}
	svc := newRouteTestGenerateService()
	svc.creditRepo = pricing
	route := &channelRouteContext{
		ChannelModel: &model.ChannelModel{ChannelID: 1, ModelName: "text-model"},
		ChannelID:    uintPtr(1),
	}
	channelID, pricingModel := pricingIdentityFromRoute(route, 1, "gpt-4o")
	if _, _, err := svc.getRequiredPricing(1, channelID, "text", pricingModel, "application/json", []byte(`{"model":"gpt-4o"}`)); err != nil {
		t.Fatalf("price concrete merge model: %v", err)
	}
	if pricing.modelName != "text-model" || pricing.channelID != 1 {
		t.Fatalf("priced alias instead of concrete model: model=%q channel=%d", pricing.modelName, pricing.channelID)
	}
}

func TestAutoSuccessRateSerializesAsFrontendPercentage(t *testing.T) {
	payload, err := json.Marshal(AggregatedChannelRef{SuccessRate: successRatePercentage(100, 95)})
	if err != nil {
		t.Fatalf("marshal Auto channel: %v", err)
	}
	var decoded struct {
		SuccessRate float64 `json:"success_rate"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode Auto channel: %v", err)
	}
	if got := decoded.SuccessRate; got != 95 {
		t.Fatalf("serialized success_rate = %v, want 95; payload=%s", got, payload)
	}
}

func TestSharedRawModelPricingAppliesAcrossChannels(t *testing.T) {
	svc := newRouteTestGenerateService()
	for _, selection := range []ChannelSelection{{ChannelID: 1, ChannelModelID: 11}, {ChannelID: 2, ChannelModelID: 22}} {
		if _, err := svc.resolveChannelRoute(selection, "image", "same-model"); err != nil {
			t.Fatalf("resolve channel %d failed: %v", selection.ChannelID, err)
		}
		cost, result, err := svc.getRequiredPricing(1, selection.ChannelID, "image", "same-model", "application/json", []byte(`{"model":"same-model","n":2}`))
		if err != nil {
			t.Fatalf("pricing channel %d failed: %v", selection.ChannelID, err)
		}
		if cost != 2 || result.UnitCost != 1 || result.Units != 2 {
			t.Fatalf("unexpected pricing for channel %d: cost=%d result=%#v", selection.ChannelID, cost, result)
		}
	}
}

func TestUserCatalogKeepsPricedSameNamePerChannelAndDropsUnpriced(t *testing.T) {
	channelRepo := fakeChannelModelServiceChannelRepo{items: map[uint]*model.Channel{
		1: {BaseModel: model.BaseModel{ID: 1}, Name: "A", Enabled: true},
		2: {BaseModel: model.BaseModel{ID: 2}, Name: "B", Enabled: true},
	}}
	modelRepo := fakeChannelModelServiceModelRepo{items: []model.ChannelModel{
		{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "same-model", Enabled: false, Capabilities: `["image"]`},
		{BaseModel: model.BaseModel{ID: 22}, ChannelID: 2, ModelName: "same-model", Enabled: true, Capabilities: `["image"]`},
		{BaseModel: model.BaseModel{ID: 23}, ChannelID: 2, ModelName: "unpriced-model", Enabled: true, Capabilities: `["image"]`},
	}}
	pricingRepo := fakePricingMapReader{items: map[string]map[uint]model.CreditPricing{
		"same-model": {0: {Model: "same-model", CreditsPerUnit: 1, UnitType: model.UnitPerImage}},
	}}
	svc := NewChannelModelService(nil, channelRepo, modelRepo, pricingRepo)

	first, err := svc.ListUserCatalog(1, 1)
	if err != nil {
		t.Fatalf("list channel A failed: %v", err)
	}
	if len(first) != 0 {
		t.Fatalf("disabled channel A model should be omitted, got %#v", first)
	}
	second, err := svc.ListUserCatalog(1, 2)
	if err != nil {
		t.Fatalf("list channel B failed: %v", err)
	}
	if len(second) != 1 || second[0].ID != 22 || second[0].ModelName != "same-model" {
		t.Fatalf("expected only priced same-model on channel B, got %#v", second)
	}
}

func TestResolveChannelRouteAllowsSyncedDefaultCapabilities(t *testing.T) {
	svc := newRouteTestGenerateService()

	for _, capability := range defaultChannelModelCapabilities() {
		if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 1, ChannelModelID: 55}, capability, "synced-model"); err != nil {
			t.Fatalf("expected synced default capability %s to resolve: %v", capability, err)
		}
	}

	encoded := defaultChannelModelCapabilitiesJSON()
	for _, capability := range defaultChannelModelCapabilities() {
		if !strings.Contains(encoded, capability) {
			t.Fatalf("default capability json %q missing %s", encoded, capability)
		}
	}
}

func TestResolveChannelRouteRejectsInvalidIdentity(t *testing.T) {
	svc := newRouteTestGenerateService()

	if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 2, ChannelModelID: 11}, "image", "same-model"); err == nil {
		t.Fatalf("expected mismatched channel/model to fail")
	}
	if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 3, ChannelModelID: 33}, "image", "same-model"); err == nil {
		t.Fatalf("expected disabled channel to fail")
	}
	if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 1, ChannelModelID: 33}, "image", "same-model"); err == nil {
		t.Fatalf("expected disabled model to fail")
	}
	if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 2, ChannelModelID: 22}, "text", "same-model"); err == nil {
		t.Fatalf("expected unsupported capability to fail")
	}
}

func TestResolveChannelRouteTreatsEmptyCapabilitiesAsAuto(t *testing.T) {
	svc := newRouteTestGenerateService()
	svc.modelRepo = fakeChannelModelReader{items: map[uint]*model.ChannelModel{
		44: {BaseModel: model.BaseModel{ID: 44}, ChannelID: 1, ModelName: "auto-model", Capabilities: "", Enabled: true},
	}}

	if _, err := svc.resolveChannelRoute(ChannelSelection{ChannelID: 1, ChannelModelID: 44}, "video", "auto-model"); err != nil {
		t.Fatalf("expected empty capabilities to behave as auto: %v", err)
	}
}

func TestTestModelUsesSelectedChannelWithoutTenantConfig(t *testing.T) {
	fake := NewFakeUpstreamServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer key-a" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ok"}]}`))
	})
	defer fake.Close()

	svc := newRouteTestGenerateService()
	svc.apiConfigRepo = nil
	svc.channelRepo = fakeChannelReader{items: map[uint]*model.Channel{
		1: {BaseModel: model.BaseModel{ID: 1}, Name: "A", BaseUrl: fake.URL(), Enabled: true},
	}}
	svc.modelRepo = fakeChannelModelReader{items: map[uint]*model.ChannelModel{
		11: {BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "same-model", Capabilities: `["image"]`, Enabled: true, ImageGenerateRoute: "generations"},
	}}

	result, err := svc.TestModel(1, 1, ModelTestInput{Model: "same-model", ChannelID: 1, ChannelModelID: 11})
	if err != nil {
		t.Fatalf("test model failed: %v", err)
	}
	if !result.Success || result.Path != "/images/generations" || result.Generation != "image" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := len(fake.Requests()); got != 1 {
		t.Fatalf("expected one upstream request, got %d", got)
	}
}

func TestProxyRawRejectsBeforeUpstreamOnInvalidIdentity(t *testing.T) {
	fake := NewFakeUpstreamServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	defer fake.Close()

	svc := newRouteTestGenerateService()
	pricing := &countingPricingReader{}
	svc.creditRepo = pricing
	svc.channelRepo = fakeChannelReader{items: map[uint]*model.Channel{
		3: {BaseModel: model.BaseModel{ID: 3}, Name: "Disabled", BaseUrl: fake.URL(), Enabled: false},
	}}
	body := []byte(`{"model":"same-model","prompt":"test"}`)
	_, err := svc.ProxyRawWithRepair(1, 1, http.MethodPost, "/v1/images/generations", "application/json", body, ChannelSelection{ChannelID: 3, ChannelModelID: 33})
	if err == nil {
		t.Fatalf("expected disabled channel request to fail")
	}
	if got := len(fake.Requests()); got != 0 {
		t.Fatalf("upstream was called %d times", got)
	}
	if pricing.calls != 0 {
		t.Fatalf("credit pricing path was called %d times", pricing.calls)
	}
}

func TestStripChannelIdentityBeforeUpstream(t *testing.T) {
	body := stripJSONChannelIdentity("application/json", []byte(`{"model":"same-model","channel_id":1,"channel_model_id":11}`))
	if string(body) != `{"model":"same-model"}` {
		t.Fatalf("unexpected stripped body: %s", body)
	}
	path := stripChannelIdentityQuery("/v1/images/generations?channel_id=1&channel_model_id=11&keep=true")
	if path != "/v1/images/generations?keep=true" {
		t.Fatalf("unexpected stripped path: %s", path)
	}
}

func TestModelTestUsesSelectedChannelWithoutTenantConfig(t *testing.T) {
	fake := NewFakeUpstreamServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	})
	defer fake.Close()

	svc := newRouteTestGenerateService()
	svc.channelRepo = fakeChannelReader{items: map[uint]*model.Channel{
		4: {BaseModel: model.BaseModel{ID: 4}, Name: "ModelTest", BaseUrl: fake.URL(), Enabled: true},
	}}
	result, err := svc.TestModel(1, 1, ModelTestInput{Model: "text-model", ChannelID: 4, ChannelModelID: 44, Generation: "text"})
	if err != nil {
		t.Fatalf("model test failed: %v", err)
	}
	if !result.Success || result.Path != "/chat/completions" {
		t.Fatalf("unexpected model test result: %#v", result)
	}
	requests := fake.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one upstream request, got %d", len(requests))
	}
	if requests[0].Path != "/v1/chat/completions" {
		t.Fatalf("unexpected upstream path: %s", requests[0].Path)
	}
	if requests[0].Header.Get("Authorization") != "Bearer key-model-test" {
		t.Fatalf("unexpected authorization header: %s", requests[0].Header.Get("Authorization"))
	}
}

func TestModelTestRejectsUnpricedBeforeUpstream(t *testing.T) {
	fake := NewFakeUpstreamServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	})
	defer fake.Close()

	svc := newRouteTestGenerateService()
	svc.channelRepo = fakeChannelReader{items: map[uint]*model.Channel{
		4: {BaseModel: model.BaseModel{ID: 4}, Name: "ModelTest", BaseUrl: fake.URL(), Enabled: true},
	}}
	svc.modelRepo = fakeChannelModelReader{items: map[uint]*model.ChannelModel{
		66: {BaseModel: model.BaseModel{ID: 66}, ChannelID: 4, ModelName: "unpriced-model", Capabilities: `["text"]`, Enabled: true},
	}}
	svc.creditRepo = fakePricingReader{items: map[string]*model.CreditPricing{}}

	_, err := svc.TestModel(1, 1, ModelTestInput{Model: "unpriced-model", ChannelID: 4, ChannelModelID: 66, Generation: "text"})
	if err == nil || !strings.Contains(err.Error(), "未配置计费") {
		t.Fatalf("expected unpriced model rejection, got %v", err)
	}
	if got := len(fake.Requests()); got != 0 {
		t.Fatalf("upstream was called %d times", got)
	}
}
