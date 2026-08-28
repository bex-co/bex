# w6 · m133 — The Postgres parameter editor is seeded from the live `pg_settings` read

**Worker:** worker6 **Goal:** the parameter editor shows and replaces only what the tenant declared, and operator-owned settings — the WAL archive/restore commands, the TLS paths, the replication group — cannot be captured into tenant config. **Status:** in progress — t001–t007 done (backend, dashboard, docs and tests landed; dashboard 379 files/2773 tests, backend `go test ./...` 60 packages, `make lint` ×4 all green). t008 closeout is open: its checks are live probes against `dashboard.bex.co` and this session has no QA credentials (`scripts/qa-login.sh` exits 2).

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Expose the declared overrides (`GetParameterSpec`) on GraphQL / REST / MCP             | 40m | —          | — **DONE**
| t002 | Rebind the editor to declared overrides; decide what "Non-default parameters" is       | 40m | t001       | — **DONE**
| t003 | Guard operator-managed and non-configuration parameters in the **backend**              | 40m | t001       | — **DONE**
| t004 | Blast radius: every parameter writer bound by the policy, correct callers regressed      | 40m | t002, t003 | — **DONE**
| t005 | Render parity sweep (REST/GraphQL/MCP/dashboard)                                        | 30m | t002, t003 | — **DONE**
| t006 | Simplify                                                                               | 20m | t005       | — **DONE**
| t007 | Test coverage                                                                          | 30m | t005       | — **DONE**
| t008 | Closeout                                                                               | 10m | t004, t007 |

## Background — found live, 2026-08-28, 72nd `/qa-find-bugs` run, journey 11

### 48 editable rows on a database created five minutes earlier and never configured

`qa-20260828-pg` (`dpg-da8fts7m2e9c73ft6vo0`, plan free, PostgreSQL 18) was created, allowed to reach `available`, and given **no** parameters. The dashboard's Insights section then renders a **"Non-default parameters"** table — columns Parameter | Setting | Source | Actions — in which every row is an editable textbox pair (`Parameter N name` / `Parameter N value`) with a `Remove <name>` button.

```
{ databaseParameterOverrides(id:"dpg-da8fts7m2e9c73ft6vo0"){ name setting source } }
-> 48 rows.   by source: {"configuration file": 45, "client": 2, "session": 1}
```

The complete list, none of them set by the tenant:

```
DateStyle, TimeZone, allow_alter_system, archive_command, archive_mode, archive_timeout,
autovacuum_worker_slots, cluster_name, default_text_search_config, default_transaction_read_only,
dynamic_shared_memory_type, full_page_writes, hot_standby, lc_messages, lc_monetary, lc_numeric,
lc_time, listen_addresses, log_destination, log_rotation_age, log_rotation_size, log_timezone,
log_truncate_on_rotation, logging_collector, max_connections, max_parallel_workers,
max_replication_slots, max_wal_size, max_worker_processes, min_wal_size, pg_stat_statements.track,
port, recovery_target_timeline, restart_after_crash, restore_command, shared_buffers,
shared_memory_type, ssl, ssl_ca_file, ssl_cert_file, ssl_key_file, statement_timeout,
transaction_read_only, wal_keep_size, wal_level, wal_log_hints, wal_receiver_timeout, wal_sender_timeout
```

What is in that list is the point:

- `archive_command` = `/controller/manager wal-archive --log-destination /controller/log/postgres.json %p %f` and `restore_command` = `/controller/manager wal-restore … %f %p` — the CloudNativePG operator's **own binary path**, and the mechanism continuous backups and PITR run on. Journey 11's promise is that backups are real; this table offers them for editing.
- `ssl`, `ssl_ca_file`, `ssl_cert_file`, `ssl_key_file` — TLS material paths.
- `wal_level=logical`, `wal_keep_size`, `max_replication_slots`, `hot_standby=on`, `wal_log_hints=on`, `recovery_target_timeline=latest`, `restart_after_crash=off`, `port=5432`, `listen_addresses`, `cluster_name` — replication / HA machinery.
- The three rows sourced `client` / `session` are not configuration at all: they exist only because of the API's **own** connection to the database, and are transient per-session values.

### Root cause — the backend keeps two distinct reads and the UI is bound to the wrong one

The backend already draws the exact distinction this erases, at `lego/backend/internal/postgres/insights.go:388-391`:

```go
// GetParameterSpec returns the currently stored parameter overrides from the
// Database CR (spec.parameters), not from pg_settings. Use ParameterOverrides for
// the live database view.
```

