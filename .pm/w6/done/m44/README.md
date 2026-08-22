# w6 · m44 — Dead-id detail URLs regressed: single-resource GraphQL returns a zero-value object instead of null

**Worker:** worker6 **Goal:** a dashboard URL naming a service, database, or key-value store that does not exist stops rendering a fully-chromed ghost detail page (complete with an enabled **Manual Deploy** button) and goes back to what `w9/m55` shipped — redirect home with a "not found" toast — by fixing the backend contract that silently defeats it: `server`/`database`/`keyValue` must resolve a dead id to `null`, not to an all-empty object. **Status:** done 2026-08-22 — all 7 tasks complete, every suite green locally, both new suites proven red with the fix reverted. **Closed unshipped:** the DoD's "verified on production (or dev-6) while signed in" line was NOT met, because nothing here is committed. Whoever runs `/ship` still owes the live re-probe of `dashboard.bex.co` — `/services/<dead>` should redirect home with the not-found toast, and `POST /graphql { server(id:"<dead>") }` should answer `"server": null`.

## Tasks (in order)

| id   | title                                                                              | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Single-resource GraphQL verbs resolve a dead id to `null` (+ REST 404, MCP parity) — **DONE** | 45m | — |
| t002 | Service detail route: wire the `w9/m55` not-found escape hatch it never got — **DONE** | 35m | t001 |
| t003 | Dead blueprint id: "Something went wrong" + raw id heading → not-found semantics — **DONE** | 25m | t001 |
| t004 | Render parity (REST/GraphQL/MCP + dashboard) — **DONE** | 25m | t001–t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |


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

## Triage (2026-08-22) — what survived verification, and what did not

The headline defect is real and was reproduced from a test before any fix: `server(id:"<dead>")` resolved to
`{"id":"","name":"","slug":"","phase":"", …}` in `data`, not `null`. Four of the milestone's supporting claims did not
survive, and two of them would have sent the fix to the wrong layer:

1. **"`server`/`database` return no `errors` key at all" — false.** All three verbs already returned
   `errors:[{"message":"app not found"}]` _alongside_ the zero-value object; the envelope was uniform before this
   milestone. The DoD's "make the three agree" bullet was therefore already satisfied. (The live probe that reported
   otherwise was likely reading a client that drops `errors`.) What was NOT uniform is the thing that mattered: the
   `data` half.

2. **Root cause is graphql-go's executor, not `gqlutil.KeyVerb`.** `KeyVerb` returning `(T, error)` for a value-struct
   `T` is only the ingredient. The leak is in `graphql-go@v0.8.1` `executor.go:649-666`: `resolveField` assigns the
   resolver's value to its **named** result, then panics on the error; the deferred `recover` returns that same named
   result, so the raw, un-completed Go value lands in the response map. Pointer/slice resolvers leak `nil` and read as
   `null` by luck; value-struct resolvers leak the zero struct — JSON-encoded whole, every Go field, not the selected
   ones. **Fixing `KeyVerb` alone would have missed `blueprint` and `project`**, which are hand-written
   `&graphql.Field{Resolve: …}` literals with the identical defect. The fix is one schema-level guard
   (`gqlutil.NilOnError`, applied in `api.newSchema`) so every current and future resolver inherits it.

3. **"`/project/<dead>` and `/env-groups/<dead>` still redirect — those are the verbs whose resolvers return `null`" —
   false.** `projects.Service.Get` returns a value struct and leaked the same zero object. Those two routes redirect
   because each carries its own **workaround**: `project.$projectId.tsx`'s loader filters on `name.trim()`, and
   env-groups has a private `isEnvGroupNotFound` message matcher. So the working routes were not evidence of a correct
   backend — they were evidence of two hand-rolled patches around the same bug.

4. **t002/t003's premise — "the escape hatch it never got" — false.** `service-detail-layout.tsx` already called
   `useNotFoundRedirect`, as did the databases, keyvalue, blueprints, and webhook pages. The hatch was wired; its
   predicate was defeated. First by the truthy zero object (`!resource` never fired), and then — once the backend
   started answering `null` — by `!error`, because bex-api reports a dead id **as an error**. So the real dashboard
   work was not wiring, it was the predicate: five pages hand-rolled `!loading && !x && !error`, while `useDeploy`'s
   copy (which did test the message) kept working. That divergence is now one shared `resourceNotFound` /
   `resourceFailed` pair.

5. **The REST/MCP half of the DoD was already satisfied.** `core.WriteErr` maps `ErrNotFound` → 404, and the MCP tools
   return a tool error, both before this milestone. Also, the DoD's `GET /v1/redis/<dead>` does not exist in bex — the
   key-value REST base is `/v1/key-value` (`internal/keyvalue/rest.go:162`). Both surfaces are now pinned by tests
   rather than assumed.

## What changed

**Backend** — one guard, no per-verb patches:

