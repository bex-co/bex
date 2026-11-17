# w3 · m438 — Operators can tell a quiet CLI week from a broken telemetry pipeline (ADR113 D6 + D8)

**Worker:** worker3 **Goal:** a server-side count of CLI requests to compare telemetry against, a telemetry counter with closed labels, and three warning-tier alerts with their panels. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Count CLI API requests by client on the server | 45m | — |
| [t002](t002.md) | Close the telemetry counter's labels | 45m | — |
| [t003](t003.md) | Three warning-tier alerts with promtool tests | 60m | t001, t002 |
| [t004](t004.md) | Collection-health panels on the CLI usage board | 45m | t003 |
| [t005](t005.md) | Docs and production check | 30m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- Production bex-api serves `bex_api_cli_requests_total{client}` and the relabelled `bex_api_cli_telemetry_events_total`, and neither takes a label value verbatim from a request body.
- `CLITelemetryIngestFailing`, `CLITelemetryIngestSilent` and `CLIExecutionErrorShareHigh` are loaded on production Prometheus and inactive, and each has promtool fire and no-fire tests.
- The CLI usage board has Telemetry coverage, Stored events by completion kind and Backoff-causing answers, and each query returns production data.
- `go test ./internal/api/ ./internal/clitelemetry/` (from `lego/backend`) and `bash scripts/gitops-validate.sh` pass.

## Source + Goal linkage

- **Source:** [ADR113](../../../docs/ADR113-cli-usage-telemetry.md) D6 and D8 (gaps G6, G7). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 2; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** nothing alerts on `POST /v1/cli-telemetry-events`. The runbook says "No requests is not proof of collector health, and quiet CLI usage is not proof of an outage". `bex_api_cli_telemetry_events_total` takes `command` from the request body and no panel or rule reads it (`lego/backend/internal/api/httpmetrics.go:188`).
- **Goal linkage:** ADR088 (every alert has a panel), ADR010 alert tiers, and trustworthy CLI usage numbers for ADR008's agent-native measures.
- **Expected outcome:** a broken or silent telemetry pipeline reaches the daily digest within hours instead of looking like a quiet week, and the counter's cardinality is bounded.
- **Why now:** a 5xx or a routing 404 silences every affected CLI for 30 minutes to an hour per occurrence, so an unnoticed outage loses far more than its own length; and m437 changes collection, so its effect should be visible on a coverage panel.
- **Render parity omitted:** internal operations only. The ingest route's answers do not change, and no REST, GraphQL, MCP or dashboard surface changes.
