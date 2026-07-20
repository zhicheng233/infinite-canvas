# Webhook Notification — Learnings

## Task 1: Go Test Infrastructure

- **Date**: 2026-07-19
- **File**: `backend/model/webhook_test.go`
- **Pattern**: Minimal inline `WebhookConfig` struct (ID + Platform) with table-driven `*testing.T` tests.
- **No SQLite driver** in go.mod — only `gorm.io/driver/mysql`. Cannot run `db.AutoMigrate()` in tests without a DB driver.
  - **Resolution**: Validate struct definition via `reflect` (field kinds, GORM tags, TableName) instead of AutoMigrate.
- **No external test libs** (testify, etc.) — project convention is raw `*testing.T` with `t.Fatalf`.
- **Test results**: All 3 tests PASS (`TestWebhookConfigStructFields`, `TestWebhookConfigTableName`, `TestWebhookConfigFieldCount`).
- **go version**: 1.26.5 (compatible with go.mod `go 1.23`).
- **Recommendation for future tasks**: If we want `db.AutoMigrate` in tests, add `gorm.io/driver/sqlite` to go.mod.

## Task 2: Bun Test Infrastructure

- **Date**: 2026-07-20
- **File**: `web/src/services/api/webhook.test.ts`
- **Bun version**: 1.3.13 (globally installed)
- **Script added**: `"test": "bun test"` in `web/package.json`
- **Test file**: Minimal test with 2 passing tests:
  - `test framework works` — `expect(1+1).toBe(2)`
  - `apiClient is defined` — verifies apiClient import from `@/services/api/client`
- **Existing tests**: `web/src/stores/use-config-store.test.ts` (8 tests already exist)
- **Result**: All 10 tests PASS (8 existing + 2 new), 0 failures, 29 expect() calls, 144ms
- **apiClient import path**: `@/services/api/client` — default export is an axios instance with `baseURL` from `resolveApiBaseUrl()`

## Task 5+6: WebhookRepo

- **Date**: 2026-07-20
- **File**: `backend/repository/webhook_repo.go`
- **Pattern**: Follows `credit_repo.go` exactly — `struct{db *gorm.DB}`, `NewWebhookRepo(db)`, pointer receiver methods.
- **Config methods**:
  - `Save(cfg)` — upsert by `(tenant_id, platform)`: `First()` then `Updates()` / `Create()`, same pattern as `ApiConfigRepo.Save`.
  - `ListEnabled(tenantID)` — `WHERE tenant_id AND enabled = true`.
  - `GetByPlatform(tenantID, platform)` — `WHERE tenant_id AND platform` returns single record.
- **Log methods**:
  - `InsertLog(log)` — simple `Create()`.
  - `ListLogs(tenantID, limit)` — `ORDER BY id DESC LIMIT limit`.
  - `LastLogForModel(tenantID, modelName, status)` — `ORDER BY id DESC LIMIT 1` for most recent log entry.
- **Upsert detail**: Uses `r.db.Model(&existing).Updates(map[string]interface{}{...})` — same as `CreditRepo.SavePricing` pattern, not the field-by-field assignment + `Save()` that `ApiConfigRepo` uses. Both are valid; `Updates(map)` is more concise for partial fields.
- **Build verification**: `gofmt -w` applied, `go build ./repository/` passes with zero errors.
- **No interface abstraction**: Direct struct usage per repo convention. No new dependencies.

## Task 7: WebhookSender

- **Date**: 2026-07-20
- **File**: `backend/service/webhook_sender.go`
- **Interface**: `WebhookSender` with single method `Send(ctx context.Context, url string, message string) error`.
- **Package-level HTTP client**: `var webhookHTTPClient = &http.Client{Timeout: 10 * time.Second}` — `http.Client` is safe for concurrent use per Go docs.
- **4 platform senders**:
  - `FeishuSender` — POST `{"msg_type":"text","content":{"text":"<message>"}}`
  - `DingTalkSender` — POST `{"msgtype":"text","text":{"content":"<message>"}}`
  - `WecomSender` — POST `{"msgtype":"text","text":{"content":"<message>"}}` (same JSON shape as DingTalk)
  - `TelegramSender` — POST `{"chat_id":"<from url>","text":"<message>"}` with `chat_id` parsed from webhook URL query string via `url.Parse()`
