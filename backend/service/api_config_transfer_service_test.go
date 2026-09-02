package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"infinite-canvas-server/crypto"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type apiConfigTransferRepoStub struct {
	data     *repository.APIConfigTransferData
	applied  *repository.APIConfigTransferApplyPlan
	applyErr error
}

func (stub *apiConfigTransferRepoStub) Load(uint) (*repository.APIConfigTransferData, error) {
	return stub.data, nil
}

func (stub *apiConfigTransferRepoStub) Apply(plan *repository.APIConfigTransferApplyPlan) error {
	stub.applied = plan
	return stub.applyErr
}

func TestAPIConfigTransferEncryptionRejectsWrongPasswordAndTampering(t *testing.T) {
	snapshot := &model.APIConfigTransferSnapshot{SchemaVersion: 1, Channels: []model.APIConfigTransferChannel{}, Pricing: []model.APIConfigTransferPricing{}, VideoPresets: []model.APIConfigTransferVideoPreset{}}
	envelope, err := encryptAPIConfigSnapshot(snapshot, "password-123")
	if err != nil {
		t.Fatalf("encrypt snapshot: %v", err)
	}
	decoded, err := decryptAPIConfigSnapshot(envelope, "password-123")
	if err != nil || decoded.SchemaVersion != 1 {
		t.Fatalf("decrypt snapshot: %#v, %v", decoded, err)
	}
	if _, err := decryptAPIConfigSnapshot(envelope, "wrong-password"); err == nil {
		t.Fatal("wrong password should fail")
	}
	tampered := envelope
	tampered.Ciphertext = tampered.Ciphertext[:len(tampered.Ciphertext)-4] + "AAAA"
	if _, err := decryptAPIConfigSnapshot(tampered, "password-123"); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
	unsupported := envelope
	unsupported.KDF.MemoryKiB++
	if _, err := decryptAPIConfigSnapshot(unsupported, "password-123"); err == nil {
		t.Fatal("unsupported KDF parameters should fail")
	}
}

func TestAPIConfigTransferExportIncludesRoutesPricingPresetsAndDecryptedKey(t *testing.T) {
	const appKey = "application-encryption-key"
	encryptedKey, err := crypto.Encrypt(appKey, "sk-exported")
	if err != nil {
		t.Fatal(err)
	}
	presetConfig := serviceTestCustomVideoConfig()
	presetJSON, _ := json.Marshal(presetConfig)
	repo := &apiConfigTransferRepoStub{data: &repository.APIConfigTransferData{
		Channels:     []model.Channel{{BaseModel: model.BaseModel{ID: 7}, Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: encryptedKey, Enabled: true, VideoAPIStandard: model.VideoAPIStandardDefault}},
		Models:       []model.ChannelModel{{BaseModel: model.BaseModel{ID: 8}, ChannelID: 7, ModelName: "video-model", Capabilities: `["video"]`, Enabled: true, VideoRoute: "custom", VideoDurations: `[5,10]`, VideoCustomConfig: string(presetJSON)}},
		Pricing:      []model.CreditPricing{{ChannelID: 7, TenantID: 11, Model: "video-model", CreditsPerUnit: 2, UnitType: model.UnitPerVideo, PricingMode: model.PricingModePerUnit}},
		MergeGroups:  []model.ModelMergeGroup{{ID: 9, ChannelID: 7, GroupName: "video", Pattern: "video", Enabled: true}},
		VideoPresets: []model.VideoConfigPreset{{ID: 10, TenantID: 11, Name: "Omni", NormalizedName: "omni", Config: string(presetJSON)}},
	}}
	service := NewAPIConfigTransferService(repo, appKey)

	result, err := service.Export(11, "password-123")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	snapshot, err := decryptAPIConfigSnapshot(result.Envelope, "password-123")
	if err != nil {
		t.Fatalf("decrypt export: %v", err)
	}
	if result.Envelope.Version != 1 || snapshot.SchemaVersion != 2 {
		t.Fatalf("unexpected envelope/payload versions: envelope=%d payload=%d", result.Envelope.Version, snapshot.SchemaVersion)
	}
	if len(snapshot.Channels) != 1 || snapshot.Channels[0].APIKey != "sk-exported" || snapshot.Channels[0].Models[0].VideoRoute != "custom" {
		t.Fatalf("unexpected exported channel: %#v", snapshot.Channels)
	}
	envelopeJSON, _ := json.Marshal(result.Envelope)
	if string(envelopeJSON) == "" || !json.Valid(envelopeJSON) || strings.Contains(string(envelopeJSON), "sk-exported") {
		t.Fatal("encrypted envelope exposed the API key")
	}
	if len(snapshot.Pricing) != 1 || snapshot.Pricing[0].ChannelRef != snapshot.Channels[0].Ref {
		t.Fatalf("unexpected exported pricing: %#v", snapshot.Pricing)
	}
	if len(snapshot.Channels[0].MergeGroups) != 1 || len(snapshot.VideoPresets) != 1 {
		t.Fatalf("missing merge groups or presets: %#v", snapshot)
	}
	if result.Summary.Models.Create != 1 || result.Summary.Pricing.Create != 1 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected export summary: %#v", result)
	}
}

