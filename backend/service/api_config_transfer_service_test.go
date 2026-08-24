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
