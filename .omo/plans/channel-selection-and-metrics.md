# Channel and ChannelModel Redesign

## TL;DR

> **Quick Summary**: Replace the failed tenant-local/client-local channel design with a server-authoritative global `Channel` and relational `ChannelModel` architecture. Preserve authentication and credit accounting, but redesign channel configuration, model discovery, request routing, performance metrics, and model selection around one canonical `(channel, model)` identity.
>
> **Deliverables**:
> - Global SuperAdmin-managed channels with encrypted API keys.
> - Relational `ChannelModel` rows for channel-specific capabilities, enablement, routes, and sync state.
> - Server-validated per-capability channel selection and upstream routing.
> - Backend-proxied new-api metrics with selectable `hours`, explicit mapping, and rate-based recommendation/sorting.
> - Unified frontend channel/model option data consumed by every image/video/text/audio selector.
> - Tests-after coverage and required database/API documentation.
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES - 4 waves plus final verification
> **Critical Path**: canonical schema/identity → server routing/catalog APIs → frontend store/options → selector migration → integration QA

## Context

### Original Request

Add channel selection to Infinite Canvas. Administrators need channel API-key configuration, upstream model discovery, model enable/disable management, channel success-rate visibility, and success rates on model selectors. The original implementation did not work well and is being redesigned.

### Confirmed Decisions

- Existing authentication and credit systems remain in place.
- Channels are global shared resources.
- Only SuperAdmin manages global channels and credentials.
- Users select channels independently for image, video, text, and audio.
- Success rate remains user-visible and supports sorting/recommendation.
- Use an independent relational `ChannelModel` table, not JSON model lists.
- Feature-related uncommitted changes may be replaced; unrelated user changes must not be touched.
- new-api metrics are called by the backend using a separate management base URL and `GET /api/perf-metrics/channels?hours=N`.

### Current Design Audit

- `backend/model/api_config.go` and `backend/handler/api_config_handler.go` implement one tenant-local `BaseUrl`/`ApiKey` and JSON model catalogs.
- `backend/model/model_call_log.go` lacks application channel identity.
- `backend/service/channel_status_service.go` aggregates local logs by generation/model, defaults no observations to 100%, and does not call new-api.
- `backend/handler/channel_status_handler.go` hardcodes tenant `0`, uses `days`, and returns raw JSON instead of the project envelope.
- `web/src/app/(user)/channel-status/page.tsx` directly calls axios and renders local uptime, not new-api channel/model metrics.
- `web/src/stores/use-config-store.ts` persists client-local API keys, strips channel identity before route metadata lookup, and allows same-name cross-channel collisions.
- `web/src/components/model-picker.tsx` reads global capability lists and has no success-rate metadata or channel filtering.
- Current dirty worktree includes feature-related channel/status/API-config changes. These are reference material and replaceable; unrelated changes remain out of scope.

### Repository Constraints

- Follow root `AGENTS.md`: handler only handles HTTP, service owns business logic, repository owns database access, model owns structures/enums, API clients live under `web/src/services/api/`, shared frontend state under `web/src/stores/`, UI copy is Chinese, and new tables update `docs/content/docs/backend/backend-database.mdx`.
- Project is not launched; no legacy data compatibility/migration fallback is required unless a current runtime dependency makes it necessary.
- User explicitly requests tests-after; this overrides the repository's default note that agents need not run builds/tests.

## Work Objectives

### Core Objective

Make `(application channel ID, raw model name, capability)` the server-validated identity for model catalogs, credentials, routing, credits, logs, metrics, and frontend selection while preserving existing auth and credit behavior.

### Target Data Model

- `Channel`: global ID, display name, enabled state, upstream base URL, encrypted API key, optional new-api channel mapping, sync metadata, timestamps.
- `ChannelModel`: channel foreign key, raw model name, capability set, enabled state, route metadata, duration/customization metadata, discovery/sync metadata, unique `(channel_id, model_name)` constraint.
- Existing global pricing remains keyed by raw model name unless the implementation audit proves a different existing pricing key is required.
- Model-call logs gain application channel identity and use `(channel_id, model, capability)` for aggregation.

### Must Have

- Server owns all channel credentials; frontend never stores or submits channel API keys for user generation.
- SuperAdmin-only channel writes; authenticated users can read enabled channels and enabled priced models.
- Per-capability channel selections are transmitted to and validated by the backend.
- Same raw model name in two channels remains distinct in routes, catalogs, logs, metrics, and UI options.
- Sync failure preserves the existing `ChannelModel` rows and exposes a retryable status.
- new-api metrics use a separately configured URL, explicit nullable mapping, bounded `hours` defaulting to 24, and no direct browser call.
- Unavailable/stale metrics are distinct from a real `0%` rate and never block generation.
- Disabled channel/model rejection occurs before credit deduction and upstream calls.
- Every model selector can sort/recommend by channel-specific success rate without changing the canonical option identity.

### Must NOT Have

- Do not continue the tenant-local `TenantApiConfig` as the source of truth for global channels.
- Do not persist raw API keys in Zustand, localStorage, API responses, logs, or metrics payloads.
- Do not key route/duration/capability metadata by raw model alone.
- Do not let the browser call new-api directly.
- Do not infer application channel identity from new-api channel IDs or model names.
- Do not mix pricing validity with channel model enablement.
- Do not return `100%` for no observations; use unavailable/unknown.
- Do not alter authentication, credit calculation semantics, `/backend-api`, or the normal `{code,data,msg}` envelope except where the channel contract requires a documented extension.
- Do not revert or overwrite unrelated existing user changes in the dirty worktree.

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - all verification is agent-executed.

### Test Decision

- **Automated tests**: Tests-after.
- **Backend**: Go `testing` and existing backend test conventions.
- **Frontend**: `web/package.json` currently has build/format scripts but no test runner dependency. Add the smallest focused frontend test setup only if required for store/option behavior; otherwise use deterministic Playwright/API QA for UI behavior.
- **Required commands**: `cd backend && go test ./...`; `cd backend && go vet ./...`; `cd web && npm run format:check`; `cd web && npm run build`.

