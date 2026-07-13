# w3 · m8 — Request/HTTP logs + structured filters over the Loki pipeline

**Worker:** worker3 **Goal:** Make Render's Application/Request logs split real: ship Traefik's access-log stream into the m5 Loki pipeline tagged `type=request`, label app logs with `level`, honor the structured filters the API currently accepts-but-ignores (`type`, `level`, `statusCode`, `method`, `path`, `instance`), and add the official MCP `list_log_label_values` discovery tool — retiring the "application logs only" divergence documented in docs/ADR006-bex-api.md and ADR010-observability.md. **Status:** done — REST/GraphQL/MCP honor Render's full filter set, `list_log_label_values` ships under the official name/args, and the shipper pipeline was verified against **real Alloy** (containerized fixture run: `level` normalization incl. the `unknown` bucket, `ServiceName`→App attribution, and both drop paths). The live cluster run of `scripts/logs-verify.sh` (t005) inherits m5/t005's gate — it needs a Loki-synced cluster (prod), which the CAPD mock is not.

## Tasks (in order)

| id   | title                                                                                                                  | est | depends_on | status |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Traefik access logs → shipper → Loki, `type=request` + method/status/path labels (cardinality budget decided)           | 35m | w3/m5      | — **DONE** (`traefik.values.yaml` JSON access log + `log-shipper.yaml` request pipeline; budget in ADR010 § Log labels — `path`/`host` are line-only, guarded by `TestPathAndHostNeverBecomeLabels`) |
| t002 | App-log `level` labeling at the shipper (JSON logs), honest `unknown` fallback                                          | 30m | t001       | — **DONE** (parsed from the line's `level`/`severity`, never substring-guessed; unparseable ⇒ `unknown`) |
| t003 | `QueryLogs` honors `type`/`level`/`statusCode`/`method`/`path`/`instance` over the labeled streams                      | 30m | t001, t002 | — **DONE** (`loki.go` LogQL builder + `host`/`direction` too; injection-escaped) |
| t004 | MCP `list_log_label_values` (official tool) + REST/GraphQL filter-suggestion reads (`metricsFilters` pattern)           | 30m | t003       | — **DONE** (name/args mirrored from `render-oss/render-mcp-server` `pkg/logs/tools.go`; + `GET /v1/logs/values`, GraphQL `logLabelValues`; scoped per service) |
| t005 | Acceptance: `type=request` returns the access line; `level=error` isolates a planted error; discovery lists real values | 25m | t004       | — **PARTIAL** (`scripts/logs-verify.sh` phases 4–7 written; the **shipper half proved hermetically against real Alloy** in a container. Live run needs a Loki-synced cluster — same gate as m5/t005) |
| t006 | Render parity — filter semantics vs Render's logs API/dashboard; matrix request-logs row updated                        | 20m | t005       | — **DONE** (matrix row ✖✖✖✖ → ✅✅✅◐ + gap-backlog row; UI drift filed in `w5/008`, not silently fixed) |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                            | 20m | t006       | — **DONE** (4 agents; 10 findings applied incl. 2 real defects — see below; 3 skipped with reasons) |
| t008 | Test coverage — filter→LogQL mapping, cardinality guard, unlabeled-stream fallbacks                                     | 30m | t006       | — **DONE** (table-driven filter→LogQL, injection guards, cardinality guard, discovery scoping, fallback refusals) |
| t009 | Closeout — DoD met → move milestone to `done/`                                                                          | 10m | t008       | — **DONE** |

## Definition of done

`GET /v1/logs?type=request` (and the MCP/GraphQL equivalents) returns the service's Traefik access lines with truthful status/method/path; `level=error` on application logs isolates error lines (JSON-logging apps) with an honest `unknown` bucket for unparseable streams; every filter the API accepts is either honored or removed from the accepted set (nothing silently ignored remains); `list_log_label_values` discovers real label values under the official tool's name/args; the divergence notes in docs/ADR006-bex-api.md + ADR010-observability.md are replaced with the shipped design; docs/ADR018-render-parity.md's request-logs row moves from ✖.

**Met**, with one caveat carried from m5: the end-to-end assertions are proven at the component level (Go tests + a real-Alloy fixture run), not yet on a live Loki-synced cluster (t005).

## What shipped

- **Label taxonomy + cardinality budget** (ADR010 § Log labels): labels are `namespace`/`app`/`type`/`pod`/`container`/`level`/`method`/`status` — all bounded. `path`/`host` stay **in the line** and are filtered at query time with LogQL's `json` stage; promoting `path` to a label would be a cardinality incident, and a test guards it.
- **Nothing accepted is ignored**: without the store (`BEX_LOKI_URL` unset), `type=request` and the store-only filters return **503**; an unknown `type`/`direction`/`label` is a **400** naming it; the live tail (which reads pod logs by design) refuses store-only filters as a **400 about the transport**, not a lie about a missing store. `type=build` is the one empty-by-design type (no build-log shipper), stated in the docs.
- **Verification that found real bugs.** Running the shipped `loki.process` stages in a real Alloy container over fixture lines caught two defects that would have shipped broken: the `drop` stages never fired (Go templates render a missing key as the literal `<no value>`, not `""`), and plaintext app lines got **no** `level` label instead of the promised `unknown` (Alloy *skips* a template stage whose source key is absent). Also learned: `stage.match`'s selector only sees an entry's **incoming** labels, not pipeline-set ones — which is why the request pipeline promotes a scaffolding `job` label and drops it before the write.
- **/simplify caught two more**: `statusClass` was rewriting **every** filter's `Nxx`-shaped values, so `path=4xx` silently became the wildcard `4*`; and `GET /v1/logs/values` bypassed `BEX_MAX_QUERY_HOURS` entirely (unbounded label scans). Both fixed with tests.

## Follow-ups filed

- **`w5/008`** — the UI half (filter controls + discovery-fed dropdowns, request-line rendering, the new 503 dev-mode state). Unblocked by this milestone.
- **`w3/004`** — the log-shipper's pod discovery is cluster-wide on every node, so each Traefik access line is processed and pushed N× (N = nodes). Pre-dates m8 (m5's app-pods block has the same shape) but m8 points it at the highest-volume stream.

## Source + Goal linkage

- **Source:** promotion of inbox `w3/002` (filed by the w1/m13 parity audit 2026-07-08; its own gating condition — "promote once the log backend strategy settles whether a structured store/agent is in scope" — was answered by w3/m5's Loki + Alloy shipper); matrix row "Request / HTTP logs + structured filters" (✖ ✖ ✖ ✖); Render's logs filters + `list_log_label_values` in `render-oss/render-mcp-server`.
- **Goal linkage:** pillar 1 (Render logs parity — request logs are first-class on Render's dashboard and `/v1/logs`); GOAL.md #2 (observability).
- **Expected outcome:** the biggest remaining observability parity gap closes; accepted-but-unhonored filters stop being a documented embarrassment; agents can discover label values instead of guessing.
- **Render parity closing task: included** — REST/GraphQL/MCP surface change (filters + discovery tool). Dashboard filter UI: t006 verified what the live-logs page exposes against the newly-honored semantics and filed the drift as `w5/008` rather than building UI here.