- **Helper**: `postWebhook(ctx, url, body)` — marshal JSON → `http.NewRequestWithContext(ctx, POST, url, bytes.NewReader(body))` → set `Content-Type: application/json` → `webhookHTTPClient.Do(req)` → check 2xx status → read error body on failure (capped at 4KB).
- **Error handling**: Returns descriptive wrapped errors on marshal failure, request creation failure, network failure, or non-2xx status (includes status code and response body).
- **No SDKs, no panics**: Pure `net/http`, always returns errors.
- **Build verification**: `gofmt -w` applied, `go build ./service/` passes with zero errors.
- **Platform values** (from `model/webhook.go` / `WebhookConfig.Platform`): `"feishu"`, `"dtalk"`, `"wecom"`, `"telegram"`.

## Task 8: WebhookPoller

- **Date**: 2026-07-20
- **File**: `backend/service/webhook_poller.go`
- **Struct fields**:
  - `mu sync.Mutex`, `ctx context.Context`, `cancel context.CancelFunc`, `running bool` — lifecycle management.
  - `wg sync.WaitGroup` — added beyond spec for `Stop()` to wait for goroutine exit (standard Go pattern).
  - `interval time.Duration` — default 5 minutes (matches `WebhookConfig.IntervalSeconds` default 300).
  - `webhookRepo`, `channelRepo`, `channelModelRepo` — injected repos.
  - `db *gorm.DB` — added beyond spec; required for cross-tenant queries (`channel_models` `/` `webhook_configs` without tenantID). Repos don't expose `ListAllEnabled()` or `DistinctModelNames()`.
  - `sender WebhookSender` — per spec; not used directly (platform-specific senders created via `senderForPlatform()` at notification time).
  - `states map[string]string` — tracks last known model state ("up"/"down").
- **Constructor**: `NewWebhookPoller(webhookRepo, channelRepo, channelModelRepo, db, sender)` — adds `db` param for cross-tenant queries.
- **Lifecycle**:
  - `Start()` — creates `context.WithCancel(context.Background())`, sets `running=true`, `wg.Add(1)`, launches goroutine.
  - `Stop()` — calls `cancel()`, `wg.Wait()` blocks until goroutine exits (sets `running=false` in defer).
  - `IsRunning()` — mutex-guarded bool read.
  - Goroutine: `defer wg.Done()` + `defer running=false`, `time.NewTicker(p.interval)`, `select` on ticker.C and ctx.Done().
- **Availability check (`checkOnce()`)**:
  1. `db.Model(ChannelModel{}).Distinct("model_name").Pluck(...)` — all unique models.
  2. Build available set: models where `channel.enabled=true AND channel_model.enabled=true` (GORM subquery via IN + Distinct).
  3. Compare old/new state; notify only on change.
  4. No sync_status used — availability = Channel.Enabled && ChannelModel.Enabled only.
  5. Models not in channel_models → not in allModels → implicitly skipped (not counted as "down").
- **Cooldown**: `webhookRepo.LastLogForModel(tenantID, model, status)` → if last log created within `cfg.CooldownMinutes` → insert `CooldownSkipped=true` log, skip send. `gorm.ErrRecordNotFound` treated as no cooldown.
- **Template rendering**: `strings.NewReplacer("{{model}}", model, "{{status}}", status, "{{time}}", t.Format(time.RFC3339))` — replaces 3 placeholders.
- **Sender dispatch**: `senderForPlatform(platform)` switch on "feishu"/"dtalk"/"wecom"/"telegram" → creates platform sender; unknown platform logged and skipped. Each send gets a 10s timeout context.
- **Logging**: Every action (send attempt, cooldown skip) creates a `WebhookLog` entry via `webhookRepo.InsertLog`. All errors logged via `log.Printf`.
- **Build verification**: `gofmt -w` applied, `go build ./service/` passes with zero errors (alongside existing `webhook_sender.go`).
- **Notable**: Cross-tenant polls iterate all `webhook_configs WHERE enabled=true` (no tenant filter) because channels/models are global while webhook configs are per-tenant. Each config contributes its own cooldown window.