### QA Policy

- Backend API and routing tasks use fake upstream and fake metrics HTTP servers, authenticated curl/API tests, and database assertions.
- Frontend tasks use Playwright against the full app, asserting selectors, exact model/channel values, success-rate labels, persistence, and failure states.
- Every task below includes one happy path and one negative/edge scenario with evidence under `.omo/evidence/`.

## Execution Strategy

### Parallel Execution Waves

```text
Wave 1 - canonical contracts and storage
├── 1. Dirty-worktree boundary and baseline inventory
├── 2. Channel and ChannelModel schema
├── 3. Channel/model/metrics contracts
├── 4. Channel-aware call-log identity
├── 5. Frontend canonical option/state contract
└── 6. Fake upstream/metrics fixtures

Wave 2 - backend services and APIs (dependency-gated subwaves)
├── Wave 2a: 7. Global Channel repository/service and SuperAdmin APIs
├── Wave 2b: 8. ChannelModel discovery/sync/enablement service
└── Wave 2c: 9. Server routing + 10. global pricing integration + 11. new-api metrics adapter (parallel), then 12. frontend API clients

Wave 3 - frontend UX and selector migration (dependency-gated subwaves)
├── Wave 3a: 13. SuperAdmin management UI + 14. metrics controls + 15. capability channel selection (parallel)
├── Wave 3b: 16. shared ChannelModel/rate option builder
└── Wave 3c: 17. image/text/audio selectors + 18. video selectors + 19. loading/error UX (parallel)

Wave 4 - tests and integration (dependency-gated subwaves)
├── Wave 4a: 20. backend integration tests + 21. frontend tests/setup (parallel)
├── Wave 4b: 22. SuperAdmin browser QA + 23. user browser/channel QA (parallel)
└── Wave 4c: 24. database/API docs and rollout review

Final Verification Wave
├── F1. Plan compliance audit
├── F2. Build/test/code quality review
├── F3. Full API/browser QA and evidence review
└── F4. Security/scope/dirty-worktree fidelity review
```

### Dependency Matrix

```text
1: none -> 2-24
2: 1 -> 7-11,20,24
3: 1 -> 7-12,15-19,20-23
4: 1 -> 9-11,20,23
5: 1 -> 12,15-19,21,23
6: 1 -> 8-11,20-23
7: 2,3 -> 8-12,13-14,20,22,24
8: 2,3,6,7 -> 9-12,13-14,16-20,22-24
9: 3,4,6,7,8 -> 12,17-18,20,23-24
10: 3,7-8 -> 20,23
11: 2,3,6,7,8 -> 12,14,16,20,22-24
12: 3,5,7-11 -> 13-19,21-23
13: 7,8,12 -> 22,24
14: 11,12 -> 16,22-24
15: 5,7,8,12 -> 16-19,21,23
16: 5,8,11,12-15 -> 17-19,21,23
17: 9,10,12,15,16 -> 20-23
18: 9,10,12,15,16 -> 20-23
19: 12,15,16 -> 21-23
20: 8-11 -> F1-F4
21: 15-19 -> F1-F4
22: 13,14,20,21 -> F1-F4
23: 9-12,15-21 -> F1-F4
24: 2,3,7-11,22-23 -> F1-F4
F1-F4: all -> handoff
```

## TODOs

- [x] 1. Establish dirty-worktree boundaries and baseline inventory

  **What to do**:
  - Record the current `git status --short` and classify feature-related files that may be replaced versus unrelated files that must not be touched.
  - Inventory actual model-selector call sites, API-config clients, backend route wiring, and test/build commands from the complete tree.
  - Do not reset, checkout, clean, or overwrite the worktree.

  **Recommended Agent Profile**: `unspecified-high`; repository-boundary and impact audit.

  **Parallelization**: Wave 1; blocks every task; blocked by none.

  **References**:
  - `AGENTS.md:7-14` - preserve user changes and avoid unrelated refactors.
  - `backend/main.go:18-84` - actual backend wiring.
  - `web/package.json:6-12` - frontend scripts.
  - `git status --short` - current dirty-worktree boundary.

  **Acceptance Criteria**:
  - A file-level boundary list exists for replaceable channel/status/API-config work and untouched unrelated work.
  - Baseline command inventory includes `cd backend && go test ./...`, `cd backend && go vet ./...`, `cd web && npm run format:check`, and `cd web && npm run build`.
  - No unrelated file is modified by this task.

  **QA Scenarios**:
  ```
  Scenario: Dirty worktree is preserved
    Tool: Bash
    Preconditions: Existing dirty worktree.
    Steps:
      1. Save `git status --short` to evidence.
      2. Run the inventory commands without reset/checkout/clean.
      3. Compare unrelated paths before and after.
    Expected Result: Unrelated paths are unchanged.
    Evidence: .omo/evidence/task-1-dirty-boundary.txt

  Scenario: Missing baseline command is reported
    Tool: Bash
    Preconditions: A required dependency or service is unavailable.
    Steps:
      1. Run each baseline command.
      2. Capture exit code and stderr rather than treating failure as success.
    Expected Result: Baseline failures are explicit and attributable.
    Evidence: .omo/evidence/task-1-baseline-failures.txt
  ```

