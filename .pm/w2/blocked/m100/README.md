# w2 · m100 — REST routes a Render client cannot reach: the shadowed `event-types` route and the missing overrides list

**Worker:** worker2 **Goal:** every REST route bex registers is reachable through the strict Render validator, and every Render list operation a client would use to audit notification overrides is served. `GET /v1/webhooks/event-types` answers with the vocabulary instead of a `400` minted by Render's `{webhookId}` template, and `GET /v1/notification-settings/overrides` returns the caller's workspace overrides in Render's envelope instead of bex's bare `404`. A reverse inventory test keeps the next bex-native literal route from being shadowed. **Status:** t001–t005 done; **t006 closeout BLOCKED** on live production verification (same credential gate as m95/m96 — see the workstream README)

## Tasks (in order)

| id   | title                                                                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | The Render validator passes a bex-native literal route through when it only matches a Render template parameter; a reverse inventory guard fails on any new shadowing — **DONE** | 45m | —          |
| t002 | Serve `GET /v1/notification-settings/overrides` scoped to the caller's workspace with `serviceId` filter and Render's cursor envelope — **DONE** | 30m | —          |
| t003 | Render parity — **DONE**                                                                                                                               | 15m | t001, t002 |
| t004 | Simplify — **DONE**                                                                                                                                    | 15m | t003       |
| t005 | Test coverage — **DONE**                                                                                                                               | 35m | t003       |
| t006 | Closeout                                                                                                                                               | 10m | t005       |

## Render semantics (t002 step 1)

Read from the **pinned spec** (`lego/backend/internal/api/openapi/render-public-api-1.json`, operation `list-notification-overrides`) on 2026-09-15. Render's live docs and a live response could not be fetched — no working production or Render credential this run — so anything not in the pinned spec is marked unverified rather than guessed at silently.

Three corrections to what t002's own Context assumed:

| t002 said | The pinned spec actually says |
| --- | --- |
| items are `notificationOverrideWithCursor` = `{notificationOverride, cursor}` | the envelope key is **`override`**, not `notificationOverride` |
| a `serviceId` query parameter | `serviceIdsParam` — named `serviceId` but **`type: array`, `style: form`**, so it repeats (`?serviceId=a&serviceId=b`) |
| — | the override object **requires a `type` field** that the schema then gives no type, enum or description |

Decisions made against that:

- **`type` is emitted as `"service"`.** It is required-but-unconstrained, so any string validates. `"service"` names what the override is attached to, which is the only reading the surrounding fields support. Flagged unverified in ADR018 — a live capture that contradicts it is a one-line change.
- **A `default` service IS listed.** Render's description ("List notification overrides matching the provided filters") does not say, and no capture was available. bex lists every in-scope service, because the per-service `GET` also always returns a value (`default`/`default`) rather than 404-ing, and because omitting defaults makes an empty list ambiguous between "this workspace has no services" and "no service has been customised". A superset is the house rule for unresolved Render detail (`internal/api/AGENTS.md`).
- **Scope is one workspace, not all of them** — a deliberate divergence. Render says an unfiltered call "returns all notification overrides for all workspaces the user belongs to"; bex routes through the same `s.List(ctx, ownerId)` every other collection uses, so it returns the named workspace (when the caller belongs to it) or the caller's own, and a foreign `ownerId` returns nothing. Matching Render's multi-workspace default would make this the only bex list that crosses a workspace boundary. Recorded in ADR018 row 230.

## Decisions

- **The validator's pass-through is method-aware.** The first cut compared paths only, and the reverse inventory immediately caught it: `POST /v1/blueprints/{deploy,generate,preview}` and `POST /v1/webhooks/{git,stripe}` all sit under paths Render parameterises, so a path-only rule called five extra routes "shadowed" when Render publishes no `POST` at those templates and nothing was shadowing them at all. The rule now requires the Render path item to actually serve the request's method. The false positives were a bug the guard found in its own subject — worth recording, because the same five routes would have been silently exempted from validation.
- **The pass-through requires the spec to have no exact literal path of its own.** If Render itself documents the literal, that operation wins and validation stays on. This is what keeps every genuine intersection validated while the bex-native literal gets through.
- **The reverse inventory scans registration sites, not the mux.** `http.ServeMux` exposes no pattern list, so the guard walks non-test `internal/**/*.go` for `.Handle`/`.HandleFunc("<METHOD> /v1/…")` literals — the same method `w1/089`'s audit used. A pattern assembled from constants or `fmt.Sprintf` is invisible to it; that limit is written on the regex, and the test fails loudly if the scan ever finds implausibly few routes, so it cannot pass vacuously.
- **The guard asserts a known set, not emptiness.** `GET /v1/webhooks/event-types` is a legitimate bex extension wearing a Render-shaped path; the test's job is that no NEW one appears unnoticed.
- **`TestRenderRouteIntersectionInventory` gained exactly one entry**, `list-notification-overrides`, from t002 — bex now genuinely serves that Render operation. t001's own change deliberately leaves the set untouched, which is the check t001's acceptance criteria asked for.

