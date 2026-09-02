package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
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