## Task 9: WebhookHandler

- **Date**: 2026-07-20
- **File**: `backend/handler/webhook_handler.go`
- **Pattern**: Follows `credit_handler.go` — `struct{services}`, `NewWebhookHandler(...)`, method receivers with `claims := c.MustGet("claims").(*service.Claims)` extraction.
- **Struct fields**: `webhookRepo *repository.WebhookRepo`, `poller *service.WebhookPoller`, `sender service.WebhookSender`.
- **7 endpoints** (all under `/backend-api/admin/webhook/` — routing by Task 10):
  1. `ListConfig(c)` — GET config → `webhookRepo.ListEnabled(tenantID)` → `model.OK(c, configs)`.
  2. `SaveConfig(c)` — PUT config → `c.ShouldBindJSON` → force `cfg.TenantID = claims.TenantID` → `webhookRepo.Save(&cfg)` → `model.OK(c, cfg)`.
  3. `TestSend(c)` — POST test → bind `{platform, message}` → `webhookRepo.GetByPlatform(tenantID, platform)` → `service.NewSender(platform)` factory → `sender.Send(ctx, cfg.WebhookURL, msg)` → `webhookRepo.InsertLog` → return `{success, error}`.
  4. `ListLogs(c)` — GET logs → `strconv.Atoi(c.DefaultQuery("limit", "50"))` → `webhookRepo.ListLogs(tenantID, limit)` → `model.OK(c, logs)`.
  5. `StartPoller(c)` — POST poller/start → extract claims (tenant-scoped) → `poller.Start()` → `{started: true}`.
  6. `StopPoller(c)` — POST poller/stop → extract claims → `poller.Stop()` → `{stopped: true}`.
  7. `PollerStatus(c)` — GET poller/status → extract claims → `poller.IsRunning()` + `poller.IntervalSeconds()` → `{running, interval_seconds}`.
- **TestSend log entry**: `WebhookLog` with `Status: "test"`, `ModelName: ""`, `Message` from input, `Success` from send error, `ResponseBody` contains error string on failure.
- **Service package changes** needed for this handler:
  - `webhook_sender.go`: Added exported `NewSender(platform string) WebhookSender` — factory for platform-specific sender (replaces unexported `senderForPlatform` in poller). Handler uses this for TestSend.
  - `webhook_poller.go`: Added `IntervalSeconds() int` method — returns `int(p.interval.Seconds())` for PollerStatus endpoint. Replaced local `senderForPlatform` with `NewSender` call to eliminate duplication.
- **Build verification**: `gofmt -w` applied to all 3 files. `go build ./handler/` + `go vet ./service/ ./handler/` both pass with zero errors.
- **No route registration**: Routes deferred to Task 10. Handler methods ready with correct signatures.

## Task 11: Frontend API Client (web/src/services/api/webhook.ts)

- **Date**: 2026-07-20
- **File**: `web/src/services/api/webhook.ts`
- **Pattern**: Follows `pricing.ts` exactly — `import apiClient from "./client"`, `res.data.data as Type` return pattern.
- **7 exported functions** matching the 7 handler endpoints:
  - `listWebhookConfigs()` → `GET /admin/webhook/config` → `WebhookConfig[]`
  - `saveWebhookConfig(input)` → `PUT /admin/webhook/config` → `WebhookConfig`
  - `testWebhookSend(input)` → `POST /admin/webhook/test` → `TestSendResult`
  - `listWebhookLogs(limit?)` → `GET /admin/webhook/logs?limit=N` → `WebhookLogItem[]`
  - `startPoller()` → `POST /admin/webhook/poller/start` → `{started: boolean}`
  - `stopPoller()` → `POST /admin/webhook/poller/stop` → `{stopped: boolean}`
  - `getPollerStatus()` → `GET /admin/webhook/poller/status` → `PollerStatus`