func TestAPIConfigTransferSchemaTwoRoundTripsNormalizedModelDomain(t *testing.T) {
	encryptedKey, err := crypto.Encrypt("app-key", "sk-schema-two")
	if err != nil {
		t.Fatal(err)
	}
	data := &repository.APIConfigTransferData{
		Channels: []model.Channel{{BaseModel: model.BaseModel{ID: 7}, Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: encryptedKey, Enabled: true, ConfigRevision: 3}},
		Catalogs: []model.CatalogModel{{BaseModel: model.BaseModel{ID: 5}, PublicKey: "public-image", DisplayName: "Public image"}},
		Models: []model.ChannelModel{{
			BaseModel: model.BaseModel{ID: 8}, ChannelID: 7, ModelName: "upstream-image", CatalogModelID: 5, UpstreamModelID: "upstream-image",
			Status: model.ModelStatusActive, DiscoveryStatus: model.DiscoveryStatusPresent, ConfigRevision: 4, Capabilities: `["image"]`, Enabled: true,
			ImageGenerateRoute: "generations", ImageEditRoute: "generations", VideoRoute: "auto", VideoDurations: `[]`,
		}},
		ProtocolDefaults: []model.ChannelProtocolDefault{{ChannelID: 7, Capability: "image", Operation: "generate", Adapter: "generations", ConfigJSON: `{"response_format":"url"}`, ConfigVersion: 2}},
		Operations: []model.ChannelModelOperation{{
			ChannelModelID: 8, Capability: "image", Operation: "edit", Enabled: true, ProtocolMode: model.ProtocolModeOverride,
			Adapter: "generations", ConfigJSON: `{"image_field":"image"}`, ConfigVersion: 3, ContractKey: "saved-contract",
		}},
		PricingRules: []model.ModelPricingRule{
			{TenantID: 11, CatalogModelID: 5, Capability: "image", Scope: model.PricingScopeDefault, ScopeID: 0, CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit, ConfigRevision: 2},
			{TenantID: 11, CatalogModelID: 5, Capability: "image", Scope: model.PricingScopeImplementation, ScopeID: 8, CreditsPerUnit: 4, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit, ConfigRevision: 5},
		},
	}
	service := NewAPIConfigTransferService(&apiConfigTransferRepoStub{}, "app-key")
	snapshot, summary, warnings, err := service.buildSnapshot(data)
	if err != nil {
		t.Fatalf("build schema two snapshot: %v", err)
	}
	if snapshot.SchemaVersion != 2 || len(warnings) != 0 || summary.Channels.Create != 1 || summary.Models.Create != 1 || summary.Pricing.Create != 2 {
		t.Fatalf("unexpected schema two summary: snapshot=%#v summary=%#v warnings=%#v", snapshot, summary, warnings)
	}
	channel := snapshot.Channels[0]
	if channel.ConfigRevision != 3 || len(channel.ProtocolDefaults) != 1 || channel.ProtocolDefaults[0].ConfigVersion != 2 {
		t.Fatalf("channel defaults did not round trip: %#v", channel)
	}
	transferredModel := channel.Models[0]
	if transferredModel.PublicKey != "public-image" || transferredModel.DisplayName != "Public image" || transferredModel.UpstreamModelID != "upstream-image" || transferredModel.ConfigRevision != 4 {
		t.Fatalf("model identity did not round trip: %#v", transferredModel)
	}
	if len(transferredModel.Operations) != 1 || transferredModel.Operations[0].ProtocolMode != model.ProtocolModeOverride || transferredModel.Operations[0].ContractKey != "saved-contract" {
		t.Fatalf("model operations did not round trip: %#v", transferredModel.Operations)
	}

	envelope, err := encryptAPIConfigSnapshot(snapshot, "password-123")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decryptAPIConfigSnapshot(envelope, "password-123")
	if err != nil || decoded.SchemaVersion != 2 {
		t.Fatalf("decrypt schema two payload: %#v %v", decoded, err)
	}
	plan, result := service.buildImportPlan(21, decoded, &repository.APIConfigTransferData{})
	if len(result.Conflicts) != 0 || result.Stats.Channels.Create != 1 || result.Stats.Models.Create != 1 || result.Stats.Pricing.Create != 2 {
		t.Fatalf("unexpected schema two import result: %#v", result)
	}
	if len(plan.Channels) != 1 || len(plan.Channels[0].Defaults) != 1 || len(plan.Models) != 1 || len(plan.Models[0].Operations) != 1 || len(plan.PricingRules) != 2 {
		t.Fatalf("normalized domain missing from import plan: %#v", plan)
	}
	if plan.PricingRules[0].Item.TenantID != 21 || plan.PricingRules[1].Item.TenantID != 21 {
		t.Fatalf("pricing rules were not rebound to target tenant: %#v", plan.PricingRules)
	}
}