- [x] 2. Replace tenant API config with global Channel and ChannelModel schema

  **What to do**:
  - Add `Channel` and `ChannelModel` models under `backend/model/` with global ownership, encrypted key, enabled state, upstream URL, optional new-api mapping, sync metadata, and unique `(channel_id, model_name)` identity.
  - Add capability representation that supports image/video/text/audio without route metadata keyed only by raw model.
  - Register the new models in `backend/main.go` AutoMigrate and remove feature dependence on tenant-local JSON model catalogs.
  - Keep existing auth/credit models intact; do not implement old-data compatibility unless required by current startup.

  **Recommended Agent Profile**: `deep`; relational design and cross-cutting identity.

  **Parallelization**: Wave 1 with 3-6; blocked by 1; blocks 7-11,20,24.

  **References**:
  - `backend/model/api_config.go:3-18` - failed single-tenant/JSON config shape to replace.
  - `backend/model/base.go:9-14` - shared GORM model fields.
  - `backend/main.go:26-39` - migration registration.
  - `AGENTS.md:22-31` - model/repository layering and database-doc requirement.

  **Acceptance Criteria**:
  - Channel and ChannelModel tables have explicit foreign key/index/unique constraints.
  - API keys are represented as encrypted-at-rest fields and are never JSON-exposed by model tags or handlers.
  - ChannelModel can represent duplicate raw model names across channels without collision.
  - `docs/content/docs/backend/backend-database.mdx` documents the final schema.

  **QA Scenarios**:
  ```
  Scenario: Two channels can own the same model name
    Tool: Go test with GORM test database
    Preconditions: Channel A and B exist.
    Steps:
      1. Insert `gemini-2.5-pro` under both channels.
      2. Query both rows by `(channel_id, model_name)`.
      3. Assert both rows exist with independent IDs and metadata.
    Expected Result: Same-name models do not collide.
    Evidence: .omo/evidence/task-2-channel-model-identity.txt

  Scenario: Duplicate model within one channel is rejected
    Tool: Go test with GORM test database
    Preconditions: Channel A exists with `gpt-5.4`.
    Steps:
      1. Insert the same model/capability identity again for Channel A.
      2. Assert the unique constraint or service validation rejects it.
    Expected Result: One canonical ChannelModel row exists.
    Evidence: .omo/evidence/task-2-duplicate-model.txt
  ```

- [x] 3. Define canonical channel/model/metrics contracts

  **What to do**:
  - Define backend DTOs for public enabled-channel/catalog reads, SuperAdmin CRUD, model sync, per-channel enablement, metrics configuration, and `hours` metrics results.
  - Define one canonical option identity containing application channel ID, ChannelModel ID or stable key, raw model name, capability, and optional rate metadata.
  - Define `success_rate: null`/unavailable separately from numeric `0`.

  **Recommended Agent Profile**: `quick`; contract and type boundary.

  **Parallelization**: Wave 1 with 2,4-6; blocked by 1; blocks 7-12,15-19.

  **References**:
  - `backend/model/response.go:9-41` - `{code,data,msg}` response helpers.
  - `backend/router/router.go:14-85` - route/auth grouping.
  - `web/src/stores/use-config-store.ts:8-62` - current types and route enums to replace or narrow.
  - User-provided new-api payload - metrics fields and nested model shape.

  **Acceptance Criteria**:
  - DTOs distinguish application channel ID from new-api `channel_id`.
  - Contracts include sync status/error/timestamp, enabled state, and rate freshness/window metadata.
  - Contract tests cover empty lists, unmapped metrics, zero rate, and unavailable metrics.

  **QA Scenarios**:
  ```
  Scenario: Contract maps zero and unavailable distinctly
    Tool: Go test
    Preconditions: Fixtures contain a model at 0% and a model absent from metrics.
    Steps:
      1. Decode both through the contract mapper.
      2. Assert one has numeric 0 and the other has nil/unavailable state.
    Expected Result: UI can distinguish no data from failure rate.
    Evidence: .omo/evidence/task-3-rate-contract.txt

  Scenario: Invalid hours contract is rejected
    Tool: Go test
    Preconditions: Metrics request validator exists.
    Steps:
      1. Validate `hours=0`, negative, non-numeric, and above maximum.
      2. Assert deterministic default/clamp behavior.
    Expected Result: No unbounded metrics request is accepted.
    Evidence: .omo/evidence/task-3-hours-validation.txt
  ```

- [x] 4. Add channel identity to model-call logs and aggregation contracts

  **What to do**:
  - Add ChannelID/ChannelModel identity to `backend/model/model_call_log.go` and update repository/service record methods.
  - Keep auth/user fields and credit-related logging intact.
  - Replace generation+raw-model aggregation keys with channel-aware keys where local logs remain useful.

  **Recommended Agent Profile**: `deep`; logging contracts and credit-adjacent behavior.

  **Parallelization**: Wave 1 with 2-3,5-6; blocked by 1; blocks 9-11,20,23.

  **References**:
  - `backend/model/model_call_log.go:3-18` - current fields lacking channel identity.
  - `backend/repository/model_call_log_repo.go` - current query methods.
  - `backend/service/model_call_log_service.go` - current record/health grouping.
  - `backend/service/generate_service.go:943-975` - current success/failure recording.

  **Acceptance Criteria**:
  - Every new proxy/generation/model-test log records the resolved Channel and ChannelModel identity.
  - Same raw model on two channels aggregates separately.
  - Existing tenant/user filters remain applied.

  **QA Scenarios**:
  ```
  Scenario: Logs separate same-name models by channel
    Tool: Go integration test
    Preconditions: Two channels use `gpt-5.4`.
    Steps:
      1. Record success on A and failure on B.
      2. Query health/aggregation for both channels.
      3. Assert rates are 100% and 0% independently.
    Expected Result: No cross-channel merge.
    Evidence: .omo/evidence/task-4-channel-log-rates.txt

  Scenario: Missing channel identity is rejected for new calls
    Tool: Go integration test
    Preconditions: Generation request has a model but no valid channel.
    Steps:
      1. Attempt to record/execute the request.
      2. Assert no successful log or credit transaction is created.
    Expected Result: New calls cannot silently become unscoped logs.
    Evidence: .omo/evidence/task-4-missing-channel.txt
  ```