## Simplify pass (t004)

- **Applied:** `bexPatternPath` returns the method alongside the path instead of each caller re-splitting the ServeMux pattern; `shadowsBexLiteral` is a pure function over two path strings, so the rule is unit-testable without a contract, a mux or a request (`TestLiteralPassThroughRuleIsNarrow` covers seven cases including the three ways it must say no).
- **Applied:** the overrides list reuses `toNotificationOverride` — the per-service GET's own projection — rather than reading `app.NotificationsToSend` again, so the list and the per-service read cannot disagree.
- **Declined:** extracting a shared "list + ownerId + filter + StablePage" helper across the sibling list routes. Each filters on a different field with a different named 400, and the shared part is already `s.List` + `core.StablePage`.

## Definition of done

Run each bullet against a composed bex-api server that mounts the strict Render request validator (`lego/backend/internal/api/render_openapi.go`), then repeat the two live bullets against production from a signed-in session.

- **The shadowed route answers.** `GET /v1/webhooks/event-types` returns `200` with the `EventTypes` vocabulary. `GET /v1/webhooks/event-types?ownerId=<tea-id>` returns the same. At filing time both returned `400 {"error":"invalid path parameter \"webhookId\"","id":"bad_request",…}` because the validator matched Render's `GET /webhooks/{webhookId}`.
- **Real intersections are still validated.** `GET /v1/webhooks/whk-<20 chars>` for an unknown id still runs Render validation and returns the canonical `404 {"error":"app not found","id":"not_found",…}`. `GET /v1/webhooks/not-an-id` still returns the validator's `400`. The list in `TestRenderRouteIntersectionInventory` is unchanged.
- **The reverse inventory guard enumerates every registered route.** A test walks every `Handle`/`HandleFunc` pattern under `lego/backend/internal` (183 non-test routes at filing time) against the pinned spec's operations and fails when a same-method Render template matches a bex literal segment with a parameter and Render has no exact literal operation for that path. On `main` before t001 it names exactly one route, `GET /v1/webhooks/event-types`; after t001 it passes and stays as the guard.
- **The overrides list is served.** `GET /v1/notification-settings/overrides` returns `200` with an array of `{notificationOverride: {serviceId, notificationsToSend, previewNotificationsEnabled}, cursor}` entries for the caller's workspace. A service patched to `notificationsToSend: failure` appears with `failure`. Whether a service left at `default` appears is decided in t002 against Render's documented response and recorded in the README. `?serviceId=<srv-id>` narrows to that service. `?cursor`/`?limit` page the way the sibling list routes do. A second workspace's services never appear, with or without `?ownerId=` naming that workspace.
- **Live on production.** `GET https://api.bex.co/v1/webhooks/event-types` → `200`, and `GET https://api.bex.co/v1/notification-settings/overrides?ownerId=<the session's tea-id>` → `200` with the workspace's overrides. The control `GET /v1/notification-settings/overrides/services/<srv-id>` still returns `200` with the per-service shape.

## Source + Goal linkage

- **Source:** `w1/089` (the shadowed `event-types` route, live `/qa-find-bugs` 2026-09-14 pass 13) and `w1/093` (the missing `list-notification-overrides` operation, pass 15), both absorbed into this milestone by `/pm-brainstorm for w1` on 2026-09-15 and materialized in w2 by user direction. Both notes carry the full repro, root cause and blast radius; they are retired to `.pm/w1/done/`.
- **Goal linkage:** pillar 1, Render-compatible REST. `docs/ADR018-render-parity.md` row 227 (Notifications) marks every surface ✅ and names every divergence except the list operation, and row 228 (Outbound event webhooks) does not record that bex's own vocabulary route is unreachable. A Render client (the pinned `render-oss/cli` train, a Terraform provider, any spec-driven client) that audits which services mute or force deploy mail gets an undocumented `404`, and an API client asking bex which webhook events exist gets a `400` about a path parameter it never sent.
- **Expected outcome:** no registered REST route is unreachable, the overrides list is either served in Render's shape or explicitly recorded as a divergence, and a reverse inventory test makes the next shadowed route a CI failure instead of a QA finding.
- **Why now:** `w1/089`'s exhaustive comparison found exactly one shadowed route out of 183. The guard is cheap while there is one and expensive after the second. `w1/093` is a 30-minute read-only projection over the same `App.spec.notificationsToSend` the per-service route already reads, and it closes a gap the ledger currently claims does not exist.
- **Render parity:** included (t003). Both tasks change the REST surface a Render client sees, and t003 confirms GraphQL and MCP already expose the same vocabulary and per-service override data, compares against Render's `retrieve-webhook` and `list-notification-overrides` operations in the pinned spec, and records the outcome in ADR018 rows 227 and 228 and in `docs/render-artifacts/notify-on-fail.md`.
