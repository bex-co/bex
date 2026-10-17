# Managed Postgres omits the documented text-similarity extension

Why: an ordinary application migration cannot enable Render's documented PostgreSQL 13–15 text-matching extension, so its similarity functions and operators cannot be used.

**Major · managed PostgreSQL package/provisioning.** Functional CLI sweep450, 2026-10-17 UTC. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 against api.bex.co, isolated human QA identity, workspace tea-daif693dqjvc73e7as3g, noninteractive JSON. Checkout source 2a409e808edb19cf05756d2f071f80f061e80336; actual serving platform SHA, OCI image and configuration remain unknown. No product implementation or ship.

## Reproduction and controlled result

[Render's supported extensions](https://render.com/docs/postgresql-extensions) list pg_similarity with an explicit exclusion for PostgreSQL16 and later. The proposed capability therefore covers Bex's existing PostgreSQL13–15 only. No Render production test or new PostgreSQL major is claimed.

Create one fresh Free PostgreSQL15 database with the retained exact native command, wait for available, then run:

```sh
bex psql <owned-id> --command 'CREATE EXTENSION IF NOT EXISTS pg_similarity;' --confirm -o json -- -X -q -t -A -v ON_ERROR_STOP=1
render psql <owned-name> --command 'CREATE EXTENSION IF NOT EXISTS pg_similarity;' --confirm -o json -- -X -q -t -A -v ON_ERROR_STOP=1
```

Both actual clients exit1 with empty stdout. The server reports extension unavailability, a missing `/usr/share/postgresql/15/extension/pg_similarity.control` and the package-install hint. This path is server-reported, not an administrative filesystem inspection. All four before/after catalogs omit pg_similarity. Actual server version is15.19. Vector0.8.6, pg_stat_statements1.10, plpgsql1.0 and available/uninstalled hstore1.8 remain unchanged. Both normal-connection readiness checks and both independent five-row ordinary SQL controls pass, including Unicode/empty values and exact UTF8 lengths; both own schemas roll back.

[Complete native outputs, controls and cleanup](evidence/cli-sweep450/complete-native-similarity-catalogs-controls-and-cleanup.json) and [exact commands/independent oracle](evidence/cli-sweep450/exact-SQL-and-independent-edit-distance-oracle.py.txt) retain every result. Create and SQL wire traffic was not intercepted.

## Local prerequisite and source boundary

The [actual local source-build reference](evidence/cli-sweep450/actual-local-similarity-results.json) compiles unmodified official pg_similarity commit33b022d57231dfd067aa6cef501777714de5c73e, SQL version1.0, in an immutable official PostgreSQL15 image. The package has module/control/SQL files and relocatable=true. No preload is configured. After local bootstrap enables the extension, the normal local owner executes the same IF NOT EXISTS migration, ordinary SQL, five real normalized/raw function cases and threshold operators. Independent ASCII byte-distance results include kitten/sitting3 and normalized4/7, book/back2 and0.5, Euler/Euller1 and5/6; identical Unicode also agrees.

An initial local oracle expected mathematical raw distance0 for empty/empty, while upstream explicitly returns1 before checking raw/normalized mode. That excluded harness mismatch and its cleanup remain retained. The corrected model explicitly records this upstream convention; no Bex issue is assigned to it. The final local reference passes and removes its container, anonymous volume, archive, compiled source and context. It establishes a real consumer, not CNPG compatibility, a production pin or fixed-product acceptance.

The [source and pinned consumer trace](evidence/cli-sweep450/contract-dedupe-and-source-research.json) follows native create → POST /v1/postgres → managed Database/CNPG, then native psql ID/name → connection-info → real external psql → PostgreSQL's extension loader. This is not a REST SQL-envelope defect or launcher rewrite.

- [database_exports.go:52–59](../../../../lego/operator/internal/controller/database_exports.go) selects six ordinary CNPG operand pins, shared by the serving projection at [database_controller.go:486–489](../../../../lego/operator/internal/controller/database_controller.go) and the logical-export client at database_exports.go320.
- [database_extensions.go:62–70](../../../../lego/operator/internal/controller/database_extensions.go) manages vector alone; applicationDatabaseSpec has one production caller at94 and no current major argument. Repository source/history contains no pg_similarity supplier or provisioning.
- Both image consumers and the one application-database producer need an explicit shared audit. Source absence and observed package unavailability support the diagnosis; the exact deployed operand is unobserved.

## Fix, acceptance and dedupe

Compose reviewed immutable compatible13–15 packages with existing operand-supplier work. Make normal application migration/functions/operators usable through the managed application database while preserving vector, retained versions, user data, readiness and export/backup/client ABI. Do not add unsupported16+ availability, a public extension selector, a CLI fork or a blanket preload requirement. Record and verify existing-parent, restore/import/export/upgrade and major-transition constraints, including already-installed objects.

Require real selected-operand/CNPG SQL fail-before/pass-after checks and fresh unchanged native hosted acceptance with exact numeric/function/operator results, preserved catalogs and full cleanup. Catalog presence or this local reference alone cannot close the work. Native input, REST, GraphQL, MCP, dashboard and Blueprint authoring retain their current contracts.

Whole-board open/blocked/done searches and targeted product/history searches found no pg_similarity item or correction. m92/m104/m109/m110 own distinct extension consumers; coordinate their shared supplier rather than duplicate machinery. DO_NOT_DO and ADR009/ADR018 were checked. No paid/shared resource, role/authority probe or product source change occurred.

The first delete attempt exits1 on a TLS handshake timeout during its preflight GET. A later exact-ID read confirms the owned database still exists; one native retry then exits0. Both attempts and the intermediate read are retained; the transient timeout is not a separate issue. Detail404, native list absence, former cached SQL connection rejection, baseline/current inventory and termination0 verify cleanup; the own credential and CA files are removed. Only the continuing isolated QA login remains. Other hosted majors/architectures, indexes, performance and lifecycle transitions are unrun. **Seven tasks/220m** schedule the repair; filing remains uncommitted.
