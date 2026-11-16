# w32 · m419 — A setting saved after a failed image deploy takes effect: IP allow-list, suspend and routing keep following the service while the previous release serves, as on Render

**Worker:** worker32 **Goal:** on a prebuilt-image web service (or an image-port private service) whose newest deploy failed because the registry refused its image, every later setting that acts without a deploy reaches the running service within seconds, as it does on a healthy one: the IP allow-list blocks and reopens, a suspend stops it, and the failed deploy still reads `update_failed` with no false outage. **Status:** blocked (t001–t005 done; t006, the production walk, waits for a production pin that contains `1660a95e3`)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Hold the serving release when the newest image release is refused before dispatch — **DONE** | 1h  | —          |
| t002 | Every setting that acts without a deploy converges over a refused image release (blast radius) — **DONE** | 45m | t001       |
| t003 | Render parity — **DONE** | 15m | t001, t002 |
| t004 | Simplify — **DONE** | 15m | t003       |
| t005 | Test coverage — **DONE** | 45m | t003       |
| t006 | Closeout (live walk on production)                                                        | 30m | t005       |

## Definition of done

On production, in `bex-canary`, with the release `bex` CLI and a free image web service (`docker.io/traefik/whoami:<tag>`) that is live:

1. `bex deploys create <srv> --image docker.io/traefik/whoami:<a tag that does not exist> --wait --confirm -o json` exits 1 and the deploy reads `update_failed`. The previous release keeps answering `200`, and the service's events show no `server_failed` (w4/533 still holds).
2. `bex services update <srv> --ip-allow-list cidr=203.0.113.0/24,description=testnet -o json`: within 10 s the URL answers `403` to the walker, whose address is outside that range. `--ip-allow-list cidr=0.0.0.0/0,description=everywhere` then answers `200` again within 10 s.
3. `POST /v1/services/<srv>/suspend`: within 10 s the URL answers `503 This service is suspended.`, and `resume` brings back `200`.
4. The next `bex deploys create <srv> --wait --confirm` (the saved, existing tag) still goes `live`.
5. t002's matrix passes: every row converges on an App whose newest image release was refused, or is recorded as not reachable on that path, with evidence.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep 25 (nonce 3221), 2026-11-15 01:19–01:52 UTC, release `bex` v0.6.0 (Render CLI v2.36.0 pin), `https://api.bex.co/v1/`, production pin `2d9bbae6bb8d`, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. The evidence is below.
- **Goal linkage:** Render parity first (ADR008). Render keeps the previous deploy live after a failed deploy, and its IP rules and suspension act on the service immediately; neither waits for a successful deploy. The IP allow-list is also an access control. A restriction that is saved, reads back and is not enforced exposes a service its owner believes is closed.
- **Expected outcome:** a typo'd image tag, the most common deploy mistake, no longer freezes the service's access controls and lifecycle until the next good deploy. The owner can lock the service down, reopen it, or suspend it while the previous release serves.
- **Why now:** the failure is silent. Every surface reads the saved setting (REST, GraphQL, MCP, dashboard), so nothing tells the owner it is not in force. It also reaches the platform-wide layers: an environment rule (w4/m28) or a new workspace rule (w9/m388, shipped 2026-11-14) never reaches such a service either.
- **Render parity:** included. The REST, GraphQL, MCP and UI shapes do not change, but the ADR018 rows that promise these settings act immediately ("Inbound IP allowlist", Suspend) gain the failed-deploy case.

## Evidence

Fixtures: three free image web services, all owned by this sweep and deleted afterwards (`DELETE` 204, then `GET` 404, URLs 404). `main` = `srv-dbsghdv35ies73daadd0`; `clone` = `srv-dbsglcv35ies73daadig`, made with `bex services create --from <main>`; `ctl` = `srv-dbsgtfcos7qc73a1u6mg`, a healthy control. The walker's address is outside `203.0.113.0/24` (TEST-NET-3). Every `curl` is a plain `GET https://<name>.onbex.co/<path>`.

