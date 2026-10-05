# w5 · m114 — Suspend ends the release it interrupts, not just its deploy row

**Worker:** worker5 **Goal:** Suspending mid-rollout cancels the in-flight release the way Cancel does, so Resume returns to the prior live release and the deploy history stays truthful. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render's suspend-during-deploy and resume behavior | 20m | — |
| t002 | Reproduce suspend mid-rollout followed by resume | 45m | t001 |
| t003 | One release-ending verb for Cancel and suspend | 45m | t002 |
| t004 | Make the close paths agree on pre-deploy status and cancel reason | 30m | t003 |
| t005 | Enforce the closed-row invariant in the database | 45m | t004 |
| t006 | Verify suspend and resume live on dev-5 | 30m | t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

- Suspending mid-rollout cancels both the deploy row (with the suspend reason) and the release (annotation stamped). Resume serves the prior live release, the canceled release's pre-deploy never runs, and a later deploy works.
- A gate-closed pre-deploy step reads `failed`, and `cancelPendingDeploys` rows carry a cancel reason.
- After a backfill, a database CHECK forbids closed rows with `pre_deploy_status = 'running'`.
- A dev-5 replay is recorded.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `60ae617b3` (w4/m171). Code-verified asymmetry: Cancel stamps `AnnotationCanceledReleaseGeneration` (`deploys/service.go:948`) and suspend doesn't. On Resume, the operator's `holdUnservedRelease` rolls the still-requested release with no open deploy row (reviewer reading; no resume test exists). m171's own DoD says Resume returns to the prior live release.
- **Goal linkage:** ADR004 app deployment lifecycle; ADR018 Render parity for deploy statuses.
- **Expected outcome:** Suspend and resume never ship code the user saw canceled, and deploy history stays truthful.
- **Why now:** Shipped on 2026-10-04: any suspend during a deploy followed by resume can run a canceled release's migration.
- **Render parity included:** deploy statuses and cancel reasons are user-visible on every surface.
