# w6 · m44 — Dead-id detail URLs regressed: single-resource GraphQL returns a zero-value object instead of null

**Worker:** worker6 **Goal:** a dashboard URL naming a service, database, or key-value store that does not exist stops rendering a fully-chromed ghost detail page (complete with an enabled **Manual Deploy** button) and goes back to what `w9/m55` shipped — redirect home with a "not found" toast — by fixing the backend contract that silently defeats it: `server`/`database`/`keyValue` must resolve a dead id to `null`, not to an all-empty object. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Single-resource GraphQL verbs resolve a dead id to `null` (+ REST 404, MCP parity) | 45m | —            |
| t002 | Service detail route: wire the `w9/m55` not-found escape hatch it never got        | 35m | t001         |
| t003 | Dead blueprint id: "Something went wrong" + raw id heading → not-found semantics   | 25m | t001         |
| t004 | Render parity (REST/GraphQL/MCP + dashboard)                                       | 25m | t001–t003    |
| t005 | Simplify                                                                            | 20m | t004         |
| t006 | Test coverage                                                                       | 45m | t004         |
| t007 | Closeout                                                                            | 10m | t006         |

## Definition of done

Verified on production (or dev-6) while signed in:

- `POST /graphql { server(id:"<dead>") }` returns `"server": null` — not `{"id":"","name":"","slug":"","phase":"", …}`. Same for `database(id:)` and `keyValue(id:)`. A dead id and a real one are distinguishable by any client (dashboard, CLI, SDK, MCP agent) without string-matching empty fields.
- The three verbs agree with each other on the error envelope: today `keyValue` returns `errors:[{message:"app not found"}]` alongside its zero-value object while `server` and `database` return **no `errors` key at all**. After this milestone, all three take the same shape.
- The REST equivalents (`GET /v1/services/<dead>`, `/v1/postgres/<dead>`, `/v1/redis/<dead>`) return 404 with the Render error body, and the MCP tools surface the same not-found, not an empty resource.
- `https://dashboard.bex.co/services/<dead-id>` no longer renders the service shell — no `SERVICE / Service Unknown` header, no 9 clickable tabs, and no enabled **Connect** / **Manual Deploy** buttons pointed at a resource that does not exist. It redirects to `/` with the `common.resourceNotFoundToast` toast, exactly as `w9/m55`'s DoD requires.
- `/databases/<dead>` and `/keyvalue/<dead>` likewise redirect instead of sitting on a detail shell (measured today: they stay put for 8s+, `document.title` reading "Page not found" while the body renders a database page).
- `/blueprints/<dead>` reports not-found like its siblings instead of `document.title` "Something went wrong ・ bex Dashboard" with the raw `bpt-…` id as its `h1`.
- A genuine backend **error** (outage, auth drift) still keeps the user on the inline error/retry UI — `w9/m55`'s error-vs-not-found distinction must survive this fix, and a test must pin both branches.
- `cd lego/backend && go test ./...` + `make lint` green; `dashboard/`: `yarn typecheck && yarn lint && yarn test` green.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co` hosting surfaces, 2026-08-22, signed in as the QA user. Evidence: `.playwright-mcp/qa-overview-dup-rows.png`, plus in-browser API probes captured in the session — `server(dead)` → `{"data":{"server":{"id":"","name":"","slug":"","phase":"", …}}}` with no `errors`; `database(dead)` → same; `keyValue(dead)` → zero-value object **plus** `errors:[{"message":"app not found"}]`; `server(real)` → correct data. Same pattern as `w6/m43`, which came from a live bex-vs-Render QA pass.
- **Regression of:** `w9/m55` — "Dead-id detail URLs redirect home" (done 2026-07-17). Its DoD explicitly covers `/services/<x>` (any tab), `/databases/<x>`, `/keyvalue/<x>`, and it was verified then against `yarn dev:local`. On production today only `/project/<dead>` and `/env-groups/<dead>` still redirect — and those are exactly the verbs whose resolvers return `null`. The mechanism (`dashboard/src/common/hooks/use-not-found-redirect.ts`) is intact; the backend's zero-value object walks straight past its `!resource` predicate, so `loadRouteResource` settles `"ready"` on a resource that does not exist.
- **Root cause (traced):** `gqlutil.KeyVerb` (`lego/backend/internal/gqlutil/gqlutil.go:114-122`) resolves `(T, error)` where `T` is a **value** struct, so `apps.Service.Get`'s `return AppView{}, err` (`lego/backend/internal/apps/service.go:1130-1136`) serializes as an all-zero object rather than `null`. `server`/`service` are wired through it at `lego/backend/internal/apps/graphql.go:916-917`; `database`/`keyValue` take the equivalent path.
- **Goal linkage:** [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) — bex-api is meant to be Render-compatible, and Render answers a dead resource id with 404, never 200-plus-an-empty-resource. Also [ADR008](../../../docs/ADR008-vision.md)'s human surface: a hosting dashboard that offers **Manual Deploy** on a service that does not exist is not telling the truth.
- **Expected outcome:** every client can tell "deleted" from "empty", and the dashboard's dead-link behavior matches the guarantee `w9/m55` already shipped.
- **Why now:** a delivered, user-visible guarantee is silently broken in production, and the blast radius grows with every deleted resource whose link is still in a bookmark, a Slack message, or a webhook payload. The fix is small and the root cause is already pinned to specific lines; left alone, every new single-resource verb built on `KeyVerb` inherits the same defect.
- **Render parity task included:** the fix changes GraphQL null semantics and REST status codes on user-facing read paths, so REST/GraphQL/MCP/UI must move together and be checked against render.com's 404 behavior.
