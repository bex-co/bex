# w6 · m399 — A CI-failure steer says when the same check also fails on the base branch

**Worker:** worker6 **Goal:** a session steered by a failing check learns when that check fails on the base branch too, so it does not spend a turn fixing a failure its change did not cause. **Status:** blocked — on time and production state: t004's first production check (2026-11-16) found the steer missed a red main, because only the base's head commit was read; the base read now walks the base's newest commits. t004 re-checks once a production deploy carries that fix, the next time main's check is red while an agent PR fails the same check. t001–t003 and t005–t007 done.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | GitHub client: read a failed check's conclusion on the PR's base branch — **DONE** | 1h | — |
| [t002](done/t002.md) | Feedback: a CI-failure steer says the check also fails on the base branch — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The session page shows the base-branch note on a CI-steered turn — **DONE** | 30m | t002 |
| [t004](t004.md) | End-to-end test and a production check of a base-branch failure | 1h | t003 |
| [t005](done/t005.md) | Render parity (Devin): CI-failure steers read alike everywhere — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

A CI-failure steer for a check that also fails on the PR's base branch carries "also failing on <base>@<sha>" in the prompt and the session page's turn origin. Steers for checks that pass on the base branch are unchanged.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` (2026-11-14), item 3. On 2026-11-13, four of five routine CI turns (8.9 to 37.9 min each) fixed a ShellCheck failure in `scripts/ci-schedule-drift.test.sh` that was already red on main.
- **Goal linkage:** ADR008 pillar 5. ADR047 D11 (PR feedback steers the session).
- **Expected outcome:** sessions stop spending turns on failures their change did not introduce.
- **Why now:** main goes red often (its `scripts` CI failed at 00:17 on 2026-11-14), and every open agent PR is steered when it does. The routine prompt covers routines only.
- **Recommendation, not a user decision:** keep steering and annotate, as Devin processes every CI failure. Dropping such steers is a divergence that needs the user's sign-off.
- **Render parity:** included (Devin): the steer and its origin are session surfaces.

## Unblock 2026-11-16 (verified evidence)

- **Affected:** t004, t008. **Source:** verified evidence, no new user decision.
- **Deploy finding:** the production deploy of `3c6654df6` (deploy.yml run 43816470794, started 06:23Z, succeeded 06:46Z) carries `3d0a2296b`, which is its ancestor. Production bex-api now runs the image pinned at `48bc2a0fe`, which also carries it. The deploy gate is clear.
- **Natural occurrence, read-only on production:** two CI-failure steers came after 06:23Z. At 08:29:21Z, session `ags-dbtbg04eqavs73ai1k80` (bex-co/bex#171, base `main`) was steered for `test`, which failed with ShellCheck SC1010 and SC2088 in the stale-key self-test. main's `test` had failed with the same warnings on `199f64120` and `ea837ce20`. The steer carried **no note** (`base_failing_ref` empty), and turn 2 spent 12 min 47 s on main's failure. The other steer, at 10:03Z on bex-co/beancount-io#186, correctly carried none: main's `backend-v2 parity and contracts` was green.
- **Cause:** the base read took only the check runs on the base's head commit. At 08:29Z main's head `a30d7e24e` had `test` still queued (it started at 08:35:45Z), and many main commits never run it (`[skip ci]` image pins, path filters).
- **Fixed in the same ship:** `github.Client.BranchCheckHistory` reads the newest base commits (at most 10, along first parents, under one `checks`/`contents` read token). Each failing check is decided by the newest of them on which a run of that name passed or failed; queued, running, cancelled and skipped runs are read past, as are commits the check did not run on. The read is bounded at 4 s inside the webhook delivery, and a read cut short keeps what it had decided. A regression test replays the 08:29Z history and fails against the head-only read. ADR047 D11 is amended.
- **Remaining gate (clears by itself):** a successful deploy.yml run whose head contains the `fix(agent-sessions): a CI-failure steer reads past a base head whose check has not finished` commit. Then, the next time main's check is red while an agent PR's suite fails the same check, t004 reads the steer's prompt and the turn origin on production (read-only) for the note. Do not turn main red on purpose. Then t008 closes the milestone. Devin is the reference: it processes every CI failure, so the steer is still sent.
