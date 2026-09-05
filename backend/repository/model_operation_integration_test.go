package repository

import (
	"testing"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

func TestModelConfigRepoPreservesOperationEnabledWithLegacyDefault(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	// Reproduce deployed schemas where the database still defaults enabled to TRUE.
	if err := db.Exec("ALTER TABLE channel_model_operations ALTER COLUMN enabled SET DEFAULT TRUE").Error; err != nil {
		t.Fatal(err)
	}
	channel := model.Channel{Name: "Switches", BaseUrl: "https://example.com", ApiKey: "encrypted", Enabled: true}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	item := model.ChannelModel{ChannelID: channel.ID, ModelName: "switch-model", Status: model.ModelStatusDraft, ConfigRevision: 1}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewModelConfigRepo(db)
	states := [][2]bool{{true, true}, {false, true}, {false, true}, {false, false}, {false, false}, {true, false}}
	for i, state := range states {
		params := SaveModelConfigParams{
			TenantID: 3, ActorUserID: 8, ModelID: item.ID, ExpectedRevision: uint(i + 1),
			PublicKey: "switch-model", DisplayName: "Switch model", UpstreamModelID: item.ModelName, Status: model.ModelStatusDraft,
			Operations: []model.ChannelModelOperation{
				{Capability: "image", Operation: "generate", Enabled: state[0], ProtocolMode: model.ProtocolModeOverride, Adapter: "generations", ConfigJSON: "{}", ConfigVersion: 1, ContractKey: "image-contract"},
				{Capability: "text", Operation: "generate", Enabled: state[1], ProtocolMode: model.ProtocolModeOverride, Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1, ContractKey: "text-contract"},
			},
		}
		if err := repo.SaveModelConfig(params); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		assertStoredOperationStates(t, db, item.ID, map[string]bool{"image": state[0], "text": state[1]})
		for j, enabled := range state {
			if params.Operations[j].Enabled != enabled {
				t.Fatalf("save %d mutated operation %d enabled", i, j)
			}
		}
		var saved model.ChannelModel
		if err := db.First(&saved, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.ConfigRevision != uint(i+2) || saved.Status != model.ModelStatusDraft {
			t.Fatalf("unexpected saved revision or status: %#v", saved)
		}
	}
}

func TestAPIConfigTransferRepoPreservesOperationEnabledWithLegacyDefault(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	if err := db.Exec("ALTER TABLE channel_model_operations ALTER COLUMN enabled SET DEFAULT TRUE").Error; err != nil {
		t.Fatal(err)
	}
	plan := &APIConfigTransferApplyPlan{
		SchemaVersion: 2,
		Channels:      []APIConfigTransferChannelOperation{{Ref: "switches", Item: model.Channel{Name: "Switches", BaseUrl: "https://example.com", ApiKey: "encrypted", Enabled: true}}},
		Models: []APIConfigTransferModelOperation{{
			ChannelRef: "switches", PublicKey: "switch-model", DisplayName: "Switch model",
			Item: model.ChannelModel{ModelName: "switch-model", Status: model.ModelStatusDraft, ConfigRevision: 1},
			Operations: []model.ChannelModelOperation{
				{Capability: "image", Operation: "generate", Enabled: false, ProtocolMode: model.ProtocolModeOverride, Adapter: "generations", ConfigJSON: "{}", ConfigVersion: 1},
				{Capability: "text", Operation: "generate", Enabled: true, ProtocolMode: model.ProtocolModeOverride, Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1},
			},
		}},
	}
	repo := NewAPIConfigTransferRepo(db)
	for i := 0; i < 2; i++ {
		if err := repo.Apply(plan); err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
		var saved model.ChannelModel
		if err := db.Where("model_name = ?", "switch-model").First(&saved).Error; err != nil {
			t.Fatal(err)
		}
		assertStoredOperationStates(t, db, saved.ID, map[string]bool{"image": false, "text": true})
		plan.Channels[0].ExistingID = saved.ChannelID
		plan.Models[0].ExistingID = saved.ID
		plan.Models[0].Item.BaseModel = saved.BaseModel
		if err := db.First(&plan.Channels[0].Item, saved.ChannelID).Error; err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := db.Model(&model.ChannelModel{}).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("model count=%d err=%v, want one model after import", count, err)
		}
	}
}

func assertStoredOperationStates(t *testing.T, db *gorm.DB, modelID uint, want map[string]bool) {
	t.Helper()
	var items []model.ChannelModelOperation
	if err := db.Where("channel_model_id = ?", modelID).Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	if len(items) != len(want) {
		t.Fatalf("operations=%d, want %d", len(items), len(want))
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		enabled, ok := want[item.Capability]
		if !ok || seen[item.Capability] || item.Enabled != enabled {
			t.Fatalf("unexpected saved operation: %#v; want %v", item, want)
		}
		seen[item.Capability] = true
		if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() || item.ConfigJSON != "{}" || item.ConfigVersion != 1 || item.ProtocolMode != model.ProtocolModeOverride {
			t.Fatalf("operation metadata was lost: %#v", item)
		}
	}
}