- `ParameterOverrides()` (`insights.go:360`) reads `pg_settings` where `source != 'default'` (`insights.go:158`) — the **observed** effective config, operator-set values included. `ParameterOverrideView` (`insights.go:89`) is documented as "one non-default pg_settings row".
- `GetParameterSpec()` (`insights.go:392`) reads `d.Spec.Parameters` — the **declared** tenant overrides.

The dashboard binds the **editor** to the first: `dashboard/src/features/databases/api/databases.graphql:364-370` defines `query DatabaseParameterOverrides` selecting `databaseParameterOverrides(id:){ name setting unit source description }`; `use-database-insights.ts:74` runs it; `insights-panel.tsx:299-307` passes `insights.parameterOverrides` straight into `<ParameterOverridesEditor overrides={…}>`.

### The save is a full replacement — stated by the component's own comment

`dashboard/src/features/databases/components/parameter-overrides-editor.tsx`:

```
// The parameter map is a full replacement, so asserting any statement-logging
// setting makes the update can_create; otherwise it is a can_operate settings
// change (docs/ADR024, mirrored from the backend).
```

and `save()` submits the whole table, not a diff:

```js
const result = await onSave(rows.map(({ name, value }) => ({ name: name.trim(), value: value.trim() })));
```

`dirty` is computed against `savedRows`, initialized to the same seeded list — so **removing a single row enables Save**, and Save then writes the remaining ~47 operator-derived values into `spec.parameters` as the tenant's declared overrides. The backend filter is one name wide: `normalizeParameterOverrides` (`lego/backend/internal/postgres/service.go:934-948`) drops only `shared_preload_libraries` and passes everything else through to `d.Spec.Parameters` (`service.go:867`).

### The declared overrides are unreadable on every surface

`grep -rn "GetParameterSpec\|parameterSpec\|ParameterSpec" lego/backend/internal/postgres/{graphql,rest,mcp}.go` returns **nothing**, and `parameters` is absent from the Postgres view — the `POST /v1/postgres` response's complete top-level key list is `id, name, plan, version, status, databaseName, databaseUser, diskSizeGB, diskAutoscalingEnabled, highAvailabilityEnabled, readReplicas, suspended, createdAt, updatedAt, region, dashboardUrl, public, ipAllowList, poolerEnabled, connectionPool, backupsEnabled, ownerId, owner`. So `spec.parameters` is effectively **write-only**: a tenant can declare overrides and can never read back what they declared. The editor had nothing correct to bind to.

## Definition of done

