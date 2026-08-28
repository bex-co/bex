# w6 · m131 — The request-log half of `w3/m8` delivers nothing in production

**Worker:** worker6 **Goal:** `type=request` returns real rows for a service receiving HTTP traffic, and the Method / Status code / Request path filters that four surfaces advertise stop being guaranteed-empty — with the production check `w3/m8` left owed finally run. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Run `scripts/logs-verify.sh` against prod; locate where the access-log stream stops       | 45m | —          |
| t002 | Decide how "store up, stream not produced" is represented, vs a genuinely quiet service   | 40m | t001       |
| t003 | Reconcile `host` discovery — and the dashboard's static fallbacks — with filterability    | 30m | t001       |
| t004 | Add the guard that would have caught a silently-zero request stream                       | 40m | t002, t003 |
| t005 | Render parity sweep (REST/GraphQL/MCP/dashboard)                                          | 30m | t002, t003 |
| t006 | Simplify                                                                                  | 20m | t005       |
| t007 | Test coverage                                                                             | 30m | t005       |
| t008 | Closeout                                                                                  | 10m | t004, t007 |

## Background — found live, 2026-08-28, 69th `/qa-find-bugs` run, journey 7

**This is not a regression.** `w3/m8` shipped this feature, but its own status line records that the production check was never run:

> "The live cluster run of `scripts/logs-verify.sh` (t005) inherits m5/t005's gate — it needs a Loki-synced cluster (prod), which the CAPD mock is not."

The request pipeline was verified only against a containerized Alloy fixture. This milestone is that owed verification plus the contract gap it exposed. `w3/m8`'s design is not being re-litigated.

### Evidence — exhaustive across all five web services in the workspace, 24h window

Every probe is `fetch` from inside the authenticated dashboard page, `credentials:'include'`, against `https://api.bex.co`.

```
GET /v1/logs?resource=<id>&limit=20&startTime=<24h ago>&type=request
  srv-da8ek3oueu1c7395jqk0  (qa-20260828-logs, own fixture)  -> 200, 0 logs
  srv-d9bj8s3eg85c7390eb9g  (beancount-cms-v2)               -> 200, 0 logs
  srv-d9nqg9dcavls73fp8m2g  (beancount-forum)                -> 200, 0 logs
  srv-d9bkcspg9s7c73d0n8ug  (agentmarketcap-1)               -> 200, 0 logs
  srv-d9ndt8hmcglc739fkp50  (eden-dash-v3)                   -> 200, 0 logs
```

The label-index reads are the decisive evidence — these come from Loki's own label index, so an empty set means the shipper never attached the label, not that one query missed:

```
GET /v1/logs/values?resource=<id>&label=method      -> 200 []        on all 5
GET /v1/logs/values?resource=<id>&label=statusCode  -> 200 []        on all 5
GET /v1/logs/values?resource=<id>&label=type        -> 200 ["app"]   on all 5
```

`type` never reports `request` for any service. On the own fixture, 9 HTTP requests were driven immediately before probing and all returned 200 — 3× `GET /qa-marker-alpha`, 2× `POST /qa-marker-beta`, 2× `GET /`, 1× `DELETE /qa-marker-gamma`, by `curl` against `https://qa-20260828-logs.onbex.co` — then a 30s wait. Zero request lines appeared.

### Controls — what makes this the request pipeline specifically, not Loki and not the shipper as a whole

