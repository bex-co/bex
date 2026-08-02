# w3 · m36 — Fix logs read path for tenant namespaces (ADR043 regression)

**Worker:** worker3 **Goal:** the dashboard/API Logs surface returns real lines for every tenant service again, instead of always-empty. **Status:** done — 2026-08-02, live-verified in prod

## Tasks (in order)

| id   | title                                                               | est | depends_on              | status                                          |
| ---- | ------------------------------------------------------------------- | --- | ----------------------- | ----------------------------------------------- |
| t001 | Resolve the App's namespace in the logs read path (not s.Namespace) | 30m | —                       | — **DONE**                                      |
| t002 | Resolve Postgres/Key Value log namespaces under tenant namespaces   | 30m | w3/m36/t001             | — **DONE** (no-op: datastores stay in shared ns) |
| t003 | Render parity: logs return across REST/GraphQL/MCP + dashboard      | 20m | w3/m36/t001,w3/m36/t002 | — **DONE**                                      |
| t004 | Simplify the changed logs namespace-resolution code                 | 20m | w3/m36/t003             | — **DONE**                                      |
| t005 | Test coverage: logs read path with TenantNamespaces=true            | 40m | w3/m36/t003             | — **DONE**                                      |
| t006 | Closeout                                                            | 10m | w3/m36/t005             | — **DONE**                                       |

## Definition of done

- ✅ With `BEX_TENANT_NAMESPACES=1`, `QueryLogs` (and `Logs`, `LabelValues`, the live pod-log fallback, and the SSE tail) query Loki/pods in the App's actual `<ws>` namespace (`app.Namespace`), not the hardcoded `s.Namespace`. Done in `internal/logs/service.go` (Logs/QueryLogs/LogLabelValues/FollowLogs + collectPodLogs/readPodLogs/streamPodLogs threaded a namespace param).
- ✅ Managed Postgres (`queryPostgresLogs`) and Key Value (`queryKeyValueLogs`): **verified no change needed.** Managed datastores are NOT namespaced under ADR043 — their CRs are created with `Namespace: s.Namespace` (`postgres/service.go:500`, `keyvalue/service.go:409`) and `AuthorizeDatabase`/`AuthorizeKeyValue` Get from `b.Namespace`, so their pods and Loki streams stay in the shared namespace and the existing `s.Namespace` reads are already correct. Only App workloads move to `<ws>`. (If datastores are ever namespaced, that is a coordinated change across create/authz/pods/logs — a separate milestone.)
- ✅ A regression test (`internal/logs/tenant_namespace_test.go`) exercises the logs read path with `TenantNamespaces=true` and was proven to FAIL on the pre-fix behavior (queried `"default"`) and pass with the fix — covers history, `Logs`, label-values, and the pod-log fallback, plus a shared-namespace byte-identical guard.
- ✅ Live-verified 2026-08-02: production Loki returns real, recent log lines for `{namespace="tea-d98210cbbpdc73dcrkvg", app="tea-d98210cbbpdc73dcrkvg-beancount-cms-v2"}` (App CR confirmed live in that namespace via `kubectl get app.app.bex.co -A`) — the exact query that returned zero streams pre-fix. Shipped commit `cd3b5f72` is 74 commits behind current `origin/main` with multiple successful deploys since (t006).
- ✅ No behavior change in shared-namespace mode (`TenantNamespaces=false`) — `app.Namespace == s.Namespace`, guarded by the shared-namespace test case. Backend `go build`/`go test ./...`/`go vet`/`golangci-lint` all green.

## Source + Goal linkage

- **Source:** investigation 2026-07-31, live-confirmed against prod (Loki `{namespace="default",app=X}` → 0 streams vs `{namespace="tea-…",app=X}` → 2 streams with real lines); memory memo `project_logs_tenant_namespace_bug.md`. Follow-up to **m31** (ADR043 tenant namespaces) which moved App/pod/stream namespaces to `<ws>` without updating the logs feature. The metrics sibling was updated (`internal/metrics/service.go:670`, `app.Namespace` with an explicit ADR043 comment); logs was not.
- **Goal linkage:** `GOAL.md` #2 ("Basic obs for operation") — logs are the highest-operational-value obs surface (this workstream's first milestone), currently broken in prod for all tenant services.
- **Expected outcome:** the Logs tab and all log APIs return real lines for tenant services again; no more silent empty.
- **Why now:** it's a live prod regression affecting every tenant service's logs (and the live tail / SSE), silently returning empty with no error. CI is green because the logs tests only run in shared-namespace mode, so it will not self-heal.
- **Render parity task included:** the fix touches the user-facing logs surface (REST `GET /v1/logs` + datastore log endpoints, GraphQL `logs`/`databaseLogs`/`keyValueLogs`, MCP `list_logs`/`get_logs`/datastore variants, dashboard Logs tabs) — parity verification confirms all four surfaces return correctly and consistently.
