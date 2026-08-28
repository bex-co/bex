# w6 · m133 — The Postgres parameter editor is seeded from the live `pg_settings` read

**Worker:** worker6 **Goal:** the parameter editor shows and replaces only what the tenant declared, and operator-owned settings — the WAL archive/restore commands, the TLS paths, the replication group — cannot be captured into tenant config. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Expose the declared overrides (`GetParameterSpec`) on GraphQL / REST / MCP             | 40m | —          |
| t002 | Rebind the editor to declared overrides; decide what "Non-default parameters" is       | 40m | t001       |
| t003 | Guard operator-managed and non-configuration parameters in the **backend**              | 40m | t001       |
| t004 | Blast radius: every parameter writer bound by the policy, correct callers regressed      | 40m | t002, t003 |
| t005 | Render parity sweep (REST/GraphQL/MCP/dashboard)                                        | 30m | t002, t003 |
| t006 | Simplify                                                                               | 20m | t005       |
| t007 | Test coverage                                                                          | 30m | t005       |
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
