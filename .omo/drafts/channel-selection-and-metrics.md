# Draft: Complete Repository Reaudit - Channels and Performance Metrics

## Requirements (confirmed)
- Add a global shared channel-selection interface to Infinite Canvas.
- All authenticated users can see globally enabled channels.
- Only SuperAdmin manages global channels, per-channel API keys, model synchronization, and model enablement.
- Configure API key per channel and automatically fetch each upstream channel's supported models.
- Allow channel-specific model enable/disable state; pricing remains global by raw model name.
- Image, video, text, and audio can each select a different channel.
- Every model selector displays the selected channel/model success rate.
- Backend proxies new-api performance metrics; browser does not call new-api directly.
- New-api metrics use a separately configured management base URL, not channel upstream URLs.
- Metrics endpoint: `GET /api/perf-metrics/channels?hours=N`; admins choose the hours window.
- Synchronize models automatically on save and support explicit refresh/retry.
- If synchronization fails, preserve the previous model list and report the failure.
- Automated tests are tests-after.

## Complete Repository Facts
- Full repository is `F:\Code\HMG\remote-infinite-canvas\infinite-canvas`.
- Root includes `Dockerfile`, `docker-compose.yml`, `.env.example`, `backend/`, `web/`, `docs/`, `scripts/`, CI, and complete source/config.
- Root `AGENTS.md` is authoritative and says the project is not yet launched; do not add old-data compatibility/migration fallback unless explicitly requested.
- Root instructions require new tables to update `docs/content/docs/backend/backend-database.mdx` (the authoritative exact path in the current root `AGENTS.md`), backend handler/service/repository layering, `{code,data,msg}` responses, frontend API clients under `web/src/services/api/`, and global state under `web/src/stores/`.
- Frontend commands are defined in `web/package.json`: `npm run dev`, `npm run build`, `npm run format`, `npm run format:check`; dependencies include Next 16, React 19, Ant Design 6, Zustand 5, TypeScript 5, and no test runner is currently declared.
- Backend module is Go 1.23 with Gin, GORM/MySQL, JWT, and x/crypto.
- The working tree is not clean: `git status` shows 61 changed/untracked areas, including backend API config, model-call logs, channel-status service/handler/tests, generate/proxy flow, frontend API config, Zustand config, image/video pages/services, and a new channel-status page.
- The current diff already adds `ModelCallLog`, channel-status aggregation, model health/log routes, video/image route metadata, and extra backend config. These are likely in-progress user changes and must be preserved; they cannot be treated as untouched baseline.
- User clarified that the original/in-progress design may be fully overturned because it does not work well; existing changes are reference material only, not an implementation contract.
- User clarified: preserve existing authentication and credit systems; redesign the channel/model/metrics feature area.
- Global channels and SuperAdmin management remain hard requirements.
- Feature-related uncommitted changes may be replaced; unrelated user changes must remain untouched.
- Previous assumptions about independent capability channels, channel-specific enablement, global pricing, and success-rate/hours behavior are not automatically hard requirements until re-confirmed.

## Research Findings
- Complete repository audit is sufficient for planning: API config remains tenant-scoped, current channel status is local-log based and hardcodes tenant 0, frontend model picker lacks channel/rate metadata, and feature-related dirty changes are replaceable reference material.