- **App logs work.** The same Loki store returned 100 app lines for `beancount-cms-v2`, 1 for the fresh fixture.
- **Build logs work.** `type=build` returned 100 lines (`hasMore: true`) for the fixture, from the same store.
- **Level normalization works.** `GET /v1/logs/values?label=level` on `beancount-cms-v2` returns `["error","unknown"]`. This matters more than it looks: `deploy/gitops/base/log-shipper.yaml:66-89` documents a prior production incident where Helm `tpl` escaping broke chart rendering, Argo CD went `ComparisonError`, and the live ConfigMap "froze at a pre-w3/m8 version (no request-log pipeline, **no level normalization**)". A live `error` level proves the deployed ConfigMap is **not** in that frozen state, which **rules out a recurrence of the `w3/m13` freeze** — the first thing a diagnosis would otherwise chase.
- **`host` discovery works,** but for a reason that is itself part of the problem: `lego/backend/internal/logs/service.go:657-661` answers `host` from `app.Status.URLs`, deliberately — "which is why it resolves even with no store wired". So `label=host` returns real hostnames (e.g. `["agentmarketcap-1.onbex.co","agentmarketcap.ai"]`) while `host` **filtering** is a Loki line filter over the access line's `RequestHost` (`logs/loki.go:273-286`) and can never match. Discovery and filtering read different sources.

### The UI completes the problem

On `/services/<id>/logs` the **Filters** popover offers Level, **Method**, **Status code**, Instance, and a free-text **Request path**. Three of those five apply only to request logs. Opening the Method dropdown live returns concrete, selectable values:

```
All methods · GET · POST · PUT · PATCH · DELETE
```

Since `label=method` returns `[]`, those come from the dashboard's **static fallback** list, not from discovery. The UI therefore presents five specific HTTP methods, every one of which returns zero rows on every service, with nothing indicating the underlying stream does not exist.

### The contract gap — the product-side defect, independent of why the pipeline is silent

`docs/ADR018-render-parity.md:182` marks this ✅ on REST/GraphQL/MCP/UI and states: "**Nothing accepted is ignored:** without the store (`BEX_LOKI_URL` unset) `type=request`, `type=build`, and the store-only filters return 503". That honest-failure path keys on **store presence only**. Here the store is present and working — app and build logs prove it — so a request-log query takes the success path and returns `200 {"logs":[],"hasMore":false}`, indistinguishable from a genuinely quiet service. There is no representation for "the store is up but this stream is not being produced".

## Definition of done

- `GET /v1/logs?resource=<web service with traffic>&type=request&startTime=<1h ago>` returns rows carrying `type=request`, verified against a service actually receiving HTTP traffic. The exact probe returns 0 on all five services today.
- `GET /v1/logs/values?label=method` and `?label=statusCode` return non-empty sets for such a service, sourced from Loki's label index. Both return `[]` on all five today.
- Filtering by a value the dashboard offers narrows to matching rows rather than returning nothing: selecting **Method = GET** in the Filters popover on a service with traffic yields the GET request lines. Today the popover offers GET/POST/PUT/PATCH/DELETE from a static fallback and every one returns zero.
- A `host` value returned by `label=host` discovery, used as a `host` filter, returns rows — or discovery no longer offers values the filter cannot match, per the decision `t003` records.
- `scripts/logs-verify.sh` has been run against production and its output recorded, discharging the check `w3/m8`'s status line left owed.
- App logs, build logs and level normalization still work. They do today — 100 app lines on `beancount-cms-v2`, 100 build lines on a fresh service, `level` values `["error","unknown"]` — and a pipeline change must not regress them.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt, 69th run, 2026-08-28, journey 7 (Logs + Metrics). Workspace `tea-d98210cbbpdc73dcrkvg` (Pro). Fixture `qa-20260828-logs` (`srv-da8ek3oueu1c7395jqk0`) created, driven with 9 HTTP requests, probed, and deleted in the same visit (`deleteService: true`, `GET` → 404); the other four services were read-only. Every probe above is a complete request + response so it can be re-run; no screenshot is cited, because the evidence is the wire format.
- **Goal linkage:** `docs/ADR010-observability.md` § Log filters and `docs/ADR018-render-parity.md` row 182 (Request/HTTP logs + structured filters), which the product marks ✅ on all four surfaces.
- **Precedent — extend, do not re-litigate.** `w3/m8` designed and shipped this; `w3/m13` fixed the Helm-escaping freeze that had previously blanked it. Neither is reopened: m8's design is sound, and m13's fix is demonstrably still holding — live level normalization is m13's own tell. What was never done is m8's production verification, which its status line explicitly defers.
- **Expected outcome:** the request-log filters four surfaces advertise actually return rows, and a stream that is not being produced is not presented as a quiet one.
- **Why now:** journey 7's promise is that filters narrow and empty states are honest. Three of the five filters the dashboard offers cannot match anything on any service, and the emptiness is indistinguishable from success. It may also silently degrade metrics: `ADR018:190` records that host/path-filtered `http_requests`/`http_latency` reads are served **from the request-log store**, so those filters rest on the same absent data — not probed this run, carried below as unverified.
- **Render parity:** the standing task is **included** — if `t002` adds a state or changes an error shape, REST/GraphQL/MCP/dashboard must move together.
- **Boundary note (`DO_NOT_DO.md` item 12 — product code vs platform GitOps responsibilities):** if `t001` finds the fix belongs in Helm/GitOps, it stays on that side. `t001` locates and records it; this milestone does not move shipper responsibilities into product code.
- **Blast radius:** the query path is shared — `QueryLogs` (`logs/service.go:460`) → `s.History` → `NewLokiSource` (`logs/loki.go:51-60`) → `lokiQueryFor` / `lokiSelectorFor` / `lokiTypeMatcher`. Any change to type handling touches app, request, build and predeploy reads together, plus the SSE tail (`FollowLogs`, `logs/service.go:743`) and the three adapters (`logs/rest.go`, `graphql.go`, `mcp.go`). The dashboard consumer is the Logs tab's Filters popover and its static fallback lists.
- **Adjacent classes:** store absent (`BEX_LOKI_URL` unset) → 503 today, must stay; store present + stream produced → rows; store present + stream never produced → the case at issue; a genuinely quiet service with a working pipeline → must remain an honest empty; unknown `type`/`label` → 400 naming it, which works today.

