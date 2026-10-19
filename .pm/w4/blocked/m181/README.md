# w4 · m181 — Keep pre-deploy jobs out of serving endpoints

**Worker:** worker4 **Goal:** a running pre-deploy command cannot receive service traffic or interrupt the previous release. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Narrow both serving Service selectors to runtime pods — **DONE** | 30m | — |
| t002 | Audit selector consumers and compatibility controls — **DONE** | 30m | w4/m181/t001 |
| t003 | Render parity and deploy-surface verification — **DONE** | 20m | w4/m181/t001, w4/m181/t002 |
| t004 | Simplify changed code — **DONE** | 15m | w4/m181/t003 |
| t005 | Test coverage for serving endpoint membership — **DONE** | 35m | w4/m181/t003 |
| t006 | Closeout with the live failure and cancel replays | 20m | w4/m181/t005 |

## Definition of done

Replay these observed journeys on an owned, disposable Free web service. Full requests and captures are in [finding.md](finding.md).

- Create the BusyBox HTTP fixture, set Idle timeout to 1 hr, and verify a successful release serves its marker and image version. Stage the other BusyBox version without deploying it.
- Save `printf 'qa-routing-start\n'; sleep 90; printf 'qa-routing-fail\n'; exit 23` as Pre-Deploy Command. During the running step, inspect both the CR-named and slug-named Services and their EndpointSlices: the healthy application pod is eligible, and the running pre-deploy pod is absent from their endpoints. Regular public GETs keep returning HTTP 200 with the previous image throughout the step and after its failure. The deploy closes `pre_deploy_failed`, with `preDeployStatus: failed` and exit code 23; the previous release remains live.
- From a fresh page, repeat with a bounded successful sleep command and cancel while it runs. Both Services keep excluding the job while it finishes. Public requests keep serving the previous image throughout and beyond the original sleep interval; the deploy remains `canceled` and its step eventually reports `succeeded`, as specified by w5/085. Saved source changes remain pending.
- The deploy list/detail and service header retain the observed truthful separation of failed/canceled deploy from running service. The failed command's marker appears once in All, Build and Application logs, including after a fresh load; preserve the passing w4/180 filtering control.
- Delete the fixture through the UI, verify service REST and public HTTP return 404, verify its children have drained, and revoke only the replay session. Code being written does not close this milestone; the endpoint and public HTTP replays must pass after deployment.

## Source + Goal linkage

- **Source:** live `qa-find-bugs` infinite loop, cycle 7, 2026-10-07 local / 2026-10-08 UTC, credentials privately loaded from `muse.env`, workspace bex (`tea-d98210cbbpdc73dcrkvg`). Fixture `srv-db3jmfs3a6ns73b87ol0`, deleted. See [finding.md](finding.md) for complete durable probes, scope, root cause and dedupe.
- **Goal linkage:** ADR008 reliable hosting; ADR004 migration-before-rollout and preservation of the serving release; ADR041 both internal service addresses; ADR018 deployment parity.
- **Expected outcome:** a pre-deploy job executes without receiving HTTP/TCP service traffic. The existing healthy release stays available through a failed or canceled rollout.
- **Why now:** the awake cancel replay returned 17 HTTP 502s in 69 requests; a separate exit-23 replay returned six 502s in 12 requests. The live EndpointSlice admitted the migration pod as ready on port 3000 alongside the ready application pod. This defeats w1/m33's availability guarantee even though the deploy terminal states are correct.
- **Render parity included:** the tenant-visible deployment availability contract changes. Verify the existing REST/GraphQL/MCP entrypoints and dashboard behavior without inventing new API shapes or changing cancellation's migration-completion policy.
- **Estimate:** six tasks, ~2h 30m. This filing schedules the fix; it does not implement it.

## Progress (2026-10-08)

t001–t005 done. `privateServiceProjection` (`image_network.go`) is the one projection behind both stable Services: `applyClusterIPService` for the primary, slug alias and image-network promotion. It now selects `app.bex.co/app` **and** `bex.co/app-id` (`appIDOrName`, so hand-applied Apps use their name, matching `appPodLabels`). Pre-deploy, build and disk-backup Job pods carry only the first label, so they leave the endpoints. Image-port Services keep their revision constraint. The immutable Deployment selector, Job ownership, log discovery and the selectorless platform ExternalName aliases are unchanged. Key Value/Postgres Services are separate projections.

Safety: every runtime pod template has carried `bex.co/app-id` since 2026-07-16 (`4c10d0066`). A read-only production check on 2026-10-08 found 19 `app.bex.co/app` pods; the only one without `bex.co/app-id` was a succeeded build Job pod. So no serving pod drops out of a Service when the selector changes.

Tests: `TestServingSelectorExcludesJobPods` checks that the selector admits the runtime pod and the previous revision's pod, with and without a public id, and refuses pre-deploy, build and disk-backup Job pods. It fails on the old app-only selector. `TestImageNetworkLeavesLegacyServicesUnchanged` now pins the absence of a revision constraint, not the selector size. Operator `make test` and `make lint` pass. ADR004 records the rule.

Not verified until the live replay: EndpointSlice membership of both Service names while a step runs, public 200s through failed and canceled steps, and the deploy-surface and log controls.

## Hosted probe — qa pass c5e2b8 (2026-10-19, not the closeout)

A `/qa-find-bugs w4` pass ran this on production, pinned to `456090929`, which contains the fix. bex-canary; the owned Free web service `qa-20261019-c5e2b8-predeploy` (`srv-dbansjq5p3cc73ad8lj0`, image `python:3.13-alpine`, `python3 -m http.server $PORT`) was deleted (204 → 404), and only that run's session was revoked.

- **Observable half only.** During the same failing 90 s pre-deploy (`dep-dbant5i5p3cc73ad8ll0`, 02:21:10–02:22:55Z, exit 23 → `pre_deploy_failed`), a 1 s probe of the public URL returned **160/160 `200`** from the previous release (02:20:26–02:24:49Z).
- The EndpointSlice membership check (Job pod excluded from both Services) needs cluster access and was **not** run. The canceled-successful-step control was not run either.
