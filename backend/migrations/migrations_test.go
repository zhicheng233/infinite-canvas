package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

func TestUpMigratesLegacyCanvasIdentityAndGenerationJobs(t *testing.T) {
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is not configured")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *config
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	defer adminDB.Close()

	databaseName := fmt.Sprintf("migration_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	defer adminDB.Exec("DROP DATABASE `" + databaseName + "`")

	testConfig := *config
	testConfig.DBName = databaseName
	db, err := sql.Open("mysql", testConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE canvas_projects (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		tenant_id BIGINT UNSIGNED NOT NULL,
		user_id BIGINT UNSIGNED NOT NULL,
		project_id VARCHAR(64) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE INDEX idx_canvas_projects_project_id (project_id)
	)`); err != nil {
		t.Fatalf("create legacy canvas table: %v", err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}

	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND column_name IN ('schema_version', 'revision')", 2)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND index_name = 'idx_canvas_projects_project_id'", 0)
	assertSchemaCount(t, db, "SELECT COUNT(DISTINCT column_name) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND index_name = 'idx_canvas_project_identity'", 3)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'generation_jobs'", 1)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('auto_routing_pools', 'auto_routing_pool_members', 'generation_attempts')", 3)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'generation_jobs' AND column_name IN ('auto_routing_pool_id', 'authorized_amount', 'settlement_transaction_id')", 3)
}

func TestModelServiceMigrationsUpgradeLegacyTablesAndRollbackCompatibilityStep(t *testing.T) {
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is not configured")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *config
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	defer adminDB.Close()
	databaseName := fmt.Sprintf("model_service_migration_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	defer adminDB.Exec("DROP DATABASE `" + databaseName + "`")
	testConfig := *config
	testConfig.DBName = databaseName
	db, err := sql.Open("mysql", testConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer db.Close()

	legacySchema := []string{
		`CREATE TABLE channels (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
			name VARCHAR(100) NOT NULL, base_url VARCHAR(500) NOT NULL, api_key VARCHAR(500) NOT NULL, enabled BOOLEAN NOT NULL DEFAULT TRUE,
			video_api_standard VARCHAR(20) NOT NULL DEFAULT 'default', PRIMARY KEY (id), INDEX idx_channels_deleted_at (deleted_at)
		)`,
		`CREATE TABLE channel_models (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
			channel_id BIGINT UNSIGNED NOT NULL, model_name VARCHAR(191) NOT NULL, capabilities VARCHAR(100), enabled BOOLEAN NOT NULL DEFAULT TRUE,
			image_generate_route VARCHAR(30), image_edit_route VARCHAR(30), video_route VARCHAR(30), video_durations VARCHAR(200),
			video_customizable BOOLEAN NOT NULL DEFAULT FALSE, video_custom_config TEXT, sort_order BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (id), UNIQUE INDEX idx_channel_model (channel_id, model_name)
		)`,
		`CREATE TABLE credit_pricing (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
			channel_id BIGINT UNSIGNED NOT NULL DEFAULT 0, tenant_id BIGINT UNSIGNED NOT NULL, model VARCHAR(100) NOT NULL,
			credits_per_unit BIGINT NOT NULL, unit_type VARCHAR(20) NOT NULL, pricing_mode VARCHAR(30) NOT NULL, pricing_rule LONGTEXT,
			PRIMARY KEY (id), UNIQUE INDEX idx_tenant_model_channel (tenant_id, model, channel_id)
		)`,
		`CREATE TABLE tenant_api_configs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, deleted_at DATETIME(3) NULL,
			tenant_id BIGINT UNSIGNED NOT NULL, base_url VARCHAR(500) NOT NULL, api_key VARCHAR(500) NOT NULL,
			models LONGTEXT, image_models LONGTEXT, video_models LONGTEXT, text_models LONGTEXT, audio_models LONGTEXT,
			model_routes LONGTEXT, model_video_durations LONGTEXT, model_video_customizable LONGTEXT,
			PRIMARY KEY (id), UNIQUE INDEX idx_tenant_api_configs_tenant_id (tenant_id)
		)`,
	}
	for _, statement := range legacySchema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO channels (id, name, base_url, api_key, enabled, video_api_standard) VALUES (3, 'Legacy', 'https://legacy.example.com', 'ciphertext', TRUE, 'default')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO channel_models (id, channel_id, model_name, capabilities, enabled, image_generate_route, image_edit_route, video_route, video_durations) VALUES (5, 3, 'legacy-image', '["image"]', TRUE, 'generations', 'edits', 'auto', '[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credit_pricing (channel_id, tenant_id, model, credits_per_unit, unit_type, pricing_mode, pricing_rule) VALUES (3, 7, 'legacy-image', 2, 'per_image', 'per_unit', '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tenant_api_configs (id, tenant_id, base_url, api_key, models, image_models, video_models, text_models, audio_models, model_routes, model_video_durations, model_video_customizable) VALUES (9, 7, 'https://legacy.example.com', 'ciphertext', '["legacy-image"]', '["legacy-image"]', '[]', '[]', '[]', '{}', '{}', '{}')`); err != nil {
		t.Fatal(err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("upgrade legacy database: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("repeat model service migrations: %v", err)
	}
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM catalog_models WHERE public_key = 'legacy-image'", 1)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM channel_model_operations WHERE channel_model_id = 5 AND capability = 'image'", 2)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM model_pricing_rules WHERE tenant_id = 7 AND capability = 'image' AND scope = 'implementation' AND scope_id = 5", 1)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM model_config_migrations WHERE source = 'tenant_api_config' AND source_id = 9 AND version = 2 AND status = 'pending'", 1)

	goose.SetBaseFS(files)
	if err := goose.SetDialect("mysql"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(db, ".", 3); err != nil {
		t.Fatalf("rollback compatibility migration: %v", err)
	}
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM model_config_migrations WHERE source = 'tenant_api_config' AND source_id = 9 AND version = 2", 0)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM tenant_api_configs WHERE id = 9", 1)
	if err := Up(db); err != nil {
		t.Fatalf("reapply compatibility migration: %v", err)
	}
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM model_config_migrations WHERE source = 'tenant_api_config' AND source_id = 9 AND version = 2", 1)
}