- A newly created Postgres with no parameters set shows an **empty** declared-override editor. Today a fresh free database seeds it with 48 rows.
- The declared overrides are readable on the API: a read returns what the tenant set, and returns empty for a fresh database. Today no surface exposes it and `parameters` is absent from the Postgres view.
- Saving the editor replaces only the tenant's declared set: setting one parameter and re-reading returns exactly that one, not 48.
- Operator-owned parameters are refused or filtered **by the backend**, verified by calling the API directly rather than through the dashboard — an attempt to set `restore_command` or `ssl_key_file` does not land in `spec.parameters`. Today `normalizeParameterOverrides` drops only `shared_preload_libraries`.
- The `pg_settings` view remains available as a read-only insight and still returns its 48 rows for a fresh free database. It is correct as a diagnostic and must not be deleted along with the fix.
- A legitimately tenant-set tuning parameter still round-trips through REST, GraphQL and MCP.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt, 72nd run, 2026-08-28, journey 11 (Postgres). Workspace `tea-d98210cbbpdc73dcrkvg` (Pro). Fixture `qa-20260828-pg` (`dpg-da8fts7m2e9c73ft6vo0`, free, PG 18) created, probed and **deleted** in the same visit (`deleteDatabase: true`, `GET` → 404; only the three pre-existing customer databases remain). **No parameter write was performed against production** — the save path is established from the component source and the backend filter, not by clicking Save.
- **Goal linkage:** `docs/ADR009-postgresql-management.md` (managed Postgres) and `docs/ADR006-bex-api.md`'s three-surface contract; `docs/ADR024` for the `can_operate` / `can_create` split the editor already mirrors.
- **Precedent — extend, do not re-litigate.** `w5/m35` ("Dashboard dead-ends: Postgres parameter-overrides editor + workspace resource caps", done; commit `2856ae05`) shipped this editor, with the stated goal of making "shipped backend capabilities the dashboard fetches but never lets a human use" usable. It wired the editor onto the read the Insights panel **already had** — the `pg_settings` one — which is exactly how the conflation entered. This is **not** a regression, and m35's decision to ship an editor is not reopened; what was missed is that the panel's existing read was the observed config, and that the correct read existed but was unexposed.
- **Expected outcome:** the parameter editor shows and replaces what the tenant declared, and operator-owned settings cannot be captured into tenant config.
- **Why now:** journey 11's promises include that backups are real. The seeded list contains `archive_command` and `restore_command` — the WAL archival and restore machinery backups run on — presented as editable tenant values with Remove buttons, alongside the TLS key/cert paths. A single edit anywhere in the table writes all of them.
- **Render parity:** the standing task is **included** — a new read is added across GraphQL/REST/MCP and the write policy changes, so all three surfaces plus the dashboard move together. Render exposes tenant-settable Postgres parameters without exposing the provider's internal archival commands; that is the shape to match.
- **Blast radius:** `SetParameterOverrides` (`insights.go:385`) → `UpdatePostgres` with `PostgresPatch{ParameterOverrides}` → `normalizeParameterOverrides` (`service.go:934`) → `d.Spec.Parameters` (`service.go:867`). The dashboard writer is `SetDatabaseParameterOverridesDocument` via `use-database-insights.ts:85`; the reader is `DatabaseParameterOverridesDocument` (`:74`) feeding `insights-panel.tsx:299-307` → `parameter-overrides-editor.tsx:70`. `sensitiveLoggingParameters` (`service.go:890`) already classifies names in this same file and is the precedent for the guard's shape.
- **Adjacent classes:** a tenant-set tuning parameter (must keep working); an operator-owned parameter (refused or filtered, by the backend); a `client`/`session`-sourced row (never configuration — must never be persisted); `shared_preload_libraries` (already filtered, must stay filtered); a statement-logging parameter (already requires `can_create` per ADR024 — that gate must survive the rebind).

## Unverified this run — carried onto the board, not presented as observed

- **Save was never clicked**, and no parameter write was made against production. The full-replacement behaviour is read from `parameter-overrides-editor.tsx`'s `save()` and its own "full replacement" comment, plus `normalizeParameterOverrides`'s single-name filter — code-established, not observed end to end. Confirm it on a disposable database before building on it.
- Which three rows carry `client` / `session` source was not individually identified — only the counts (2 client, 1 session) and the plausible names.
- **What the operator does when `spec.parameters` contains an operator-owned key was not determined** — whether CloudNativePG rejects it, silently wins, or fights it in a reconcile loop is unknown, and it decides how severe the consequence is. No cluster access this run.
- REST and MCP parameter-**write** paths were not exercised; only the GraphQL read was.
- Whether the same conflation exists for Key Value was not checked.

## Verified working this run — recorded so the fix does not break it

The rest of journey 11 is honest. Free-tier `backupsEnabled` is `false`, `POST /v1/postgres/{id}/recovery-info` returns `{"enabled":false,"backups":[]}`, and `GET /v1/postgres/{id}/export` returns `[]` — an honest empty, not a fabricated list. Connection info is gated behind an explicit **Reveal connection info** button, with the copy "Connection strings and the database password. Revealed only when you ask — never shown automatically." `externalConnectionString` is correctly omitted for a non-public database (`public: false`) and is populated when `Public` is set (`service.go:680`); the string offered is the cluster-internal `.svc` host, which is right for a non-public database.

One cosmetic nit, not worth its own item: the Details list renders an **"External host"** term with an empty definition for a non-public database, instead of omitting the row or showing a dash.

## Implementation record (2026-08-27)

Every claim in the Background was re-verified against the tree before any code was written — the `pg_settings`/`spec.parameters` split at `insights.go`, the GraphQL binding to the wrong one, the one-name-wide `normalizeParameterOverrides`, and the editor's full-replacement `save()`. All held.

**t001 — the declared set is readable.** `Service.ParameterSpec` returns `spec.parameters` as a name-sorted `[]ParameterSpecView`, exposed as REST `GET /v1/postgres/{id}/parameters`, GraphQL `databaseParameterSpec`, and MCP `list_postgres_parameters`. The scope-matrix guard caught all three as unclassified operations and they were regenerated as `OpClassRead`, matching the sibling `parameter-overrides` read.

