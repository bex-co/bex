# w5 · m123 — Release lifecycle as one pure decision function

**Worker:** worker5 **Goal:** The operator decides wake, park, hold, roll and pre-deploy for a release in one pure function with an exhaustive invariant test, so the five same-day lifecycle fixes can't silently regress. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extract planRelease and make the executors only carry out its plan — **DONE** | 2h | w5/m114/t010 |
| t002 | Exhaustive invariant table over the release facts — **DONE** | 1h | t001 |
| t003 | End the served-release wait when its pods can't be created — **DONE** | 45m | t001 |
| t004 | One wake and park test harness with a lagging cache — **DONE** | 1h | t001 |
| t005 | Render parity — **DONE** | 15m | t002, t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

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

## Evidence — 2026-10-06

**One decision per pass (t001).** In `lego/operator/internal/controller/release_plan.go`:

- `observeRelease` reads the App's Deployment and, for a web or private service, its Service once per pass, and nothing before a release has served. `servingPriorRelease` and its up to three reads per pass are gone; the build hold reads through the same function.
- `planRelease(facts, now)` is pure. It decides `parked` once (suspended, or idle with no rollout awaiting its verdict), whether the pass may stamp Deploying, and one act: roll, wake, run the pre-deploy step, hold on a failed step, hold on a failed rollout, hold while parked, or start the served release first.
- The executors read the plan: `holdNewerRelease`, the rollout's Deploying stamp, `settleHeldRuntime`, `convergeServingRuntime` and the build hold. `plan.parked(app)`, `plan.replicas == 0` and `rolloutAwaitingVerdict`'s own Deployment read are gone. An executor that learns a fact mid-pass (a step that passed on this pass, nothing to restore) records it and carries out the act planned from it.
- Behavior is unchanged. The existing wake, park and hold suites passed against the refactor before any test changed (envtest included). The exceptions were five call sites of `desiredReplicas`, which now returns the plan, and `TestIdleMidRolloutDeferralIsBounded`, which called the removed predicate and now runs through passes (t004).
- One deliberate unification: the deferred park keys on `newerReleaseUnserved`, as every hold does. The old check differed only for an `activeRevision` stored before revisions were `rev-N`.
- The observed scale stays where a settle runs without a plan, or after a failed step ended the pass before its own scale write (`settleFailureOverPriorRelease`, `settleFailedRolloutMessage`). It describes the Deployment as it is, not the pass's decision.

**Invariant table (t002).** `release_plan_test.go` runs `planRelease` over all 165,888 combinations of facts: each flag both ways, every verdict and template, awake replicas 0 and 1, and each clock unset, recent and past its bound. It checks 12 rules:

- the five named here: parked never deploys; a newer release stays off a parked Deployment; the release template goes only over a ready or overdue served release; an idle App does not park before its rollout's verdict; a wake only for an idle service, once, for a release that has not failed;
- the w1/m156 step hold, and that every hold has a served runtime;
- five converse rules, so that removing a fix outright fails too: a failed rollout holds the served release, a failed step never runs again, an idle service wakes for a newer release, an idle service parks once the grace has passed, and an overdue served release lets the newer release roll.

14 mutants of `planRelease`, each removing or weakening one fix, fail the rule they name. `TestReleasePlanLearnsMidPass` covers the two mid-pass facts.

**The served-release wait (t003).**

- `servedWakeBudget` counts from the Deployment's `Available=False` transition, not from pod age. Pods a ReplicaSet cannot create never aged, so the newer release, perhaps the fix, was held indefinitely. An awake service whose only pod was just replaced held a hotfix for the full 5 minutes though it had been down longer.
- The bound is now the same for a wake, a resume and a paid service with no ready pod: 5 minutes after the served release lost availability. The pods List on each poll is gone.
- While the Deployment reports `ReplicaFailure`, Ready reads `ServedReleaseCannotStart` (new in `lego/types`) with the ReplicaSet's message. bex-api classifies it as a stall diagnosis and a failure reason, so the deploy row shows it instead of ordinary progress.
- ADR004 documents the clock, the diagnosis, the paid bound and the one-decision design.

