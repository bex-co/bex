# w2 · m612 — Cross-surface service creation from Build Source and linked-service deploy-by-`buildId`

**Worker:** worker2 **Goal:** Wire `buildSourceId` into GraphQL `createService`, MCP `create_web_service`/`create_cron_job`, and Dashboard `/services/new`, and upgrade linked-service Manual Deploy and Deploy Detail so users can deploy the latest build or pick a specific `buildId` (`bld-…`) and trace build provenance cleanly. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add `buildSourceId` argument to GraphQL `createService` in `apps/graphql.go` (validating mutual exclusion with `repo`/`image`, region match, and native start-command rules via shared `apps.Service`) | 35m | — |
| t002 | Add optional `buildSourceId` to MCP `create_web_service` and `create_cron_job` in `apps/mcp.go` (preserving `mcpParitySuperset` / `TestMCPParity*`) with unit tests | 35m | t001 |
| t003 | Wire `buildSourceId` into Dashboard `/services/new` (`web`, `pserv`, `worker`, `cron`) via `ServiceSourcePicker` (`buildSource` tab), adapting form fields for native vs Docker/image sources | 1h | t001 |
| t004 | Wire `buildId` through Dashboard `events.graphql` (`triggerDeploy`), GraphQL codegen, and `useTriggerDeploy`, and upgrade `ManualDeployButton` for Build-Source-linked services ("Deploy latest build" + "Deploy a specific build…") | 1h5m | t001 |
| t005 | Surface `buildSourceId` and `buildId` provenance on linked-service Deploy Detail (`deploy-detail-page.tsx`) and `ServiceSourceCard` | 35m | t004 |
| t006 | Update all 8 dashboard locale files (`en`, `zh`, `de`, `es`, `ja`, `ko`, `pt`, `ru`) and add backend + Vitest coverage for create-from-build-source and deploy-by-`buildId` | 40m | t003, t004, t005 |
| t007 | Render parity | 20m | t002, t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- Creating a `web_service`, `private_service`, `background_worker`, or `cron_job` linked to an existing Build Source via `buildSourceId` works identically across REST (`POST /v1/services`), GraphQL (`createService`), MCP (`create_web_service`, `create_cron_job`), and Dashboard `/services/new` — enforcing Render's mutual exclusion (`repo`/`branch`/`image`/`buildFilter`/`runtime`), git-source region match (`build_source_region_mismatch`), and native-runtime `startCommand` rules.
- On any Build-Source-linked service in the Dashboard, the header Manual Deploy menu shows **"Deploy latest build"** and **"Deploy a specific build…"** (never "Deploy latest image"), lets the user select a succeeded `bld-…` build from that Build Source (with protected-environment confirmation when applicable), and navigates to the created deploy.
- Deploy Detail for a linked service clearly displays its `buildId` (`bld-…`) and Build Source (`bds-…` / name) provenance.
- `cd lego/backend && go test ./...`, `make lint`, and `cd dashboard && yarn lint && yarn test` pass, and all 8 dashboard locales have zero missing keys.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w2` (2026-11-23), materialized via `/pmx w2`. Verified at `0f9166bc7f`: `apps/graphql.go:1537-1585` (`createService`) and `apps/mcp.go:194-410` (`create_web_service`/`create_cron_job`) omit `buildSourceId`; `dashboard/src/features/services/api/events.graphql:203` + `use-trigger-deploy.ts` omit `buildId` even though backend `deploys/graphql.go:239` already accepts it; `manual-deploy-button.tsx:85-95` treats `service.repo == ""` as image-backed ("Deploy latest image").
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and one-Core-three-adapters ([ADR006](../../../docs/ADR006-bex-api.md)) — closes the cross-surface create gap and fixes misleading UX on every Build-Source-linked service.
- **Expected outcome:** Users and agents can create services directly from Build Sources on every surface and manually deploy or roll back to any succeeded `bld-…` from the Dashboard.
- **Why now:** Backend `Create` (`BuildSourceID`) and `triggerDeploy(buildId)` already exist; wiring the remaining surfaces unlocks daily linked-service workflows immediately.
- **Render parity:** Included — updates GraphQL, MCP, and Dashboard service-creation and manual-deploy surfaces.
- **Brainstorm recommendations, not user decisions:** Both `create_web_service` and `create_cron_job` are already in `mcpParitySuperset` (`lego/backend/internal/api/mcp_parity_test.go:262`), so adding optional `buildSourceId` preserves `TestMCPParityInventory` counts while bringing MCP into parity with REST `POST /v1/services`.
