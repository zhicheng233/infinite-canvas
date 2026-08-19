package model

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUserUsernameGlobalUniqueIndexRejectsDuplicateAcrossTenantsOnMySQL(t *testing.T) {
	dsn := os.Getenv("AUTH_POLICY_TEST_DSN")
	if dsn == "" {
		t.Skip("AUTH_POLICY_TEST_DSN is required for auth policy MySQL tests")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open auth policy test database: %v", err)
	}
	tableName := "auth_model_users_" + time.Now().Format("20060102150405.000000000")
	tableName = strings.ReplaceAll(tableName, ".", "")
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(tableName); err != nil {
			t.Errorf("drop auth model user table: %v", err)
		}
	})
	tableDB := db.Table(tableName)
	if err := tableDB.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate auth model user table: %v", err)
	}
	if err := tableDB.Create(&User{TenantID: 11, Username: "global_name", PasswordHash: "first-hash"}).Error; err != nil {
		t.Fatalf("insert first tenant user: %v", err)
	}

	err = tableDB.Create(&User{TenantID: 22, Username: "global_name", PasswordHash: "second-hash"}).Error

	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		t.Fatalf("second tenant insert error = %v, want MySQL duplicate key 1062", err)
	}
}
