# w7 · m67 — Auth-DB off-cluster backups: Barman Cloud plugin for kratos/hydra/openfga

**Worker:** worker7 **Goal:** identity, OAuth-client, and authz data survive correlated storage loss — the three auth CNPG clusters join the same drilled Barman Cloud plugin backup path as `bex-db`. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on   |
| ---- | --------------------------------------------------------------------------- | --- | ------------ |
| t001 | ObjectStore CR `auth-dbs` + out-of-band credential Secret — **DONE**                    | 30m | —            |
| t002 | Plugin blocks + serverNames + nightly ScheduledBackups on the three clusters — **DONE** | 45m | t001         |
| t003 | WAL-archiver staleness alerts for the three auth DBs — **DONE**                         | 30m | t002         |
| t004 | Prod restore drill: kratos-db into a throwaway cluster + drill record — **DONE**        | 60m | t002         |
| t005 | Amend ADR031 (policy table + serverName convention) + ADR012 consequences — **DONE**    | 30m | t004         |
| t006 | Simplify — **DONE**                                                                     | 20m | t005         |
| t007 | Test coverage: fail-closed CI guard — every CNPG Cluster carries a plugin — **DONE**    | 45m | t005         |
| t008 | Closeout — **DONE**                                                                     | 15m | t007         |

## Definition of done

All three auth CNPG clusters (`kratos-db`, `hydra-db`, `openfga-db`) continuously WAL-archive and take a nightly base backup to `s3://bex-tfstate/auth-dbs/<name>-pg<major>/` with 7d retention, verified live in prod (`kubectl get backup -n auth` shows completed backups; the bucket shows `base/` + `wals/` under each serverName). A staleness alert per auth DB fires after >26h without an archived WAL segment. A production restore drill recovered `kratos-db` into a throwaway recovery cluster and verified a known identity row, recorded under `drills/`. ADR031's policy table covers the three stores and ADR012's "no off-cluster backup" consequence is corrected. CI fails if any CNPG Cluster manifest under `deploy/gitops/` lacks a Barman plugin block (allowlisted exceptions only).

## Source + Goal linkage

- **Source:** 2026-07-31 persistent-store backup audit (`/pm` handoff from the conversation that inventoried every store against ADR031; no inbox note — materialized directly). The gap is already on record: `docs/ADR012-auth.md` §Consequences ("auth DBs still lack an off-cluster backup schedule; HA covers a node failure, not correlated storage loss").
- **Goal linkage:** platform durability/security hardening — w7's charter (GOAL.md V0 #7 lineage) and `docs/ADR031-platform-data-backup.md`'s consolidated backup policy, which this completes for the last critical-data holdouts.
- **Expected outcome:** losing an auth-DB PVC (node rebuild gone wrong, volume corruption, cluster loss) no longer permanently destroys every user identity, OAuth client, API-key grant, and OpenFGA tuple; the restore path is drilled, not theoretical.
- **Why now:** these are the only remaining stores holding unrecoverable critical data with PVC-only durability, and the fix is the cheapest on the backup-gap list — the exact mechanism (Barman Cloud plugin v0.13.0 + `ObjectStore` CR + `ScheduledBackup`) is already running and was PITR-drilled on `bex-db` in the same cluster on 2026-07-28.
- **Render parity omitted:** pure platform-infra mechanism (gitops manifests + alerts + runbook); no REST/GraphQL/MCP/UI surface changes.