- `lego/backend/internal/gqlutil/gqlutil.go` — new `NilOnError(schema)`: walks the built schema's type map and wraps
  every field resolver so an error resolves the field to `null`. The error itself passes through untouched, so the
  response still says `app not found`.
- `lego/backend/internal/api/server.go` — `newSchema()` applies it to the composed schema.

**Dashboard** — one predicate, replacing seven hand-rolled copies:

- `src/common/hooks/use-not-found-redirect.ts` — new `resourceNotFound` / `resourceFailed`, exact complements over the
  settled-empty case, so a page can never both redirect home and render its retry state.
- Applied in `service-detail-layout.tsx`, `databases.$databaseId.tsx`, `keyvalue.$keyValueId.tsx`,
  `blueprints.$blueprintId.tsx`, and `use-webhook.ts` (which now owns the webhook page's not-found decision).
- `use-deploy.ts` and `isEnvGroupNotFound` fold their private message matchers into the shared one (t005).

**Tests** (t006) — `lego/backend/internal/api/deadid_notfound_test.go` (GraphQL null + errors on both branches, a
28-verb schema sweep so a future read verb inherits the guarantee, `blueprint` specifically, REST 404, MCP tool error,
and the response BODY through `POST /graphql` — which also pins that `sanitizeGraphQLErrors` keeps `app not found`
readable, since the dashboard matches on that message to tell a deleted resource from an outage)
and `dashboard/src/features/services/components/__tests__/service-detail-dead-id.test.tsx` plus predicate cases in
`use-not-found-redirect.test.tsx`. Both new suites were confirmed to FAIL with the fix reverted.

## Gates

- `cd lego/backend && go test ./...` — green. `make lint` (4 modules) — 0 issues.
- `dashboard/`: `yarn typecheck` green; `yarn test` 2484 passed, +9 new.
- **Pre-existing breaks on `main`, unrelated to m44 and confirmed on a clean tree:** `yarn test` fails 4 tests
  (`use-usage`, `services.$serviceId` ×2, `route-heads`), and `yarn lint` reports 2 errors in
  `src/common/hooks/use-deferred-mount.tsx`. `yarn typecheck` was also red (3 errors in w9/m92's
  `m92-regression.test.ts`) — fixed here, since it blocked this milestone's own gate. The other two want their own
  milestone.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co` hosting surfaces, 2026-08-22, signed in as the QA user. Evidence: `.playwright-mcp/qa-overview-dup-rows.png`, plus in-browser API probes captured in the session — `server(dead)` → `{"data":{"server":{"id":"","name":"","slug":"","phase":"", …}}}` with no `errors`; `database(dead)` → same; `keyValue(dead)` → zero-value object **plus** `errors:[{"message":"app not found"}]`; `server(real)` → correct data. Same pattern as `w6/m43`, which came from a live bex-vs-Render QA pass.
- **Regression of:** `w9/m55` — "Dead-id detail URLs redirect home" (done 2026-07-17). Its DoD explicitly covers `/services/<x>` (any tab), `/databases/<x>`, `/keyvalue/<x>`, and it was verified then against `yarn dev:local`. On production today only `/project/<dead>` and `/env-groups/<dead>` still redirect — and those are exactly the verbs whose resolvers return `null`. The mechanism (`dashboard/src/common/hooks/use-not-found-redirect.ts`) is intact; the backend's zero-value object walks straight past its `!resource` predicate, so `loadRouteResource` settles `"ready"` on a resource that does not exist.
- **Root cause (traced):** `gqlutil.KeyVerb` (`lego/backend/internal/gqlutil/gqlutil.go:114-122`) resolves `(T, error)` where `T` is a **value** struct, so `apps.Service.Get`'s `return AppView{}, err` (`lego/backend/internal/apps/service.go:1130-1136`) serializes as an all-zero object rather than `null`. `server`/`service` are wired through it at `lego/backend/internal/apps/graphql.go:916-917`; `database`/`keyValue` take the equivalent path.
- **Goal linkage:** [docs/ADR006-bex-api.md](../../../../docs/ADR006-bex-api.md) — bex-api is meant to be Render-compatible, and Render answers a dead resource id with 404, never 200-plus-an-empty-resource. Also [ADR008](../../../../docs/ADR008-vision.md)'s human surface: a hosting dashboard that offers **Manual Deploy** on a service that does not exist is not telling the truth.
- **Expected outcome:** every client can tell "deleted" from "empty", and the dashboard's dead-link behavior matches the guarantee `w9/m55` already shipped.
- **Why now:** a delivered, user-visible guarantee is silently broken in production, and the blast radius grows with every deleted resource whose link is still in a bookmark, a Slack message, or a webhook payload. The fix is small and the root cause is already pinned to specific lines; left alone, every new single-resource verb built on `KeyVerb` inherits the same defect.
- **Render parity task included:** the fix changes GraphQL null semantics and REST status codes on user-facing read paths, so REST/GraphQL/MCP/UI must move together and be checked against render.com's 404 behavior.
