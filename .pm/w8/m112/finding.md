# Managed Postgres cannot enable documented time-series tables

Why: an ordinary migration cannot enable Render's documented TimescaleDB capability, so applications cannot use its basic hypertables and time bucketing.

**Major · managed PostgreSQL extension delivery/configuration.** Functional CLI sweep453,2026-10-17 UTC. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 against api.bex.co, isolated human QA in tea-daif693dqjvc73e7as3g, noninteractive JSON. Checkout c05648614e7a72b6d827319f06b5ea2aa567a778; actual serving platform SHA/OCI/config unknown. No product implementation or ship in this filing.

## Native reproduction and healthy neighbors

[Render's extension contract](https://render.com/docs/postgresql-extensions) includes TimescaleDB for fresh databases and excludes Community features. This finding exercises only basic Apache-compatible operations, on Bex's existing PostgreSQL17; the fix retains the existing13–18 major set with release compatibility reviewed per major.

Create one fresh explicit Free PostgreSQL17 through the native CLI, wait for available, then run:

```sh
bex psql <owned-id> --command 'CREATE EXTENSION IF NOT EXISTS timescaledb;' --confirm -o json -- -X -q -t -A -v ON_ERROR_STOP=1
render psql <owned-name> --command 'CREATE EXTENSION IF NOT EXISTS timescaledb;' --confirm -o json -- -X -q -t -A -v ON_ERROR_STOP=1
```

Both actual clients exit1 with empty stdout. PostgreSQL reports extension unavailability, a missing `/usr/share/postgresql/17/extension/timescaledb.control` and the package-install hint. The path is server-reported; no administrative filesystem inspection occurred. Four identical before/after catalogs omit TimescaleDB. Actual server17.11 retains vector0.8.6, pg_stat_statements1.11, plpgsql1.0 and available/uninstalled hstore1.8.

Both native connections and both ordinary own-table transactions pass. Eight independently expected initial/insert/update/delete snapshots agree for UTC hourly aggregates spanning both sides of the Unix epoch, Unicode sensor names and signed integer totals. All own schema/data changes roll back. [Complete native receipts and cleanup](evidence/cli-sweep453/complete-native-timeseries-catalogs-controls-and-cleanup.json), [exact SQL and model](evidence/cli-sweep453/exact-SQL-and-independent-UTC-bucket-DML-model.py.txt). Native create/SQL wire was not intercepted.

## Real local consumer

The [immutable official local image](evidence/cli-sweep453/actual-local-Apache-hypertable-results.json) runs TimescaleDB2.22.1 with PostgreSQL17.6/arm64 in actual Apache mode, network none and no published port. After local-only owner/empty-database bootstrap, the normal local owner enables the extension without administrative extension bootstrap. A real hypertable has four chunks and four distinct physical table relations. Its actual time_bucket results match all four independent snapshots, including negative epoch time, microsecond boundaries, Unicode and signed values. Rollback removes the own schema and hypertable catalog entry.

No Community functions, performance or general query-planner claims are made. The source tag2.22.1 resolves to687331da33dce1be390d277cbaa0eef62e6ede9d; source/control/interface hashes are retained. The official image digest and observed module version are separate provenance; no CNPG supplier, production release policy or all-major ABI acceptance is inferred.

Two local harness exclusions remain intact: an unnecessary final setting read refused by the ordinary connection, and a later trial that connected to the image's temporary initialization server before shutdown. No authority was changed. The corrected harness removes that diagnostic and waits for the final Postgres PID1 plus ready SQL socket; the complete consumer then passes. All three exact local containers/anonymous volumes/contexts are removed; the public base image cache remains.

## Boundary, repair and shared scope

The [producer and pinned-consumer trace](evidence/cli-sweep453/contract-dedupe-and-source-research.json) follows native create → POST /v1/postgres → Database/CNPG, then native psql ID/name → connection-info → real external psql → the server extension loader. The SQL error does not originate in a Bex REST SQL envelope or launcher rewrite.

- [database_exports.go:52–59](../../../lego/operator/internal/controller/database_exports.go) selects six ordinary CNPG operand pins. The serving projection consumes the map at [database_controller.go:486–489](../../../lego/operator/internal/controller/database_controller.go); logical export uses its helper at database_exports.go320.
- [database_controller.go:531–547](../../../lego/operator/internal/controller/database_controller.go) configures dedicated preload with pg_stat_statements alone.
- [database_extensions.go:62–70](../../../lego/operator/internal/controller/database_extensions.go) manages vector alone, with one production applicationDatabaseSpec caller at94. No TimescaleDB supplier/configuration exists in current product source or targeted history.

Observed package absence and these source boundaries locate extension delivery/configuration. The actual deployed operand/config is unobserved. Supply reviewed compatible per-major packages and required Apache-mode loading through existing CNPG configuration, preserving pg_stat_statements and appropriate plan bounds. Prove normal migration and basic hypertable SQL; avoid unnecessary blanket activation, new public selectors, CLI forks, tenant package installation or new PostgreSQL majors.

The [primary logical-backup guide](https://www.tigerdata.com/docs/deploy/self-hosted/backup-and-restore/logical-backup) requires restore preparation/completion and warns about parallel or incomplete filtered catalog restoration. Those procedures are unrun here and have a dedicated task. Audit both shared image consumers and the managed application-database producer across existing-parent transition, suspend/resume, backup/export/import/restore and compatible upgrades. Preserve actual chunk metadata/data, continued writes, ordinary SQL, existing extensions and client ABI. Use real selected operands/CNPG SQL fail-before/pass-after tests and fresh native hosted acceptance; neither this diagnostic image nor package presence alone closes the repair.

Whole-board open/blocked/done and targeted product/history searches find no TimescaleDB item or correction. m92/m104/m109/m110/m111 have different module consumers; compose their common supplier mechanism. DO_NOT_DO, ADR009 and ADR018 were checked. Hosted other majors/architectures/lifecycles remain unrun.

Native delete exits0. Detail404, native list absence, former cached endpoint rejection, credential/CA removal, preserved baseline/current inventory and termination0 establish cleanup. The isolated QA login remains for the loop. **Eight tasks/300m** schedule the fix; this new filing is uncommitted.