- **5 exported types**: `WebhookConfig`, `WebhookLogItem`, `PollerStatus`, `TestSendInput`, `TestSendResult`
- **Test file**: `web/src/services/api/webhook.test.ts` — uses `jest.spyOn()` to mock `apiClient.get/put/post`, tests all 7 functions. 8 tests, all pass.
- **Bun test quirk**: `jest.mock(modulePath)` doesn't work like Jest in Bun v1.3.13 — must use `jest.spyOn(obj, "method")` instead.
- **Result**: `bun test` — 16 pass (8 existing + 8 new), 0 fail, 98ms.

## Task 14: F4 Review Fixes — Webhook Tab (page.tsx)

- **Date**: 2026-07-20
- **File**: `web/src/app/(user)/admin/api-config/page.tsx`
- **Issue 1 (already fixed)**: `WEBHOOK_PLATFORMS` used `"dingtalk"` but backend expects `"dtalk"`.
  - **Resolution**: Confirmed `"dtalk"` on line 37 and `PLATFORM_LABELS["dtalk"]` on line 40. No instance of `"dingtalk"` remains in this file.
- **Issue 2 (new)**: Cooldown input + save missing from "轮询控制" Card.
  - **Changes**:
    1. Added `cooldownMinutes` state (default 10) and `savingCooldown` state (lines 167-168).
    2. `fetchWebhookConfigs` reads `cooldown_minutes` from the first feishu config to initialize `cooldownMinutes` (lines 245-248).
    3. Added UI row below the interval row: `<span>冷却时间(分钟):</span>` + `<InputNumber>` bound to `cooldownMinutes` + "保存冷却" `<Button>` (lines 1159-1179).
    4. Save handler iterates `WEBHOOK_PLATFORMS` and calls `saveWebhookConfig({ platform, cooldown_minutes: cooldownMinutes })` for each.
  - **Model field**: `WebhookConfig.cooldown_minutes?: number` already exists in `web/src/services/api/webhook.ts`.
  - **Follows existing pattern**: Cooldown save matches the interval save pattern (loading state, try/catch, message feedback), with per-platform iteration since `cooldown_minutes` is a per-config field.

## Task 13: Frontend API Client Comprehensive Tests

- **Date**: 2026-07-20
- **File**: `web/src/services/api/webhook.test.ts`
- **Test count**: 21 tests (8 original + 13 new), 37 expect() calls, 82ms.
- **Coverage per function**:
  - `listWebhookConfigs` — 3 tests: empty array, populated config items, API error rejection
  - `saveWebhookConfig` — 3 tests: full body, partial update, API error rejection
  - `getPollerStatus` — 3 tests: running=true, running=false, API error rejection
  - `listWebhookLogs` — 4 tests: with limit, without limit (undefined param), result shape verification, API error rejection
  - `testWebhookSend` — 3 tests: success result, fail result (success:false + error message), API error rejection
  - `startPoller` — 2 tests: started response, API error rejection
  - `stopPoller` — 2 tests: stopped response, API error rejection
- **Error path pattern**: Use `jest.spyOn(apiClient, "method").mockRejectedValue(new Error("..."))` — NOT `mockResolvedValue` with non-zero code envelope. The axios interceptor pipeline is bypassed when using `mockResolvedValue`, so the interceptor's rejection logic for non-zero `code` doesn't execute. Test error propagation with `mockRejectedValue` instead.
- **Happy path pattern**: `jest.spyOn(apiClient, "method").mockResolvedValue({ data: { data: mockValue } })` — the `mockResponse()` helper wraps data in the axios response envelope.
- **Result shape tests**: Verify both the function's return value matches the mock and that specific fields (e.g., `result.success`, `result[0].platform`) have expected types/values.
- **`listWebhookLogs` without limit**: Function passes `{ params: { limit: undefined } }` — axios strips undefined params from the URL, but the JS call argument still contains the object. Use `toHaveBeenCalledWith("/admin/webhook/logs", { params: { limit: undefined } })` to match.
- **Result**: All 21 tests PASS, 0 fail, 82ms.
