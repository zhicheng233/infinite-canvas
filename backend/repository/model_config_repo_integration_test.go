package repository

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"infinite-canvas-server/model"
)

func TestModelConfigRepoSavesModelAndLegacyPricingProjectionAtomically(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	channel := model.Channel{Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: "encrypted", Enabled: true, ConfigRevision: 1}
	catalog := model.CatalogModel{PublicKey: "old-public", DisplayName: "Old public"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatal(err)
	}
	implementation := model.ChannelModel{ChannelID: channel.ID, ModelName: "upstream-old", CatalogModelID: catalog.ID, UpstreamModelID: "upstream-old", Status: model.ModelStatusDraft, DiscoveryStatus: model.DiscoveryStatusPresent, ConfigRevision: 1, Capabilities: "[]", Enabled: false}
	if err := db.Create(&implementation).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewModelConfigRepo(db)
	params := SaveModelConfigParams{
		TenantID: 3, ActorUserID: 8, ModelID: implementation.ID, ExpectedRevision: 1,
		PublicKey: "public-image", DisplayName: "Public image", UpstreamModelID: "upstream-image", Status: model.ModelStatusActive,
		Capabilities: `["image"]`, ImageGenerate: "generations", ImageEdit: "edits", VideoRoute: "auto", VideoDurations: "[]",
		Operations: []model.ChannelModelOperation{{Capability: "image", Operation: "generate", Enabled: true, ProtocolMode: model.ProtocolModeOverride, Adapter: "generations", ConfigJSON: "{}", ConfigVersion: 1, ContractKey: "contract"}},
		Pricing:    []model.ModelPricingRule{{Capability: "image", CreditsPerUnit: 4, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit}},
		BeforeJSON: `{}`, AfterJSON: `{"status":"active"}`,
	}
	if err := repo.SaveModelConfig(params); err != nil {
		t.Fatalf("save model configuration: %v", err)
	}
	if err := db.First(&implementation, implementation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if implementation.ConfigRevision != 2 || implementation.ModelName != "upstream-image" || !implementation.Enabled {
		t.Fatalf("unexpected saved implementation: %#v", implementation)
	}
	var savedCatalog model.CatalogModel
	if err := db.Where("public_key = ?", "public-image").First(&savedCatalog).Error; err != nil {
		t.Fatal(err)
	}
	var operation model.ChannelModelOperation
	if err := db.Where("channel_model_id = ?", implementation.ID).First(&operation).Error; err != nil {
		t.Fatal(err)
	}
	if operation.Adapter != "generations" || operation.ContractKey != "contract" {
		t.Fatalf("unexpected operation: %#v", operation)
	}
	var pricing model.ModelPricingRule
	if err := db.Where("tenant_id = ? AND catalog_model_id = ? AND scope = ? AND scope_id = ?", 3, savedCatalog.ID, model.PricingScopeImplementation, implementation.ID).First(&pricing).Error; err != nil {
		t.Fatal(err)
	}
	var shadow model.CreditPricing
	if err := db.Where("tenant_id = ? AND channel_id = ? AND model = ?", 3, channel.ID, "upstream-image").First(&shadow).Error; err != nil {
		t.Fatal(err)
	}
	if shadow.CreditsPerUnit != 4 || shadow.UnitType != model.UnitPerImage {
		t.Fatalf("unexpected legacy pricing projection: %#v", shadow)
	}
	if err := repo.SaveModelConfig(params); !errors.Is(err, ErrModelConfigRevisionConflict) {
		t.Fatalf("stale revision error=%v, want ErrModelConfigRevisionConflict", err)
	}
}

func TestModelConfigRepoRollsBackWholeSaveOnAuditFailure(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	channel := model.Channel{Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: "encrypted", Enabled: true, ConfigRevision: 1}
	catalog := model.CatalogModel{PublicKey: "stable-public", DisplayName: "Stable"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatal(err)
	}
	implementation := model.ChannelModel{ChannelID: channel.ID, ModelName: "stable-upstream", CatalogModelID: catalog.ID, UpstreamModelID: "stable-upstream", Status: model.ModelStatusDraft, DiscoveryStatus: model.DiscoveryStatusPresent, ConfigRevision: 1, Capabilities: "[]"}
	if err := db.Create(&implementation).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.ModelConfigAuditLog{}); err != nil {
		t.Fatal(err)
	}
	err := NewModelConfigRepo(db).SaveModelConfig(SaveModelConfigParams{
		TenantID: 3, ActorUserID: 8, ModelID: implementation.ID, ExpectedRevision: 1,
		PublicKey: "rolled-back-public", DisplayName: "Rolled back", UpstreamModelID: "rolled-back-upstream", Status: model.ModelStatusActive,
		Capabilities: `["text"]`, ImageGenerate: "auto", ImageEdit: "auto", VideoRoute: "auto", VideoDurations: "[]",
		Operations: []model.ChannelModelOperation{{Capability: "text", Operation: "generate", Enabled: true, ProtocolMode: model.ProtocolModeOverride, Adapter: "openai", ConfigJSON: "{}", ConfigVersion: 1}},
		Pricing:    []model.ModelPricingRule{{Capability: "text", CreditsPerUnit: 2, UnitType: model.UnitPerToken, PricingMode: model.PricingModePerUnit}},
	})
	if err == nil {
		t.Fatal("save should fail when audit persistence fails")
	}
	if err := db.First(&implementation, implementation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if implementation.ConfigRevision != 1 || implementation.ModelName != "stable-upstream" || implementation.Status != model.ModelStatusDraft {
		t.Fatalf("model update escaped rollback: %#v", implementation)
	}
	var count int64
	if err := db.Model(&model.CatalogModel{}).Where("public_key = ?", "rolled-back-public").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("catalog rollback count=%d err=%v", count, err)
	}
	for _, value := range []any{&model.ChannelModelOperation{}, &model.ModelPricingRule{}, &model.CreditPricing{}} {
		if err := db.Model(value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%T rollback count=%d err=%v", value, count, err)
		}
	}
}

