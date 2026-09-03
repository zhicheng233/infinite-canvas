package repository_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

func TestFeatureGuideRepoConcurrentUpdatesKeepVersions(t *testing.T) {
	db := openFeatureGuideRepoTestDB(t)
	initial := model.FeatureGuide{
		Surface: model.FeatureGuideSurfaceCanvas, Title: "画布", Pages: `["初始"]`, Version: 1,
	}
	if err := db.Create(&initial).Error; err != nil {
		t.Fatalf("seed feature guide: %v", err)
	}
	svc := service.NewFeatureGuideService(repository.NewFeatureGuideRepo(db))
	versions := make(chan int, 2)
	errorsBySave := make(chan error, 2)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for _, page := range []string{"并发更新一", "并发更新二"} {
		page := page
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			item, err := svc.Save(model.FeatureGuideSurfaceCanvas, model.FeatureGuidePayload{Title: "画布", Pages: []string{page}})
			if err == nil {
				versions <- item.Version
			}
			errorsBySave <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(versions)
	close(errorsBySave)
	for err := range errorsBySave {
		if err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}
	seen := map[int]bool{}
	for version := range versions {
		seen[version] = true
	}
	if len(seen) != 2 || !seen[2] || !seen[3] {
		t.Fatalf("versions=%v, want 2 and 3", seen)
	}
	var final model.FeatureGuide
	if err := db.Where("surface = ?", model.FeatureGuideSurfaceCanvas).First(&final).Error; err != nil {
		t.Fatalf("load final feature guide: %v", err)
	}
	var pages []string
	if err := json.Unmarshal([]byte(final.Pages), &pages); err != nil {
		t.Fatalf("decode final pages: %v", err)
	}
	if final.Version != 3 || len(pages) != 1 || (pages[0] != "并发更新一" && pages[0] != "并发更新二") {
		t.Fatalf("final=%#v pages=%#v", final, pages)
	}
}

func openFeatureGuideRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDIT_TEST_DSN is required for feature guide repository tests")
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
	databaseName := fmt.Sprintf("feature_guide_repo_test_%d_%d", os.Getpid(), time.Now().UnixNano())
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
	if err := db.AutoMigrate(&model.FeatureGuide{}); err != nil {
		t.Fatalf("migrate feature guide table: %v", err)
	}
	return db
}
