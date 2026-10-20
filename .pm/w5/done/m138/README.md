# w5 · m138 — Postgres read replicas can be changed after create

**Worker:** worker5 **Goal:** a tenant adds, renames and removes a managed Postgres database's read replicas after create, on every surface, with the change recorded and announced, instead of recreating the database. **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `PATCH /v1/postgres/{id}` accepts `readReplicas` with create's checks — **DONE**                     | 60m | —          |
| t002 | GraphQL and MCP `update_postgres` take the same replica set — **DONE**                               | 45m | t001       |
| t003 | Record the change: `postgres_read_replicas_changed` plus the missing disk-autoscaling event — **DONE** | 60m | t001       |
| t004 | Dashboard HA panel adds and removes read replicas — **DONE**                                         | 60m | t002       |
| t005 | dev-5 walk: add then remove a replica; correct ADR018's HA row — **DONE**                            | 45m | t003, t004 |
| t006 | Render parity — **DONE**                                                                             | 30m | t005       |
| t007 | Simplify — **DONE**                                                                                  | 20m | t006       |
| t008 | Test coverage — **DONE**                                                                             | 45m | t006, t007 |
| t009 | Closeout — **DONE**                                                                                  | 10m | t008       |

Total: **9 tasks, about 6h 15m**.

## Definition of done

