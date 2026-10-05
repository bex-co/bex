# w5 · m123 — Release lifecycle as one pure decision function

**Worker:** worker5 **Goal:** The operator decides wake, park, hold, roll and pre-deploy for a release in one pure function with an exhaustive invariant test, so the five same-day lifecycle fixes can't silently regress. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extract planRelease and make the executors only carry out its plan | 2h | w5/m114/t010 |
| t002 | Exhaustive invariant table over the release facts | 1h | t001 |
| t003 | End the served-release wait when its pods can't be created | 45m | t001 |
| t004 | One wake and park test harness with a lagging cache | 1h | t001 |
| t005 | Render parity | 15m | t002, t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- "Parked" is computed once per pass, and so is `servingPriorRelease`.
- The invariant test enumerates every fact combination and asserts the five invariants named in t002; reverting any fix fails it.
- A served release whose ReplicaSet can't create pods stops holding the newer release after the budget and surfaces a diagnosis, and a paid service's young unready pod doesn't delay a hotfix past the documented bound.
- The existing `wake_*` suites pass, and operator `make test` and lint are green.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `e13afdd59` (w1/m172), `5468ec870` (w6/m147), `f5e107410` (w6/076), `27de38ff6` and `c33c9e7d6` (w6/m147 t001/t003): five lifecycle fixes in one area within 24 hours, each adding a branch.
- **Goal linkage:** ADR004 deployment lifecycle; ADR008 free-tier sleep economics (wake/park correctness).
- **Expected outcome:** Lifecycle changes land in one function, with a test that names every invariant.
- **Why now:** The next lifecycle fix should land in a testable seam rather than as another branch.
- **Render parity included (light):** t003 changes the deploy status a stuck release surfaces.
- **Depends on w5/m114** (task dependency on its closeout) **and w5/074**.
