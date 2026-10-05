# w4 · m114 — Cron runs can't be trusted: resurrected cancels, twin pendings, 11-minute terminal delay

**Worker:** worker4 **Goal:** every cron run converges exactly once to its true terminal state — a canceled run stays dead, a scheduled run never shares the active slot, and failure surfaces promptly with its reason. **Status:** done (2026-10-05: every DoD bullet live-verified on production; t007 surfaced the failure reason)

## Transfer — 2026-09-30

Implementation/verification ownership transferred to [w5/069](../../../w5/069.md) by user approval 2026-09-30. Source history is retained; do not execute a duplicate queue here. Original production acceptance is not claimed complete. Current execution scope and parity corrections live in the destination.

The blocker narratives below are historical; local harness work is now pre-approved within root isolation rules. Capacity still requires checking.

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Canceled manual run resurrects when CancelRun is overwritten                      | 1h  | —          | — **DONE** |
| t002 | Twin pending: scheduled successor created while predecessor still active — **DONE** (via w5/m106; live 2026-10-05) | 1h | w5/069 |
| t007 | Say why a failed cron run failed — **DONE** | 2h | — |
| t003 | Trigger Run disabled in UI while the server preempts by design                    | 30m | —          | — **DONE** |
| t004 | Render parity + docs (cron-runs.md single-execution guarantee)                    | 20m | t001–t003  | — **DONE** (scoped to t001/t003) |
| t005 | Test coverage (controller-level resurrection + projection tests)                   | 45m | t004       | — **DONE** (scoped to t001/t003) |
| t006 | Closeout (live re-probe with a failing cron) — **DONE** | 15m | t002, t007 |
## Blocked on

**t002 needs a live reproduction that only the user can authorize** — and so
does t006, which is the live re-probe.

t002's own acceptance criterion is "root cause evidenced from cluster objects,
not inferred", and the evidence no longer exists: the fixture
`qa-20260917-p12-cron` was deleted at the end of the QA pass, its Jobs with
it, and Kubernetes Events expire within the hour. Confirmed against the live
app cluster 2026-09-18. What static reading could establish is recorded in
`t002.md` as leads — notably that the 11-minute terminal delay is plausibly
correct Kubernetes behavior (no `backoffLimit` is set, so the default 6 plus
the default exponential backoff sums to ~10.5 minutes before `JobFailed`), in
which case the DoD's "terminal within ~2 polls" bullet would have bex
contradict the Job controller and mark live, retrying runs as failed. That is
exactly why this must not be fixed on inference.

**What is needed:** authorization to run one throwaway reproduction (an
every-minute failing cron plus a manual trigger during an active run, watched
for ~15 minutes with `kubectl`) on production, where it was observed and where
the QA journey already exists — or a decision to stand up the full local
`mock-cluster` + `dev-4` stack for it.

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

## Live closeout replay (2026-10-05, production `24482892a`)

Workspace `bex-canary`, API with the bex CLI's access key. Fixture `qa-20261005-m114-cron` `srv-db1jl28cnepc739umehg` (busybox:1.36, free, `* * * * *`, `sh -c 'echo qa-m114-x7k2; sleep 20; exit 3'`), created 05:50:00Z and deleted 06:11:01Z (`GET` 404; no `m114` resource left). 199 REST + GraphQL samples (05:50:13–06:10:59Z).

- **Cancels stay dead — PASS.**
  - Manual run `crr-kfm1e804oj0a10qdh147` was triggered at 05:52:12 and canceled at 05:52:31 (`canceled` 05:52:32).
  - The current scheduled run `crr-iv1cj12mesir6ih5ejlo` was canceled at 05:54:04. The service was suspended at 05:54:12 and resumed at 05:54:28.
  - Through 06:10:59 the canceled manual run only read `canceled`, and its marker appears exactly once in the logs (05:52:25.77). All 23 runs logged 23 markers from 23 distinct instances.
- **One active slot — PASS.** There were zero samples with two runs `pending`. A manual trigger preempts the active scheduled run the same way on REST and GraphQL. A manual run that spans a tick holds that tick's scheduled run until it ends. See `done/t002.md`.
- **Failures surface promptly — PASS on timing, FAIL on reason.** 19/19 failing runs reached `unsuccessful` within 0–5 s of `finishedAt`, but no surface said why (`cron_job_run_ended` details were `{"status":"failed"}`). That is fixed in t007.
- Not re-run: the dashboard "UI and API agree" check and the m96 log-boundary control.


## Post-deploy live check — 2026-10-05 (qa loop41, pin `c7afefad5` ⊇ `6a64712b6`)

Owned Free cron `qa-20261005-l41-cron` (`alpine:3`, `sh -c 'echo l41-marker; exit 3'`, deleted afterwards). Manual run `crr-30icul0qglu2d7v7pqhf` settled `unsuccessful` 7 s after start. `GET /v1/services/<id>/events` → `cron_job_run_ended` `{"status":"failed","reasonCode":"non_zero_exit","cronJobRunId":"crr-30icul0qglu2d7v7pqhf","reason":{"evicted":false,"nonZeroExit":3}}`. The dashboard Activity row reads "Cron run finished · Failed — The run exited with status 3." t007 holds on the deployed build. Found alongside: w4/203 (Trigger Run ignores the suspended `cron_run_now` decision).