func TestAPIConfigTransferRoundTripsAutoRoutingPools(t *testing.T) {
	encryptedKey, err := crypto.Encrypt("app-key", "sk-route")
	if err != nil {
		t.Fatal(err)
	}
	channels := []model.Channel{
		{BaseModel: model.BaseModel{ID: 1}, Name: "Channel A", BaseUrl: "https://a.example.com", ApiKey: encryptedKey, Enabled: true},
		{BaseModel: model.BaseModel{ID: 2}, Name: "Channel B", BaseUrl: "https://b.example.com", ApiKey: encryptedKey, Enabled: true},
	}
	models := []model.ChannelModel{
		{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "shared-image", Capabilities: `["image"]`, Enabled: true, ImageGenerateRoute: "generations", ImageEditRoute: "edits"},
		{BaseModel: model.BaseModel{ID: 22}, ChannelID: 2, ModelName: "shared-image", Capabilities: `["image"]`, Enabled: true, ImageGenerateRoute: "generations", ImageEditRoute: "edits"},
	}
	contract, err := autoRoutingContract(&channels[0], &models[0], "image")
	if err != nil {
		t.Fatal(err)
	}
	data := &repository.APIConfigTransferData{
		Channels: channels,
		Models:   models,
		AutoRoutingPools: []model.AutoRoutingPool{{
			BaseModel: model.BaseModel{ID: 9}, PublicModelName: "shared-image", Capability: "image", ContractKey: contract, Enabled: true, MaxAttempts: 2,
			Members: []model.AutoRoutingPoolMember{{BaseModel: model.BaseModel{ID: 91}, PoolID: 9, ChannelModelID: 11, Priority: 3, Enabled: true}, {BaseModel: model.BaseModel{ID: 92}, PoolID: 9, ChannelModelID: 22, Enabled: true}},
		}},
	}
	service := NewAPIConfigTransferService(&apiConfigTransferRepoStub{}, "app-key")
	snapshot, summary, warnings, err := service.buildSnapshot(data)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(warnings) != 0 || summary.AutoRoutingPools.Create != 1 || len(snapshot.AutoRoutingPools) != 1 || len(snapshot.AutoRoutingPools[0].Members) != 2 {
		t.Fatalf("unexpected exported route pools: snapshot=%#v summary=%#v warnings=%#v", snapshot.AutoRoutingPools, summary.AutoRoutingPools, warnings)
	}

	plan, result := service.buildImportPlan(1, snapshot, &repository.APIConfigTransferData{})
	if result.Stats.AutoRoutingPools.Create != 1 || result.Stats.AutoRoutingPools.Skip != 0 || len(plan.AutoRoutingPools) != 1 || len(plan.AutoRoutingPools[0].Members) != 2 {
		t.Fatalf("unexpected route pool import: result=%#v plan=%#v", result, plan.AutoRoutingPools)
	}
}

