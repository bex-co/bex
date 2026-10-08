# w4 · m181 — Keep pre-deploy jobs out of serving endpoints

**Worker:** worker4 **Goal:** a running pre-deploy command cannot receive service traffic or interrupt the previous release. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Narrow both serving Service selectors to runtime pods | 30m | — |
| t002 | Audit selector consumers and compatibility controls | 30m | w4/m181/t001 |
| t003 | Render parity and deploy-surface verification | 20m | w4/m181/t001, w4/m181/t002 |
| t004 | Simplify changed code | 15m | w4/m181/t003 |
| t005 | Test coverage for serving endpoint membership | 35m | w4/m181/t003 |
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
