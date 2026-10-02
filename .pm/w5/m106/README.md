# w5 · m106 — Prevent cron overlap during manual preemption

**Worker:** worker5 **Goal:** Keep the recurring schedule paused while a manual replacement cancels prior execution. **Status:** todo (t001/t002/t003 done; live verification and ship blocked by session permissions)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render scheduling, preemption and retry behavior — **DONE** | 20m | — |
| t002 | Correlate live Jobs and Pods and reproduce overlap — **DONE** | 45m | t001 |
| t003 | Keep scheduled runs paused throughout manual preemption — **DONE** | 1h | t002 |
| t004 | Verify live preemption and cancellation across schedule ticks | 40m | t003 |
| t005 | Render parity and surface disposition | 20m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 30m | t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

The reproduced cancel-and-catch-up race no longer overlaps actual container execution. Tests cover pending foreground deletion and scheduled runs absent from API history. Local replay verifies scheduled delay, manual preemption, cancel persistence and suspend/resume, with Job/Pod/API evidence and cleanup. Original production incident acceptance remains with w4/m114; no production or global race-freedom claim follows from a local pass.

## Source + Goal linkage

- **Source:** w5/069, promoted 2026-10-01 after its live verification reproduced a mechanism defect; original note retained in verification-origin.md.
- **Goal linkage:** ADR008 dependable hosting and ADR038/ADR018 Render cron behavior.
- **Expected outcome:** Manual preemption holds scheduling paused through old-Pod termination and replacement creation.
- **Why now:** In disposable dev-5, scheduled Job `tea-davcqapjg4r2nk99chtg-w5-cron-069-29848159` and manual Job `…-run-15622c09` both began execution at 2026-10-01T21:19:24Z after a 21:18:49Z trigger. `cancelPending` forced manualCronRunActive false, reopening the schedule during foreground deletion; catch-up started before replacement.
- **Render parity included:** User-facing cron execution semantics are shared across REST, GraphQL, MCP and dashboard; each uses the same backend intent/operator mechanism.

## Research and initial verification — 2026-10-01

[Render](https://render.com/docs/cronjobs#single-run-guarantee) documents one active run, manual cancel-then-replace, delayed scheduled runs and a twelve-hour limit; no retry policy is established. Existing w8/028 backoffLimit0 policy remains credited. [Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/) documents missed-tick catch-up. Local long run ended before the next scheduled Job started at21:18:32 for its original21:18:00 tick. A failing scheduled run executed once and reached Failed21:16:09 after exit23at21:16:05; API history matched. Then manual preemption reproduced real overlap, saved in `/tmp/w5-069-evidence/overlap-confirmed.json`. Direct one-App reconciliation bypasses the manager watch/cache delivery path; evidence is local mechanism proof, not production incident closure.

## Ship checkpoint — 2026-10-01 local / 2026-10-02 UTC

Implementation is present but uncommitted. Scheduling is paused before foreground deletion and held through replacement; fresh reads identify active Jobs owned by the current CronJob, extra cancellations are persisted before deletion, and canceled history is retained. Focused before/after regressions passed, including unrelated/terminal Job preservation. All three Simplify reviews completed; obsolete parameters/guards and the one-use test wrapper were removed. Full operator `make test` passed, including envtest. The all-module lint run found only excessive complexity in the new test (other modules and dead-code checks passed); its cancellation assertion was extracted into a helper, focused tests passed again, and final operator lint passed with zero issues. Markdown was formatted with the installed Prettier 3.4.2 binary after offline npx could not resolve its package metadata.

Live baseline evidence is retained in [evidence/before.json](evidence/before.json). The fixed replay resumed at 2026-10-01T21:26:15Z but the required manual-preemption/cancel/suspend-resume sequence was not completed before the environment changed. Do not mark live verification or closeout done. Local scheduled-run snapshots alone do not prove the fix's full live acceptance.

**Run-level ship block:** the newly restricted sandbox makes `.git` read-only. `git pull --rebase --autostash origin main` failed opening `.git/FETCH_HEAD` with `Operation not permitted`; no commit or push was attempted through another route. Browser cleanup was also denied (`MCP tool call requires approval, but approval policy is never`). Restore Git write/network access and authorized local-browser access to finish the replay, cleanup, board closeout and ship.

**Cleanup still required:** disposable service `srv-davcqdhjg4r2nk99ci50`, App `tea-davcqapjg4r2nk99chtg-w5-cron-069`, workspace `tea-davcqapjg4r2nk99chtg`, identity `40bcdebd-78a9-44ea-9c06-5f7da064670f`. The last stored CronJob snapshot (2026-10-01T22:04:35Z) has `suspend: false`; current live state could not be rechecked. Stop files for the scoped local reconcile/observer loops were written. Delete the service and workspace before deleting the identity, then verify Kubernetes residue. No other dev stack was touched.
