# w8 · m156 — A running service's build stays in the registry, and a rollback is offered only where its build is stored

**Worker:** worker8 **Goal:** the image a service is running, and the ten most recent builds it has run on, stay in the cluster registry whatever else is pushed, so failed deploys never cost a service its ability to start; a deploy whose build is gone is not offered as a rollback target and is refused at once with a sentence that says so. **Status:** blocked (2026-11-12: t010 needs a cluster where a served release's image cannot be pulled; this host's local mock cluster cannot start worker nodes with `fs.inotify.max_user_instances=128`, see § Blocked. Before that: t001–t005 done and shipped: the retention fix, a rollback to a build that is gone is refused, a missing build's pull failure says so, and a pull failure of the live release is recorded with no deploy open; t010, the service's shown status, and the closing tasks t006–t009 are open)

## Tasks (in order)

| id   | title                                                                                | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | The registry's `live-` retention rule comes before the catch-all — **DONE**          | 30m | —                |
| t002 | Prove the retention rules on a real Zot — **DONE**                                   | 30m | t001             |
| t003 | Rollback refuses a target whose build is no longer stored — **DONE**                 | 60m | t001             |
| t004 | A missing build is reported in product terms, without the registry's host — **DONE** | 30m | t003             |
| t005 | A pull failure on the live release is recorded when no deploy is open — **DONE**     | 45m | t004             |
| t010 | A service whose live release cannot pull its image is not shown as Running           | 90m | t005             |
| t006 | Render parity                                                                        | 20m | t003, t005, t010 |
| t007 | Simplify                                                                             | 15m | t006             |
| t008 | Test coverage                                                                        | 30m | t006             |
| t009 | Closeout (live replay on production)                                                 | 30m | t008             |

## Definition of done

1. A real Zot with the rendered config keeps the ten newest `live-` manifests after more than ten promotions and six later pushes (`scripts/verify-zot-retention.py`).
2. On production, a service with 14 live-in-turn builds, after a cleanup pass: a rollback to the ninth previous build goes live; a rollback to the eleventh answers 409 naming the missing build and adds no deploy row.
3. A Free service with one live build and six later builds that failed at startup, left idle past a cleanup pass: the next request wakes it and it answers from the live build. `bex restart <srv>` on the same service brings up a new instance.
4. The dashboard shows Rollback only on rows whose build is stored.
5. No `failureReason` contains `zot.bex-registry.svc`.
6. A pull failure on a live release with no deploy open writes `image_pull_failed`, and the service is not shown as `Running`.
7. Fixtures deleted afterwards.

## Running services (read before closing)

The rule protects images once the registry runs it: the operator rewrites the config at its first reconcile after the deploy and restarts Zot once to load it (Zot reads its config only at start). An image the old rule already removed is not brought back. A service that is running such a build keeps running, but cannot start a new instance until it is deployed again. Finding those services needs a read of the cluster (each App's live image against the registry), which this worker does not have. It is reported to the user with this milestone.

## Source + Goal linkage

- **Source:** inbox note `w8/266` (promoted 2026-11-11), `/qa-find-bugs-cli` w8 loop sweeps `65de`, `428a` and `d230`. Its finding follows.
- **Goal linkage:** Render parity ("Your service continues running its most recent successful deploy (if any), with zero downtime"; "You can only roll back to a particular deploy if its build artifact is still available") and platform reliability: a healthy service must not go down because other builds were pushed.
- **Expected outcome:** failed deploys cannot evict the live image; rollback offers only targets that can work.
- **Why now:** blocker. Any service with five builds pushed after its live one is one restart, reschedule or wake away from an outage that no rollback can end. Render parity is included: the rollback refusal reaches REST, GraphQL, MCP and the dashboard.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweeps `65de`, `428a`, `d230`)

**Severity:** blocker (a healthy service goes down at its next start and no rollback can restore it; rollback offers targets that cannot work). **Attribution:** infrastructure/runtime (registry retention config) and server (rollback guard). **Estimate:** milestone-sized, ~3h. **Source:** `/qa-find-bugs-cli` w8 loop sweeps `65de`, `428a` and `d230`, 2026-11-11 UTC, released `bex v0.5.0` (Render CLI v2.35.0), `https://api.bex.co/v1/`, production pin `f609163ada8e` at the rollbacks and the outage (`7ca0227aa477` when the first 14 builds were made), workspace `bex-canary`, QA human device login. The pinned CLI has no rollback command; the rollbacks are REST with the CLI's token.