## Unverified this run — carried onto the board, not presented as observed

- **The root cause is not established.** No cluster access this run. Nothing beyond "no request lines and no `method`/`statusCode` labels reach the API for any service" was observed. The candidate stop points named in `t001` are read from the shipper config, not measured. This milestone must not be read as asserting a cause.
- The **metrics consequence** (`ADR018:190` — host/path-filtered `http_requests`/`http_latency` served from the request-log store) was **not probed**. It is a code-read inference and its own claim; if it holds it may deserve a separate finding.
- Only **this workspace** was examined. Whether request logs are absent platform-wide or specific to tenant namespace `tea-d98210cbbpdc73dcrkvg` was not determined.
- **MCP** `list_logs` / `list_log_label_values` were not exercised live; REST and GraphQL were.
- The dashboard's **Status code** and **Request path** filters were seen in the popover but not individually submitted; only Method's option list was opened. The zero-row outcome is established at the API level for all of them.
- Whether **`direction`, `instance` and `level`** filters narrow correctly was not systematically tested; `level` discovery returning `["error","unknown"]` on one service is the only level evidence.

## Rejected this run — recorded so the next hunt does not re-file it

`type=app&type=build` and `type=app&type=request&type=build` return `400 bad request: log type "build" must be requested on its own` — a deliberate, clearly-worded validation rule, not the empty result an early probe appeared to show (the probe helper counted `logs.length` without surfacing the status).

Worth a cleanup note during `t004`, but **not a user-facing defect and not its own task**: `lokiTypeMatcher` (`logs/loki.go`) still carries three build-plus-other branches (`wantApp && wantRequest && wantBuild` → `""`, `wantApp && wantBuild`, `wantRequest && wantBuild`) that `validate()` makes unreachable, and its `len(q.Types)==0` comment claims "Build logs are only included when the caller explicitly requests them — `type=build` or `type=all`" while `NormalizeTypes` maps `all` to `nil`, landing in that same branch and excluding build. Live: `type=all` returns app-only (1 line) where `type=build` returns 100.
