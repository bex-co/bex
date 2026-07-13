# w1 · m27 — Close the control-plane disaster-recovery gaps

**Worker:** worker1 **Goal:** All three of bex's critical stores (etcd, OpenBao, control-plane Postgres) have both a real backup and a *proven* restore path — today only etcd can claim that; `bex-db` (bex-api's entire control-plane store) has zero backup at all. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                                                     | est | depends_on           |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Add `spec.backup.barmanObjectStore` to `bex-postgres/cluster.yaml`, mirroring the `etcd-backup-s3`/`openbao-backup-s3` Secret pattern against the same Hetzner Object Storage bucket (its own prefix); enable scheduled backup + WAL archiving for PITR | 45m | —                     |
| t002 | Create the `bex-db-backup-s3` Secret out-of-band per the existing runbook convention; verify a real backup actually lands in object storage                                                                             | 30m | t001                  |
| t003 | Perform and document a real `bex-db` restore drill: recover to a new CNPG cluster via `bootstrap.recovery` (the same mechanism `docs/ADR009-postgresql-management.md` already documents for tenant Postgres), verify a known real control-plane row survives | 45m | t002                  |
| t004 | Add an Alertmanager rule for `bex-db` backup staleness to the `w3/m6` rule pack (same pattern as `w1/m26/t004`'s Zot rule)                                                                                                | 20m | t001                  |
| t005 | Perform the OpenBao restore drill for real, closing `ADR015`'s standing ⚠ warning: real snapshot → real restore per the documented runbook → verify a known tenant secret is recovered and unseal succeeds with the existing `.env` keys | 45m | —                     |
| t006 | Re-verify the etcd restore runbook against the current post-`m19` multi-node prod topology — the existing "What was tested" note is from 2026-07-05 on a single-node local CAPD cluster, predating the rearchitecture   | 30m | —                     |
| t007 | `docs/ADR031-platform-data-backup.md`: document bex's platform data-backup policy across all three stores (what's backed up, where, retention, restore mechanism) and record the drill results from t003/t005/t006; update `ADR011`/`ADR015`'s stale warnings in place (ADR030 is reserved by `w8/m7/t003`'s pricing doc — don't collide) | 40m | t003, t005, t006      |
| t008 | Simplify: run `/simplify` over the config/code this milestone changed                                                                                                                                                    | 30m | t007                  |
| t009 | Test coverage: where feasible, a CI-checkable assertion that `bex-db`'s backup config is present (e.g. a structural test analogous to `tier_resources_test.go`'s pattern); document the rest as drill records (most of this milestone is infra/runbook, not unit-testable) | 30m | t007                  |
| t010 | Closeout: verify DoD, mark done, move to `w1/done/m27/`                                                                                                                                                                  | 15m | t008, t009            |

## Definition of done

`bex-db` has a real, running backup with a verified restore path proving real control-plane data survives; OpenBao's restore path has been executed for real at least once (closing the standing ⚠ warning); etcd's restore runbook is re-confirmed against the current multi-node topology; all three are documented in one consolidated platform-data-backup ADR.

## Source + Goal linkage

- **Source:** codebase survey during `/pm-brainstorm more milestones to work on`, 2026-07-13 (`deploy/gitops/charts/bex-postgres/cluster.yaml:7`, `docs/ADR015-openbao-backup-restore.md:90`).
- **Goal linkage:** platform reliability — w1's own founding charter, first ordering principle ("de-risk the live system"); this is the highest-severity unaddressed gap a systematic sweep found.
- **Expected outcome:** all three of bex's critical stores (etcd, OpenBao, control-plane Postgres) have both a real backup and a *proven* restore path — today only etcd can claim that.
- **Why now:** `bex-db` currently has zero backup and holds unrecoverable, non-git-sourced state (unlike etcd's App CRs, which are also GitOps-sourced elsewhere); OpenBao's untested-restore warning has stood since `m19.1`. This is real, present risk, not speculative hardening.
- **Render parity omitted:** pure infra/mechanism (GitOps charts, Alertmanager rules, ADR docs) — no REST/GraphQL/MCP/UI surface.