## Evidence

One free native Node service, `qa-20261111-65de-tools`, auto-deploy off. Times are UTC.

**1. Rollback targets.** Between 11:53 and 12:24 UTC it got 14 deploys, D5 to D18, each with a build command that writes its own stamp file, and each went live in turn. The server answers `ok stamp=<file>`, so a response names the build.

```text
$ bex services update <srv> --build-command "echo QA-STAMP-<n> > qa-stamp-<n>.txt" --confirm
$ bex deploys create <srv> --wait --confirm                                  # n = 5 … 18
```

From 15:52 UTC, three and a half hours later, `POST /v1/services/<srv>/rollback {"deployId": …}` against eight of them. Every call answered `201` with `trigger: rollback` and the target's image reference.

| Target | Built image | Builds back from D18 | Result |
| --- | --- | --- | --- |
| D5 | `gen-8` | 13 | `update_failed` after 1 min 51 s |
| D8 | `gen-14` | 10 | `update_failed` |
| D9 | `gen-16` | 9 | `update_failed` |
| D13 | `gen-24` | 5 | `update_failed` |
| D14 | `gen-26` | 4 | `update_failed` |
| D15 | `gen-28` | 3 | `update_failed` |
| D16 | `gen-30` | 2 | `live` in 54 s, answers `stamp=qa-stamp-16.txt` |
| D17 | `gen-32` | 1 | `live` in 44 s, answers `stamp=qa-stamp-17.txt` |

- The service kept answering from its live build through every failure.
- Each failed rollback wrote an `image_pull_failed` event 15 s in ("The image could not be pulled") and closed about 110 s later with this `failureReason`:

```text
image pull is failing: zot.bex-registry.svc:5000/tea-…/tea-…-qa-20261111-65de-tools:gen-8@sha256:93b4c5d3…:
Back-off pulling image "zot.bex-registry.svc:5000/…": ErrImagePull: rpc error: code = NotFound desc = failed to pull
and unpack image "zot.bex-registry.svc:5000/…@sha256:93b4c5d3…": failed to resolve image: …: not found
```

- Nothing warns beforehand. GraphQL `deployActions` reports `rollback … outcome: allowed, precondition: null`, `bex deploys list` shows all 13 older deploys as `deactivated`, and the dashboard offers Rollback on every `deactivated` row (`dashboard/src/features/deploys/lib/deploy-status.ts:47-48`).

The boundary is exactly five tags. Each build pushes `gen-N` and each promotion pushes `live-N` for the same manifest. The five newest at 12:24 were `live-34`, `gen-34`, `live-32`, `gen-32` and one of D16's pair: D18, D17 and D16.

**2. The running service.** After the rollbacks the service ran D16's build, `gen-30@sha256:8f4aca3e…`, from 16:05:29. Then five deploys whose start command exits at once:

```text
$ bex services update <srv> --build-command "echo QA-STAMP-<n> > qa-stamp-<n>.txt" --start-command "sh -c 'echo QA-NEVER-LIVE-<n>; exit 1'" --confirm
$ bex deploys create <srv> --wait --confirm                                  # n = 19 … 23, 16:11 – 16:22
```

Each closed `update_failed` ("container exited shortly after start and is restarting repeatedly") and the service kept answering `stamp=qa-stamp-16.txt`, as it should. The start command was then restored and the service left alone.

```text
16:38:11    event service_hibernated                        (Free, 15 idle minutes)
17:22:02.9  curl https://<service>/         → no answer in 90 s   (edge log: 499 after 89.6 s)
17:22:04    a new instance is created;  17:22:08  event service_woken
17:23:51    curl                            → no answer in 30 s
17:25:12 →  503 {"error":"service not ready","retryAfter":5}       every poll until 17:34
17:27:48    POST …/rollback {D16's deploy}  → 201;  17:29:52 update_failed:
            image pull is failing: …:gen-30@sha256:8f4aca3e…: … failed to resolve image: … not found
17:30:15    POST …/rollback {D18's deploy}  → 201;  17:32:24 update_failed, the same error for gen-34
17:32:34    bex deploys create <srv> --wait → live 17:34:41;  curl → 200 ok stamp=qa-stamp-23.txt
```