- [x] 5. Redesign frontend canonical option and selection state

  **What to do**:
  - Replace client-local API-key/channel authority in `web/src/stores/use-config-store.ts` with server-sourced channel/catalog state and four selected channel IDs.
  - Preserve only small UI preferences locally; do not persist server API keys or full business catalogs in localStorage.
  - Provide selectors/utilities that return channel-scoped ChannelModel options, raw request model, route metadata, and rate metadata together.

  **Recommended Agent Profile**: `deep`; state identity and migration boundary.

  **Parallelization**: Wave 1 with 2-4,6; blocked by 1; blocks 12,15-19,21,23.

  **References**:
  - `web/src/stores/use-config-store.ts:16-47` - current config shape with client API keys.
  - `web/src/stores/use-config-store.ts:209-311` - current persist/merge behavior.
  - `web/src/stores/use-config-store.ts:336-459` - current channel encoding and request resolution.
  - `AGENTS.md:33-55` - frontend state and persistence rules.

  **Acceptance Criteria**:
  - No user-facing persisted state contains channel API keys.
  - Four capability selections are independent and resolve only enabled server channels/models.
  - Same raw model names remain distinct by ChannelModel identity.
  - Request config cannot be built without a valid server channel/model identity.

  **QA Scenarios**:
  ```
  Scenario: Capability selections persist without secrets
    Tool: Browser/store test
    Preconditions: Server catalog has channels A-D.
    Steps:
      1. Select A/B/C/D for image/video/text/audio.
      2. Persist and reload the store.
      3. Inspect persisted storage for API-key fields.
    Expected Result: Four selections survive and no channel secret is persisted.
    Evidence: .omo/evidence/task-5-selection-state.txt

  Scenario: Stale ChannelModel cannot produce a request
    Tool: Browser/store test
    Preconditions: Persist a model that server catalog later disables.
    Steps:
      1. Reload with updated catalog.
      2. Resolve request config.
      3. Assert fallback or explicit unavailable state and no raw request config.
    Expected Result: Stale selection fails closed.
    Evidence: .omo/evidence/task-5-stale-model.txt
  ```

- [x] 6. Add deterministic fake upstream and metrics fixtures

  **What to do**:
  - Create reusable test servers for upstream `/models`, generation/proxy endpoints, and new-api `/api/perf-metrics/channels?hours=N`.
  - Include two channels exposing the same raw model with different responses/rates, sync failures, malformed metrics, timeout, and unavailable mapping.

  **Recommended Agent Profile**: `quick`; deterministic test support.

  **Parallelization**: Wave 1 with 2-5; blocked by 1; blocks 8-11,20-23.

  **References**:
  - `backend/service/generate_service_test.go` - existing Go HTTP test style.
  - `web/package.json:6-12` - available frontend command surface.
  - User-provided new-api metrics response.

  **Acceptance Criteria**:
  - Fixtures capture request path/query/headers/body and can switch response scenarios per test.
  - Test data contains no production credentials.

  **QA Scenarios**:
  ```
  Scenario: Fixture captures channel and hours requests
    Tool: Go test
    Preconditions: Fake servers started.
    Steps:
      1. Call `/models` through Channel A.
      2. Call metrics with `hours=24`.
      3. Assert captured URL, query, and absence of metrics Authorization.
    Expected Result: Fixtures verify routing contracts.
    Evidence: .omo/evidence/task-6-fixtures.txt

  Scenario: Fixture simulates malformed metrics
    Tool: Go test
    Preconditions: Fake metrics server configured with invalid JSON.
    Steps:
      1. Call the metrics adapter.
      2. Assert typed unavailable/error result and no panic.
    Expected Result: Degradation is deterministic.
    Evidence: .omo/evidence/task-6-malformed-metrics.txt
  ```

- [x] 7. Implement global Channel repository/service and SuperAdmin API

  **What to do**: Add repository CRUD and service validation for global channels; add SuperAdmin-only create/update/disable/delete/list routes; add authenticated redacted enabled-channel read. Preserve encrypted-key update semantics and never return secrets.

  **Recommended Agent Profile**: `deep`; authorization and secret handling.
  **Parallelization**: Wave 2a; blocked by 2-3; blocks 8-14,20,22,24.
  **References**: `backend/router/router.go:22-85`; `backend/handler/api_config_handler.go:40-191`; `backend/repository/api_config_repo.go`; `backend/service/auth_service.go`.
  **Acceptance Criteria**: Only SuperAdmin writes; ordinary reads contain enabled channels and redacted fields; blank API key preserves existing encrypted key; disabling removes channel from user reads without deleting historical references.
  **QA Scenarios**:
  ```
  Scenario: SuperAdmin creates a channel
    Tool: curl/API test
    Preconditions: SuperAdmin JWT and test DB.
    Steps: POST channel `Primary` with base URL and synthetic key; GET list; assert channel exists and key is absent.
    Expected Result: Global channel is created and secret is redacted.
    Evidence: .omo/evidence/task-7-superadmin-crud.json
  Scenario: Normal admin cannot mutate channels
    Tool: curl/API test
    Preconditions: Admin JWT without SuperAdmin role.
    Steps: POST and DELETE channel; assert authorization envelope and unchanged DB.
    Expected Result: Global writes are denied.
    Evidence: .omo/evidence/task-7-admin-denied.json
  ```

- [x] 8. Implement ChannelModel discovery, sync, and enablement

  **What to do**: Add repository/service methods to fetch `/models` with decrypted channel key, upsert ChannelModel rows, preserve existing rows on failure, assign capabilities/routes, and toggle channel-specific enabled state. Expose sync status/retry and model management to SuperAdmin.

  **Recommended Agent Profile**: `deep`; upstream synchronization and relational state.
  **Parallelization**: Wave 2b; blocked by 2,3,6,7; blocks 9-19,20,22-24.
  **References**: `backend/service/generate_service.go:81-124`; `web/src/services/api/image.ts:672-690`; `web/src/app/(user)/admin/api-config/page.tsx:112-255`; `backend/model/api_config.go`.
  **Acceptance Criteria**: Successful sync upserts normalized ChannelModels; failed sync preserves old rows and records error; duplicate names within one channel are rejected; enablement is independent per channel; sync never exposes keys.
  **QA Scenarios**:
  ```
  Scenario: Sync creates channel models
    Tool: curl/API test with fake upstream
    Preconditions: `/models` returns `gpt-5.4` and `gemini-2.5-pro`.
    Steps: Trigger sync; GET catalog; assert both rows and success status.
    Expected Result: Catalog is updated for only the target channel.
    Evidence: .omo/evidence/task-8-sync-success.json
  Scenario: Sync failure preserves rows
    Tool: curl/API test with fake upstream
    Preconditions: Existing `gpt-5.4`; `/models` returns 503.
    Steps: Trigger sync; GET catalog/status; assert old row remains and error is recorded.
    Expected Result: Failed sync is non-destructive.
    Evidence: .omo/evidence/task-8-sync-failure.json
  ```

