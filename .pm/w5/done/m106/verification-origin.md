# Verify cron concurrency after the shipped retry correction

Why: Current Job and run-history evidence must resolve the remaining single-run finding without rebuilding retry handling that already shipped.

Estimate: ~60m on a healthy dev-5; inbox-sized. Source: w4/m114 t002/t006, narrowed by shipped w8/028. Approved transfer to worker5 on 2026-09-30; w5/069 owns verification, while the original production closeout remains gated on evidence.

## Steps

1. **Research Render behavior first (~15m).** Read https://render.com/docs/cronjobs#single-run-guarantee and `docs/render-artifacts/cron-runs.md`; record dated sources and any available authenticated capture. Render documents one active run, manual preemption, and delaying the next scheduled run until the current one finishes. It does not establish a no-retry policy. Sequential retries alone do not violate single-run concurrency. Label retry behavior unverified; do not claim `backoffLimit: 0` is proven Render parity.
2. On healthy isolated dev-5, correlate CronJobs, Jobs, Pods, terminal conditions and API/dashboard run history (~30m). Exercise a failing scheduled run, a long run spanning the next scheduled tick, manual preemption, and cancel followed by suspend/resume. Inspect execution overlap, not just two rows whose API status is `pending`. Check delayed scheduling explicitly; do not equate ForbidConcurrent with Render's entire scheduling contract.
3. Record timestamps and resolve or narrow the original finding (~15m). Credit `.pm/w8/done/028.md` and `lego/operator/internal/controller/cron_run_once_test.go` for the already-shipped retry correction. Keep any remaining defect open with exact cluster objects and separately sized follow-up. Update the misleading single-run/retry inference in the original evidence with a dated correction, preserving history.

## Acceptance

- Dated Render comparison distinguishes scheduling, preemption and retry semantics.
- Evidence establishes whether actual executions overlap; canceled executions stay canceled.
- A tick during an active run is checked for delayed execution rather than silently assumed compatible.
- Current retry behavior and terminal-state timing are measured; no live Job is labeled failed just because a timer elapsed.
- w4/m114 t002/t006 are reconciled only to the extent evidence supports. A local pass does not assert the original production DoD ran.
- If a remaining mechanism fix exceeds this verification note, promote/file through /pm without duplicating ownership.

## Files and resources

- `lego/operator/internal/controller/app_controller.go`
- `lego/operator/internal/controller/cron_run_once_test.go`
- `docs/ADR018-render-parity.md` — Cron job row
- `docs/render-artifacts/cron-runs.md`
- `.pm/w4/blocked/m114/README.md` and its t002/t006
- `.pm/w8/done/028.md`
- dev-5 CronJob/Job/Pod objects and corresponding API/dashboard history

Local harness recovery is pre-approved under root instructions within isolation boundaries. Do not change shared-VM settings or another workstream's resources incidentally. No one-off jobs or broader scheduling product is in scope.
