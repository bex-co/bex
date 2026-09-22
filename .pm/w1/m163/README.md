# w1 · m163 — A cron job or static site whose newest build failed or is still building ignores suspend, resume and schedule edits

**Worker:** worker1 **Goal:** suspend, resume and schedule edits act immediately on cron jobs and static sites even while the newest release has no image, the way they already do for web, private and worker services since `w1/m156`–`m158`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Render evidence: suspend on a cron job or static site acts immediately regardless of the latest build     | 20m | —          |
| t002 | Extend the hold to cron jobs: converge `suspend` and `schedule` on the CronJob serving the prior image     | 50m | t001       |
| t003 | Static sites: suspend and resume over a failed or in-flight build converge the published revision's routing | 40m | t001       |
| t004 | Live: a cron job with a failed newest build stops firing within one schedule tick of suspend              | 45m | t002, t003 |
| t005 | Render parity                                                                                             | 20m | t004       |
| t006 | Simplify                                                                                                  | 15m | t005       |
| t007 | Test coverage                                                                                             | 40m | t005       |
| t008 | Closeout                                                                                                  | 10m | t007       |

## Definition of done

- **Cron.** After suspend, a cron job whose latest deploy reads `build_failed` (or is still building) records no new run; resume and a schedule edit take effect without a new release; the CronJob never carries the unbuilt release's image. Observed live (t004), not reasoned.
- **Static.** A static site over a failed or in-flight build stops serving on suspend and serves its last published revision on resume.
- **Control (must not regress).** A healthy cron job keeps firing; web/private/worker behavior from `m156`–`m158` is unchanged.

## Root cause

- `Reconcile` returns on `resolveDeployImage`'s halt (`app_controller.go:597-600`) before the runtime dispatch (`:2079` → `reconcileCronJob`, where `spec.suspend` is written at `:3545`).
- `holdPendingArtifact` (`:5019`) deliberately returns `held=false` for cron jobs and static sites via `scalableRuntime` (`:4954`); `m156`–`m158` fixed the class for the scalable types and recorded these two as excluded, not verified.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-20, item 3 — traced from code as the last unfixed member of the `w1/m156`–`m158` class.
- **Goal linkage:** `docs/ADR004-app-deployment.md` (a failed deploy leaves the previous revision serving); `docs/ADR018-render-parity.md` (suspend and resume act immediately).
- **Expected outcome:** a tenant can stop a misbehaving cron job or unpublish a static site regardless of build state; no more "suspended but still running" cron jobs.
- **Why now:** the hold mechanism to extend is fresh (`m157`/`m158`) and the exclusion is explicit in code; leaving it is a known, user-facing gap.
- **Render parity:** included — suspend/resume/schedule are exposed on REST, GraphQL, MCP and the dashboard.