- [x] 9. Make generation/proxy/model-test routing server-authoritative

  **What to do**: Require channel ID + ChannelModel identity in generation/proxy requests; validate enabled channel/model/capability server-side; resolve URL/key/route from ChannelModel; carry identity through repair/retry and logs; reject before credit spend.

  **Recommended Agent Profile**: `ultrabrain`; highest-risk request identity change.
  **Parallelization**: Wave 2c with 10-11; blocked by 3-4,6-8; blocks 12,17-18,20,23-24.
  **References**: `backend/service/generate_service.go:65-230,283-655`; `backend/handler/proxy_handler.go:20-105`; `backend/service/model_test_service.go`; `web/src/services/api/image.ts:390-405`; `web/src/services/api/video.ts:95-140`.
  **Acceptance Criteria**: Same model on A/B reaches distinct fake upstreams; backend ignores client credentials; invalid/disabled identity makes no credit transaction/upstream call; proxy success preserves upstream status/body/credit headers.
  **QA Scenarios**:
  ```
  Scenario: Same model routes through selected channel
    Tool: curl with two fake upstreams
    Preconditions: A/B both enable `gemini-2.5-pro`.
    Steps: Send requests with A then B; assert each fake receives only its selected request and raw model.
    Expected Result: Server routing follows validated ChannelModel.
    Evidence: .omo/evidence/task-9-routing.json
  Scenario: Forged channel or disabled model fails before spend
    Tool: curl/database assertion
    Preconditions: Channel B/model disabled.
    Steps: Submit request naming B; assert non-zero envelope, no upstream request, no credit log.
    Expected Result: Request fails closed.
    Evidence: .omo/evidence/task-9-routing-denied.json
  ```

- [x] 10. Preserve global pricing while integrating ChannelModel checks

  **What to do**: Make credit pricing lookup use raw model name globally, while availability uses selected ChannelModel; update estimate/spend/test paths and admin pricing UI so pricing is not duplicated per channel.

  **Recommended Agent Profile**: `deep`; billing behavior preservation.
  **Parallelization**: Wave 2c with 9,11; blocked by 3,7-8; blocks 12,20,23.
  **References**: `backend/service/credit_service.go`; `backend/service/credit_pricing_calculator.go`; `backend/handler/credit_handler.go`; `backend/handler/api_config_handler.go:85-106`; `web/src/services/api/pricing.ts`.
  **Acceptance Criteria**: One raw model price serves multiple channels; disabling in A does not disable B; unpriced models cannot generate; existing credit amounts remain unchanged.
  **QA Scenarios**:
  ```
  Scenario: Global price applies to two channels
    Tool: Go/API integration test
    Preconditions: Same model enabled in A/B and one price exists.
    Steps: Estimate and generate through A/B; assert same pricing and separate channel logs.
    Expected Result: Pricing is global, availability is channel-specific.
    Evidence: .omo/evidence/task-10-global-pricing.txt
  Scenario: Unpriced model is rejected
    Tool: curl/API test
    Preconditions: ChannelModel enabled but no price.
    Steps: Request catalog and generation; assert omission/rejection and no upstream call.
    Expected Result: No free path exists.
    Evidence: .omo/evidence/task-10-unpriced.json
  ```

- [x] 11. Add independent new-api metrics adapter and recommendation data

  **What to do**: Add global metrics URL config managed by SuperAdmin; call `/api/perf-metrics/channels?hours=N` server-side; map optional new-api channel IDs to application Channels; join nested model metrics by `(mapped channel, raw model)`; validate/clamp hours and expose unavailable/stale metadata; add sort/recommendation fields without auto-routing.

  **Recommended Agent Profile**: `deep`; external API and security boundary.
  **Parallelization**: Wave 2c with 9-10; blocked by 2-3,6-8; blocks 12,14,16,20,22-24.
  **References**: User-provided metrics payload; `backend/config/config.go`; `backend/model/response.go`; `backend/service/generate_service.go:92-124`; `backend/router/router.go`.
  **Acceptance Criteria**: Exact hours query is sent with no browser call/auth key; invalid hours defaults/clamps; zero and unavailable differ; unmapped channel/model is unavailable; metrics failure does not remove catalogs or block generation.
  **QA Scenarios**:
  ```
  Scenario: Metrics adapter maps selected window
    Tool: curl with fake new-api
    Preconditions: Metrics URL configured; fixture has channel/model rates.
    Steps: GET backend metrics with `hours=24`; assert fake saw exact query/no Authorization and response has mapped rates.
    Expected Result: Backend-only metrics mapping works.
    Evidence: .omo/evidence/task-11-metrics.json
  Scenario: Metrics outage is non-blocking
    Tool: curl/API integration test
    Preconditions: Metrics fake returns 503; catalog/upstream works.
    Steps: Request metrics then generate; assert unavailable metrics and successful generation.
    Expected Result: Metrics are advisory.
    Evidence: .omo/evidence/task-11-metrics-outage.json
  ```

