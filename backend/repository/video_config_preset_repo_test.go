package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type presetSQLLogger struct {
	sql []string
}

func (sqlLogger *presetSQLLogger) LogMode(logger.LogLevel) logger.Interface { return sqlLogger }
func (*presetSQLLogger) Info(context.Context, string, ...interface{})       {}
func (*presetSQLLogger) Warn(context.Context, string, ...interface{})       {}
func (*presetSQLLogger) Error(context.Context, string, ...interface{})      {}
func (sqlLogger *presetSQLLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	sqlLogger.sql = append(sqlLogger.sql, sql)
}

func TestVideoConfigPresetRepoUsesTenantQualifiedStableQueries(t *testing.T) {
	sqlLogger := &presetSQLLogger{}
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{DSN: "unused:unused@tcp(localhost:3306)/unused?parseTime=true", SkipInitializeWithVersion: true}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 sqlLogger,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	repo := NewVideoConfigPresetRepo(db)
	if _, err := repo.ListByTenant(17); err != nil {
		t.Fatalf("list dry run failed: %v", err)
	}
	if err := repo.DeleteByTenantAndID(17, 23); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("delete dry run error=%v, want not found", err)
	}
	joined := strings.Join(sqlLogger.sql, "\n")
	if !strings.Contains(joined, "WHERE tenant_id = 17") || !strings.Contains(joined, "ORDER BY normalized_name ASC, id ASC") {
		t.Fatalf("list SQL is not tenant-scoped and stable:\n%s", joined)
	}
	if !strings.Contains(joined, "tenant_id = 17 AND id = 23") {
		t.Fatalf("delete SQL is not tenant-scoped:\n%s", joined)
	}
}

func TestNormalizeVideoConfigPresetCreateErrorMapsDuplicateKey(t *testing.T) {
	err := normalizeVideoConfigPresetCreateError(&mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry"})
	if !errors.Is(err, ErrVideoConfigPresetNameConflict) {
		t.Fatalf("error=%v, want preset name conflict", err)
	}
	original := errors.New("write failed")
	if got := normalizeVideoConfigPresetCreateError(original); !errors.Is(got, original) {
		t.Fatalf("error=%v, want original error", got)
	}
}