func TestModelConfigRepoDefaultPricingUpdatesNormalizedAndLegacyProjections(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	catalog := model.CatalogModel{PublicKey: "public-video", DisplayName: "Public video"}
	channel := model.Channel{Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: "encrypted", Enabled: true, ConfigRevision: 1}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	implementations := []model.ChannelModel{
		{ChannelID: channel.ID, ModelName: "video-v1", CatalogModelID: catalog.ID, UpstreamModelID: "video-v1", Status: model.ModelStatusActive, ConfigRevision: 1},
		{ChannelID: channel.ID, ModelName: "video-v2", CatalogModelID: catalog.ID, UpstreamModelID: "video-v2", Status: model.ModelStatusActive, ConfigRevision: 1},
	}
	if err := db.Create(&implementations).Error; err != nil {
		t.Fatal(err)
	}
	pricing := model.ModelPricingRule{CreditsPerUnit: 6, UnitType: model.UnitPerVideo, PricingMode: model.PricingModePerUnit}
	if err := NewModelConfigRepo(db).SaveDefaultPricing(4, 9, catalog.ID, "video", pricing); err != nil {
		t.Fatalf("save default pricing: %v", err)
	}
	var normalized model.ModelPricingRule
	if err := db.Where("tenant_id = ? AND catalog_model_id = ? AND capability = ? AND scope = ?", 4, catalog.ID, "video", model.PricingScopeDefault).First(&normalized).Error; err != nil {
		t.Fatal(err)
	}
	if normalized.CreditsPerUnit != 6 || normalized.ScopeID != 0 {
		t.Fatalf("unexpected normalized pricing: %#v", normalized)
	}
	var shadows []model.CreditPricing
	if err := db.Where("tenant_id = ? AND channel_id = 0", 4).Order("model ASC").Find(&shadows).Error; err != nil {
		t.Fatal(err)
	}
	if len(shadows) != 3 || shadows[0].Model != "public-video" || shadows[1].Model != "video-v1" || shadows[2].Model != "video-v2" {
		t.Fatalf("unexpected legacy pricing projections: %#v", shadows)
	}
}