- [x] 12. Add frontend channel/catalog/metrics API clients

  **What to do**: Add typed clients under `web/src/services/api/` for redacted channel reads, SuperAdmin CRUD/sync, ChannelModel updates, metrics URL/hours, and recommendation data; use shared client/envelope/auth behavior.

  **Recommended Agent Profile**: `quick`; typed API boundary.
  **Parallelization**: Wave 2c after 9-11; blocked by 3,5,7-11; blocks 13-19,21-23.
  **References**: `web/src/services/api/client.ts`; `web/src/services/api/admin.ts`; `web/src/services/api/api-config.ts`; `web/src/services/api/pricing.ts`; `backend/model/response.go`.
  **Acceptance Criteria**: Clients never accept keys; all requests use `/backend-api` and unwrap business errors; metrics client represents unavailable separately from 0%.
  **QA Scenarios**:
  ```
  Scenario: Client loads redacted catalog and metrics
    Tool: Frontend API test/mock
    Preconditions: Mock backend returns channels, models, and rates.
    Steps: Call clients with `hours=24`; assert paths, auth, and parsed values.
    Expected Result: Typed data is usable by selectors.
    Evidence: .omo/evidence/task-12-api-client.json
  Scenario: Envelope error rejects
    Tool: Frontend API test/mock
    Preconditions: Mock returns HTTP 200 with non-zero code.
    Steps: Call admin client; assert rejection with server message.
    Expected Result: Business failure is not treated as success.
    Evidence: .omo/evidence/task-12-envelope-error.txt
  ```

- [x] 13. Build SuperAdmin Channel and ChannelModel management UI

  **What to do**: Replace the tenant API-config model table with a global channel management page. Support channel CRUD/disable, write-only key, model sync/retry, per-channel model enablement/capabilities/routes, sync status, and metrics URL settings using existing Ant Design/Tailwind conventions.

  **Recommended Agent Profile**: `visual-engineering`; admin workflow and table state.
  **Parallelization**: Wave 3a with 14-15; blocked by 7,8,12; blocks 22,24.
  **References**: `web/src/app/(user)/admin/api-config/page.tsx:98-255,325-465,547-749`; `web/src/app/(user)/admin/layout.tsx`; `web/src/services/api/admin.ts`.
  **Acceptance Criteria**: SuperAdmin can create/edit/disable/sync; keys are masked/write-only; model enablement is channel-specific; sync failure leaves previous rows visible; non-SuperAdmin cannot access mutation controls.
  **QA Scenarios**:
  ```
  Scenario: SuperAdmin manages two channels and syncs models
    Tool: Playwright
    Preconditions: SuperAdmin session and fake upstreams.
    Steps: Create A/B; sync both; disable `gpt-5.4` only in A; assert B remains enabled and keys stay masked.
    Expected Result: Relational channel/model management works.
    Evidence: .omo/evidence/task-13-admin-management.png
  Scenario: Sync error is recoverable
    Tool: Playwright
    Preconditions: Existing channel models; sync endpoint returns 503.
    Steps: Click sync; assert error/retry state and old model rows remain.
    Expected Result: Admin does not lose catalog data.
    Evidence: .omo/evidence/task-13-sync-error.png
  ```

- [x] 14. Add metrics hours and recommendation controls

  **What to do**: Add SuperAdmin-selectable hours, refresh state, last-success/stale/error display, channel/model rate cards, and sort/recommendation controls. Ensure real 0% is distinct from unavailable.

  **Recommended Agent Profile**: `visual-engineering`; metrics UX.
  **Parallelization**: Wave 3a with 13,15; blocked by 11-12; blocks 16,22-24.
  **References**: `web/src/app/(user)/channel-status/page.tsx` (replace local-log UI); `web/src/components/model-picker.tsx`; `web/src/lib/app-theme.ts`; `web/src/services/api/client.ts`.
  **Acceptance Criteria**: Changing hours sends exact query; rates display with channel/model identity; sorting/recommendation is deterministic and never auto-routes; unavailable/stale state is explicit.
  **QA Scenarios**:
  ```
  Scenario: Rates sort by selected window
    Tool: Playwright
    Preconditions: Same capability has models at 0%, 50%, 100%.
    Steps: Choose `hours=24`; select success-rate descending sort; assert exact option order and labels.
    Expected Result: Recommendation data drives visible ordering only.
    Evidence: .omo/evidence/task-14-rate-sorting.png
  Scenario: Unavailable rate is not 0%
    Tool: Playwright
    Preconditions: One ChannelModel has no mapped metrics.
    Steps: Open metrics/selector view; assert unavailable label and that it sorts after numeric rates.
    Expected Result: Missing data is not misrepresented.
    Evidence: .omo/evidence/task-14-unavailable-rate.png
  ```

- [x] 15. Add independent per-capability channel selection

  **What to do**: Add image/video/text/audio channel selectors to the shared store/UI; load enabled global channels from the server; derive each capability's ChannelModel list from its selected channel; recover when a channel is disabled.

  **Recommended Agent Profile**: `visual-engineering`; shared state and repeated UI.
  **Parallelization**: Wave 3a with 13-14; blocked by 5,7,8,12; blocks 16-19,21,23.
  **References**: `web/src/stores/use-config-store.ts:117-207`; `web/src/components/model-picker.tsx:21-83`; `web/src/app/(user)/image/page.tsx`; `web/src/app/(user)/video/page.tsx`; `web/src/components/video-settings-panel.tsx`.
  **Acceptance Criteria**: Four selectors retain independent channel IDs; changing one does not mutate others; model lists update from selected channel only; stale selection falls back safely.
  **QA Scenarios**:
  ```
  Scenario: Four capability channels remain independent
    Tool: Playwright
    Preconditions: A-D have distinct catalogs.
    Steps: Select A/B/C/D for image/video/text/audio; reload; assert all four selections and lists persist.
    Expected Result: Per-capability state is independent.
    Evidence: .omo/evidence/task-15-capability-channels.png
  Scenario: Disabled selected channel falls back
    Tool: Playwright
    Preconditions: Image uses A; A becomes disabled.
    Steps: Refresh server catalog; assert image selection falls back and cannot submit A model.
    Expected Result: Stale channel is never sent.
    Evidence: .omo/evidence/task-15-channel-fallback.png
  ```

