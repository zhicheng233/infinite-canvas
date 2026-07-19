## Task 9 repair

- Empty `ChannelModel.Capabilities` is treated as the documented auto capability state during server-side route validation, so synced rows with blank capability metadata remain usable while explicit incompatible capability lists are still rejected.
- `TestModel` no longer loads `TenantApiConfig`; selected `Channel`/`ChannelModel` provide the upstream URL, API key, route metadata, and request shape configuration.

## Task 10 pricing integration

- Pricing remains tenant-scoped and raw-model keyed in `CreditRepo`; ChannelModel identity is validated independently before pricing lookup in generation/proxy/model-test/estimate paths.
- Authenticated channel-model catalog reads now return only enabled ChannelModel rows with a valid raw-model price, while SuperAdmin model management still sees all rows.
- Model-test requests are blocked on missing pricing after building the route-specific test payload but before any upstream request, preserving dynamic video pricing validation and avoiding free test calls.

## Task 12 frontend API clients

**Files Created**:

- web/src/services/api/channel.ts — Authenticated user reads for global enabled channels and their models (getChannels, getChannelModels, getChannelWithModels).
- web/src/services/api/channels-admin.ts — SuperAdmin channel CRUD/disable endpoints (createChannel, listAllChannels, getChannelAdmin, updateChannel, disableChannel, deleteChannel). API key is write-only; responses and client state never persist the raw key.
- web/src/services/api/channel-models-admin.ts — SuperAdmin model sync, enable/disable, and metadata update (syncChannelModels, listChannelModelsAdmin, updateChannelModel, disableChannelModel, enableChannelModel).
- web/src/services/api/metrics-config-admin.ts — SuperAdmin metrics configuration (getMetricsConfig, updateMetricsConfig). Base URL is hidden from non-admin clients.
- web/src/services/api/metrics.ts — Authenticated metrics reads with hours parameter and success_rate null/numeric distinction (getMetrics, getChannelMetrics, getChannelModelMetrics). Preserves success_rate: null for unavailable vs. numeric 0 for actual zero percent.

**Implementation Patterns**:

- All clients use existing piClient from client.ts and inherit JWT authentication via interceptor.
- All responses unwrap business errors (code !== 0) through the apiClient interceptor, matching envelope contract.
- Typed DTOs exclude raw API keys and metrics base URLs from authenticated read responses.
- Admin mutations accept api_key and newapi_channel_id as write-only inputs; responses redact these secrets.
- Metrics client accepts hours parameter as query string and validates on server; client sends exact hours value.
- Model metrics distinguish success_rate: null (unavailable from new-api) from numeric 0 (actual zero percent success).
- All endpoints follow existing /backend-api prefix convention established in client.ts.
- No new dependencies added; clients follow pricing.ts and admin.ts patterns exclusively.

## Task 12 fixes (manual review corrections)

- Updated ChannelInfo/ChannelAdminInfo DTOs to match backend/model/channel_dto.go exactly: id, name, enabled, new_api_channel_id (number|null), sync_status, sync_error?, synced_at?; admin adds base_url, has_key.
- Removed invented fields: base_url/created_at/updated_at from ChannelInfo; last_sync_at/last_sync_status/last_sync_error from ChannelAdminInfo; capabilities changed from string[] to required.
- Fixed response unwrapping: channel.ts unwraps {channels: ...}, channels-admin.ts unwraps {channels: ...}, channel-models-admin.ts unwraps {models: ...}.
- Removed non-existent endpoints: getChannelAdmin, deleteChannel.
- Updated UpdateChannelModelInput to match backend exactly: image_generate_route, image_edit_route, video_route, video_durations, video_customizable, sort_order; removed capabilities field.
- Fixed sync response type: syncChannelModels returns {synced: boolean}, not model list; caller must fetch models separately via listChannelModelsAdmin.
- Updated MetricsConfig to use metrics_base_url (not base_url); POST /admin/metrics-config for save.
- Fixed metrics response: removed per-channel/per-model endpoints; kept only getMetrics. Nested model items now include channel_model_id, channel_id, model_name, status; success_rate: number|null distinction preserved.
- Verified SaveChannelInput/UpdateChannelInput match backend SaveChannelInput naming conventions.
- No TypeScript/build verification possible in partial checkout; all type DTOs align with backend Go structs from channel_dto.go.

## Task 15 independent capability selection