**A parity finding changed the shape of t001.** Render's own pinned `postgresDetail` schema carries `parameterOverrides` — a `{name: value}` map of the declared set — and bex's Postgres view omitted it. So the Render-shaped answer was not only a bex-native side route: `PostgresView.ParameterOverrides` now carries the declared set on the Postgres object itself, which is what closes the DoD's "`parameters` is absent from the Postgres view" bullet properly. Render's list schema does not declare the key and neither schema sets `additionalProperties: false`, so the shared view is safe; the conformance gate is green.

**t002 — the editor is rebound.** `ParameterOverridesEditor` now takes `parameters: ParameterSpecView[]` (`{name, value}`) instead of pg_settings rows, and the Insights panel feeds it `insights.parameterSpec`. The **Source** column is gone from the editor — a declared parameter has exactly one source, and the column's presence was part of what made the observed configuration look editable.

"Non-default parameters" is resolved by keeping both, clearly separated, as the DoD required: **Parameter overrides** (declared, editable, "Saving replaces the whole set") and **Non-default parameters** (observed, read-only, "including the ones the platform manages"). The pg_settings read was not deleted — it is a correct diagnostic, and still returns its ~48 rows.

**t003 — the guard, in the backend.** `operatorManagedParameterNames` refuses a write naming any platform-owned parameter, with a 400 listing what was rejected. It sits in `PostgresPatch.validate()`, which is the single choke point: `applyPostgresPatch` is the **only** writer of `d.Spec.Parameters` in the codebase (t004's blast-radius sweep — grep returned exactly one), and every surface reaching it goes through `UpdatePostgres`, which validates before applying anything. Four groups, each justified in the code comment: WAL archival/recovery, TLS material, replication/HA/pod control, and not-configuration. Whole families match by prefix (`archive_`, `ssl`, `recovery_target`, `unix_socket_`, `syslog_`), the same defensive shape `sensitiveLoggingParameterPrefixes` already uses one function away.

**One addition beyond the filed scope, flagged rather than smuggled:** `pg_stat_statements.track` is refused too. The operator projects `track=all` every reconcile (ADR009 § legacy query-insights convergence) but the controller's merge lets a tenant key of the same name **win**, so a tenant could have blanked the top-queries panel with no other sign. Same class as `shared_preload_libraries`, one line in the same map.

`shared_preload_libraries` deliberately keeps its **silent drop** rather than joining the refusal set: that drop is a published contract (its doc comment, the MCP tool schema) with its own client-side message in the editor. Dropping the new set silently would instead leave a tenant believing they had set `archive_command`. The asymmetry is commented at both sites and asserted by a test.

**t005 — Render parity.** Render exposes tenant-settable parameters and does not expose provider archival internals, so refusing the operator-owned set moves bex toward Render's shape rather than away. No ADR018 divergence row; ADR018's MCP inventory went 184 → 185 (`list_postgres_parameters`, `Extension`) with the pinned test updated in the same commit, as that file demands. ADR009 gained a "Parameter overrides: two reads, one of them editable" section with the declared/observed table and the refusal policy.

**t007 — coverage.** Service-level: `ParameterSpec` empty on a fresh database and sorted after a write; 15 refusal cases including prefix families, case and whitespace variants, and a mixed write proving refusal is atomic; 12 legitimate tuning parameters (`work_mem`, `shared_buffers` — ADR009's own example — `max_connections`, `random_page_cost`, …) proving the guard does not over-refuse. API-level, per the DoD's "by calling the API directly rather than through the dashboard": both the dedicated PUT route and PATCH refuse and name the offender, nothing lands, and a legitimate write still succeeds through the same path. Dashboard: empty editor on a database that declares nothing, a removal saving only the declared survivors, and no Source column.

The guard tests were verified non-tautological by disabling the check — all 15 refusal cases go red.

### What t008 still owes

The remaining DoD bullets are live probes and this session has neither QA credentials (`scripts/qa-login.sh` exits 2) nor a deployed build. Two items must be carried honestly into the closeout:

- **Still unverified, and it was unverified when filed:** what CloudNativePG actually does when `spec.parameters` contains an operator-owned key — reject the Cluster patch, silently win, or fight it in a reconcile loop. The guard now makes it unreachable through the API, so the question is about **existing** databases whose `spec.parameters` may already carry captured values from before this fix. Closeout should check the three production databases for captured keys; if any has them, removing them is a data-repair task, not this milestone's.
- **Key Value was not checked** for the same conflation (the Background flagged it as unverified). It is a separate resource with its own insights surface; if the same shape exists there it deserves its own note.