- [x] 16. Build shared ChannelModel option builder with rate sorting

  **What to do**: Create one shared transformation that joins selected channel models with global pricing and metrics, returns canonical IDs, labels, raw request model, capability, route metadata, and rate freshness; use it for all pickers.

  **Recommended Agent Profile**: `deep`; canonical identity and data joining.
  **Parallelization**: Wave 3b; blocked by 5,8,11-15; blocks 17-19,21,23.
  **References**: `web/src/components/model-picker.tsx:70-98`; `web/src/stores/use-config-store.ts:159-180,385-419`; `web/src/app/(user)/admin/api-config/page.tsx:754-785`.
  **Acceptance Criteria**: Same raw model on A/B yields distinct option keys and correct rates/routes; no option lacks channel identity; unavailable rate is not numeric zero; unpriced/disabled rows are excluded from user options.
  **QA Scenarios**:
  ```
  Scenario: Same-name options retain separate rates
    Tool: Frontend test/Playwright
    Preconditions: `gemini-2.5-pro` on A=0%, B=100%.
    Steps: Build options for A/B; assert distinct values and matching labels/rates.
    Expected Result: Option identity is ChannelModel-specific.
    Evidence: .omo/evidence/task-16-option-identity.png
  Scenario: Missing metrics remain unavailable
    Tool: Frontend test
    Preconditions: ChannelModel absent from metrics.
    Steps: Build options; assert unavailable metadata and deterministic sort position.
    Expected Result: No fabricated rate.
    Evidence: .omo/evidence/task-16-option-missing-metrics.txt
  ```

- [x] 17. Migrate image/text/audio selectors and request builders

  **What to do**: Replace every image/text/audio model selector with shared options; pass server-validated channel/model identity through request APIs; preserve existing generation bodies, quality, size, voice, and response parsing.

  **Recommended Agent Profile**: `visual-engineering`; broad selector migration.
  **Parallelization**: Wave 3c with 18-19; blocked by 9-10,12,15-16; blocks 20-23.
  **References**: `web/src/services/api/image.ts:557-669`; `web/src/services/api/audio.ts:1-100`; `web/src/app/(user)/image/page.tsx`; `web/src/app/(user)/page.tsx`; `web/src/components/layout/app-config-modal.tsx`.
  **Acceptance Criteria**: Every image/text/audio selector displays rate and selected channel; requests use raw model plus validated channel identity; no client API key is sent in logged-in mode.
  **QA Scenarios**:
  ```
  Scenario: Image/text/audio route to selected channels
    Tool: Playwright with fake upstreams
    Preconditions: Distinct channel selected per capability.
    Steps: Trigger one request per capability; inspect fake upstream model/path/channel assertions.
    Expected Result: All requests use the selected channel.
    Evidence: .omo/evidence/task-17-nonvideo-routing.png
  Scenario: Empty capability catalog blocks generation
    Tool: Playwright
    Preconditions: Selected channel has no enabled priced audio model.
    Steps: Open audio picker and generate action; assert empty state and no request.
    Expected Result: No invalid request is sent.
    Evidence: .omo/evidence/task-17-empty-audio.png
  ```

- [x] 18. Migrate video selectors and route metadata

  **What to do**: Make video route/duration/customization metadata resolve from ChannelModel; preserve existing provider branches and polling; remove raw-model route collisions.

  **Recommended Agent Profile**: `deep`; provider branching and long polling.
  **Parallelization**: Wave 3c with 17,19; blocked by 9-10,12,15-16; blocks 20-23.
  **References**: `web/src/services/api/video.ts:80-140,313-407`; `web/src/stores/use-config-store.ts:354-419`; `web/src/components/video-settings-panel.tsx`; `web/src/app/(user)/video/page.tsx`.
  **Acceptance Criteria**: Same raw video model can use different routes per channel; duration options and metrics match selected ChannelModel; no new hard-coded model-name route branches.
  **QA Scenarios**:
  ```
  Scenario: Video route follows ChannelModel
    Tool: Playwright with fake providers
    Preconditions: A maps model to seedance; B maps same raw model to openai.
    Steps: Create task with A then B; assert provider-specific create/poll paths.
    Expected Result: Route metadata follows channel identity.
    Evidence: .omo/evidence/task-18-video-channel-route.png
  Scenario: Unsupported reference remains blocked
    Tool: Playwright
    Preconditions: Selected route does not support reference audio/video.
    Steps: Add unsupported reference and start generation; assert localized error and no polling.
    Expected Result: Existing provider guards remain intact.
    Evidence: .omo/evidence/task-18-video-reference-error.png
  ```

- [x] 19. Implement loading, stale, disabled, error, and empty states

  **What to do**: Add shared UX for channel/catalog loading, metrics refresh, sync failures, disabled models, unavailable rates, and server rejection. Keep Chinese copy and existing theme conventions.

  **Recommended Agent Profile**: `visual-engineering`; cross-cutting state UX.
  **Parallelization**: Wave 3c with 17-18; blocked by 12,15-16; blocks 21-23.
  **References**: `web/src/app/(user)/admin/api-config/page.tsx:112-142,250-255`; `web/src/services/api/image.ts:369-383`; `web/src/services/api/video.ts:739-752`; `AGENTS.md:47-65`.
  **Acceptance Criteria**: No request is issued while channel/model identity is unresolved; disabled selections fall back; metrics errors do not erase options; loading/error layouts are stable.
  **QA Scenarios**:
  ```
  Scenario: Loading state blocks stale generation
    Tool: Playwright
    Preconditions: Catalog request is pending.
    Steps: Click generate; assert loading state and zero fake upstream calls; resolve catalog.
    Expected Result: Requests wait for a valid catalog.
    Evidence: .omo/evidence/task-19-loading-guard.png
  Scenario: Disabled model disappears after refresh
    Tool: Playwright
    Preconditions: Selected model becomes disabled server-side.
    Steps: Refresh catalog; assert fallback selection and disabled submission.
    Expected Result: UI and server guards agree.
    Evidence: .omo/evidence/task-19-disabled-refresh.png
  ```