```text
# clone: its newest deploy fails on a tag that does not exist; the previous release keeps serving
01:30:30 $ bex deploys create <clone> --image docker.io/traefik/whoami:v0.0.99-qa3221 --wait --confirm -o json
         → exit 1, "status": "update_failed"; build log "==> Deploy failed: the image … was not found"
         curl → 200 (previous release)
01:35:51 $ bex services update <clone> --ip-allow-list cidr=203.0.113.0/24,description=qa-3221-testnet -o json
         → exit 0, serviceDetails.ipAllowList = [{203.0.113.0/24, qa-3221-testnet}]; event ip_allow_list_changed
         curl every 5–10 s from 01:35:57 to 01:50:05 → 200 every time (never 403)
01:43:16 POST /v1/services/<clone>/suspend → 202; GET reads "suspended": "suspended", suspenders ["user"];
         `bex services instances <clone>` → []; event suspender_added
         curl every 6–10 s from 01:43:22 to 01:50:05 → 200 every time (whoami answers from instance …-sehe3hoau8mfpugutrgn)

# main: the same change in the other direction locks the owner out
01:38:30 $ bex services update <main> --ip-allow-list cidr=203.0.113.0/24,… → curl 403 from +5 s (healthy, as expected)
01:39:44 $ bex deploys create <main> --image docker.io/traefik/whoami:v0.0.99-qa3221 --wait --confirm → exit 1, update_failed
01:40:04 $ bex services update <main> --ip-allow-list cidr=0.0.0.0/0,description=everywhere -o json
         → exit 0, reads [{0.0.0.0/0, everywhere}]; event ip_allow_list_changed
         curl every 6–7 s from 01:40:10 to 01:42:26 → 403 every time
01:45:42 $ bex deploys create <main> --wait --confirm   (the saved, existing tag)
01:47:21 → live; curl 200 from 01:47:22. The reopen took effect only with the next successful deploy.

# ctl: a healthy control, same plan and image
$ bex services update <ctl> --ip-allow-list cidr=203.0.113.0/24,…   → 403 at +4 s
$ bex services update <ctl> --ip-allow-list cidr=0.0.0.0/0,…        → 200 at +0 s
POST /v1/services/<ctl>/suspend → 202; 503 "This service is suspended." at +5 s
```

### Recheck: a bad tag saved through `services update --image` (sweep 26, 2026-11-15 02:10–02:14 UTC)

Same pin, workspace and CLI. Fixture `srv-dbsh95v35ies73daafcg`, a free `whoami:v1.10.1` web service, since deleted (`GET` 404, URL 404).

```text
$ bex services update <srv> --image docker.io/traefik/whoami:v0.0.98-qaad95 -o json   → saved; curl 200 (no deploy)
$ bex deploys create <srv> --wait --confirm -o json   → exit 1, dep-dbsha34os7qc73a1u9hg update_failed (10 s); curl 200
02:12:46 $ bex services update <srv> --ip-allow-list cidr=203.0.113.0/24,description=qa-ad95-testnet -o json → saved
         curl every ~9 s for 95 s → 200 every time (never 403)
```

The saved-image path (`spec.image`) behaves like the per-deploy override (`spec.releaseConfig.image`), so t002's "a bad tag saved and then deployed" row reproduces on production.

## Root cause (traced at `fd60fd674`; line numbers refreshed at `48c93ceed`)

- **The refused image is re-read on every pass, before anything converges.** For a service whose ports come from its image (`AppSpec.HasImagePorts`: prebuilt-image web services stamped since w4/m243, and image-port private services), `Reconcile` resolves the release's image metadata before dispatch (`lego/operator/internal/controller/app_controller.go:770-779`). On a refusal it returns through `failBeforeDispatch` (`:777`, `:7101`). That function records the failure over the serving release (Rollout condition, `settleFailureOverPriorRelease`), which is what w4/533 needed to stop the false `server_failed`. It then returns, and `dispatchRuntime` (`:1087`), which reconciles the Ingress, the IP allow-list middlewares (`reconcileIPAllowListMiddleware`, `:4421`) and the replica plan that a suspend lowers to zero, never runs.
- **A later save reuses the failed candidate.** `reusableArtifactImage` (`release_identity.go:720`) returns `ActiveReleaseConfig().Image` first. That is the per-deploy override's image (`deploys/service.go:964-966` writes `spec.releaseConfig.image`), or `spec.image` when the bad tag was saved through `services update --image`. Every reconcile that follows an allow-list save, a suspend, an environment- or workspace-rule fan-out, and so on therefore refuses the same image again and stops at `:777`. This lasts until a new deploy changes the release.
- **The hold exists for builds, not images.** `resolveDeployImage` holds the serving release through `holdPendingArtifact` (`:7496`), which converges shared children, replicas and the serving route on the prior release's port. It does so only for a pending source after a recorded build failure, or for a static site (`:1010-1015`, w8/289, w32/003, w1/m157). An image refused before dispatch is not a recorded build failure, so it never reaches the hold.

## Fix

