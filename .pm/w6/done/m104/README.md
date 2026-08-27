# w6 · m104 — Cancel on an image-backed service's deploy never reaches the App CR

**Worker:** worker6 **Goal:** Cancel produces the same honest, self-consistent outcome regardless of whether the App is repo-backed or image-backed — extending `w6/m52`'s already-shipped fix (which corrected `settleCanceledRelease`'s VALUE from `PhaseFailed` to `PhaseCanceled`) to a gate that m52 never widened. **Status:** done — fix shipped in `6c43d439` (annotation stamp moved out of the `if a.Spec.Repo != ""` gate; build-artifact deletion stays repo-gated); backend + operator regression tests green; ADR018 parity ledger updated.

## Tasks (in order)

| id   | title                                                                                                                             | est | depends_on | status      |
| ---- | --------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ----------- |
| t001 | Decide and name the target behavior for both branches of canceling an image-backed deploy, checking Render parity first          | 20m | —          | — **DONE**  |
| t002 | Extend `deploys.Service.Cancel` to stamp `AnnotationCanceledReleaseGeneration` unconditionally, not just for repo-backed Apps     | 30m | t001       | — **DONE**  |
| t003 | Verify the no-prior-release branch: an image-backed App's first-deploy cancel now resolves to `PhaseCanceled`                     | 30m | t002       | — **DONE**  |
| t004 | Verify the prior-release-exists branch: canceling a later deploy reverts the running pod to the previous successful image        | 40m | t002       | — **DONE**  |
| t005 | Blast radius: regression-test cancel across every image-backed resource type; settle the static_site+Image question              | 40m | t003, t004 | — **DONE**  |
| t006 | Confirm REST/GraphQL/MCP show no divergence and the Cancel-dialog copy is accurate for the prior-release branch                   | 20m | t004       | — **DONE**  |
| t007 | Render parity                                                                                                                      | 30m | t005, t006 | — **DONE**  |
| t008 | Simplify                                                                                                                            | 20m | t007       | — **DONE**  |
| t009 | Test coverage                                                                                                                       | 30m | t007       | — **DONE**  |
| t010 | Closeout                                                                                                                            | 10m | t009       | — **DONE**  |

## Definition of done

