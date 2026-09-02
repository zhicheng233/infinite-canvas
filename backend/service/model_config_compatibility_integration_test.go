package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"infinite-canvas-server/crypto"
	"infinite-canvas-server/model"
)

func TestLegacyAPIConfigMigrationProjectsConfigurationAndIsIdempotent(t *testing.T) {
	db := openModelConfigServiceTestDB(t)
	encryptedKey, err := crypto.Encrypt("application-key", "sk-legacy")
	if err != nil {
		t.Fatal(err)
	}
	legacy := model.TenantApiConfig{
		TenantID: 17, BaseUrl: "https://legacy.example.com/", ApiKey: encryptedKey,
		Models: `["gpt-image-2","video-one"]`, ImageModels: `["gpt-image-2"]`, VideoModels: `["video-one"]`, TextModels: `[]`, AudioModels: `[]`,
		ModelRoutes:         `{"image_generate:gpt-image-2":"chat","image_edit:gpt-image-2":"generations","video:video-one":"xai"}`,
		ModelVideoDurations: `{"video-one":[10,5,10]}`, ModelVideoCustomizable: `{"video-one":true}`,
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy config: %v", err)
	}
	service := NewLegacyAPIConfigService(db)
	if err := service.MigrateAll(); err != nil {
		t.Fatalf("migrate legacy config: %v", err)
	}

	var channel model.Channel
	if err := db.Where("name = ?", "旧版 API（租户 17）").First(&channel).Error; err != nil {
		t.Fatalf("load compatibility channel: %v", err)
	}
	if channel.ApiKey != encryptedKey || channel.BaseUrl != "https://legacy.example.com" || !channel.Enabled {
		t.Fatalf("unexpected compatibility channel: %#v", channel)
	}
	var imageModel model.ChannelModel
	if err := db.Where("channel_id = ? AND model_name = ?", channel.ID, "gpt-image-2").First(&imageModel).Error; err != nil {
		t.Fatalf("load image model: %v", err)
	}
	if imageModel.ImageGenerateRoute != "chat" || imageModel.ImageEditRoute != "generations" || !imageModel.LegacyUnreviewed {
		t.Fatalf("unexpected image projection: %#v", imageModel)
	}
	var videoModel model.ChannelModel
	if err := db.Where("channel_id = ? AND model_name = ?", channel.ID, "video-one").First(&videoModel).Error; err != nil {
		t.Fatalf("load video model: %v", err)
	}
	if videoModel.VideoRoute != "xai" || videoModel.VideoDurations != "[5,10]" || !videoModel.VideoCustomizable {
		t.Fatalf("unexpected video projection: %#v", videoModel)
	}
	assertModelConfigCount(t, db, &model.CatalogModel{}, 2)
	assertModelConfigCount(t, db, &model.ChannelProtocolDefault{}, 5)
	assertModelConfigCount(t, db, &model.ChannelModelOperation{}, 3)
	var migration model.ModelConfigMigration
	if err := db.Where("source = ? AND source_id = ? AND version = ?", legacyAPIConfigMigrationSource, legacy.ID, legacyAPIConfigMigrationVersion).First(&migration).Error; err != nil {
		t.Fatalf("load migration record: %v", err)
	}
	if migration.Status != "completed" || migration.TargetID != channel.ID {
		t.Fatalf("unexpected migration: %#v", migration)
	}

	channelRevision, imageRevision := channel.ConfigRevision, imageModel.ConfigRevision
	if err := service.MigrateAll(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if err := db.First(&channel, channel.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&imageModel, imageModel.ID).Error; err != nil {
		t.Fatal(err)
	}
	if channel.ConfigRevision != channelRevision || imageModel.ConfigRevision != imageRevision {
		t.Fatalf("repeat migration changed revisions: channel %d -> %d, model %d -> %d", channelRevision, channel.ConfigRevision, imageRevision, imageModel.ConfigRevision)
	}
	assertModelConfigCount(t, db, &model.Channel{}, 1)
	assertModelConfigCount(t, db, &model.ChannelModel{}, 2)
}

func TestLegacyAPIConfigMigrationRecordsInvalidJSONIssue(t *testing.T) {
	db := openModelConfigServiceTestDB(t)
	legacy := model.TenantApiConfig{TenantID: 18, BaseUrl: "https://legacy.example.com", ApiKey: "encrypted-key", Models: `[`, ImageModels: `[]`, VideoModels: `[]`, TextModels: `[]`, AudioModels: `[]`}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewLegacyAPIConfigService(db).MigrateAll(); err != nil {
		t.Fatalf("invalid record should be isolated: %v", err)
	}
	var migration model.ModelConfigMigration
	if err := db.Where("source = ? AND source_id = ?", legacyAPIConfigMigrationSource, legacy.ID).First(&migration).Error; err != nil {
		t.Fatalf("load failed migration: %v", err)
	}
	if migration.Status != "failed" {
		t.Fatalf("migration status=%q, want failed", migration.Status)
	}
	var issue model.ModelConfigMigrationIssue
	if err := db.Where("migration_id = ?", migration.ID).First(&issue).Error; err != nil {
		t.Fatalf("load migration issue: %v", err)
	}
	if issue.Resource != "tenant_api_config" || issue.Reason == "" {
		t.Fatalf("unexpected migration issue: %#v", issue)
	}
	if err := NewLegacyAPIConfigService(db).MigrateAll(); err != nil {
		t.Fatalf("repeat invalid migration: %v", err)
	}
	assertModelConfigCount(t, db, &model.ModelConfigMigrationIssue{}, 1)
	assertModelConfigCount(t, db, &model.Channel{}, 0)
}

func TestLegacyAPIConfigDualWriteRollsBackWhenProjectionFails(t *testing.T) {
	db := openModelConfigServiceTestDB(t)
	if err := db.Migrator().DropTable(&model.ModelConfigAuditLog{}); err != nil {
		t.Fatalf("remove audit table: %v", err)
	}
	legacy := model.TenantApiConfig{
		TenantID: 19, BaseUrl: "https://legacy.example.com", ApiKey: "encrypted-key",
		Models: `["text-model"]`, ImageModels: `[]`, VideoModels: `[]`, TextModels: `["text-model"]`, AudioModels: `[]`,
	}
	if err := NewLegacyAPIConfigService(db).Save(&legacy, 9); err == nil {
		t.Fatal("dual write should fail when the audit record cannot be persisted")
	}
	assertModelConfigCount(t, db, &model.TenantApiConfig{}, 0)
	assertModelConfigCount(t, db, &model.Channel{}, 0)
	assertModelConfigCount(t, db, &model.CatalogModel{}, 0)
}

func openModelConfigServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is required for model configuration integration tests")
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
	databaseName := fmt.Sprintf("model_config_service_test_%d_%d", os.Getpid(), time.Now().UnixNano())
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
		&model.TenantApiConfig{}, &model.Channel{}, &model.CatalogModel{}, &model.ChannelModel{},
		&model.ChannelModelOperation{}, &model.ChannelProtocolDefault{}, &model.ModelPricingRule{},
		&model.ModelConfigAuditLog{}, &model.ModelConfigMigration{}, &model.ModelConfigMigrationIssue{}, &model.CreditPricing{},
	); err != nil {
		t.Fatalf("migrate model configuration test tables: %v", err)
	}
	return db
}

func assertModelConfigCount(t *testing.T, db *gorm.DB, value any, expected int64) {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(value).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", value, err)
	}
	if count != expected {
		t.Fatalf("%T count=%d, want %d", value, count, expected)
	}
}