1. **t001.** When `resolveImageNetwork` refuses the candidate and `releaseHasServed(app)`, record the failure exactly as `failBeforeDispatch` does today. Then hold the serving release instead of returning: route through `holdPendingArtifact` (or its image-port equivalent), so routing, middlewares, replicas, suspension and hibernation keep following the App on the serving release's image and network (`Status.ServingNetwork`). Without a served release, the ordinary failure stays. Once a pass has recorded the refusal for the same image and release generation, stop re-reading the registry on later passes.
2. **t002.** Verify every setting that acts without a deploy on such an App, in envtest and where cheap on production. The list: service IP allow-list restrict and reopen, the environment layer (w4/m28), the workspace layer (w9/m388), suspend and resume, free-tier auto-sleep and wake, custom domain add and remove, `numInstances` and autoscaling, maintenance mode, the edge-cache profile, and the image-port private service type. Cover each pre-dispatch refusal reason: 404 not found, 401/403 `ImagePullFailed`, other `ImageRead`. A bad tag saved through `services update --image` and then deployed must behave the same as a per-deploy override.

## Unverified

- The rows of t002 other than the service allow-list (both the override and the saved-image paths) and suspend; the code path predicts them.
- Whether the repeated refusal re-reads the registry on every requeue: `failBeforeDispatch` returns the error, so controller-runtime retries with backoff. Each read may count against Docker Hub's anonymous rate limit.
- **Checked, not affected (sweep 31, 2026-11-15 04:17–04:35 UTC):** a failed rollout, where the image exists but never becomes ready (`ProgressDeadlineExceeded`, w4/m103). Fixture: a free `nginx:1.27-alpine` web service (`srv-dbsj4t735ies73daagvg`, since deleted).
  - With `services update --health-check-path /qa-46cc-nope` (nginx answers 404) and a deploy, the deploy waited, naming the stall ("startup health check has not succeeded … GET /qa-46cc-nope on port 80"). It ended `update_failed` after 920 s, and the old release answered 200 throughout.
  - Then `--ip-allow-list cidr=203.0.113.0/24` answered 403 from +8 s, and reopening answered 200 from +4 s.
  - So the stop is specific to an image refused before dispatch.
- Render's exact timings. Its docs say IP rules and suspension act on the service, and a failed deploy leaves the previous one live. A delay until the next good deploy is not a documented behavior.

## Dedupe

- `rg -n -i 'failBeforeDispatch|w4/533|ImageMetadataUnavailable' .pm docs` → w4/533 (done): stops the false `server_failed` on this path. w4/534 (done): the refusal's wording. Neither covers later saves.
- w1/m157, w8/289, w32/003 (done): hold the serving release after a failed **build**, or for a static site. That is the mechanism this milestone extends to image refusals.
- w4/m243 t007 (done): a failed deploy must not take the live service down. It holds; this is the inverse, a live service that ignores its settings.
- `rg -n -i 'suspend.*keeps serving|after a failed (image )?deploy' .pm` → no open item.
- w4/m418 (filed 2026-11-15 from w4/673; fix `364fff7d4` on `main`, not deployed as of `dfc31e46d`) is a neighbor, not a duplicate. It makes a configuration-only release of an image-backed service rerun the image it is serving instead of the saved tag, through a backend `rollout` helper. This milestone is on the operator side: settings must converge while the saved image is refused. Both pick the image a release runs, so whichever lands second rebases on the other's selection. `364fff7d4` changes only the backend (`rollout.SelectServedImage`, used by environment saves, env-group writes and Blueprint plans). The operator path this milestone fixes (`failBeforeDispatch` before `dispatchRuntime`) is untouched, and so are the settings that act without a release: IP allow-list and suspend. Once deployed, an environment "Save and deploy" after a refused image may run the served image and go live, so t001's regression test should not rely on that path to reproduce the stall.
- Not deploy lag: `app_controller.go:770-779` and `release_identity.go:720-724` are unchanged in substance on `origin/main` `48c93ceed`; the refactors #158 and #161 only moved lines.

## Implementation (2026-11-16, worker32)