func TestAPIConfigTransferImportAllowsChannelsToShareBaseURL(t *testing.T) {
	service := NewAPIConfigTransferService(&apiConfigTransferRepoStub{}, "app-key")
	snapshot := sharedURLTransferSnapshot()

	plan, result := service.buildImportPlan(5, snapshot, &repository.APIConfigTransferData{})
	if result.Stats.Channels.Create != 2 || result.Stats.Channels.Skip != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("shared URL channels should be created: %#v", result)
	}
	if len(plan.Channels) != 2 || len(plan.Models) != 2 || len(plan.Pricing) != 2 || len(plan.MergeGroups) != 2 {
		t.Fatalf("shared URL dependencies should remain importable: %#v", plan)
	}

	existing := &repository.APIConfigTransferData{
		Channels: []model.Channel{
			{BaseModel: model.BaseModel{ID: 1}, Name: "Image Channel", BaseUrl: "https://shared.example.com", ApiKey: "key"},
			{BaseModel: model.BaseModel{ID: 2}, Name: "Mixed Channel", BaseUrl: "https://shared.example.com", ApiKey: "key"},
		},
		Models: []model.ChannelModel{
			{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "image-model", Capabilities: `["image"]`, VideoDurations: `[]`},
			{BaseModel: model.BaseModel{ID: 12}, ChannelID: 2, ModelName: "mixed-model", Capabilities: `["image","video"]`, VideoDurations: `[]`},
		},
		Pricing: []model.CreditPricing{
			{BaseModel: model.BaseModel{ID: 21}, TenantID: 5, ChannelID: 1, Model: "image-model", CreditsPerUnit: 1, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
			{BaseModel: model.BaseModel{ID: 22}, TenantID: 5, ChannelID: 2, Model: "mixed-model", CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
		},
		MergeGroups: []model.ModelMergeGroup{
			{ID: 31, ChannelID: 1, GroupName: "image", Pattern: "image", Enabled: true},
			{ID: 32, ChannelID: 2, GroupName: "mixed", Pattern: "mixed", Enabled: true},
		},
	}
	plan, result = service.buildImportPlan(5, snapshot, existing)
	if result.Stats.Channels.Update != 2 || result.Stats.Channels.Skip != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("reimport should update shared URL channels: %#v", result)
	}
	if result.Stats.Models.Update != 2 || result.Stats.Pricing.Update != 2 || result.Stats.MergeGroups.Update != 2 {
		t.Fatalf("reimport should update shared URL dependencies: %#v", result.Stats)
	}
}

func TestAPIConfigTransferImportAllowsNewNameOnExistingBaseURL(t *testing.T) {
	service := NewAPIConfigTransferService(&apiConfigTransferRepoStub{}, "app-key")
	snapshot := sharedURLTransferSnapshot()
	snapshot.Channels = snapshot.Channels[:1]
	snapshot.Pricing = snapshot.Pricing[:1]
	data := &repository.APIConfigTransferData{Channels: []model.Channel{{BaseModel: model.BaseModel{ID: 9}, Name: "Existing Channel", BaseUrl: "https://shared.example.com", ApiKey: "key"}}}

	_, result := service.buildImportPlan(5, snapshot, data)
	if result.Stats.Channels.Create != 1 || result.Stats.Channels.Skip != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("different name on shared URL should be created: %#v", result)
	}
}

func TestAPIConfigTransferImportRejectsDuplicateChannelNamesAndReferences(t *testing.T) {
	service := NewAPIConfigTransferService(&apiConfigTransferRepoStub{}, "app-key")
	for name, channels := range map[string][]model.APIConfigTransferChannel{
		"duplicate reference": {
			{Ref: "same", Name: "Channel A", BaseURL: "https://a.example.com", APIKey: "key", VideoAPIStandard: "default"},
			{Ref: "same", Name: "Channel B", BaseURL: "https://b.example.com", APIKey: "key", VideoAPIStandard: "default"},
		},
		"duplicate name": {
			{Ref: "a", Name: "Same Channel", BaseURL: "https://a.example.com", APIKey: "key", VideoAPIStandard: "default"},
			{Ref: "b", Name: "Same Channel", BaseURL: "https://b.example.com", APIKey: "key", VideoAPIStandard: "default"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, result := service.buildImportPlan(5, &model.APIConfigTransferSnapshot{SchemaVersion: 1, Channels: channels}, &repository.APIConfigTransferData{})
			if result.Stats.Channels.Skip != 2 || len(result.Conflicts) != 2 {
				t.Fatalf("duplicates should be rejected: %#v", result)
			}
		})
	}
}

