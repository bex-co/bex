# w8 · m84 — Make packaged pgvector usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary application connections can use the supplied vector extension in new and existing supported managed databases. **Status:** blocked — t001–t005 done; sweep367 verifies fresh custom-identity PG18 through both native clients; t006 still needs hosted existing-parent backfill

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Reconcile vector in the application database | 65m | — — **DONE 2026-10-16** |
| [t002](done/t002.md) | Trace shared database lifecycle and backfill | 40m | t001 — **DONE 2026-10-16** |
| [t003](done/t003.md) | Render parity | 25m | t002 — **DONE 2026-10-16** |
| [t004](done/t004.md) | Simplify | 15m | t003 — **DONE 2026-10-16** |
| [t005](done/t005.md) | Test coverage | 50m | t003 — **DONE 2026-10-16** |
| [t006](t006.md) | Closeout | 25m | t004, t005 |

Six tasks,220m total.

## Definition of done

- Both native clients use fresh permitted Free managed databases with default/custom names and run the original IF NOT EXISTS vector migration plus real vector values/distance/table operations successfully as the ordinary application user.
- The owned CNPG extension resource reconciles for existing parents without changing physical identity or user data. Readiness/errors, version/schema preservation, restore/import/upgrade dispositions, retain/delete and no-op convergence are verified on the supported dependency.
- Maintained actual-CNPG/SQL regressions fail before/pass after; hstore, pg_stat_statements and current client/query surfaces remain correct. Required checks and live/source limits are explicit; preenabled Bex policy is documented honestly.
- Owned fixtures/metadata/private connection files are absent, original baselines remain and quotas terminate at0. Completed tasks/milestone move to done through /pm with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** [functional native SQL sweep353](evidence/cli-sweep353/README.md), [provisioning diagnosis](finding.md).
- **Goal linkage:** ADR008 AI-native usable hosting, ADR009 managed PostgreSQL and ADR018 PostgreSQL compatibility.
- **Expected outcome:** a normal embedding-storage migration can use the vector package already present on the managed server.
- **Why now:** two native vector migrations fail while two hstore controls and normal SQL pass; vector0.8.6 is packaged but uninstalled. Shared source has no vector provisioning, and the locked CNPG dependency already provides the extension mechanism. No matching repair is found.
- **Render parity included:** pgvector is a documented PostgreSQL extension capability; preserve ordinary client contracts and state the preenabled Bex difference explicitly.

## Hosted replay — sweep367

[Fresh custom-identity native acceptance](evidence/live-vector-sweep367/README.md): both original migrations, real vectors/HNSW, expected dimension failures with independent rollback proof and hstore/catalog preservation pass. Exact own fixture/credentials/routes are cleaned. Default identity and hosted existing-parent backfill remain open, so t006 stays todo and this milestone remains blocked.

## Hosted default identity — sweep368

[Render-created default identity acceptance](../../evidence/postgres-native-notifications-sweep368/README.md) now passes both original migrations, real vectors/HNSW, rollback and extension controls through both native clients, with exact cleanup. Together with sweep367, fresh default/custom identities and both native create paths are verified on PG18. Hosted existing-parent backfill remains unverified; t006 stays todo and this milestone remains blocked.

## PostgreSQL13 lifecycle replay — sweep371

[PG13/vector0.8.1 native suspend/resume acceptance](evidence/live-pg13-resume-sweep371/README.md) passes19 data/catalog controls, both original migrations, normal HNSW use, suspend preview, actual connection closure, name-selected Render resume and independent retained-row/index reads. All exact resources/credentials are cleaned. Fresh PG13 and PG18 paths are now live exercised; hosted pre-fix existing-parent backfill remains open, so t006 stays todo and the milestone remains blocked.

## Fresh PostgreSQL14 native acceptance — sweep373

[PG14 large-object and vector neighbor](../../evidence/postgres-large-objects-sweep373/README.md) adds15 passing vector controls on the same own fresh Free PG14 fixture: both original migrations, real HNSW/distance/data, dimension-error transaction rollback, hstore neighbors, catalogs and exact schema cleanup. PG13, PG14 and PG18 fresh paths are now live exercised. Hosted pre-fix existing-parent backfill remains unverified; t006 stays todo and this milestone stays blocked.

## Fresh PostgreSQL15 native acceptance — sweep376

[PG15 isolation and vector acceptance](../../evidence/postgres-native-isolation-sweep376/README.md) adds15 passing vector controls after27 real two-native-client isolation/error recovery stages. Both original migrations, real HNSW/distance/data, expected dimension errors with independent rollback reads, hstore/catalog neighbors and exact schema/database/credential cleanup pass. Fresh PG13/14/15/18 paths are live exercised; hosted pre-fix existing-parent backfill remains open. t006 stays todo and the milestone stays blocked.

## Fresh PostgreSQL16 native acceptance — sweep377

[PG16 binary COPY and vector neighbor](../../evidence/postgres-native-binary-copy-sweep377/README.md) adds15 passing fresh vector controls after the15-command/27-check binary import/export journey. Both original migrations, real HNSW/distance/data, expected dimension errors/independent rollback, hstore/catalog neighbors and exact schema/database/credential cleanup pass. Fresh PG13/14/15/16/18 are live exercised; fresh17 and hosted pre-fix existing-parent backfill remain unverified. t006 stays todo and the milestone stays blocked.

## Fresh PostgreSQL17 native acceptance — sweep378

[PG17 cursors and vector acceptance](../../evidence/postgres-native-cursors-sweep378/README.md) adds15 passing fresh vector controls after32 native cursor/prepared-statement stages. Both original migrations, real HNSW/distance/data, dimension-error rollback, hstore/catalog neighbors and exact fixture/credential cleanup pass. Fresh majors13–18 are now live exercised; hosted pre-fix existing-parent backfill remains open. t006 stays todo and the milestone stays blocked.

## Half-precision, sparse and binary index execution — sweep443

[Fresh PG17 typed-vector acceptance](evidence/live-vector-index-types-sweep443/README.md) passes six real native query transactions through Bex/Render ID/name selectors. Independent Python distances/order and actual three-row HNSW scans agree for halfvec, sparsevec and bit. Four identical catalogs prove schema/index rollback and unchanged preenabled vector0.8.6. Exact database/cached route/credential cleanup passes. Tiny-fixture index execution does not establish general approximate recall or hosted existing-parent backfill; t006 stays todo and m84 stays blocked.