func TestFeatureGuideMigrationPreservesSiteAnnouncements(t *testing.T) {
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is not configured")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CREDIT_TEST_DSN: %v", err)
	}
	adminConfig := *config
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	defer adminDB.Close()
	databaseName := fmt.Sprintf("feature_guide_migration_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	defer adminDB.Exec("DROP DATABASE `" + databaseName + "`")
	testConfig := *config
	testConfig.DBName = databaseName
	db, err := sql.Open("mysql", testConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE site_announcements (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, content TEXT NOT NULL, PRIMARY KEY (id))`); err != nil {
		t.Fatalf("create existing announcement table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO site_announcements (content) VALUES ('保留公告')`); err != nil {
		t.Fatalf("seed existing announcement: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE feature_guides (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		created_at DATETIME(3) NULL,
		updated_at DATETIME(3) NULL,
		deleted_at DATETIME(3) NULL,
		surface VARCHAR(20) NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		title VARCHAR(100) NOT NULL DEFAULT '',
		pages LONGTEXT NOT NULL,
		version BIGINT NOT NULL DEFAULT 1,
		PRIMARY KEY (id),
		UNIQUE INDEX idx_feature_guides_surface (surface),
		INDEX idx_feature_guides_deleted_at (deleted_at),
		CONSTRAINT chk_feature_guides_surface CHECK (surface IN ('canvas', 'image', 'video'))
	)`); err != nil {
		t.Fatalf("create interrupted feature guide migration table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO feature_guides (surface, enabled, title, pages, version) VALUES ('canvas', FALSE, '画布功能引导', '[]', 1)`); err != nil {
		t.Fatalf("seed interrupted feature guide migration: %v", err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("upgrade existing database: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'feature_guides'", 1)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'feature_guides' AND column_name IN ('surface', 'enabled', 'title', 'pages', 'version')", 5)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'feature_guides' AND index_name = 'idx_feature_guides_surface' AND non_unique = 0", 1)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM feature_guides WHERE enabled = FALSE AND pages = '[]' AND version = 1 AND ((surface = 'canvas' AND title = '画布功能引导') OR (surface = 'image' AND title = '图片生成功能引导') OR (surface = 'video' AND title = '视频生成功能引导'))", 3)
	assertSchemaCount(t, db, "SELECT COUNT(*) FROM site_announcements WHERE content = '保留公告'", 1)
}

func assertSchemaCount(t *testing.T, db *sql.DB, query string, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if count != expected {
		t.Fatalf("schema count = %d, want %d", count, expected)
	}
}
