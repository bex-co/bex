# w2/m93 · t004 — Initial readiness assessment

**Date:** 2026-09-09 · **Mode:** read-only

## Commands

```bash
KUBECONFIG=… bash scripts/verify-workspace-scoped-identity.sh
KUBECONFIG=… bash scripts/registry-migration-readiness.sh
bash scripts/registry-migration-readiness.test.sh
```

## Initial live result

**`insufficient_evidence`** — expected and correct.

Reasons at assessment time (repository + architecture; live Loki after GitOps roll will restate):

1. Continuous coverage of `{type="platform",service="zot|static-server"}` did not exist before this milestone's shipper change.
2. Prior Loki retention was 168h (< 14d), so even if streams had existed they could not prove a 14-day window.
3. Therefore the m92 calendar start (2026-09-08) **cannot** be the evidenced coverage start.

## Earliest eligible Phase 4 date

`max(2026-09-22, evidence_start + 14d)` where `evidence_start` is the first hour after the rolled shipper produces continuous zot + static-server lines under 504h retention. Record the concrete RFC3339 start in `.pm/w2/035.md` once observed live; do not backdate.

F2/F3 remain open. No cleanup executed. Phase 4 authorization remains a separate STOP gate on `.pm/w2/035.md`.

## Hermetic proof

`scripts/registry-migration-readiness.test.sh` — clean / legacy_reads_detected / truncated / dark / gap / source_error / malformed / untombstoned / unscoped all pass.
