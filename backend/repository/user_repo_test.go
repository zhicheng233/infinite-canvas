package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type userListSQLLogger struct {
	sql []string
}

func (sqlLogger *userListSQLLogger) LogMode(logger.LogLevel) logger.Interface { return sqlLogger }
func (*userListSQLLogger) Info(context.Context, string, ...interface{})       {}
func (*userListSQLLogger) Warn(context.Context, string, ...interface{})       {}
func (*userListSQLLogger) Error(context.Context, string, ...interface{})      {}
func (sqlLogger *userListSQLLogger) Trace(_ context.Context, _ time.Time, trace func() (string, int64), _ error) {
	sql, _ := trace()
	sqlLogger.sql = append(sqlLogger.sql, sql)
}

func TestUserRepoList_buildsTenantScopedKeywordQueries(t *testing.T) {
	tests := []struct {
		name       string
		query      UserListQuery
		wantSQL    []string
		withoutSQL []string
	}{
		{
			name:       "blank keyword keeps stable tenant pagination",
			query:      UserListQuery{TenantID: 17, Page: 2, PageSize: 3, Keyword: "   "},
			wantSQL:    []string{"tenant_id = 17", "ORDER BY id DESC", "LIMIT 3 OFFSET 3"},
			withoutSQL: []string{"username LIKE", "AND id ="},
		},
		{
			name:       "all digit keyword searches exact id",
			query:      UserListQuery{TenantID: 17, Page: 1, PageSize: 20, Keyword: "42"},
			wantSQL:    []string{"tenant_id = 17", "AND id = 42"},
			withoutSQL: []string{"username LIKE"},
		},
		{
			name:       "other keyword searches partial username",
			query:      UserListQuery{TenantID: 17, Page: 1, PageSize: 20, Keyword: "  ali  "},
			wantSQL:    []string{"tenant_id = 17", "username LIKE '%ali%'"},
			withoutSQL: []string{"AND id ="},
		},
		{
			name:       "username wildcards use an explicit escape marker",
			query:      UserListQuery{TenantID: 17, Page: 1, PageSize: 20, Keyword: "a!b_%"},
			wantSQL:    []string{"tenant_id = 17", "username LIKE '%a!!b!_!%%' ESCAPE '!'"},
			withoutSQL: []string{`\\_`, `\\%`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sqlLogger := &userListSQLLogger{}
			db, err := gorm.Open(gormmysql.New(gormmysql.Config{
				DSN:                       "unused:unused@tcp(localhost:3306)/unused?parseTime=true",
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DryRun:                 true,
				DisableAutomaticPing:   true,
				SkipDefaultTransaction: true,
				Logger:                 sqlLogger,
			})
			if err != nil {
				t.Fatalf("open dry-run database: %v", err)
			}

			_, _, err = NewUserRepo(db).List(test.query)
			if err != nil {
				t.Fatalf("list users: %v", err)
			}

			joined := strings.Join(sqlLogger.sql, "\n")
			for _, fragment := range test.wantSQL {
				if !strings.Contains(joined, fragment) {
					t.Fatalf("SQL missing %q:\n%s", fragment, joined)
				}
			}
			for _, fragment := range test.withoutSQL {
				if strings.Contains(joined, fragment) {
					t.Fatalf("SQL unexpectedly contains %q:\n%s", fragment, joined)
				}
			}
			if strings.Count(joined, "tenant_id = 17") != 2 {
				t.Fatalf("count and page queries must both be tenant scoped:\n%s", joined)
			}
		})
	}
}

func TestUserListQueryNormalize_appliesPagingDefaultsAndTrimsKeyword(t *testing.T) {
	query := (UserListQuery{TenantID: 17, Page: 0, PageSize: 0, Keyword: "  alice  "}).Normalize()

	if query.Page != 1 || query.PageSize != 20 || query.Keyword != "alice" {
		t.Fatalf("unexpected normalized query: %+v", query)
	}
}
