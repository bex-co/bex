# w4 · m114 — Cron runs can't be trusted: resurrected cancels, twin pendings, 11-minute terminal delay

**Worker:** worker4 **Goal:** every cron run converges exactly once to its true terminal state — a canceled run stays dead, a scheduled run never shares the active slot, and failure surfaces promptly with its reason. **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Canceled manual run resurrects when CancelRun is overwritten                      | 1h  | —          |
| t002 | Twin pending: scheduled successor created while predecessor still active          | 1h  | —          |
| t003 | Trigger Run disabled in UI while the server preempts by design                    | 30m | —          |
| t004 | Render parity + docs (cron-runs.md single-execution guarantee)                    | 20m | t001–t003  |
| t005 | Test coverage (controller-level resurrection + projection tests)                   | 45m | t004       |
| t006 | Closeout (live re-probe with a failing cron)                                      | 15m | t005       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Cancels stay dead.** Trigger a manual run, cancel it, cancel any other run, suspend/resume (or just wait a reconcile): the canceled manual run never returns to `pending` and never executes again.
- **One active slot.** With an every-minute failing cron, the run list never shows two `pending`/`Running` runs at once; a manual trigger during an active run either preempts cleanly (exactly one active run at every instant) or is rejected everywhere consistently.
- **Failures surface promptly.** A run whose attempts stop (backoff exhausted or wedged) reaches `unsuccessful` within ~2 reconcile polls of its last attempt, with the Job condition reason visible; no 10+-minute silent `pending` tail.
- **UI and API agree on trigger-during-active.** Either the dashboard allows Trigger Run with a preemption warning, or the server rejects it like the dashboard implies — not one of each.
- **No regression to the m96 log-boundary proof.** The 3-record live/history convergence this pass demonstrated stays covered (t005 pins it; the milestone must not break it).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-17 (w4-targeted run, `muse.env` credentials). Fixture: cron job `qa-20260917-p12-cron` (`srv-dam3habs0ils73bgp0v0`, deleted after the pass), schedule `* * * * *`, command `echo p12fail-marker; exit 3`. Full incident timeline with run ids, attempt timestamps, and GraphQL responses is in t001/t002.
- **Goal linkage:** the single-execution guarantee `docs/render-artifacts/cron-runs.md` states (and w6/039 built the suspend-mitigation for) plus run-history truthfulness on the dashboard Events page. A user watching Recent Runs currently sees canceled runs come back executing, two Running rows at once, and failures landing 11 minutes after the last attempt.
- **Expected outcome:** run-status convergence a user can believe (cancel means cancel; one active run; prompt terminal states), with the trigger/tick race closed or honestly documented.
- **Why now:** t001's defect is a live footgun with a known one-line-family fix (the recreation guard consults a single-slot intent that any later cancel overwrites); t002's paradox (successor scheduled run created mid-flight) questions the ForbidConcurrent story and needs cluster truth before it bites a paying cron.
- **Explicitly out:** the m96 log-boundary fix (re-probed live this pass: 3 records render 3 in both live and history, whitespace/empty/unterminated edges byte-identical — holds, not re-filed); client+server schedule-expression validation (both reject, not re-filed); suspend/resume transport itself (both flip phase correctly, not re-filed).