- [x] 20. Add backend integration tests for schema, auth, routing, credits, sync, and metrics

  **What to do**: Add tests-after coverage using fake servers for Channel/ChannelModel CRUD, SuperAdmin authorization, secret redaction, model sync failure preservation, same-name routing, pre-credit validation, global pricing, channel-aware logs, metrics hours/mapping, and tenant isolation.

  **Recommended Agent Profile**: `unspecified-high`; backend integration QA.
  **Parallelization**: Wave 4a; blocked by 8-11; blocks final wave.
  **References**: `backend/service/generate_service_test.go`; `backend/service/model_call_log_service_test.go`; `backend/service/channel_status_service_test.go`; `backend/handler/api_config_handler_test.go`; Tasks 2-11.
  **Acceptance Criteria**: `cd backend && go test ./...` passes; tests prove no raw keys, no cross-tenant reads, no credit spend for invalid identity, and correct same-name channel routing.
  **QA Scenarios**:
  ```
  Scenario: Backend suite passes
    Tool: Bash
    Preconditions: Full backend dependencies available.
    Steps: Run `cd backend && go test ./...`; capture output and exit code.
    Expected Result: Zero test failures.
    Evidence: .omo/evidence/task-20-backend-tests.txt
  Scenario: Tenant isolation fails closed
    Tool: Go integration test
    Preconditions: Two tenants and global channels exist.
    Steps: Request user catalog/metrics under each tenant; assert no private data crosses boundaries.
    Expected Result: Authenticated user reads are correctly scoped.
    Evidence: .omo/evidence/task-20-isolation.txt
  ```

- [x] 21. Add frontend store/API/option tests or minimal test setup

  **What to do**: Test canonical option identity, four capability selections, rate join/sort, unavailable metrics, stale fallback, no-secret persistence, and request payload identity. Add only the smallest justified runner setup because `web/package.json` has no test script.

  **Recommended Agent Profile**: `quick`; focused frontend behavior.
  **Parallelization**: Wave 4a; blocked by 15-19; blocks final wave.
  **References**: `web/package.json:6-12`; `web/src/stores/use-config-store.ts`; `web/src/components/model-picker.tsx`; Tasks 5,12,15-19.
  **Acceptance Criteria**: Tests pass through the chosen runner or equivalent deterministic browser/API QA covers every untested behavior; no same-name option collision or secret persistence remains.
  **QA Scenarios**:
  ```
  Scenario: Frontend tests pass
    Tool: Bash
    Preconditions: Test setup exists.
    Steps: Run the repository-defined frontend test command and `npm run format:check`.
    Expected Result: Zero test/format failures.
    Evidence: .omo/evidence/task-21-frontend-tests.txt
  Scenario: Same-name model options remain distinct
    Tool: Frontend test
    Preconditions: A/B expose the same raw model.
    Steps: Build options and inspect persisted state.
    Expected Result: Encoded identity differs and no API key is persisted.
    Evidence: .omo/evidence/task-21-option-collision.txt
  ```

- [~] 22. Execute SuperAdmin browser workflow QA

  **Blocked**: Requires running MySQL/Redis + configured `.env` with `INIT_ADMIN_USERNAME`/`INIT_ADMIN_PASSWORD`. Backend also needs fake upstream and metrics servers. Not available in this environment.

- [~] 23. Execute user per-capability routing and selector QA

  **Blocked**: Same infrastructure dependencies as T22 - requires running backend with MySQL/Redis, fake upstreams, and configured SuperAdmin/User sessions.

- [x] 24. Update database/API/progress docs and review rollout scope

  **Done**: Updated `backend-database.mdx` (added channel_id/channel_model_id to model_call_logs), `todo.mdx` (added SuperAdmin channel management + channel-status items), and `pending-test.mdx` (added 7 channel-related test items). Docs match current implementation state.

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  - Verify all Must Have/Must NOT Have rules, Channel/ChannelModel identity, SuperAdmin authorization, server-side routing, metrics mapping, evidence files, and no unrelated dirty-file changes.
  - Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`.

- [x] F2. **Build/Test/Quality Review** — `unspecified-high`
  - Run `go vet`, `go test ./...`, frontend format check/build, and inspect changed files for raw secrets, unsafe URLs, envelope drift, unhandled errors, and route identity collisions.
  - Output: `Backend [PASS/FAIL] | Frontend [PASS/FAIL] | Tests [N pass/N fail] | VERDICT`.

- [~] F3. **Real API/Browser QA**
  - **Blocked**: Same infrastructure dependencies as T22/T23 - requires running MySQL/Redis, fake upstreams, and configured SuperAdmin/User sessions.

- [x] F4. **Security/Scope Fidelity Review** — `deep`
  - Verify channel credentials never reach clients, only SuperAdmin writes, no cross-tenant data leakage, metrics URL is server-only, pricing remains global, and feature-related replacement does not touch unrelated worktree changes.
  - Output: `Auth [PASS/FAIL] | Secrets [PASS/FAIL] | Isolation [PASS/FAIL] | Scope [CLEAN/ISSUES] | VERDICT`.

## Commit Strategy

- `feat(channels): add global channel and channel model tables`
- `feat(channels): add superadmin catalog and sync APIs`
- `feat(proxy): validate selected channel and model server-side`
- `feat(metrics): add new-api metrics adapter and recommendations`
- `feat(web): add channel-aware model selection`
- `test(channels): cover routing catalogs and metrics`
- `docs(channels): document data model and API behavior`

Do not stage or commit unrelated existing user changes. Before each commit, inspect `git diff -- <intended files>` and `git status --short`.

## Success Criteria

```text
Backend: gofmt validation, go vet ./..., go test ./...
Frontend: npm run format:check and npm run build pass
Behavior: Channel and ChannelModel are the only source of channel-specific model identity; selected capability channels route correctly
Metrics: backend-only new-api calls honor bounded hours; rates distinguish unavailable from zero and drive selector recommendation/sorting
Security: API keys remain server-side/encrypted; only SuperAdmin can mutate global channels
Docs: docs/content/docs/backend/backend-database.mdx and the appropriate API/progress docs reflect the final design
Scope: unrelated dirty-worktree changes remain untouched
```