- An earlier wake of this service took 12 s.
- D16's deploy carries the image the live release was running, so its failure names the live image: the manifest is gone.
- The outage lasted 12 min 38 s and ended only with a new build. No rollback could end it: D18's build, the newest that had been live, was gone as well.
- While the service was down, the deploy list showed the rollback deploy as `live` and GraphQL `service.phase` read `Running`. Its events were `service_hibernated` and `service_woken`; there was no `image_pull_failed` or `server_failed`. The log held edge request lines and no application line.
- From 16:22 until the cleanup pass the service was one restart away from this, and nothing showed it.

## Expected

- bex's own design: `liveRetentionCount = 10` "so a run of builds that never go live cannot evict the image a running service needs to reschedule … and recent rollback targets survive with it" (`lego/operator/internal/registry/zotconfig.go:50-57`).
- Render, [Rollbacks](https://render.com/docs/rollbacks): "Render retains a fixed number of recent build artifacts for each service, based on your workspace plan." "You can only roll back to a particular deploy if its build artifact is still available."
- Render, [Deploys](https://render.com/docs/deploys), on a failed deploy: "Your service continues running its most recent successful deploy (if any), with zero downtime."

So:

- The build a service is running stays in the registry for as long as it is the live release, and the ten most recent builds it has run on stay, whatever else is pushed. Failed deploys never cost a service its ability to start.
- A deploy whose build is no longer stored is not offered as a rollback target, and the verb refuses it at once with a message that says so.

## Root cause

1. **The `live-` rule is unreachable.** `buildBaseZotConfig` renders two `keepTags` entries in this order: `{patterns: [".*"], mostRecentlyPushedCount: 5}`, then `{patterns: ["^live-.*"], mostRecentlyPushedCount: 10}` (`zotconfig.go:442-448`). Zot gives each tag to the first entry whose pattern matches and applies each entry's count inside its own group: `getTagPolicy` in `pkg/retention/retention.go` returns at the first match (read at tag v2.1.22, the version deployed, `deploy/gitops/base/zot.yaml:27`). `.*` matches every tag, so the second group is always empty and a repository keeps its five newest tags of any kind.
2. **No test runs the rule.** `TestZotConfigStorageMigrationAddsLiveTagRetention` (`lego/operator/internal/registry/creds_test.go:1115`) and the canonical JSON (`:502`) check that the entry is present. `scripts/verify-zot-gc.py` runs a real Zot, with the single `.*` entry (`:50`).
3. **Rollback never asks whether the build exists.** `RollbackEligible` needs a `live` or `deactivated` status and a non-empty `ResolvedImage` (`lego/backend/internal/deploys/actioncaps.go:40-45`). A static site's target is checked against its retained publication (`restorableRollbackTarget`, `:94-103`, w4/m197); an image target is not. w1/m152 t009 left "Build-artifact retention limits on rollback eligibility" out of scope.
4. **Every container start checks the manifest.** `tenantImagePullPolicy` is `Always` (`lego/operator/internal/controller/app_controller.go:1653-1662`), so a node's cached copy does not help once the manifest is gone.

5. **A pull failure outside a deploy is not reported.** `observedImagePullFailure` writes `image_pull_failed` only for an open deploy row (`lego/backend/internal/store/reconciler.go:1436-1462`). A wake, a restart or a reschedule of the live release has no open row, so the service's events stay silent and the activator answers `service not ready` (`lego/operator/cmd/activator/main.go:346-365`). w8/263 is the same gap for a crash.

**What this means for a running service.** Its image stays in the registry only while one of its two tags is among the repository's five newest. Five builds that fail at startup push it out. From the next cleanup pass on (every 3 h, `ZotGCDelay`, `zotconfig.go:71-74`), any new instance of the live release fails to pull: a restart, a reschedule, a Free instance waking. Evidence 2 is the Free wake.

## Fix

1. Put the `^live-.*` entry first and the catch-all second, in the canonical config and in the storage carried onto existing configs (`setZotStorage`, `zotconfig.go:324`).
2. Prove it on a real Zot: extend `scripts/verify-zot-gc.py` with both entries. Push twelve builds with a `live-` tag each and six more without, run retention, and assert the ten newest `live-` manifests and the five newest others survive.
3. Rollback: for a target in the cluster registry, ask the registry for the manifest before accepting. Refuse a missing one with 409, for example "that deploy's build is no longer stored; deploy commit `<sha>` instead". Use the same check in GraphQL `rollbackService`, MCP `rollback_deploy` and the dashboard's row action. An external image is not ours to keep and stays as it is.
4. When a cluster-registry image is missing, say so in `failureReason` in product terms. Do not print the containerd error or the registry's internal host.
5. Report a pull failure on the live release when no deploy is open: write `image_pull_failed` with its alert email, and do not show the service as `Running`.
6. ADR018's Rollback row: record how many builds are kept.

## Acceptance

- A real Zot with the rendered config keeps ten `live-` manifests after more than ten promotions and six later pushes.
- On production, a service with 14 live-in-turn builds, after a cleanup pass: rollback to the ninth previous build goes live; rollback to the eleventh answers 409 naming the missing build and adds no deploy row.
- A Free service with one live build and six later builds that failed at startup, left idle past a cleanup pass: the next request wakes it and it answers from the live build. `bex restart <srv>` on the same service brings up a new instance.
- The dashboard shows Rollback only on rows whose build is stored.
- No `failureReason` contains `zot.bex-registry.svc`.

## Unverified

- When the cleanup passes ran: one between 12:24 and 15:52, the next between 16:22 and 17:22. With the 3 h interval that puts them at 13:22–14:22 and 16:22–17:22.
- The smallest number of failed builds that removes a live image. Five were pushed, after a rollback; four after an ordinary deploy should leave its `live-` tag in the window.
- Paid services were not run. A restart, a reschedule or a scale-out goes through the same pull (root cause 4).
- How long the edge holds a request for an instance that cannot start. The first request was given up by the client at 90 s; from about three minutes in, requests answered 503 at once.
- Whether Zot counts a re-pushed `live-N` tag as newly pushed. It decides which builds stay after a rollback.
- Zot's source was read through a fetch of the tagged file, not run. Its docs say entries are combined with "a logical OR", which reads like a union; the code and the measured boundary say first match.
- Render's per-plan artifact counts are not in its docs.

## Dedupe

w1/m183 (done) added the `live-` rule and the tag push; this is that rule not taking effect. w1/139 (done) is the outage note it came from. w1/m152 t009 left retention out of rollback eligibility. w8/262 is the Free two-deploy limit and pointed here from its Unverified section; w8/261 is the rollback no-op guard. w8/263 is a crash on a live service going unreported; root cause 5 is the image-pull side of it. w4/m197 (done) is the static-site equivalent of the missing check. None covers the rule order or an image target whose build is gone.

## Cleanup

The service was deleted after the recovery deploy; `GET /v1/services/<id>` answers 404.

## Blocked (2026-11-12, `/loopx w8`)

- **Gate:** t010 changes the phase a served release reports when its own image cannot be pulled, which the wake, hibernate, failed-rollout hold and `rolloutPending` paths all read; its context asks for it to be built against a pod that really cannot pull, on a cluster. t006–t009 depend on it.
- **What was tried:** `bash scripts/mock-cluster.sh` (pre-approved) after installing kind v0.33.0, kubectl v1.36.1, clusterctl v1.14.4, helm v4.3.1 and yq into a scratch directory. The management cluster and the app cluster's control plane came up; both worker machines stayed at `CGroupsReady: Waiting for cgroups ready`, and inside them systemd logged `Failed to create inotify object: Too many open files`. The host has `fs.inotify.max_user_instances = 128` and `fs.inotify.max_user_watches = 65536`; raising them needs sudo. The half-built cluster was deleted.
- **To unblock (user, one command):** `sudo sysctl -w fs.inotify.max_user_instances=512 fs.inotify.max_user_watches=524288` (and the same lines in `/etc/sysctl.d/` to keep them), then `/unblock w8` or a new `/loopx w8`; the bring-up and t010 continue from there. Or name another cluster where an image can be removed from the registry under a running service.
- **Production meanwhile:** the retention fix (t001–t005) is deployed (pin `3d66946c9e16`); DoD 2, 3 and 5 can be replayed on production whenever t009 runs.
