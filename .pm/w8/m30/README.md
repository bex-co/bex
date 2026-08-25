# w8 · m30 — Restore paid KeyValue backup protection

**Worker:** worker8 **Goal:** every paid KeyValue instance has a nightly, encrypted, restorable recovery point again — and the two systemic gaps the 2026-08-25 verification exposed (cutover purge destroying backup history; critical backup alerts firing unseen) are closed so this failure mode cannot recur silently. **Status:** in progress — t003 done; t001/t004/t005 code-complete in the working tree awaiting `/ship` (t004 root cause found: bex.co has no MX record, so all 512 Alertmanager emails to the placeholder address were undeliverable; receiver now targets the operator inbox. t005 root cause found: the drill workflow's env predated ADR050 encryption and never passed AGE_BACKUP_PRIVATE_KEY — fixed + drift-guarded in `scripts/github-actions-validate.sh` 4b). t002/t006/t009 gated on the ship + nightly evidence.

## Tasks (in order)

| id   | title                                                                    | est | depends_on                         |
| ---- | ------------------------------------------------------------------------ | --- | ---------------------------------- |
| t001 | Ship the backup-encrypt 0644 fix and roll the operator                   | 30m | —                                  |
| t002 | Re-verify scheduled kvbak success + alert resolution                     | 30m | t001                               |
| t003 | Cutover purge semantics: migration deletes must not erase backup history — **DONE** | 45m | — |
| t004 | Route backup-staleness alerts to a human-visible channel                 | 45m | —                                  |
| t005 | Complete the queued OpenBao restore drill (run 32814333448)              | 30m | t001                               |
| t006 | KeyValue live restore re-drill (ADR031 cadence trigger)                  | 45m | t002                               |
| t007 | Simplify — `/simplify` over the code this milestone changed              | 30m | t001, t003, t004                   |
| t008 | Test coverage — meaningful tests for the shipped behavior                | 45m | t001, t003, t004                   |
| t009 | Closeout                                                                 | 15m | t002, t005, t006, t007, t008       |

## Definition of done

Three consecutive scheduled nightly `kvbak-*` runs succeed for all three paid KeyValue instances (both tenant namespaces) and `BackupCronJobStale` is quiet; a migration/cutover delete provably cannot purge a recreated instance's backup prefix (guard or runbook gate recorded); backup-staleness alerts reach a channel a human actually sees; the OpenBao restore drill is green and recorded in ADR031; a post-fix KeyValue restore drill has re-earned its ADR031 row.

## Source + Goal linkage

- **Source:** 2026-08-25 backup verification ([drill record](../../../docs/drills/2026-08-25-backup-verification.md), ADR031 drill entry); handed off to w8 by user direction 2026-08-25. Supersedes inbox note `w5/049` (moved to `w5/done/049.md`).
- **Goal linkage:** platform data-protection policy ([docs/ADR031-platform-data-backup.md](../../../docs/ADR031-platform-data-backup.md), [docs/ADR050-encrypted-platform-backups.md](../../../docs/ADR050-encrypted-platform-backups.md)) — the ≤24 h paid-KeyValue RPO objective is currently breached; managed-datastore trust is a core Render-parity pillar.
- **Expected outcome:** paid KeyValue RPO back within objective with nightly automation (not manual Jobs); backup history survives future namespace/topology migrations; a silently failing backup can no longer go unnoticed for days.
- **Why now:** the nightly pipeline is failing **today** — every scheduled kvbak Job since 2026-08-23 has failed; the same-session manual recovery points age out of usefulness within 24 h. Two production defects compounded into near-zero recovery points for all three paid instances; only detection worked, and nobody saw it.
- **Render parity:** omitted — no REST/GraphQL/MCP/dashboard surface changes; this is operator-internal backup mechanism, platform alerting, and runbook work.