## Current Design Audit: Confirmed Failure Sources
- Backend `TenantApiConfig` is still one row per tenant with one `BaseUrl`/`ApiKey`; it cannot represent globally shared SuperAdmin-managed channels.
- `ApiConfigHandler.Get/Save/Catalog` still scopes configuration and pricing through `claims.TenantID`, so the current admin/API contract is tenant-scoped rather than global.
- `ModelCallLog` records tenant/user/generation/raw model but no application channel identity, so the current `ChannelStatusService` cannot distinguish the same model across channels.
- `ChannelStatusHandler.GetChannelStatus` calls `GetChannelStatus(0, days)` and returns raw JSON instead of the authenticated `{code,data,msg}` contract; this hardcodes a fake tenant and is not a valid global/new-api metrics adapter.
- Current `ChannelStatusService` calculates local log uptime by generation+model and defaults no records to 100%; it does not consume new-api `/api/perf-metrics/channels?hours=N`, does not map new-api channel IDs, and may report healthy results for models with no observations.
- Frontend `channel-status/page.tsx` calls `/backend-api/channel-status?days=...` directly with axios, bypasses the shared API client/envelope/auth conventions, and displays a local model-health dashboard rather than channel/model success-rate data from new-api.
- Frontend `ModelChannel` data is held in the persisted client store with API keys, despite the current product direction that logged-in users use server-managed upstream configuration; this duplicates authority and risks routing the selected model through a different source than the backend.
- `encodeChannelModel`/`modelOptionName` strips channel identity before route metadata lookup: `modelRoutes` are keyed by raw model, so two channels exposing the same model can collide in route/duration/customizable configuration.
- `selectableModelsByCapability` returns global capability arrays and `ModelPicker` has no success-rate metadata or channel filter; the visible picker therefore cannot guarantee that the displayed option, rate, route metadata, and request credentials refer to the same channel.
- Capability-specific request functions resolve a channel from client-local config, while authenticated backend proxy routing resolves the tenant API config independently; a client-selected channel is not transmitted as a server-validated channel identity.
- The existing admin page models enablement through pricing validity and one tenant catalog; it does not provide a separate global channel catalog with channel-specific enablement.
- Existing in-progress files are therefore reference material, not safe incremental foundations: the redesign should establish one canonical channel/model identity and make every catalog, request, log, metric, and selector consume it.

## Technical Decisions
- Remove the old plan's compatibility-migration requirement; the repo instructions explicitly allow direct new design because the project is not launched.
- Existing uncommitted implementation may be replaced or refactored; use it only to learn domain vocabulary and known failure modes.
- Preserve existing response-envelope and tenant/auth/credit semantics unless the redesigned requirements explicitly replace them.
- Application channel ID is independent from new-api `channel_id`; use an explicit optional mapping for metrics unless the redesign changes this.
- Keep raw API keys out of API responses, frontend user state, and logs.
- Re-evaluate the previously confirmed channel/pricing/metrics decisions if they were part of the failed design rather than hard requirements.
- User selected an independent relational `ChannelModel` table instead of JSON model lists.
- `Channel` owns global upstream identity/credentials and enablement; `ChannelModel` owns channel-specific model identity, capabilities, enabled state, route metadata, and synchronization data.
- Keep global pricing by raw model name unless later repository evidence requires a separate pricing key.

## Decisions and Defaults
- No blocking business questions remain.
- Global channels and SuperAdmin-only management remain non-negotiable.
- Per-capability channel selection, global pricing, separate new-api metrics URL, selectable `hours`, and success-rate sorting/recommendation remain in scope.
- Application channels map to new-api metrics by an explicit nullable mapping.
- Invalid `hours` values are clamped to a bounded range with default 24.
- Stale/unavailable metrics remain visible as unavailable and never block generation.
- Disabled channels/models are rejected server-side before credit spend.
- Same raw model names are distinct by `(channel_id, model_name)`.
- Feature-related dirty files may be replaced while unrelated dirty files remain untouched.

## Scope Boundaries
- INCLUDE: global channels, encrypted keys, model discovery, channel-specific enabled catalogs, independent per-capability channel selection, backend metrics proxy with hours, success-rate labels and sorting/recommendation across all selectors, tests and `docs/content/docs/backend/backend-database.mdx` updates required by repository conventions.
- EXCLUDE: unrelated canvas, billing, auth, or provider-route redesign except where channel context must flow through existing behavior.
- EXCLUDE: backward compatibility for pre-existing database rows unless required by current runtime behavior or explicitly requested.

## Test Strategy Decision
- Infrastructure status: Go tests exist; frontend package has build/format scripts but no test runner dependency. The repository's “do not build” note is a default workflow preference, overridden here by the user's explicit tests-after requirement; the plan will keep verification focused and add only the smallest justified frontend test setup.
- Automated tests: tests-after.
- Agent-executed QA: required for backend APIs and frontend workflows.
