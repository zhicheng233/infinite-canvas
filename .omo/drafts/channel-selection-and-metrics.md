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
- Root instructions require new tables to update `docs/backend-database.md`, backend handler/service/repository layering, `{code,data,msg}` responses, frontend API clients under `web/src/services/api/`, and global state under `web/src/stores/`.
- Frontend commands are defined in `web/package.json`: `npm run dev`, `npm run build`, `npm run format`, `npm run format:check`; dependencies include Next 16, React 19, Ant Design 6, Zustand 5, TypeScript 5, and no test runner is currently declared.
- Backend module is Go 1.23 with Gin, GORM/MySQL, JWT, and x/crypto.
- The working tree is not clean: `git status` shows 61 changed/untracked areas, including backend API config, model-call logs, channel-status service/handler/tests, generate/proxy flow, frontend API config, Zustand config, image/video pages/services, and a new channel-status page.
- The current diff already adds `ModelCallLog`, channel-status aggregation, model health/log routes, video/image route metadata, and extra backend config. These are likely in-progress user changes and must be preserved; they cannot be treated as untouched baseline.

## Research Findings
- Pending complete backend audit: API config, auth/role boundaries, proxy/routing, pricing, model logs, existing channel support, migrations, and runnable tests.
- Pending complete frontend audit: all selector call sites, store/config migration, admin/API surfaces, auth, metrics/status UI, and test/build setup.
- Pending complete test/spec audit: CI, scripts, lockfiles, test files, SDD frameworks, and exact baseline commands.

## Technical Decisions
- Remove the old plan's compatibility-migration requirement; the repo instructions explicitly allow direct new design because the project is not launched.
- Preserve existing response-envelope and tenant/auth/credit semantics unless the complete code audit shows a narrower extension point.
- Application channel ID is independent from new-api `channel_id`; use an explicit optional mapping for metrics.
- Keep raw API keys out of API responses, frontend user state, and logs.

## Open Questions
- Are the existing uncommitted changes the user's current implementation of this feature/related prerequisites, and should the next plan cover only the remaining gaps rather than re-plan those files?

## Scope Boundaries
- INCLUDE: global channels, encrypted keys, model discovery, channel-specific enabled catalogs, independent per-capability channel selection, backend metrics proxy with hours, success-rate labels across all selectors, tests and database/API docs required by repository conventions.
- EXCLUDE: unrelated canvas, billing, auth, or provider-route redesign except where channel context must flow through existing behavior.
- EXCLUDE: backward compatibility for pre-existing database rows unless required by current runtime behavior or explicitly requested.

## Test Strategy Decision
- Infrastructure status: pending complete audit.
- Automated tests: tests-after.
- Agent-executed QA: required for backend APIs and frontend workflows.
