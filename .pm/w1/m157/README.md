# w1 · m157 — A service whose newest build failed or is still building cannot wake, resume, sleep or scale its serving release

**Worker:** worker1 **Goal:** while a newer release has no image yet (its build is queued, running, waiting on a registry credential, or failed), the release that is actually serving keeps following the App. It sleeps when idle, wakes on a request, suspends and resumes, and scales. The pending or failed release never reaches the pod template, and a build in flight keeps its own polling. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Runtime convergence over a release still waiting for its image: the prior release's replicas and routing follow the App while the build halts | 1h30m | — |
| t002 | Blast radius: every `buildFromSource` halt against every runtime transition, plus the legacy Ready marker and the disk restore | 45m | t001 |
| t003 | Live: resume, idle sleep and wake over a failed build, and a wake while a build runs, on production | 1h | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 45m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Run on throwaway free web services from `bex-co/bex` `examples/hello-go` (docker, `dockerContext: examples/hello-go`) that serve `200`. Only states observed at filing time are listed as failing:

- **Resume over a failed build.**
  1. Set `dockerfilePath` to a missing file and deploy, so the newest deploys read `build_failed` while the first release keeps serving.
  2. Suspend, then resume.
  3. Request the URL every second.

  Within 60 s of the resume the URL answers the prior release's own `200`. At filing time 112 of 113 requests over 180 s got `503 {"error":"service hibernated","retryAfter":5}`, the URL was still `503` 4.5 minutes later, and the phase read Running throughout (`w1/done/m156/README.md` § Live verification, `qa-20260915-m156b`).
- **Idle sleep and wake over a failed build.** Left without requests, the same service records `service_hibernated`; a request then gets the prior release's `200` within 60 s. Traced at filing time: the idle check never runs while the build halts (§ Root cause). `qa-20260915-m156b` stayed Running for 28 idle minutes, but outside scanner requests also reached it, so that half was not isolated.
- **A wake while a build runs.** A hibernated free service whose next deploy is still building answers the prior release's `200` within 60 s of a request. The deploy keeps building and then rolls out normally.
- **The failed or pending release stays off the pod.** After each wake or resume the pod serves the prior release, a failed deploy stays `build_failed`, and no new build Job starts for that release.
- **Control (must not regress).** A healthy free service still resumes and wakes: at filing time `qa-20260915-m156d` answered `200` 11.5 s after resume and `qa-20260915-m156c` 11.5 s after its first request. A successful build still rolls out.

## Evidence (production, 2026-09-15, operator `1cb1f2d27`)

Reproduced during `w1/m156` t003; the full transcripts are in `w1/done/m156/README.md` § Live verification.

```text
qa-20260915-m156b (srv-dakhhdfr0t2c73fc63gg), MESSAGE=m156-build
10:09:22Z  first deploy live; URL 200 m156-build
10:09:23Z  PATCH envSpecificDetails.dockerfilePath ./Dockerfile.qa-missing → dep-dakhikvqniac73emh130 build_failed
10:10:07Z  dep-dakhilfr0t2c73fc63i0 build_failed ("failed to read dockerfile"); phase Running; URL 200 (prior release)
10:38:46Z  suspend → Hibernated
10:38:57Z  resume → 202; service_resumed 10:39:02Z
10:38:58–10:41:59Z  112/113 `503 service hibernated`, 1 `503 no available server`, no 200; phase Running
10:43:25Z  URL still 503; phase Running
```

Controls on the same operator build: `qa-20260915-m156d` (healthy, suspend and resume) `200` in 11.5 s; `qa-20260915-m156c` (healthy, idle sleep and wake) `200` in 11.5 s.

## Root cause (line numbers at `710ebe07e`)

- **The build halts before the runtime.** `Reconcile` returns on `resolveDeployImage`'s halt (`lego/operator/internal/controller/app_controller.go:591-594`) before `dispatchRuntime` → `reconcileKubernetes` (`:595`, `:709`). `resolveDeployImage` halts whenever `buildFromSource` does (`:692-696`). `buildFromSource` halts on:
  - a recorded terminal failure (`:755-757`, and `:923-925` on the quiesced pass);
  - a registry-credential wait (`:776-781`);
  - the build caps (`:804-808`, `:887-890`);
  - a build still waiting for capacity or running (`:941-951`).
