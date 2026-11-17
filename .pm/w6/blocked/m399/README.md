# w6 · m399 — A CI-failure steer says when the same check also fails on the base branch

**Worker:** worker6 **Goal:** a session steered by a failing check learns when that check fails on the base branch too, so it does not spend a turn fixing a failure its change did not cause. **Status:** blocked — on a natural occurrence only: the base-read fix (`787685319`) has been live on production since 2026-11-16 16:28Z, but main's check has not since been red while an agent PR failed the same check (last read 2026-11-17 18:03Z). t004 checks the next such steer on production. t001–t003 and t005–t007 done.

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

## Unblock 2026-11-17 (verified evidence)

- **Affected:** t004, t008. **Source:** verified evidence, no new user decision.
- **Deploy finding, the deploy gate holds.** `787685319` (`fix(agent-sessions): a CI-failure steer reads past a base head whose check has not finished`) is an ancestor of `db48cde2bae9`. Production pin `fc4fbf28a` ("pin platform images to db48cde2bae9", 2026-11-17 12:08Z, deploy.yml run 44076479949) pins that commit, and production `bex-api` runs its digest `bex-operator@sha256:ced899147db9…` (read at 18:03Z). Every pin since `546945798` (2026-11-16 16:22Z, image `7aaa8700b`, deploy.yml run 43903148613, succeeded 16:28Z) carries the fix, so production has walked the base's history since 2026-11-16 16:28Z.
- **Natural occurrence: none yet.** Read-only, at 2026-11-17 18:03Z, on the `bex-db` replica (`bex-db-1`) plus `gh api`:
  - main's `test` (`scripts (test)`) was red from 2026-11-17 08:09:20Z to 09:11:18Z (failed on `e542e3525`, `3bb04c834`, `144c243f6` and `35b5636b8`; green again on `2337de545`), and from 13:17:06Z to 13:39:43Z (failed on `0bc130b9f`; green on `b7cb2734c`). No agent PR's `test` failed in either window. bex-co/bex#176 (`ags-dbtv8rm5a01c73ddotu0`) and #178 (`ags-dbu0j26c1l2s739d3kcg`) ran `scripts (test)` at 08:52Z and 08:56Z, and both passed.
  - Only two CI-failure steers came after 16:28Z, and both were for checks that passed on main. Both correctly carry no note (`base_failing_ref` empty), so the DoD's "steers for checks that pass on the base branch are unchanged" half holds on production. They are bex-co/bex#172 `backend` at 2026-11-16 17:35:08Z (main's `test (backend)` passed on `28f3d20b4` at 17:09Z) and bex-co/bex#174 `lint` at 20:25:46Z (main's `go lint` passed on `bdbea508f` at 19:10Z).
  - bex-co/beancount-io had no failing agent-PR run after the deploy. Its main went red once (`CI (skills)`, 2026-11-16 20:58Z), and no agent PR failed then.
- **Remaining gate (clears by itself, no owner action):** the next natural occurrence. That is a `ci_failure` steer created after 2026-11-17 18:03Z for a check whose newest passed-or-failed run, among the base branch's newest 10 first-parent commits at the steer's `created_at`, had failed. Do not turn main red on purpose. Devin is still the reference: it processes every CI failure, so the steer is still sent.
- **How a later worker detects it, read-only:**
  1. Fetch the production kubeconfig (root `AGENTS.md`). Pin the control-plane host key per run with `BEX_SSH_KNOWN_HOSTS_FILE` (a `* <key>` file built from `~/.ssh/bex_cp_host_ed25519.pub`); do not edit `~/.ssh/known_hosts`.
  2. On the replica (`kubectl -n bex-system get pods -l cnpg.io/cluster=bex-db -L role`, then `psql -d bex` in its `postgres` container), list the new steers: `SELECT f.created_at, f.session_id, f.source_key, f.base_failing_ref, f.delivered_turn, f.body, s.repo, s.pr_number FROM agent_session_feedback f JOIN agent_sessions s ON s.id = f.session_id WHERE f.kind = 'ci_failure' AND f.created_at > '2026-11-17 18:03Z' ORDER BY f.created_at;`
  3. For each steer, take each failing check named in `body` and find its newest passed-or-failed run on the base before `created_at`: use `gh run list --repo <repo> --workflow '<workflow>' --branch main`, or `gh api repos/<repo>/commits/<sha>/check-runs` over the base's first-parent commits. As a shortcut, `gh run list --repo <repo> --branch main --status failure` lists the red windows to match against.
- **Pass:** when that base run failed, the steer is the occurrence. Its row must have `base_failing_ref` set to the PR's base branch (`main`), and its `body` must say "It also fails on main at <sha>". The session's turn `delivered_turn` must show the note in its origin (`baseFailingRef` on GraphQL, REST and MCP, and the session page's turn origin). Then t004 is done and t008 closes the milestone.
- **Fail:** when the note is missing on an occurrence, t004 diagnoses it as on 2026-11-16. When no steer matched, re-park with the new read time.
