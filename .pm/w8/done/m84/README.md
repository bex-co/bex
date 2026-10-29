# w8 · m84 — Make packaged pgvector usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary application connections can use the supplied vector extension in new and existing supported managed databases. **Status:** done (2026-10-29)

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


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m84
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Closeout — 2026-10-29 (`/loopx w8`)

Remaining gate was hosted existing-parent backfill. Verified read-only on production (pin `b33dd43c4428`) with the production kubeconfig — catalog reads and an expression query inside a read-only transaction as each database's own application owner, no writes, no fixture:

| Cluster (created) | Database CR (created) | `vector` ensure / applied | As app owner |
| --- | --- | --- | --- |
| `dpg-d9nqg95cavls73fp8m20` (2026-08-21) | 2026-10-16 | present / true | `vector` 0.8.6; `'[1,2,3]'::vector <-> '[1,2,4]'` = 1 |
| `dpg-d9rrkoc4h4mc73edurp0` (2026-08-21) | 2026-10-16 | present / true | 0.8.6; 1 |
| `dpg-d9rs3ee0ccis738kc7c0` (2026-08-21) | 2026-10-16 | present / true | 0.8.6; 1 |
| `dpg-db34l9bor55s73cqpq9g` (2026-10-07, custom db `qa_r16_data`) | 2026-10-16 | present / true | 0.8.6; 1 |
| `dpg-db9r654flioc739mcpbg` (2026-10-17, PG14, bex-canary) | 2026-10-17 | present / true | 0.8.6; 1 |

The four pre-fix parents (created before the 2026-10-16 fix) got their owned CNPG `Database` resource from the backfill, keep their original cluster identity and creation time, and their ordinary owner can use `vector`. Fresh-parent acceptance across PG13–18, both native clients, default and custom identities, suspend/resume and typed-vector indexes was already recorded in sweeps 367–443. Restore/import/upgrade dispositions are covered by the t005 regressions, not re-run live.
