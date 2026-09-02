package service

import (
	"testing"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

func TestNormalizeModelOperationsResolvesChannelInheritanceAndModelOverride(t *testing.T) {
	defaults := []model.ChannelProtocolDefault{{ChannelID: 7, Capability: "image", Operation: "edit", Adapter: "generations", ConfigJSON: `{"image_field":"image"}`, ConfigVersion: 2}}
	operations, effective, err := normalizeModelOperations(7, []model.SaveModelOperationInput{
		{Capability: "image", Operation: "edit", Enabled: true, Mode: model.ProtocolModeInherit, Config: map[string]any{}},
		{Capability: "text", Operation: "generate", Enabled: true, Mode: model.ProtocolModeOverride, Adapter: "openai", Config: map[string]any{"temperature": 0.2}},
	}, defaults)
	if err != nil {
		t.Fatalf("normalize operations: %v", err)
	}
	if len(operations) != 2 || operations[0].Adapter != "" || operations[0].ProtocolMode != model.ProtocolModeInherit {
		t.Fatalf("unexpected stored operations: %#v", operations)
	}
	image := effective["image:edit"]
	if image.Source != "channel" || image.Adapter != "generations" || image.ConfigVersion != 2 || image.Config["image_field"] != "image" || image.ContractKey == "" {
		t.Fatalf("unexpected inherited protocol: %#v", image)
	}
	text := effective["text:generate"]
	if text.Source != "model" || text.Adapter != "openai" || text.ContractKey == "" || text.ContractKey == image.ContractKey {
		t.Fatalf("unexpected overridden protocol: %#v", text)
	}
}

func TestReadinessAndPricingPreferImplementationOverride(t *testing.T) {
	item := &model.ChannelModel{BaseModel: model.BaseModel{ID: 21}, CatalogModelID: 12, DiscoveryStatus: model.DiscoveryStatusPresent}
	prices := []model.ModelPricingRule{
		{BaseModel: model.BaseModel{ID: 1}, CatalogModelID: 12, Capability: "image", Scope: model.PricingScopeDefault, CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
		{BaseModel: model.BaseModel{ID: 2}, CatalogModelID: 12, Capability: "image", Scope: model.PricingScopeImplementation, ScopeID: 21, CreditsPerUnit: 5, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
	}
	effectivePricing := effectivePricingByCapability(prices, item.CatalogModelID, item.ID)
	if effectivePricing["image"].CreditsPerUnit != 5 || effectivePricing["image"].EffectiveSource != "implementation" {
		t.Fatalf("implementation price did not override default: %#v", effectivePricing)
	}
	operations := []model.ModelOperationInfo{{Capability: "image", Operation: "generate", Enabled: true, Effective: model.EffectiveProtocolInfo{Adapter: "generations"}}}
	if issues := readinessIssues(item, operations, effectivePricing); len(issues) != 0 {
		t.Fatalf("ready model has issues: %#v", issues)
	}
	delete(effectivePricing, "image")
	issues := readinessIssues(item, operations, effectivePricing)
	if len(issues) != 1 || issues[0].Code != "pricing_missing" {
		t.Fatalf("missing pricing issues=%#v", issues)
	}
}

func TestReadinessFlagsIncompleteDraftButPreservesLegacyActiveState(t *testing.T) {
	draft := &model.ChannelModel{Status: model.ModelStatusDraft, DiscoveryStatus: model.DiscoveryStatusPresent}
	issues := readinessIssues(draft, nil, nil)
	if len(issues) != 1 || issues[0].Code != "operation_missing" {
		t.Fatalf("draft readiness issues=%#v", issues)
	}
	legacy := &model.ChannelModel{Status: model.ModelStatusActive, LegacyUnreviewed: true, DiscoveryStatus: model.DiscoveryStatusPresent}
	data := repository.ModelConfigData{Models: []model.ChannelModel{*legacy}}
	info, err := buildModelConfigInfo(&data, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != model.ModelStatusActive || info.Ready || !info.LegacyUnreviewed {
		t.Fatalf("legacy active state should be preserved for review: %#v", info)
	}
}