**One harness (t004).** In `lifecycle_fixture_test.go`:

- `lifecycleFixture(t, app, opts...)`, with `withObjects`, `withInterceptor`, `withScheme` and `withLaggingCache`, builds the cluster for every wake, park, hold, suspended-route and idle-decision test. It replaces 29 hand-built fixtures and `failedRolloutFixture`.
- `setDeploymentStatus` with named states replaces the five `mark*` helpers, and the shared lifecycle helpers moved there.
- `laggingClient` serves each pass's App reads from the pass's first read; w5/074's tests use it too. The parking test runs on it and asserts the stored phase after every pass, and the deferral bound is checked through passes.

**Render parity (t005).** REST and MCP (`toRenderDeploy`) and GraphQL (`stallReason`) carry the deploy row's stall reason as text, and the dashboard renders that text with no per-reason logic, so the new diagnosis reads the same everywhere. Render's deploy API has no stall field; bex's is an extra. A Render deploy whose instances cannot start stays in progress and then fails, as bex's does. Match; nothing filed.

**`/simplify` (t006)**, three reviews. Applied:

- The quality review found the table checked each rule in one direction only: removing a fix outright (never waking, an unbounded deferral, a budget that never ends) passed. Five converse rules were added; all 14 mutants fail.
- `deploying` is a method of the plan, not a copied field. The served runtime travels on the replica plan, so `holdNewerRelease`, `settleHeldRuntime`, `convergeServingRuntime` and `convergeServingRoute` lost their separate `prior` parameter.
- Reuse: `progressDeadlineExceeded` reads through `deploymentCondition`, `failedRolloutOverServed` calls `newerReleaseUnserved`, bex-api's failure reasons are `stallDiagnosis` plus `PreDeployFailed` (a new stall reason is one edit), and `reasonRolloutProgressing` replaces two literals.
- Tests: one lagging wrapper; the suspended-route, never-served and activity fixtures converted; getters share one read; one `updateApp` helper; status names prefixed `status`; stale `holdUnservedRelease` comments fixed; the wake log keeps the generation an integer.

Skipped:

- Reading the Deployment and Service only when a hold can apply. The efficiency review measured about 4 µs per pass, under 1% of a pass, and the condition would tie the read to `planRelease`'s rules where the table cannot see them.
- Reusing the observed Deployment in the rollout's `CreateOrUpdate`, which needs a deep copy that costs what the read does.
- Passing `idle` and `replicas` into `facts()`, a named field in place of the embedded plan, and splitting `actHoldStep`: no gain worth the churn.

**Tests (t007).** New: `TestReleasePlanInvariants`, `TestReleasePlanLearnsMidPass`, `TestServedReleaseThatCannotCreatePodsIsDiagnosedThenRolledOver`, `TestHotfixWaitsAtMostTheBudgetFromLostAvailability`, and the new reason in bex-api's stall and failure-reason tables. Rewritten: `TestWakeRollsNewReleaseWhenServedPodStaysUnready` (the availability clock, with a just-replaced pod), `TestIdleMidRolloutDeferralIsBounded` (through passes) and `TestParkingPassNeverWritesDeploying` (lagging cache, stored phase after every pass). Beyond the table's 14 mutants, reverting each fix fails its test:

- the Deploying guard fails the parking test, which sees the Deploying writes;
- the guard together with w5/074's version check fails the same test on the stored phase, which only the lagging cache exposes;
- a budget that never ends fails the three served-release tests;
- dropping the `ReplicaFailure` diagnosis fails the cannot-create test;
- reverting w5/074 alone fails its test on the shared lagging wrapper;
- dropping the reason from bex-api's stall list fails both of its tables.

**Gates.**

- Operator `make test` green (controller 95 s, coverage 86.1%, was 85.9%).
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
- bex-api's store tests pass; `/ship`'s gate runs the real-dependency backend suite before the push.
- t001 depended on w5/m114's closeout. m114's code, the one release-ending verb, shipped; only its live replay is parked, and this refactor does not touch it.