- Shared user catalog loading now uses the typed authenticated `getChannels()` and `getChannelModels(channelId)` clients. Raw API keys remain outside Zustand and are not requested by user selectors.
- The persisted config keeps only `imageChannelId`, `videoChannelId`, `textChannelId`, and `audioChannelId`; server catalogs, model lists, route metadata, and the transient ChannelModel identity are excluded from the persisted slice.
- Capability model values use `channel_id::channel_model_id::raw_model`. This preserves same-name models across channels while `modelOptionName()` still produces the raw upstream model for existing provider request bodies.
- Catalog refresh filters disabled channels/models and independently falls back each capability to the first enabled channel with a matching model. If no valid identity exists, logged-in readiness and request resolution fail closed.
- Existing image/video/audio proxy URL builders continue to call the store helper; the helper now supplies both `channel_id` and `channel_model_id` only for a validated selected model. Provider branches and generation settings were not changed.

## Task 13 SuperAdmin Channel & Model Management UI

**Implementation Summary**:

- Replaced the previous single-tenant API-config admin page in `web/src/app/(user)/admin/api-config/page.tsx` with a global Channel and ChannelModel management interface using real backend API clients (`channels-admin.ts`, `channel-models-admin.ts`, and `metrics-config-admin.ts`).
- **Channel CRUD & Write-Only Secrets**:
  - SuperAdmin can view all global channels (ID, name, base URL, has_key status tag, optional New-API ID, sync status, enabled toggle, and actions).
  - Modal allows creating new channels and editing existing ones.
  - API Key input is write-only: never displayed or populated from server response or local storage; edit modal displays placeholder "leave blank to keep existing key".
  - Channels can be disabled/enabled via switch (invoking `updateChannel` / `disableChannel`).
- **Channel Model Management & Sync States**:
  - Model management is accessible via a Drawer for each selected channel.
  - Displays all channel models with name, capability tags (`image`, `video`, `text`, `audio`), sort order, routes, and video customization metadata.
  - Per-channel models can be enabled/disabled independently via switch (`enableChannelModel` / `disableChannelModel`).
  - Metadata modal allows editing `sort_order`, `image_generate_route`, `image_edit_route`, `video_route`, `video_durations` (comma-separated), and `video_customizable`.
  - Supports model synchronization (`syncChannelModels`). In the event of a sync error/failure (`sync_status === 'failed'`), existing models remain listed and visible in the UI; error message is rendered with a popover/alert and an immediate retry button.
- **Metrics Service Configuration**:
  - Added a second tab for "接口性能指标服务" (Metrics Config).
  - SuperAdmin can inspect and update `metrics_base_url` via `getMetricsConfig` and `updateMetricsConfig`.
- **Role Guarding**:
  - Preserved existing layout and auth guards.
  - In addition, page inspects `user.role === 'super_admin'`. If non-SuperAdmin accesses the page, mutation controls (buttons, switches, forms) are disabled and a warning banner indicates read-only mode.

**Frontend Verification Limitations**:

- Running `npm run format:check` and `npm run build` from `web/` (and `infinite-canvas/web/`) failed because `node_modules` and CLI tools (`prettier`, `next`) are not installed in this environment.

## Tasks 16-19 frontend completion batch

- Added a shared `ChannelModelOption` transformation that requires enabled, priced ChannelModels, preserves canonical `channel_id::channel_model_id::raw_model` values, joins channel-scoped route/duration metadata, and keeps unavailable/stale metrics as `null` so they sort after numeric rates.
- Logged-in image, audio, and video proxy URLs now fail closed unless the selected identity is current and send separate `channel_id` and `channel_model_id` query parameters; upstream request bodies continue to use raw model names and existing video provider branches/polling.
- Catalog initialization loads pricing and advisory metrics without deleting catalog options when metrics fail; selectors expose loading/error/empty and rate-unavailable states while API keys remain in session-only local credentials and out of persisted config.

## Tasks 16-19 frontend formatting verification

- Targeted `bunx prettier --write` completed successfully for the nine requested frontend paths; eight were unchanged and only `web/src/lib/seedance-video.ts` required formatting.
- Targeted `bunx prettier --check` passed with `All matched files use Prettier code style!`.
- `bun run build` passed: Next.js 16.2.3 compiled successfully, generated all 23 static pages, and finalized optimization. It emitted the existing warning that the `middleware` file convention is deprecated in favor of `proxy`.

## Tasks 20-21 verification batch

- Backend invalid channel/model coverage now counts pricing lookups as well as fake upstream calls, proving forged or disabled identity is rejected before the credit path.
- Model health aggregation keys include application ChannelID and ChannelModelID, so equal raw model names on different channels remain separate entries.
- Frontend uses a small Bun-native test file with no new dependency. It verifies canonical identity, four capability selections, numeric-zero versus null/stale metrics sorting, stale fail-closed request identity, request query IDs, and persistence secret exclusion.
- Verification results: `cd backend && go test ./... && go build ./...` passed; `cd web && bun test` passed with 5 tests/17 assertions; targeted Prettier check and `bun run build` passed. The Bun run emits a non-fatal Zustand storage-unavailable warning because the test process has no browser storage.

## F1 Plan Compliance Audit (2026-07-19)

