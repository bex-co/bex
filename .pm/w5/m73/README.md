# w5 · m73 — Key Value metrics: dashboard sends typed id + bex-api matches on the resolved CR name

**Worker:** worker5 **Goal:** the dashboard Key Value metrics page (`/keyvalue/red-…` — disk, memory, connections) shows real series instead of silently-empty charts **Status:** todo (t001–t005 done; t006 gates on post-deploy live verification)

## Tasks (in order)

| id   | title                                                              | est | depends_on | status |
| ---- | ------------------------------------------------------------------ | --- | ---------- | ------ |
| t001 | Dashboard: pass the typed id to DatastoreMetricsPanel              | 30m | —          | DONE   |
| t002 | bex-api: build datastore PromQL matchers from the resolved CR name | 45m | —          | DONE   |
| t003 | Render parity (closing)                                            | 15m | t001, t002 | DONE   |
| t004 | Simplify (closing)                                                 | 20m | t003       | DONE   |
| t005 | Test coverage (closing)                                            | 30m | t004       | DONE   |
| t006 | Closeout (closing)                                                 | 10m | t005       | todo   |

## Definition of done

- `dashboard/src/routes/keyvalue.$keyValueId.tsx` passes `keyValue.id` (the `red-…` typed id / CR name) to `DatastoreMetricsPanel`, with a route test asserting it (mirroring the Postgres page's `database.id`).
- `lego/backend/internal/metrics/datastore.go` derives every Prometheus matcher (PVC pattern, pod regex, CNPG cluster label) from the **authorized CR's** `metadata.name`, never from the raw `DatastoreMetricQuery.Resource` input.
- `cd lego/backend && go test ./...` and `yarn test` (dashboard) green.
- Observable end state: against a stack with Prometheus wired, `datastoreMetrics(kind: "keyvalue", resource: "<red-…>")` returns non-empty disk/kv_memory/kv_connections series and the dashboard page renders them; a display-name input fails as an explicit error, not silent empty charts.

## Progress log

- 2026-08-17 — t001–t005 done. Dashboard sends `keyValue.id` (route test pins it); `DatastoreMetrics` builds all matchers from the authorized CR's `metadata.name` (`var resource`, fail-closed by construction); REST/GraphQL/MCP confirmed to share the one identifier contract; MCP + panel prop docs now say "typed id, not display name". `/simplify` applied two fixes (panel JSDoc, `var resource`); the hand-rolled call-recording mock was a conscious keep. Suites green: backend `go build ./... && go test ./...` exit 0, dashboard 2204/2204, `make lint-backend` 0 issues. **t006 held:** the DoD's observable end state needs a Prometheus-wired stack — the local kind cluster runs no monitoring, so verification is post-deploy on prod (`/keyvalue/red-d9p49kdrtmes73c34ovg` charts populate). Follow-up filed: `w5/045` (the panel's error swallowing that hid this bug).

## Source + Goal linkage

- **Source:** `w5/044` (promoted 2026-08-17 at user direction; the note records the full root-cause chain: the page has passed the display name since the panel's introduction in `a5cf1f75`/w3/m10, `findKeyValue` matches CR `metadata.name` only, and the panel's `errorPolicy: "all"` swallows the resulting NotFound into "No data in range"). Sizing note: promoted to a milestone by explicit user direction although the implementation is small, so the backend hardening + verification + closing tasks travel with the one-line fix.
- **Goal linkage:** [docs/ADR010-observability.md](../../../docs/ADR010-observability.md) (metrics over every surface) and the [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md) ledger — a shipped, user-facing observability surface is broken on prod.
- **Expected outcome:** `/keyvalue/<red-…>` metrics charts populate (verified live or against a dev stack); no silent-empty failure mode remains for identifier-shaped mistakes.
- **Why now:** reported broken on prod 2026-08-17 (`red-d9p49kdrtmes73c34ovg`); the bug has existed since the panel shipped, so every Key Value metrics view is empty.
- Render parity closing task **included**: the fix touches the dashboard UI surface; the datastore-metrics API itself is a bex extension (no Render equivalent), so parity here means REST/GraphQL/MCP/UI consistency, not render.com comparison.
