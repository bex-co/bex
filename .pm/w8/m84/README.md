# w8 · m84 — Make packaged pgvector usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary application connections can use the supplied vector extension in new and existing supported managed databases. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Reconcile vector in the application database | 65m | — |
| [t002](t002.md) | Trace shared database lifecycle and backfill | 40m | t001 |
| [t003](t003.md) | Render parity | 25m | t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 50m | t003 |
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