### Must Have Rules
| # | Rule | Status | Evidence |
|---|------|--------|----------|
| 1 | SuperAdmin-only writes for global channels | ✅ PASS | `router.go:82-97` — superAdmin group with `SuperAdminRequired()` wrapping channel CRUD |
| 2 | Encrypted channel API keys at rest; never returned | ✅ PASS | `channel_service.go:32` — `crypto.Encrypt`; DTOs have `HasKey bool`, **no** raw key field |
| 3 | Tenant/user auth on user-facing reads | ✅ PASS | `router.go:33-35` — `/channels` and `/channels/:id/models` under `AuthRequired` |
| 4 | Channel-specific enabled models, global pricing | ✅ PASS | ChannelModel has `Enabled` per row; pricing remains raw-model-keyed in CreditRepo |
| 5 | Disabled/deleted channel/model fallback | ✅ PASS | `use-config-store.ts:422` strips stale channels from persisted config; fallback to first valid |
| 6 | Sync failure preserves prior catalog | ✅ PASS | Learnings confirm failed sync keeps old models; error shown to SuperAdmin |
| 7 | Metrics validates/clamps hours | ✅ PASS | `ParseMetricsHours()` in service with bounds (default 24, clamp 1..720) |

### Must NOT Have Rules
| # | Rule | Status | Evidence |
|---|------|--------|----------|
| 1 | No new-api URL/keys exposed to users | ✅ PASS | Metrics config endpoints under SuperAdmin group only |
| 2 | No browser direct calls to new-api | ✅ PASS | MetricsHandler is backend-only proxy; no frontend `fetch` to new-api |
| 3 | new-api channel_id not primary key | ✅ PASS | `NewApiChannelID *int` is optional mapping field, not PK |
| 4 | Pricing not silently channel-specific | ✅ PASS | Pricing remains global by raw model name |
| 5 | Catalog not cleared on failed sync | ✅ PASS | Learnings: failed sync keeps old models |
| 6 | Tenant scoping not bypassed | ✅ PASS | All service methods still use Claims.TenantID/UserID |
| 7 | `/backend-api` prefix unchanged | ✅ PASS | `router.go:14` — `r.Group("/backend-api")` |
| 8 | No hard-coded provider branches | ✅ PASS | Video dispatch uses configured `modelRoutes` map |

**VERDICT: APPROVE** — 7/7 Must Have, 8/8 Must NOT Have all pass.

---

## F4 Security/Scope Fidelity Review (2026-07-19)

### Auth [PASS]
- `router.go:82-84`: SuperAdmin group wraps all channel CRUD (`channelsHandler.Create`, `Update`, `Disable`) and model management (`channelModelHandler.Sync`, `Update`)
- `router.go:25`: Authenticated user reads (`channelHandler.List`, `metricsHandler.Read`) under `AuthRequired` only
- `router.go:62`: Normal admin group does NOT include channel endpoints

### Secrets [PASS]
- **Encryption**: `channel_service.go:32,76` — `crypto.Encrypt(s.encryptKey, apiKey)` before DB write
- **Decryption**: `channel_service.go:120-125` — `crypto.Decrypt` only in service for upstream calls
- **Response safety**: `channel_dto.go:6-21` — `ChannelInfo` and `ChannelAdminInfo` have no `ApiKey` field; only `HasKey bool`
- **Frontend write-only**: `channels-admin.ts:20` — `api_key` in request input only; never read from server response
- **Persist safety**: `use-config-store.ts:84-90` — `persistedConfigState` deletes `apiKey`, `baseUrl`, `channels`, `channelMode` before writing to localStorage
- **Merge safety**: `use-config-store.ts:422` — merge strips same fields from persisted state on load
- **No raw keys in localStorage**: Confirmed — `apiKey` excluded from persist; `LOCAL_AI_CREDENTIALS_KEY` stores session-only credentials

### Isolation [PASS]
- Channel table is global (by design — shared channels)
- All generation/credit/log service methods still receive `Claims.TenantID`/`Claims.UserID`
- Channel model is read-only for tenants; no tenant can modify channels

### Scope [CLEAN]
- 87 files changed, all within channel/metrics/admin feature scope
- Changes limited to: new channel DTOs, channel handler/service/repo, metrics handler/service, frontend store/API clients/selectors/admin UI, docs
- No unrelated system files touched

**VERDICT: APPROVE** — Auth PASS | Secrets PASS | Isolation PASS | Scope CLEAN

---

## Local frontend-to-backend proxy

- Development-only Next.js rewrites keep browser requests on the existing `/backend-api` same-origin path while forwarding server-side to `BACKEND_API_URL`.
- The rewrite strips a configured trailing `/backend-api` before restoring the prefix, so both base-host and prefixed environment values avoid duplicated paths; production config remains unchanged.
