# w6 · m156 — Deploy latency watchdog: a green main never waits half a day to deploy

**Worker:** worker6 **Goal:** when a deployable commit on main waits more than 6 h for its production pin, an issue opens automatically, naming why, and the remaining starvation cause is fixed. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure the oldest undeployed deployable commit and classify why it waits](t001.md) | 75m | — |
| t002 | [Scheduled watchdog: open an issue past 6 h, close it on recovery](t002.md) | 60m | w6/m156/t001 |
| t003 | [Replay the last week's long gaps and fix the remaining starvation cause](t003.md) | 75m | w6/m156/t001 |
| t004 | [Simplify](t004.md) | 15m | w6/m156/t002, w6/m156/t003 |
| t005 | [Test coverage](t005.md) | 20m | w6/m156/t004 |
| t006 | [Closeout](t006.md) | 10m | w6/m156/t005 |

## Definition of done

- A script computes, for `origin/main`, the oldest deployable commit (one touching deploy.yml's production-input paths) not yet contained in the current `pin platform images` commit, and its age. It classifies the wait as `main red`, `superseded`, `cancelled while pending`, `waiting for a runner`, `Argo not synced` or `in progress`, from GitHub run and job data.
- A scheduled, read-only workflow (on `bex-ci`) opens one issue when that age exceeds 6 h, naming the commit, the age and the cause. It updates the issue while the condition holds and closes it once a pin includes the commit.
- Replaying the 7 days to 2026-11-02 finds all four gaps over 10 h (10-27→28 15.7 h, 10-28→29 15.6 h, 10-29→30 11.3 h, 10-30→31 24.9 h) and names each one's cause. Any cause that is still structural (e.g. supersession or pending-slot cancellation) gets a fix shipped, or a recorded decision against it, and w6/087's record is updated.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6`, 2026-11-02, proposal 1. Evidence: 40 pins in 7 days (median gap 1.8 h, p90 9.2 h, max 24.9 h). 29 deployable commits waited through the 10-30 17:49 → 10-31 18:40 gap and 16 through 10-27→28. Since 10-30, 60 deploy runs ended 39 cancelled / 5 failed / 16 succeeded. w6/087 closed with a ">12 h ⇒ reopen" rule that nothing checks, so at least two qualifying gaps went unnoticed. `ci-red-streak.yml` only sees repeated failures.
- **Goal linkage:** ADR008 reliable delivery. Finished work must reach production promptly, which every live-verification DoD on the board depends on.
- **Expected outcome:** a stalled deploy is visible within 6 h with its cause named, instead of being discovered by a worker hours later. The remaining starvation cause is fixed.
- **Why now:** w6/m155 has just removed runner contention, one of the causes, and w6/100's interim Argo fix removed another. This is the right moment to measure what is left before the next stall.
- **Render parity:** omitted. CI/deploy infrastructure, no REST/GraphQL/MCP/UI surface.
