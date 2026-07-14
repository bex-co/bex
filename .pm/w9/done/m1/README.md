# w9 · m1 — Deploy detail page: Manual Deploy jumps to a per-deploy page with its logs

**Worker:** worker9 **Goal:** Render parity for the deploy moment — clicking **Manual Deploy** (or a rollback/deploy event) lands the user on a per-deploy page (`/services/<id>/deploys/<deployId>`, Render's `/web/srv-…/deploys/dep-…`) showing that deploy's status header and its logs (build → pre-deploy → runtime), live-following while the deploy is in flight. **Status:** done

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | GraphQL single-deploy read: `deploy(serviceId, deployId)` | 30m | — | — **DONE** |
| t002 | GraphQL logs time window: `startTime`/`endTime` args (REST/MCP parity) | 30m | — | — **DONE** |
| t003 | Dashboard deploy detail page: header + deploy-scoped log viewer | 60m | t001, t002 | — **DONE** |
| t004 | Jump on deploy: Manual Deploy / rollback navigate to the deploy page; event rows link | 30m | t003 | — **DONE** |
| t005 | local-bex stub: single deploy + windowed logs so the page runs offline | 30m | t003 | — **DONE** |
| t006 | Render parity check across REST/GraphQL/MCP/UI | 30m | t004, t005 | — **DONE** |
| t007 | Simplify pass over the milestone's diff | 30m | t006 | — **DONE** |
| t008 | Test coverage for the deploy page + new GraphQL args | 45m | t006 | — **DONE** |
| t009 | Closeout | 15m | t008 | — **DONE** |

## Definition of done

From the dashboard, clicking **Manual Deploy** on a service navigates to `/services/<serviceId>/deploys/<deployId>` for the deploy the mutation just created. That page shows (a) a header with the deploy's status badge, trigger, created/started/finished times, image, and pre-deploy outcome, and (b) a log viewer scoped to the deploy's time window carrying `type=build`, `type=predeploy`, and the service's own `type=app` lines, refreshing until the deploy reaches a terminal status. Deploy rows on the Events tab link to their deploy page. `deploy(serviceId, deployId)` and `logs(startTime:, endTime:)` answer over GraphQL exactly as REST/MCP already do (same fields, same error shapes — unknown deployId → not found, malformed time → bad request). The page works offline under `yarn dev:local`. `make lint`, backend `go test ./...`, and dashboard `yarn typecheck && yarn lint && yarn test` all pass.

**Verification (2026-07-14):** `go test ./...` (`lego/backend`) and `yarn typecheck && yarn lint && yarn test` (`dashboard`, 971 tests) both pass; `make lint-backend` is 0 issues (full `make lint` from `lego/operator` has 17 pre-existing issues, none in files this milestone touched — `cmd/pg-sni-proxy`, `internal/controller`, `internal/build`). The full offline flow (`Deploy(serviceId, deployId)` found/not-found/cross-service, `TriggerDeploy` → `update_in_progress` → `live` over 5s, `CancelDeploy`, `RollbackService`, windowed `type=build`/`type=predeploy` logs) was exercised end-to-end against the running `local-bex.mjs` stub over its real GraphQL wire — the exact operations the dashboard's Apollo client issues. A live browser click-through (`yarn dev:local` + Playwright) could not be completed in this session — the shared Playwright browser profile was held by a concurrent session throughout; the dashboard's own component/route tests (manual-deploy-button, deploy-header, use-deploy, use-deploy-logs, services.$serviceId.events) cover the same interactions at the React level instead.

## Source + Goal linkage

- **Source:** user request 2026-07-14 (`/pm for w9`): "render.com feature parity to manual deploy and then it will automatically jump to the deploy page … with deployment logs", anchored on the live Render page `dashboard.render.com/web/srv-cr1aprdds78s739qrbg0/deploys/dep-d9bb06vlk1mc73fgp9pg` (inspected 2026-07-14: header = date + status badge + commit link/message; body = log viewer with search, time-range = the deploy's window, instance-tagged lines, follow-to-bottom; Manual Deploy navigates straight to the new deploy's page).
- **Goal linkage:** Render-parity dashboard (docs/ADR018-render-parity.md — Deploys row is API-only today; docs/ADR006-bex-api.md "one core, thin adapters": the GraphQL gaps t001/t002 close are surface drift). Builds directly on w2/m5 (deploy history + trigger), w2/m10 (cancel/rollback), w7/m28 (build logs in Loki), w1/m33 (pre-deploy logs).
- **Expected outcome:** the deploy moment — the highest-anxiety moment in the product — gets Render's feedback loop: one click, land on the deploy, watch it build and go live. No more triggering a deploy and hunting through the Events tab + Logs tab to see what happened.
- **Why now:** all prerequisites just landed (w2/m30 manual-deploy body, w7/m28 build-log shipping, w1/m33 pre-deploy status + logs); this is the visible payoff that composes them. Render parity is included as t006 because this is feature work touching GraphQL and the dashboard UI.
