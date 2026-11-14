# w4 · m351 — A first deploy whose start command crashes is marked failed in about 40 s but keeps restarting for 15 minutes, so a crash that clears serves traffic under a deploy that reads Failed

**Worker:** worker4 **Goal:** once a never-served release settles failed, it stops, whichever path settled it. A crash-settled first deploy has no running or restarting instance and comes back only through a new deploy, restart or resume, as w1/m179 promised and as Render cancels a failed deploy. **Status:** todo (t001–t004 done 2026-11-14; t005 replays the live DoD after the next production deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Stop a never-served release when the startup-crash settle fails it, not only at the progress deadline — **DONE** | 45m | — |
| [t002](done/t002.md) | Decide and align the pull-failure early settle: stop it too, or keep its deploy open — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Simplify — **DONE** | 15m | t002 |
| [t004](done/t004.md) | Test coverage — **DONE** | 30m | t002 |
| [t005](t005.md) | Closeout (live replay on production) | 30m | t003, t004 |

## Definition of done

All on production, on Free busybox image web services created with `POST /v1/services`:

1. **A crash stops when it fails.** Use `dockerCommand: sh -c 'echo qaX starting; echo "qaX fatal" >&2; exit 3'`. Poll the deploy, `GET /v1/services/{id}/instances` and GraphQL `service { phase replicas }` every 10 s.
   - The deploy reads `update_failed` with the existing "container exited shortly after start … (last exit code 3)" reason, as today.
   - Within 60 s of that, `/instances` answers `[]`, and no `qaX starting` line is logged after it for 10 minutes.
   - The header's "Instances 0" then agrees with `/instances`.
2. **A crash that would clear never serves under a failed deploy.**
   - Use `dockerCommand: sh -c 'now=$(date +%s); if [ $now -lt <now+75> ]; then exit 3; fi; mkdir -p /tmp/w; echo ok > /tmp/w/index.html; exec httpd -f -p 10000 -h /tmp/w'`.
   - After the deploy reads `update_failed`, `curl https://<name>.onbex.co/` never answers 200 with `ok` for 5 minutes.
   - The header never reads "Service Running" with "Latest deploy: Failed".
   - A Manual Deploy afterwards goes live and serves `ok`.
3. **The pull-failure path matches t002's decision.** For an image tag that does not exist (`docker.io/library/nginx:qa-<ts>-no-such-tag`), either the instance is gone within 60 s of `update_failed`, or the deploy stays open until the image appears or the deadline passes. In both cases a failed deploy row never ends up serving.
4. **Unchanged:**
   - a crash in a deploy over a serving release keeps the serving release live, with no restart of the old pods;
   - a first release that fails at the progress deadline still stops at zero (w1/m179);
   - a service with a healthy first deploy goes live as before.
5. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 214, 2026-11-12 17:33–17:52 UTC (`muse.env`, workspace `bex-canary`; pin `77531cc764d3`). The probes are under § Finding.
- **Governing docs:**
  - [w1/m179](../../w1/done/m179/README.md) (done 2026-10-16): "a release that never served and settled failed has no running or restarting pod". It was implemented at the progress deadline only (`stopsFirstRelease`).
  - [w4/454](../done/454.md) (done 2026-11-01) added the early startup-crash settle, `permanentRolloutCrash`. Its live acceptance checked the deploy row (`update_failed` at +22 s) but not the instance afterwards.
  - ADR004 records the three-timer contract.
- **Goal linkage:** Render parity for the failure path of a first deploy. Render fails a deploy whose process "exited or crashed during startup", and "cancels the deploy" whose instances never pass their health check. A failed deploy never serves.
- **Expected outcome:**
  - a broken start command costs one failed deploy and no instance time;
  - the deploy row, header, instances API and URL agree;
  - a deploy marked failed is never the one serving.
- **Why now:** major. The deploy history, the CLI's `--wait` and any CI gate read "failed" while the release goes live on its own minutes later. Every crashing Free service also burns workspace instance hours and log volume for 15 minutes.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard shape changes. The operator stops the workload at the verdict it already records, and every surface already reports that state correctly.

## Finding (round 214)

### 1. A crash keeps restarting after its deploy failed

Fixture `qa-20261112-r214-crash` (`srv-dbqvh0ns9cvc73b61v2g`), created from the dashboard New Web Service → Existing Image, `busybox:1.36`, Docker Command `sh -c 'echo qa214 starting; echo "qa214 fatal: CONFIG_PATH is not set" >&2; exit 3'`.

- **The deploy fails fast and truthfully.** `dep-dbqvh0ns9cvc73b61v30`: `created` 17:33:54, `startedAt` 17:34:22, `update_failed` at 17:34:52.
  - Reason: "container exited shortly after start and is restarting repeatedly (last exit code 3) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (10000)."
  - `deploy_ended` `{"deployStatus":"failed","fullDeployStatus":"update_failed",…}`. Phase `Failed`.
- **It keeps restarting.** `GET /v1/logs?…&text=starting` (17:30Z to now) listed starts at 17:34:15, 17:34:18, 17:34:38, 17:35:07, 17:35:54, 17:37:23 and 17:40:19: the kubelet's back-off, 13 min after the failure.
  - `GET /v1/services/{id}/instances` → `[{"id":"srv-dbqvh0ns9cvc73b61v2g-le3hfq2i2k1ckmcf7nkr","createdAt":"2026-11-12T17:34:11Z"}]` on every poll from 17:35:28 to 17:48:21.
  - GraphQL → `{"phase":"Failed","replicas":1}`.
- **The surfaces disagree.** The dashboard header read "Service Failed · Instances 0" while `/instances` listed the restarting pod.

### 2. A crash that clears goes live under the failed deploy

Fixture `qa-20261112-r214-flaky` (`srv-dbqvkms5b3as73bg3erg`, `POST /v1/services`, Free `busybox:1.36`). Its Docker Command exits 3 until 17:43:01, then runs `httpd` serving `flaky-ok`.

- `dep-dbqvkms5b3as73bg3es0`: created 17:41:47. GraphQL phase `Failed` from 17:42:24, the row `update_failed` from 17:42:39 (`deploy_ended` `"failed"`).
- `curl https://qa-20261112-r214-flaky.onbex.co/` every 8 s answered `503 This service's first deploy is in progress…` until 17:44:24. From 17:44:33 it answered `200 flaky-ok`.
- Afterwards:
  - the only deploy is still `update_failed`;
  - GraphQL `{"phase":"Running","replicas":1}`, `/instances` lists 1;
  - the header reads "Service Running · Latest deploy: Failed", and the deploys list shows only "Failed … First Deploy";
  - Overview lists the service as Running;
  - Events hold `deploy_started` and `deploy_ended failed` and nothing else.

### Mechanism (code read at `origin/main`, unchanged since the pin)

- **The early settle.** `reportRolloutProgress` calls `permanentRolloutCrash` (`lego/operator/internal/controller/app_controller.go:3787-3788`, and `web_image_http.go:238-239` for web image discovery). It fires after `crashSettleRestarts = 2` (`:4129`) on never-Ready, all-pods CrashLoopBackOff (`:4143-4177`).
  - It settles through `settleFailedRolloutMessage` (`:4001`). For a release that never served, that only sets `Phase=Failed` and `Ready=False` (`:4032-4037`).
  - It records **no** `ConditionRollout` verdict: that write sits in the `releaseHasServed` branch (`:4004-4011`).
- **The stop.** `failedFirstRelease` (`release_plan.go:380-386`) is true only when `ConditionRollout` is False for the release generation. That is what makes `planRelease` run the workload at zero (`stopped`, `:156-158`).
  - The verdict is written only by `settleFailedRollout` at the progress deadline when `stopsFirstRelease` holds (`app_controller.go:3954`, `:3966-3975`). Its comment says "Only the deadline stops … not the early pull-failure settle".
  - So after the crash settle, the Deployment keeps 1 replica, and the kubelet keeps restarting the container until the 900 s deadline.
- **The flaky case.** When a restart succeeds, the pod turns Ready and `settleReady` (`:3493`) moves the phase to Running, so the release serves.
  - The deploy row is already terminal. `CanTransitionDeploy` (`lego/backend/internal/store/deploy_lifecycle.go:131-133`) admits no move out of `update_failed`, so the row stays Failed.

### Target behavior

The startup-crash settle of a never-served release records the same release-scoped `ConditionRollout` False verdict the deadline path records under `stopsFirstRelease`: nothing served, the Deployment is on this release, and no pod of it is Ready. So `planRelease` stops the workload in that pass.

- The service stays Failed with no instance.
- A new deploy, a restart or a resume moves the release generation past the verdict, as for the deadline stop.
- A crash over a serving release is unchanged: the served template is restored and keeps serving (w1/m172).

### Deadline check (fixture 1)

The progress-deadline stop still works; it just comes 16 minutes after the failure.

- `/instances` listed the pod on every poll through 17:50:28, and answered `[]` at 17:50:43.
- The logged starts were 17:34:15, 17:34:18, 17:34:38, 17:35:07, 17:35:54, 17:37:23, 17:40:19, 17:45:22 and 17:50:35: nine starts after the deploy read `update_failed` at 17:34:52.
- GraphQL still read `{"phase":"Failed","replicas":1}`, the configured count.

### Unverified

- **The pull-failure settle** (`permanentRolloutPullFailure`, `:4099`). It closes the row early too and, by its comment, deliberately keeps the pod retrying. Whether a pull that clears inside the window then serves under a failed row was not run (t002).
- **Private services and background workers.** They take the same `permanentRolloutCrash` path (w4/454: "Applies to web, private and worker"), but were not run this round.
- **Render's exact behavior** after a startup crash on a first deploy is taken from its docs, not captured.

### Dedupe

- w1/m179 (done) implemented the stop at the deadline only, and its DoD names "not the early pull-failure settle".
- w4/454 (done) added the crash settle without the stop; its acceptance didn't check the instance.
- w8/039 (done) covers the deadline's stall diagnosis.
- `grep -rlE "keeps? (restarting|crash-?looping)" .pm` and `grep -rl "permanentRolloutCrash" .pm` find no open item.
- Not deploy lag: `git log 77531cc764d3..origin/main -- lego/operator/internal/controller/app_controller.go lego/operator/internal/controller/release_plan.go` is empty.
- w4/609 (open) is a separate finding, also seen here: "==> Deploying image" prints after the app's first lines.

### Also verified this round (all pass)

- **Failure UX.** The deploy page shows the failure reason, the crash output (stdout and stderr, both `type=app`) and the closing `==> Deploy failed: …` line.
- **Logs search.** `text=fatal` on REST, and the dashboard Logs search box, list only the crash lines.
