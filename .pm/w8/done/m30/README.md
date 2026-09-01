# w8 · m30 — Restore paid KeyValue backup protection

**Worker:** worker8 **Goal:** every paid KeyValue instance has a nightly, encrypted, restorable recovery point again — and the two systemic gaps the 2026-08-25 verification exposed (cutover purge destroying backup history; critical backup alerts firing unseen) are closed so this failure mode cannot recur silently. **Status:** done.

## Tasks (in order)

| id   | title                                                                    | est | depends_on                         |
| ---- | ------------------------------------------------------------------------ | --- | ---------------------------------- |
| t001 | Ship the backup-encrypt 0644 fix and roll the operator — **DONE**        | 30m | —                                  |
| t002 | Re-verify scheduled kvbak success + alert resolution — **DONE**          | 30m | t001                               |
| t003 | Cutover purge semantics: migration deletes must not erase backup history — **DONE** | 45m | — |
| t004 | Route backup-staleness alerts to a human-visible channel — **DONE**      | 45m | —                                  |
| t005 | Complete the queued OpenBao restore drill (run 32814333448) — **DONE**   | 30m | t001                               |
| t006 | KeyValue live restore re-drill (ADR031 cadence trigger) — **DONE**       | 45m | t002                               |
| t007 | Simplify — `/simplify` over the code this milestone changed — **DONE**   | 30m | t001, t003, t004                   |
| t008 | Test coverage — meaningful tests for the shipped behavior — **DONE**     | 45m | t001, t003, t004                   |
| t009 | Closeout — **DONE**                                                  | 15m | t002, t005, t006, t007, t008       |

## Definition of done

Three consecutive scheduled nightly `kvbak-*` runs succeed for every paid KeyValue still in the estate and `BackupCronJobStale` is quiet; the original three-instance set is accounted for explicitly if an instance is deleted before the gate closes. A migration/cutover delete provably cannot purge a recreated instance's backup prefix (guard or runbook gate recorded); backup-staleness alerts reach a channel a human actually sees; the OpenBao restore drill is green and recorded in ADR031; a post-fix KeyValue restore drill has re-earned its ADR031 row.

## Completion evidence

- Both paid KeyValues still in production have four consecutive scheduled successes (2026-08-29 through 2026-09-01), seven retained encrypted objects apiece, and no firing `BackupCronJobStale`. The third incident-era instance was normally deleted after observed successes on 2026-08-28 and 2026-08-29, so it is no longer part of the live estate.
- The cutover annotation preserves the shared per-id prefix while normal deletes still purge; envtest covers both directions and the namespace-cutover runbook requires the guard.
- Provider telemetry marks four critical platform-backup notifications to the operator inbox `delivered` on 2026-08-31.
- OpenBao run `33007385050` restored and verified a fresh encrypted snapshot, cleaned the marker/target, and ended 3/3.
- KeyValue run `33467191581` restored scheduled object `2026-09-01T03:39:20Z.rdb.gz.age`, verified the marker with AOF off and on, tore down the target, and left both sources `Ready`.

## Source + Goal linkage

- **Source:** 2026-08-25 backup verification ([drill record](../../../../docs/drills/2026-08-25-backup-verification.md), ADR031 drill entry); handed off to w8 by user direction 2026-08-25. Supersedes inbox note `w5/049` (moved to `w5/done/049.md`).
- **Goal linkage:** platform data-protection policy ([docs/ADR031-platform-data-backup.md](../../../../docs/ADR031-platform-data-backup.md), [docs/ADR050-encrypted-platform-backups.md](../../../../docs/ADR050-encrypted-platform-backups.md)) — the ≤24 h paid-KeyValue RPO objective was breached at intake; managed-datastore trust is a core Render-parity pillar.
- **Expected outcome:** paid KeyValue RPO back within objective with nightly automation (not manual Jobs); backup history survives future namespace/topology migrations; a silently failing backup can no longer go unnoticed for days.
- **Why now:** at intake, every scheduled kvbak Job since 2026-08-23 was failing and the same-session manual recovery points would age out of usefulness within 24 h. Two production defects compounded into near-zero recovery points for all three paid instances; only detection worked, and nobody saw it.
- **Render parity:** omitted — no REST/GraphQL/MCP/dashboard surface changes; this is operator-internal backup mechanism, platform alerting, and runbook work.