- **t001.** Over a served release, `failBeforeDispatch` records the Rollout verdict as w4/533 did, then holds through `holdPendingArtifact`. The serving release's Services, Ingress and middlewares, replicas, suspension and sleep keep following the App, and the pod template is never touched. `settlePriorRelease` reads "the latest image could not be read; the previously deployed release keeps serving". `imageReadFailureRecorded` (the Rollout verdict for this release, with reason `ImagePullFailed` or `ImageMetadataUnavailable`) sends later passes straight to the hold, so the registry is not read again. The verdict is written before the hold, so no exit of the hold can lose it.
  - The verdict was already final: `failedRolloutOverServed` holds such a release off the pod template (`actHoldVerdict`), so a retried read that succeeds could never roll it.
  - A save after the refusal holds too, as w8/289 holds one after a failed build: a `services update --image` back to a good tag leaves the served pod template alone until the next deploy.
  - With no served release, the refusal stays an ordinary failure (`Failed`, Ready `ImagePullFailed`, nothing dispatched).
- **Design note** in [ADR004](../../../../docs/ADR004-app-deployment.md) § A release without an image does not freeze the serving one either.

### t002 matrix

Every row runs on a served `whoami:v1.10.3` image web service (`PortModeWebImageV1`) whose newest deploy the fake registry refused. The specs are in `lego/operator/internal/controller/refused_image_hold_envtest_test.go` ("Settings over a refused image release (w32/m419)"). Each row asserts that the refused release's Rollout verdict stays at its generation (the deploy row stays `update_failed`), that the phase stays Running or Hibernated with Ready on the serving release (no `server_failed`), and that the pod template stays the served one.

| Row | Result | Spec |
| --- | --- | --- |
| Service IP allow-list restrict / reopen | converges: `-ip-allow` Middleware chained, then removed | table, all six entries |
| Environment layer (`spec.environmentIPAllowList`, w4/m28) | converges: `-env-ip-allow` | "converges every other setting" |
| Workspace layer (`spec.workspaceIPAllowList`, w9/m388) | converges: `-ws-ip-allow` | same |
| Suspend / resume | converges: 0 replicas, hosts on the activator's suspended responder (`bex-activator-<app>:8888`), Hibernated; resume brings 1 replica and the route back | table, all six entries |
| Free-tier auto-sleep / wake | converges: route to the activator, then 0 replicas; a fresh last-active wakes it on the serving release | "sleeps and wakes a free service" |
| Custom domain add / remove | converges: the Ingress gains and loses the host | "converges every other setting" |
| `numInstances` | converges: 2, then 1 | same; pserv too |
| Autoscaling | converges: enabling min 2 / max 3 seeds 2 running; disabling returns to 1 | same |
| Maintenance mode | converges: the route moves to the maintenance responder and back | same |
| Edge-cache profile | not an operational setting: `cacheProfile` is release identity (`release_identity.go`, `releaseIdentityInput.CacheProfile`) and bex-api opens a settings-change deploy for it (`apps.SetCacheProfile` → `rollouts().Patch`). That release reads its own image, so over a per-deploy image it deploys the saved tag and goes live | same, last step |
| Image-port private service | converges: suspend, resume, `numInstances`; the Services keep the serving network's port 80; the next deploy goes live | "holds an image-port private service" |

Refusal reasons, each running the allow-list and suspend rows and then a way out: 404 (`DeployReasonImageNotFound`), 401 after the Bearer exchange and 403 (`ImagePullFailed`, access wording), 400 (`ImagePullFailed`, `DeployReasonImageRead` wording), and an image index without the hosting platform (`ImageMetadataUnavailable`). Paths: a per-deploy `--image` override (404, 401, 400, other) and a bad tag saved with `services update --image` and then deployed (404, 403). For the saved paths, saving the good tag again rolls nothing until the deploy. The way out is the next deploy, or a rollback to the served release (401). Each goes live with its own revision.

- **Proof:** with the operator change reverted and the specs kept, 9 of the 10 fail (the allow-list is never projected, or the refused tag is read again on every pass). The tenth, the first-release guard, passes either way by design.
- **Production rows** are the closeout walk (t006): DoD items 1–4 cover the service allow-list and suspend on the override path.

## Gate (2026-11-16)

- **What waits:** t006, the live walk of DoD items 1–5 on production. The operator change shipped in `1660a95e3`. The production pin at the time, `b9f42ac4648e` (pinned by `bc48eb74e`), predates it.
- **Release condition:** the newest `chore(deploy): pin platform images to <sha>` commit on `main` names a `<sha>` that contains the fix: `git merge-base --is-ancestor 1660a95e3 <sha>`. The deploy runs on its own schedule, so the gate clears without anyone acting.
- **Then:** a worker walks DoD 1–5 with the release `bex` CLI in `bex-canary` (free `docker.io/traefik/whoami` fixtures, deleted afterwards with `GET` 404), records the timestamps here, and closes the milestone (t006).
