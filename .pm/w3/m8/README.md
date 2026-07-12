# w3 · m8 — Request/HTTP logs + structured filters over the Loki pipeline

**Worker:** worker3 **Goal:** Make Render's Application/Request logs split real: ship Traefik's access-log stream into the m5 Loki pipeline tagged `type=request`, label app logs with `level`, honor the structured filters the API currently accepts-but-ignores (`type`, `level`, `statusCode`, `method`, `path`, `instance`), and add the official MCP `list_log_label_values` discovery tool — retiring the "application logs only" divergence documented in docs/ADR006-bex-api.md and ADR010-observability.md. **Status:** todo (gated on w3/m5 closeout — the pipeline this rides)

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Traefik access logs → shipper → Loki, `type=request` + method/status/path labels (cardinality budget decided) | 35m | w3/m5      |
| t002 | App-log `level` labeling at the shipper (JSON logs), honest `unknown` fallback                                 | 30m | t001       |
| t003 | `QueryLogs` honors `type`/`level`/`statusCode`/`method`/`path`/`instance` over the labeled streams             | 30m | t001, t002 |
| t004 | MCP `list_log_label_values` (official tool) + REST/GraphQL filter-suggestion reads (`metricsFilters` pattern)  | 30m | t003       |
| t005 | Acceptance: `type=request` returns the access line; `level=error` isolates a planted error; discovery lists real values | 25m | t004       |
| t006 | Render parity — filter semantics vs Render's logs API/dashboard; matrix request-logs row updated               | 20m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                    | 20m | t006       |
| t008 | Test coverage — filter→LogQL mapping, cardinality guard, unlabeled-stream fallbacks                            | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                                 | 10m | t008       |

## Definition of done

`GET /v1/logs?type=request` (and the MCP/GraphQL equivalents) returns the service's Traefik access lines with truthful status/method/path; `level=error` on application logs isolates error lines (JSON-logging apps) with an honest `unknown` bucket for unparseable streams; every filter the API accepts is either honored or removed from the accepted set (nothing silently ignored remains); `list_log_label_values` discovers real label values under the official tool's name/args; the divergence notes in docs/ADR006-bex-api.md + ADR010-observability.md are replaced with the shipped design; docs/ADR018-render-parity.md's request-logs row moves from ✖.

## Source + Goal linkage

- **Source:** promotion of inbox `w3/002` (filed by the w1/m13 parity audit 2026-07-08; its own gating condition — "promote once the log backend strategy settles whether a structured store/agent is in scope" — was answered by w3/m5's Loki + Alloy shipper); matrix row "Request / HTTP logs + structured filters" (✖ ✖ ✖ ✖); Render's logs filters + `list_log_label_values` in `render-oss/render-mcp-server`.
- **Goal linkage:** pillar 1 (Render logs parity — request logs are first-class on Render's dashboard and `/v1/logs`); GOAL.md #2 (observability).
- **Expected outcome:** the biggest remaining observability parity gap closes; accepted-but-unhonored filters stop being a documented embarrassment; agents can discover label values instead of guessing.
- **Why now:** m5 is two tasks from closeout and this rides its exact pipeline (shipper + Loki + `LogQuery`→LogQL mapping) — the design is freshest now, and t001's label taxonomy is cheapest to settle before the shipper config ossifies.
- **Render parity closing task: included** — REST/GraphQL/MCP surface change (filters + discovery tool). Dashboard filter UI: the live-logs page (w5/m6) already renders type/filter controls — t006 verifies what it exposes against the newly-honored semantics and files UI drift as a w5 note rather than building UI here.
