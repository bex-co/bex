# w6 · m399 — A CI-failure steer says when the same check also fails on the base branch

**Worker:** worker6 **Goal:** a session steered by a failing check learns when that check fails on the base branch too, so it does not spend a turn fixing a failure its change did not cause. **Status:** blocked — on time and production state: t004's production check needs the deploy that carries m399, then a red check on main while an agent PR's suite fails the same check. t001–t003 and t005–t007 done.

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
