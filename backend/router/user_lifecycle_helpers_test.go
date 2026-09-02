package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"infinite-canvas-server/config"
	"infinite-canvas-server/handler"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type lifecycleFixture struct {
	db          *gorm.DB
	router      http.Handler
	actor       *model.User
	target      *model.User
	foreign     *model.User
	suffix      string
	actorToken  string
	targetToken string
}

type lifecycleAPIResponse struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

type lifecycleUserSeed struct {
	tenantID uint
	username string
	role     model.UserRole
	password string
}

type lifecycleRequest struct {
	method string
	path   string
	token  string
	body   map[string]string
}

type lifecycleLogin struct {
	username string
	password string
	wantCode int
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	t.Helper()
	dsn := os.Getenv("ACCOUNT_LIFECYCLE_TEST_DSN")
	if dsn == "" {
		t.Skip("ACCOUNT_LIFECYCLE_TEST_DSN is required for account lifecycle tests")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open lifecycle test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.CreditAccount{},
		&model.CreditTransaction{},
		&model.CanvasProject{},
		&model.GenerationRecord{},
		&model.GenerationJob{},
		&model.GenerationAttempt{},
		&model.RechargeOrder{},
		&model.ModelCallLog{},
	); err != nil {
		t.Fatalf("migrate lifecycle tables: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin lifecycle test transaction: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	tenantID := uint(time.Now().UnixNano()%500_000_000 + 1_500_000_000)
	fixture := &lifecycleFixture{db: tx, suffix: suffix}
	actor := fixture.createUser(t, lifecycleUserSeed{tenantID: tenantID, username: "task4actor_" + suffix, role: model.RoleTenantAdmin, password: "ActorPass1"})
	target := fixture.createUser(t, lifecycleUserSeed{tenantID: tenantID, username: "task4target_" + suffix, role: model.RoleTenantAdmin, password: "TargetPass1"})
	foreign := fixture.createUser(t, lifecycleUserSeed{tenantID: tenantID + 1, username: "task4foreign_" + suffix, role: model.RoleUser, password: "ForeignPass1"})

	cfg := &config.Config{JWTKey: "task-4-lifecycle-test-key"}
	userRepo := repository.NewUserRepo(tx)
	authService := service.NewAuthService(cfg, userRepo, nil, repository.NewCreditRepo(tx), nil)
	userService := service.NewUserService(userRepo)
	authHandler := handler.NewAuthHandler(authService, userService)
	userHandler := handler.NewUserHandler(userService)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Setup(engine, Dependencies{AuthService: authService, AuthHandler: authHandler, UserHandler: userHandler})

	fixture.router = engine
	fixture.actor = actor
	fixture.target = target
	fixture.foreign = foreign
	fixture.actorToken = fixture.login(t, lifecycleLogin{username: actor.Username, password: "ActorPass1"})
	fixture.targetToken = fixture.login(t, lifecycleLogin{username: target.Username, password: "TargetPass1"})
	return fixture
}

func (fixture *lifecycleFixture) createUser(t *testing.T, seed lifecycleUserSeed) *model.User {
	t.Helper()
	hash, err := service.HashPassword(seed.password)
	if err != nil {
		t.Fatalf("hash lifecycle password: %v", err)
	}
	user := &model.User{TenantID: seed.tenantID, Username: seed.username, PasswordHash: hash, DisplayName: seed.username, Role: seed.role, Status: model.UserActive}
	if err := fixture.db.Create(user).Error; err != nil {
		t.Fatalf("seed lifecycle user %q: %v", seed.username, err)
	}
	return user
}

func (fixture *lifecycleFixture) request(t *testing.T, input lifecycleRequest) (*httptest.ResponseRecorder, lifecycleAPIResponse) {
	t.Helper()
	var payload bytes.Buffer
	if input.body != nil {
		if err := json.NewEncoder(&payload).Encode(input.body); err != nil {
			t.Fatalf("encode %s %s body: %v", input.method, input.path, err)
		}
	}
	request := httptest.NewRequest(input.method, input.path, &payload)
	request.Header.Set("Content-Type", "application/json")
	if input.token != "" {
		request.Header.Set("Authorization", "Bearer "+input.token)
	}
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, request)
	var response lifecycleAPIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode %s %s response %q: %v", input.method, input.path, recorder.Body.String(), err)
	}
	return recorder, response
}

func (fixture *lifecycleFixture) login(t *testing.T, input lifecycleLogin) string {
	t.Helper()
	_, response := fixture.request(t, lifecycleRequest{method: http.MethodPost, path: "/backend-api/auth/login", body: map[string]string{"username": input.username, "password": input.password}})
	if response.Code != input.wantCode {
		t.Fatalf("login %q code=%d msg=%q, want %d", input.username, response.Code, response.Msg, input.wantCode)
	}
	if input.wantCode != 0 {
		return ""
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("decode login token: %v", err)
	}
	return result.Token
}