- **Why suspend works and resume does not.** A suspended App reuses its serving image (`release_identity.go:316-318`), never reaches the build, and parks normally. Resume clears that. The release's artifact fingerprint still differs from the served one (`release_identity.go:264`), so the pass builds, meets the recorded failure, and halts with replicas at 0.
- **Why the idle check and scale never run.** `desiredReplicas` (`:2021`, `:2301-2324`) runs inside `reconcileKubernetes`. An awake service over a failed build is never checked for idleness, an asleep one is never woken, and manual scale and autoscale are ignored.
- **Terminal failures quiesce.** The recorded-failure halt returns no requeue (`:755-757`), so only a watch event re-enters, and it halts again.
- **Why the tests missed it.** `failed_build_prior_release_phase_test.go` calls `r.fail` directly and pins only the phase. `wake_over_failed_release_test.go` covers the pre-deploy hold only, because `w1/m156` withdrew its build hold in review.

## Blast radius

- **Who is hit.**
  - Any service, free or paid, whose newest build failed: unreachable after a suspend and resume, with manual scale and autoscale ignored.
  - A free service whose newest build failed: unreachable after its next sleep, which a failed build also stops from happening while it is awake.
  - A free service asleep while a push builds: a request gets `503` until the build finishes and rolls, which takes minutes.
- **Types.** Web and private services here. Background workers are `w1/m158`. Cron jobs and static sites have no Deployment to scale.
- **Every halt and transition.** t002 enumerates them.

## Design constraints (from the `w1/m156` review, `w1/done/104.md`)

- **Keep the build's requeue.** A parked App with a build in flight must still come back at the build's cadence, because nothing watches build Jobs. Do not overwrite Building or BuildQueued with Hibernated on every poll.
- **Respect the disk lifecycle.** Converge only after `reconcileDiskLifecycle`, which scales a service to 0 for a restore and must win.
- **Do not run the full runtime on every build poll.** Skip routing writes when nothing changed, complete autoscaling transitions, and persist status only when it changed.
- **Legacy markers.** `recordedBuildVerdict` (`:1046-1055`) falls back to a Ready-only build-failure marker. A status write that replaces Ready must not erase it, or the failed build is dispatched again.
- **The template never advances.** Only replicas and routing move. Writing the pending release's config onto the prior image is the class `w1/m152` (blocked) records for canceled releases.

## Adjacent classes

- **`w1/101`.** "A prior release exists" is decided by `status.image` in the failure-phase settle. This hold keys on `status.activeRevision`, as `w1/m156` does.
- **`w1/m152` (blocked).** A canceled config-change release still ships its config.

## Dedupe

- `w1/done/104.md`: this milestone's source, promoted.
- `w1/done/m156`: the pre-deploy analogue. Its `convergeServingRuntime` and `ingressRoutes` are reused.
- `w6/m124`: the phase over a failed build (Running or Hibernated), which stays.
- `w2/m82` t006: the terminal build gate that stops a failed build re-running, which stays.

## Source + Goal linkage

- **Source:** `w1/104`, promoted 2026-09-15 by user decision during `/loop-worker w1`, after it reproduced live in `w1/m156` t003.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: Render keeps the previous deploy live after a failed deploy, a free service spins up on the next request, and suspend and resume act immediately;
  - `w6/m116`: free-service defaults.
- **Expected outcome:** a failed or in-flight build never takes the serving release offline. Sleep, wake, suspend, resume and scale keep working, and the phase matches what the URL does.
- **Why now:** the most common deploy failure, a broken build, silently and permanently takes a service offline after an ordinary suspend or sleep, and it reproduced on production. `w1/m156` fixed the pre-deploy half, so this closes the class.
- **Render parity is included** because the phase and wake behavior surface through REST, GraphQL, MCP and the dashboard header.