- Canceling the very first deploy of a fresh image-backed service (Existing Image source, no Git repo) with no prior successful release settles the service to `phase: "Canceled"` (not stuck at `"Deploying"`) within one reconcile — verified via a raw GraphQL `service(id:){phase}` query the same way this hunt confirmed the bug, and the underlying pod stops being recreated against the canceled image.
- Canceling an in-flight deploy of an image-backed service that DOES have a prior successful release causes the running pod's image to actually match the previous release's image, not just the deploy row's `status` field — matching the Cancel dialog's own promise, "The last successful deploy remains live."
- `AnnotationCanceledReleaseGeneration` is stamped by `deploys.Service.Cancel` for both repo-backed and image-backed Apps (grep-verified: no remaining `if a.Spec.Repo != ""` gate around the annotation write), while the build-Job/kpack-Image deletion calls remain correctly scoped to repo-backed Apps only.
- The fix is verified live for at least two of the four affected image-backed types (e.g. background worker and web service) — spot-check of one shared reconcile path, matching `w6/m52`'s own verification shape.
- A genuine build/deploy failure still reports `PhaseFailed` (regression control case from `w6/m52`, must not break), and canceling an already-terminal deploy still returns the existing 409.
- Regression tests cover both branches (no-prior-release, prior-release-exists) for an image-backed App, alongside `m52`'s existing repo-backed control cases; `go test ./...` (operator + backend) and `make lint` all green.
- Render-parity closing task applies: this touches App-CR-level behavior surfaced via REST/GraphQL/MCP + dashboard (`docs/ADR018-render-parity.md` ledger).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 12th run of the day, 2026-08-26.
- **Goal linkage:** [ADR004](../../../docs/ADR004-app-deployment.md) (deploy lifecycle) and [ADR006](../../../docs/ADR006-bex-api.md) (bex-api Render parity, the platform's status-must-agree-with-history contract). Directly extends `w6/m52` (done, `.pm/w6/done/m52/`).
- **Expected outcome:** Cancel produces the same honest, self-consistent outcome regardless of whether the App is repo-backed or image-backed.
- **Why now:** the fix is small and precisely scoped (one gating condition to relax, per m52's own precedent), and the failure mode is worse than what m52 fixed — not merely the wrong terminal value, but no terminal value ever, plus the underlying rollout silently continues unaffected by the "Cancel" the user just performed.

## Live evidence (2026-08-26, workspace `bex`, service `qa-20260826-srcswitch` / `srv-da7a4pqpjh2s73adl3jg`, deploy `dep-da7a4pqpjh2s73adl3k0`)

**Repro:**

1. Created a fresh Background Worker (`qa-20260826-srcswitch`) from Existing Image `docker.io/library/nginx:alpine` (no Git repo — `spec.Repo == ""`, `spec.Image` set). It crash-looped: `nginx: [emerg] chown("/var/cache/nginx/client_temp", 101) failed (1: Operation not permitted)` — expected under bex's non-root tenant sandbox (ADR022), not itself a bug.
2. Clicked Cancel on the still-open first deploy, confirmed through the "Cancel this deploy? The in-progress deploy will be stopped. The last successful deploy remains live." dialog.
3. Deploy detail + Deploys list correctly settled to Canceled: REST `GET /v1/services/srv-.../deploys` returned `{"deploy":{"id":"dep-da7a4pqpjh2s73adl3k0","status":"canceled",...,"finishedAt":"2026-08-26T08:25:08.281025Z"}}`.
4. But the service's own phase never moved. Raw GraphQL query from the page (`fetch('https://api.bex.co/graphql', {credentials:'include',...})`) run at 08:26 (nearly 3 minutes after cancel): `query { service(id: "srv-da7a4pqpjh2s73adl3jg") { id name phase suspended latestDeployId } }` returned `{"phase":"Deploying","latestDeployId":""}`. REST `GET /v1/services/srv-da7a4pqpjh2s73adl3jg` agreed: `"phase":"Deploying"`.
5. Confirmed this isn't just a stale phase value — the underlying pod was still crash-looping in the background at 08:26:01 (same instance `fdt9j`, same `chown` error), a full minute after the cancel confirmation, proving Cancel had zero effect on the running workload.
6. Confirmed the stuck state is recoverable (not a permanent brick): calling the live `setImage` GraphQL mutation to point the service at `docker.io/library/redis:alpine` (a working image) triggered a normal new deploy that reached `phase: "Running"` / deploy `"Live"` ~25s later. The service's env var (`QA_SRCSWITCH_MARKER`) survived the switch (`envVarKeys` still listed it).

**Root cause:** `lego/backend/internal/deploys/service.go:639-707` (`Cancel`). The ONLY code that tells the operator a release was canceled — stamping `appv1alpha1.AnnotationCanceledReleaseGeneration` (line 671) — sits inside `if a.Spec.Repo != "" { ... }` (line 661). For an image-backed App (`a.Spec.Image != ""`, `a.Spec.Repo == ""`), that whole block is skipped; `Cancel` falls straight through to `s.Store.CloseDeploy(ctx, deployID, store.DeployCanceled, "")` (line 696), which closes the DB row but never touches the App CR at all.

On the operator side, `lego/operator/internal/controller/release_identity.go:210-218` (`prepareAppReleaseDecision`) only takes the `canceled: true` branch when `canceledReleaseGeneration(app)` finds that annotation (`release_identity.go:278-282`). Since it's never written for image-backed Apps, `releaseDecision.canceled` is always false, so `lego/operator/internal/controller/app_controller.go:522-524` never calls `settleCanceledRelease` — the reconcile proceeds to `resolveDeployImage`/`dispatchRuntime` exactly as if nothing happened, forever trying to converge the Deployment to the (possibly crash-looping) image, with `app.Status.Phase` stuck at whatever it already was.

This directly extends `w6/m52` (done, `.pm/w6/done/m52/`): that milestone fixed `settleCanceledRelease`'s no-release branch to set `PhaseCanceled` instead of `PhaseFailed` — but only for Apps that actually reach `settleCanceledRelease`. m52's own t003 doc says the branch "is only true once the backend's Cancel verb has stamped AnnotationCanceledReleaseGeneration" and never checked whether that stamp happens for every source kind. It doesn't: it's `spec.Repo`-gated. m52's blast-radius claim ("affects all 5 App-typed service kinds... the mechanism is one shared reconcile path") is true only for the repo-backed dimension — an image-backed instance of any of those 5 types never reaches the fixed code at all. This is worse than the bug m52 fixed: not merely the wrong terminal phase, but no terminal phase ever, and the underlying rollout is left completely unaffected by a Cancel the user just performed and was told succeeded.

**Target behavior (named, not "make consistent"):**

- No-prior-release branch (this repro's case): image-backed Cancel must reach `PhaseCanceled`, exactly matching what m52 already established as correct for repo-backed Apps in the same situation.
- Prior-release-exists branch (a later deploy of an already-live image-backed service gets canceled mid-rollout): Cancel must cause the Deployment to actually revert to serving the previous successful image — not just report "canceled" while a new bad image keeps rolling out. This is what `settleCanceledRelease`'s `if app.Status.Image != "" { return r.dispatchRuntime(ctx, app, app.Status.Image, port) }` branch already does correctly for repo-backed Apps; it needs to be reachable for image-backed ones too. This branch was reasoned from code this run, not live-reproduced (our repro was a first-ever deploy with no prior image) — flagged as **Unverified** below until t004 exercises it live.

**Fix:** extend `deploys.Service.Cancel` (service.go:639-707) to stamp `AnnotationCanceledReleaseGeneration` unconditionally, for both repo-backed and image-backed Apps — keep the build-Job/kpack-Image deletion (lines 675-694) scoped inside `if a.Spec.Repo != ""` since there is no build artifact to delete for an image source. REST, GraphQL, and MCP (`lego/backend/internal/deploys/{rest,graphql,mcp}.go`) all call this single `Cancel` method (confirmed by grep), so one backend fix reaches all three wire surfaces plus the dashboard, which itself calls REST/GraphQL — no per-surface divergence to fix separately.

**Blast radius:** every service TYPE that can be image-backed is affected identically (one shared reconcile path, per m52's own established blast-radius shape) — web, private, worker, cron. Static Site's "New Service" create form DOES offer an "Existing Image" source tab too (confirmed live by navigating to `/services/new?type=static_site`) — untested whether the backend actually accepts/persists `spec.Image` for a static_site or whether `validateTypeSpecificCreate`/the publish pipeline forces it to always be repo-backed; t005 settles this and extends coverage to static_site if the combination is actually reachable.

**Adjacent classes:** a genuine build/deploy failure must keep reporting `PhaseFailed` (m52's own control case, must not regress); a cancel of an already-terminal deploy must keep returning bex's existing 409 (`"deploy %q is already terminal"`, service.go:701) unchanged — this fix only touches the App-CR side-effect of an accepted cancel, not the cancel's own eligibility rules.

**Unverified this run:** the with-prior-release revert-to-previous-image branch (reasoned from code, not live-reproduced — no image-backed service in the QA workspace had a prior successful release before time ran out); whether static_site can actually be created image-backed on the live backend.

**Render:** bex's own code comment (service.go:636) says "matches Render: an image-backed deploy has no build to interrupt in the first place" — that's about the BUILD specifically (there genuinely is none to interrupt for an image source). Whether Render's own Cancel for a Docker-image-backed service reaches a clean terminal state or has the same gap is not verified live this run (no render.com account available) — n/a beyond the existing code comment's framing, which m52 itself already relied on for the repo-backed case.

**Also confirmed this run (deploy-lag, not filed as a bug):** the "source switching in settings" feature (commit `fa7b9993`, `.pm/w5/done/m76`) is NOT yet deployed to production — live GraphQL's `setRepo` mutation only accepts `(id, repo)` with no `branch` argument, while `dashboard/src/features/services/api/branch.graphql` on `main` expects `(id, repo, branch)`; the live dashboard settings page for a service still renders the pre-m76 "Change source type / Switch to a Git repository" UI, none of whose copy exists anywhere in the checked-out repo.