func TestModelConfigRepoDiscoveryPreservesExistingConfigurationAndMarksMissing(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	channel := model.Channel{Name: "Primary", BaseUrl: "https://api.example.com", ApiKey: "encrypted", Enabled: true, ConfigRevision: 1}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	existing := []model.ChannelModel{
		{ChannelID: channel.ID, ModelName: "kept", UpstreamModelID: "kept", Status: model.ModelStatusActive, DiscoveryStatus: model.DiscoveryStatusPresent, ConfigRevision: 7, Capabilities: `["image"]`, Enabled: true, ImageEditRoute: "generations"},
		{ChannelID: channel.ID, ModelName: "missing", UpstreamModelID: "missing", Status: model.ModelStatusActive, DiscoveryStatus: model.DiscoveryStatusPresent, ConfigRevision: 4, Capabilities: `["video"]`, Enabled: true, VideoRoute: "xai"},
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	report, err := NewModelConfigRepo(db).ApplyDiscovery(2, 5, channel.ID, []string{"kept", "new-draft"}, time.Now())
	if err != nil {
		t.Fatalf("apply discovery: %v", err)
	}
	if report.Created != 1 || report.Unchanged != 1 || report.Missing != 1 {
		t.Fatalf("unexpected discovery report: %#v", report)
	}
	if err := db.First(&existing[0], existing[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if existing[0].ConfigRevision != 7 || existing[0].ImageEditRoute != "generations" || !existing[0].Enabled {
		t.Fatalf("existing configuration was reset: %#v", existing[0])
	}
	if err := db.First(&existing[1], existing[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if existing[1].DiscoveryStatus != model.DiscoveryStatusMissing || existing[1].VideoRoute != "xai" {
		t.Fatalf("missing model was deleted or reset: %#v", existing[1])
	}
	var created model.ChannelModel
	if err := db.Where("channel_id = ? AND model_name = ?", channel.ID, "new-draft").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.Status != model.ModelStatusDraft || created.Enabled {
		t.Fatalf("discovered model should require configuration: %#v", created)
	}
}

func TestAPIConfigTransferRepoRollsBackCreatedChannelWhenReferenceFails(t *testing.T) {
	db := openModelConfigRepoTestDB(t)
	plan := &APIConfigTransferApplyPlan{
		SchemaVersion: 2,
		Channels: []APIConfigTransferChannelOperation{{
			Ref:  "primary",
			Item: model.Channel{Name: "Imported", BaseUrl: "https://import.example.com", ApiKey: "encrypted", Enabled: true, ConfigRevision: 1},
		}},
		PricingRules: []APIConfigTransferPricingRuleOperation{{
			ChannelRef: "primary", UpstreamModelID: "missing-model", PublicKey: "public-image",
			Item: model.ModelPricingRule{TenantID: 2, Capability: "image", Scope: model.PricingScopeImplementation, CreditsPerUnit: 2, UnitType: model.UnitPerImage, PricingMode: model.PricingModePerUnit},
		}},
	}
	if err := NewAPIConfigTransferRepo(db).Apply(plan); err == nil {
		t.Fatal("invalid channel model reference should fail the import transaction")
	}
	var count int64
	for _, value := range []any{&model.Channel{}, &model.CatalogModel{}, &model.ModelPricingRule{}, &model.CreditPricing{}} {
		if err := db.Model(value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%T import rollback count=%d err=%v", value, count, err)
		}
	}
}

func openModelConfigRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is required for model configuration repository tests")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *config
	adminConfig.DBName = ""
	adminDB, err := gorm.Open(mysql.Open(adminConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("model_config_repo_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatalf("create test database: %v", err)
	}
	adminSQL, err := adminDB.DB()
	if err != nil {
		t.Fatalf("get admin database handle: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Exec("DROP DATABASE `" + databaseName + "`").Error
		_ = adminSQL.Close()
	})
	testConfig := *config
	testConfig.DBName = databaseName
	db, err := gorm.Open(mysql.Open(testConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Channel{}, &model.CatalogModel{}, &model.ChannelModel{}, &model.ChannelModelOperation{},
		&model.ChannelProtocolDefault{}, &model.ModelPricingRule{}, &model.CreditPricing{}, &model.ModelConfigAuditLog{},
	); err != nil {
		t.Fatalf("migrate model configuration repository tables: %v", err)
	}
	return db
}
