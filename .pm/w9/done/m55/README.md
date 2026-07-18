# w9 · m55 — Dead-id detail URLs redirect home

**Worker:** worker9 **Goal:** a dashboard URL naming a resource that does not exist (e.g. `https://dashboard.bex.co/databases/dadsfasd`) never strands the user on an inline "Not found" block — the app redirects to `/` (or the nearest live parent for nested ids), on every id-bearing detail route. **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on                  |
| ---- | ------------------------------------------------------------------------------------ | --- | --------------------------- |
| t001 | Shared redirect-on-not-found mechanism (loader redirect + client hook + toast)       | 40m | — — **DONE**                |
| t002 | Apply to datastore/simple detail routes: databases, keyvalue, blueprints, env-groups | 30m | t001 — **DONE**             |
| t003 | Apply to shell routes: services layout, project, webhook, workspace alias            | 40m | t001 — **DONE**             |
| t004 | Nested deploy detail: dead deploy id redirects to the service's Deploys tab          | 20m | t001 — **DONE**             |
| t005 | Render parity: compare dead-id behavior with Render's dashboard                      | 20m | t002, t003, t004 — **DONE** |
| t006 | Simplify: `/simplify` over the changed code                                          | 20m | t005 — **DONE**             |
| t007 | Test coverage: redirect vs stay-put (error) branches per route family                | 40m | t005 — **DONE**             |
| t008 | Closeout                                                                             | 10m | t006, t007 — **DONE**       |

## Implementation notes (2026-07-17, closeout)

- **Mechanism shipped as a client hook only** (`dashboard/src/common/hooks/use-not-found-redirect.ts`), not the loader-`redirect()` variant t001 also sketched: an SSR 302 cannot carry the toast, and the loader's `loadRouteResource` not-found state already drives the head title, so the hook (fired on hydration/settle) is the single uniform path — it covers direct visits, client navigations, and post-mount deletions alike, always with the toast. Verified live via `yarn dev:local` + Playwright: `/databases/dadsfasd`, `/keyvalue/red-garbage`, `/project/prj-garbage`, `/services/no-such-svc/logs`, `/w/tea-garbage/settings`, and the alias shim `/d/dpg-garbage` all land on `/` with the toast; `/services/eden-cms-v2/deploys/dep-garbage` lands on that service's Deploys tab.
- Routes that previously conflated a failed query with not-found (databases, keyvalue, blueprints) gained a real inline error state (`dashboard/src/common/components/resource-load-error.tsx`) so outages stay put instead of redirecting; services/webhook/env-groups/project keep their existing error branches. 16 orphaned `*.notFound*` locale keys removed across en+zh.
- **t005 Render parity:** the milestone diff is `dashboard/` + `.pm/` only — no `lego/` (REST/GraphQL/MCP error dialects untouched). Render's own authenticated dashboard behavior for a dead resource URL is **unverified** (no live probe available; `docs/render-artifacts/dashboard-routes.md` captures unmatched _routes_ as 404s but not dead _ids_ on live routes). bex's redirect-home-with-toast is a user-decided behavior (2026-07-17 request), recorded here as deliberate; no ADR018 row exists for dashboard dead-id handling, so no ledger edit.

## Definition of done

Visiting each of these with a garbage id while logged in lands on `/` (with a "not found" toast), never an inline dead-end: `/databases/<x>`, `/keyvalue/<x>`, `/blueprints/<x>`, `/env-groups/<x>`, `/services/<x>` (any tab), `/project/<x>`, `/webhook/<x>`, `/w/<garbage-tea-id>`. `/services/<live>/deploys/<garbage>` lands on that service's Deploys tab. A backend/query **error** (outage, 401 drift) still shows the existing inline retry/error UI — it must NOT redirect (a flaky backend must not bounce users home). The Render-alias shims (`/d/…`, `/r/…`, `/web/…`, …) inherit the behavior via their canonical targets. `yarn typecheck && yarn lint && yarn test` green; vitest covers redirect-on-not-found and stay-on-error for the shared helper plus at least one route per family.

## Source + Goal linkage

- **Source:** user request 2026-07-17 (`/pm for w9`): "if a resource does not exist, like https://dashboard.bex.co/databases/dadsfasd, redirect user to the /. check all of such places." Route survey 2026-07-17: every id-bearing detail route settles to an _inline_ not-found block via the shared `loadRouteResource` `ready|not-found|error` pattern (`dashboard/src/common/lib/document-head/index.ts:106`); none redirect, and the not-found copy/affordances vary per route (some have no navigation affordance at all).
- **Goal linkage:** deploy-experience polish (w9's theme) — dead links from stale bookmarks, deleted resources, or mistyped CLI-derived URLs recover to a useful page instead of a dead end; also converges the eight per-route not-found blocks onto one shared behavior.
- **Expected outcome:** observable per the DoD — garbage-id URLs land on `/` with a toast; error states unchanged.
- **Why now:** every detail route already flows through the one `loadRouteResource` seam, so the change is cheap and mechanical today; each new detail route added before this ships would copy the inline-block pattern and widen the migration. Render parity task included: this is a user-facing UI surface (REST/GraphQL/MCP are untouched — the parity check is dashboard-vs-Render-dashboard behavior only).