- `PATCH /v1/postgres/{id}` with `readReplicas: [{name}]` replaces the replica set (omitted leaves it unchanged, `[]` removes every replica) and answers the updated Render-shaped postgres object; the same change round-trips through GraphQL, MCP `update_postgres` and the dashboard HA panel, and `dryRun` previews it without writing.
- The same refusals as create apply, by name and code: a plan below `PostgresReadReplicaMinCPU`, more than `MaxReadReplicas()`, under `PostgresReadReplicaMinStorageGB` effective storage, an invalid or duplicate replica name.
- A replica-set change files one audit effect and delivers `postgres_read_replicas_changed`; toggling disk autoscaling delivers `postgres_disk_autoscaling_enabled_changed`. The `webhook_effects_test.go` absence guard is replaced by positive tests.
- On dev-5 a `basic-1gb` database at 10 GB gains a replica whose CNPG instance becomes ready and whose connection info appears on all four surfaces, then loses it again (instance gone, connection info gone).
- ADR018's "HA · failover · read replicas" row tells the truth about post-create replica changes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` (2026-10-19), item 4, chosen by the user the same day. Render's pinned `postgresPATCHInput` (`lego/backend/internal/api/openapi/render-public-api-1.json`) carries `readReplicas`; bex's strict PATCH struct (`lego/backend/internal/postgres/rest.go:454-474`) has no such field, so a Render client sending it gets 400. `PostgresPatch` (`postgres/service.go:1096-1119`) has no replica field, `webhook_effects_test.go:292-302` pins that no producer exists for `postgres_read_replicas_changed`, and the dashboard HA panel only displays replicas (`dashboard/src/features/databases/components/ha-panel.tsx`). Pinned divergences: `docs/render-artifacts/postgres-version-upgrade.md`, `docs/render-artifacts/datastore-webhook-events.md`.
- **Goal linkage:** ADR008's Render-alternative core; ADR018 Render parity; ADR009 managed Postgres.
- **Expected outcome:** scaling reads on a database no longer means recreating it; a Render client's `readReplicas` PATCH works unchanged; HA/replica webhooks have a real producer.
- **Why now:** the mechanism already exists — the operator reconciles `spec.readReplicas` on every pass (`lego/operator/internal/controller/database_controller.go:829,889`), Blueprint apply already changes it after create, and admission already compares desired and current replica sets (`postgres/admission.go:38-89`) — so only the update surfaces are missing. `basic-1gb` (0.5 CPU) already meets the replica minimum, so the feature is usable today, and ADR018 currently marks the row ✅ wrongly.
- **Render parity included:** this changes REST, GraphQL, MCP and the dashboard, so t006 re-checks all four against Render's contract.
- **Out of scope:** `postgres_cluster_leader_changed` — HA needs a ≥1-CPU plan and none ships yet (ADR009 §5's larger plans are an open pricing decision), so it would have no producer.

## Evidence (2026-10-19)

- **DoD 1–2 (all surfaces, refusals, dry-run).** `TestReadReplicaSetChangesAfterCreate` runs set/replace/clear/legacy-name and the five refusals (`_NAME_INVALID`, `_NAME_DUPLICATE`, `_LIMIT_EXCEEDED`, `_PLAN_UNSUPPORTED`, `_STORAGE_UNSUPPORTED`) on REST, GraphQL and MCP, live and dry-run; refusals and previews write nothing and record no event. `TestPostgresReadReplicasPatchThroughComposedStack` proves the pinned Render validator + strict handler path in production shape.
- **DoD 3 (events).** On dev-5's bex-api database, four `postgres.SetReadReplicas` rows for four changes (REST add, UI add, UI remove, REST clear) and one `postgres.SetDiskAutoscaling` for the toggle; `webhookEventTypes` serves both new types and a webhook subscribing to them was created (`whk-dbbggchjg4r6361g7d90`, disabled).
- **DoD 4 (dev-5, real CNPG).** `dpg-dbbfrn1jg4r30sm5h680` (`basic_1gb`, 10 GB): REST PATCH `r1` → CNPG 2/2 ready; REST, GraphQL, MCP and the dashboard all show `r1` with internal `…-ro.<ns>.svc` and external `…-ro-r1.db.bex.co` hosts (`.playwright-mcp/m138-ha-panel-r1.png`); dashboard add `r2` → 3/3 (`m138-ha-panel-r1-r2.png`), the name rule refuses `Reader_2` in the dialog (`m138-add-invalid.png`); dashboard remove `r2` (confirm, `m138-remove-confirm.png`) → 2/2; REST `[]` → 1/1, `readReplicas: []`, no replica connection strings. Re-run on the final (post-simplify) binary: an immediate GET after PATCH lists the new replica; hosts follow once reported.
- **DoD 5 (ledger).** ADR018's HA row describes post-create changes, the name rule, spec-listed reads and the GraphQL host fix; `webhooks-api.md` (55 types), `datastore-webhook-events.md`, `postgres-version-upgrade.md` updated.
- **Pending skeleton.** Forced pending (held `Database` query) vs ready: HA card 728×236 both at 1280px, 358×276 both at 390px (`m138-ha-{1280,390}-{pending,ready}.png`); `HAPanel pending preview` test pins the header and Read replicas row shapes.
- **Suites.** Backend `go test ./...` green (unit; the real-DB run is the ship gate); `make lint` 0 issues in all four modules; dashboard 5318/5319 — the one failure (`capabilities-startup.test.tsx`, w6/079) also fails on clean HEAD in this checkout and passes in CI (`test-dashboard` green at `f4d9dfb20`), a stale local `node_modules`; both `tsc` projects and knip clean; eslint cannot start in this checkout (pre-existing, as in m137).
- **Environment (local cluster, reported per AGENTS.md).** The shared CAPD operator was ~8 minutes older than w8/m50 (no reader instances) — rebuilt `bex-operator:dev` from this tree, imported on all three nodes, applied current CRDs and `bex-manager-role`, restarted the Deployment (local env/node pins untouched; no `make deploy`). Added a cluster-scoped `hcloud-volumes` StorageClass aliasing `rancher.io/local-path` (the operator hardcodes the Hetzner class for tenant Postgres; no tenant Postgres had ever provisioned locally), and in the walk tenant's own namespace a `m138-walk-allow-apiserver` egress policy (the local apiserver sits in a private range the tenant egress policy excludes). No other workstream's namespaces were touched.