func sharedURLTransferSnapshot() *model.APIConfigTransferSnapshot {
	return &model.APIConfigTransferSnapshot{
		SchemaVersion: 1,
		Channels: []model.APIConfigTransferChannel{
			{Ref: "image", Name: "Image Channel", BaseURL: "https://shared.example.com", APIKey: "image-key", Enabled: true, VideoAPIStandard: "default", Models: []model.APIConfigTransferModel{{ModelName: "image-model", Capabilities: []string{"image"}, Enabled: true, VideoDurations: []int{}}}, MergeGroups: []model.APIConfigTransferMergeGroup{{GroupName: "image", Pattern: "image", Enabled: true}}},
			{Ref: "mixed", Name: "Mixed Channel", BaseURL: "https://shared.example.com", APIKey: "mixed-key", Enabled: true, VideoAPIStandard: "default", Models: []model.APIConfigTransferModel{{ModelName: "mixed-model", Capabilities: []string{"image", "video"}, Enabled: true, VideoDurations: []int{}}}, MergeGroups: []model.APIConfigTransferMergeGroup{{GroupName: "mixed", Pattern: "mixed", Enabled: true}}},
		},
		Pricing: []model.APIConfigTransferPricing{
			{Model: "image-model", ChannelRef: "image", CreditsPerUnit: 1, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
			{Model: "mixed-model", ChannelRef: "mixed", CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
		},
	}
}

func TestAPIConfigTransferImportMergesValidItemsAndSkipsChannelConflicts(t *testing.T) {
	const appKey = "application-encryption-key"
	existingKey, _ := crypto.Encrypt(appKey, "sk-old")
	presetConfig := serviceTestCustomVideoConfig()
	presetJSON, _ := json.Marshal(presetConfig)
	repo := &apiConfigTransferRepoStub{data: &repository.APIConfigTransferData{
		Channels: []model.Channel{
			{BaseModel: model.BaseModel{ID: 1}, Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: existingKey, Enabled: true},
			{BaseModel: model.BaseModel{ID: 2}, Name: "Collision", BaseUrl: "https://collision.example.com", ApiKey: existingKey, Enabled: true},
		},
		Models:       []model.ChannelModel{{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "shared-model", Capabilities: `["text"]`, Enabled: true, VideoDurations: `[]`}},
		Pricing:      []model.CreditPricing{{BaseModel: model.BaseModel{ID: 12}, TenantID: 5, Model: "shared-model", ChannelID: 0, CreditsPerUnit: 1, UnitType: model.UnitPerToken, PricingMode: model.PricingModePerUnit}},
		MergeGroups:  []model.ModelMergeGroup{{ID: 13, ChannelID: 1, GroupName: "shared", Pattern: "old", Enabled: true}},
		VideoPresets: []model.VideoConfigPreset{{ID: 14, TenantID: 5, Name: "Omni", NormalizedName: "omni", Config: string(presetJSON)}},
	}}
	snapshot := &model.APIConfigTransferSnapshot{
		SchemaVersion: 1,
		Channels: []model.APIConfigTransferChannel{
			{Ref: "primary", Name: "Primary", BaseURL: "https://api.example.com/", APIKey: "sk-new", Enabled: true, VideoAPIStandard: "default", Models: []model.APIConfigTransferModel{{ModelName: "shared-model", Capabilities: []string{"text", "image"}, Enabled: true, VideoDurations: []int{}}}, MergeGroups: []model.APIConfigTransferMergeGroup{{GroupName: "shared", Pattern: "new", Enabled: true}}},
			{Ref: "new", Name: "New Channel", BaseURL: "https://new.example.com", APIKey: "sk-created", Enabled: true, VideoAPIStandard: "default", Models: []model.APIConfigTransferModel{{ModelName: "new-model", Capabilities: []string{"image"}, Enabled: true, VideoDurations: []int{}}}},
			{Ref: "collision", Name: "Collision", BaseURL: "https://different.example.com", APIKey: "sk-collision", Enabled: true, VideoAPIStandard: "default", Models: []model.APIConfigTransferModel{{ModelName: "skipped-model", Capabilities: []string{"text"}, Enabled: true, VideoDurations: []int{}}}},
		},
		Pricing: []model.APIConfigTransferPricing{
			{Model: "shared-model", CreditsPerUnit: 3, UnitType: model.UnitPerToken, PricingMode: model.PricingModePerUnit},
			{Model: "new-model", ChannelRef: "new", CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
			{Model: "skipped-model", ChannelRef: "collision", CreditsPerUnit: 1, UnitType: model.UnitPerToken, PricingMode: model.PricingModePerUnit},
		},
		VideoPresets: []model.APIConfigTransferVideoPreset{{Name: "Omni", Config: *presetConfig}},
	}
	envelope, err := encryptAPIConfigSnapshot(snapshot, "password-123")
	if err != nil {
		t.Fatal(err)
	}
	service := NewAPIConfigTransferService(repo, appKey)
	result, err := service.Import(5, model.APIConfigTransferImportInput{Password: "password-123", Envelope: envelope})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !result.Applied || result.Stats.Channels.Create != 1 || result.Stats.Channels.Update != 1 || result.Stats.Channels.Skip != 1 {
		t.Fatalf("unexpected channel stats: %#v", result)
	}
	if result.Stats.Models.Create != 1 || result.Stats.Models.Update != 1 || result.Stats.Models.Skip != 1 {
		t.Fatalf("unexpected model stats: %#v", result.Stats.Models)
	}
	if result.Stats.Pricing.Create != 1 || result.Stats.Pricing.Update != 1 || result.Stats.Pricing.Skip != 1 {
		t.Fatalf("unexpected pricing stats: %#v", result.Stats.Pricing)
	}
	if repo.applied == nil || len(repo.applied.Channels) != 2 || len(repo.applied.Models) != 2 || len(repo.applied.Pricing) != 2 {
		t.Fatalf("unexpected apply plan: %#v", repo.applied)
	}
	var updated model.Channel
	for _, operation := range repo.applied.Channels {
		if operation.Ref == "primary" {
			updated = operation.Item
		}
	}
	decrypted, err := crypto.Decrypt(appKey, updated.ApiKey)
	if err != nil || decrypted != "sk-new" {
		t.Fatalf("imported API key=%q err=%v", decrypted, err)
	}
}

func TestAPIConfigTransferImportReturnsApplyFailure(t *testing.T) {
	repo := &apiConfigTransferRepoStub{data: &repository.APIConfigTransferData{}, applyErr: errors.New("transaction failed")}
	service := NewAPIConfigTransferService(repo, "app-key")
	snapshot := &model.APIConfigTransferSnapshot{SchemaVersion: 1, Channels: []model.APIConfigTransferChannel{}, Pricing: []model.APIConfigTransferPricing{}, VideoPresets: []model.APIConfigTransferVideoPreset{}}
	envelope, _ := encryptAPIConfigSnapshot(snapshot, "password-123")
	if _, err := service.Import(0, model.APIConfigTransferImportInput{Password: "password-123", Envelope: envelope}); err == nil {
		t.Fatal("apply failure should be returned")
	}
}

func TestAPIConfigTransferRejectsUnknownModelRoutes(t *testing.T) {
	base := model.APIConfigTransferModel{ModelName: "route-model", Capabilities: []string{"image", "video"}, VideoDurations: []int{}}
	for name, mutate := range map[string]func(*model.APIConfigTransferModel){
		"image generation": func(item *model.APIConfigTransferModel) { item.ImageGenerateRoute = "unknown" },
		"image edit":       func(item *model.APIConfigTransferModel) { item.ImageEditRoute = "unknown" },
		"video":            func(item *model.APIConfigTransferModel) { item.VideoRoute = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			item := base
			mutate(&item)
			if _, err := transferModelToRecord(&item); err == nil {
				t.Fatal("unknown imported route was accepted")
			}
		})
	}
}
