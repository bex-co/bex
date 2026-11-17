# w33 · m435 — A Blueprint's `initialDeployHook` runs once after the service's first successful deploy, beside `preDeployCommand`, as Render documents

**Worker:** worker33 **Goal:** the one-time hook stops standing in for the pre-deploy command: a first release runs `preDeployCommand`, goes live, and then runs the hook once with the service's disk; a hook failure never fails the deploy. **Status:** done (2026-11-17). t001–t006 and t008–t010 shipped in `d098bb097` (operator, bex-api, ADRs) and `c513acc1b` (dashboard). On pin `58ef086ab21d`, [t011](done/t011.md) replayed definition-of-done items 1–4 and 6 on production, and they held (§ Production replay). Item 5 has no subject anywhere in production (§ Closed (2026-11-17)). [t007](done/t007.md) was closed as superseded by that replay on the user's decision, not walked locally (§ Unblocked (2026-11-17): t007 superseded).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record the hook's mechanism in ADR004 and ADR049 and amend the ADR018 backlog row — **DONE** | 40m | — |
| [t002](done/t002.md) | Operator: the first release runs `preDeployCommand`, and the hook runs once after that release is live — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Operator: the hook sees the service's disk at its mount path — **DONE** | 50m | t002 |
| [t004](done/t004.md) | The hook's outcome on tenant surfaces: deploy log, status, and bex-api's awaiting-pre-deploy read — **DONE** | 45m | t002 |
| [t005](done/t005.md) | Blueprint: `afterFirstDeployCommand` is accepted as the deprecated alias — **DONE** | 30m | — |
| [t006](done/t006.md) | Services created under the old mechanism carry over — **DONE** | 30m | t002 |
| [t007](done/t007.md) | Local-cluster walk: a migrate pre-deploy, a seed hook and a disk — **DONE** (superseded by t011's production replay on the user's decision of 2026-11-17; not walked locally) | 40m | t003, t004, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](done/t011.md) | Closeout (live replay on production) — **DONE** | 45m | t009, t010 |

## Definition of done

On production, in `bex-canary`, with owned fixtures deleted afterwards (by-id `GET` 404):

1. A Blueprint declares a paid web service with a disk, a `preDeployCommand` and an `initialDeployHook`, each leaving its own harmless marker (the hook's marker is a file under the disk's mount path). On the first deploy the pre-deploy command runs before the rollout, the deploy reaches `live`, and only then the hook runs, once; the marker file is readable by the running service.
2. A second deploy of the same service runs the pre-deploy command and does not run the hook. A Blueprint re-sync does not run it either.
3. A service whose hook exits non-zero keeps its first deploy `live` and serving; the failure is readable in the deploy's log; the next deploy runs the hook again after it goes live, and once it succeeds it never runs again.
4. A Blueprint that spells the field `afterFirstDeployCommand` validates, plans and behaves as in 1; one that declares both spellings is refused at the field's path.
5. A service that completed its hook before this milestone shipped does not run it again.
6. `make test` and `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for one aspect of render parity (Blueprints), 2026-11-16, item 1; claims re-verified against `bdf087c00` before filing (`/pmx them all to w33`).
- **Render reference:** render.com/docs/blueprint-spec, read 2026-11-16. `initialDeployHook`: "Web services, private services, and background workers only. Render runs this command after a service's _first_ successful deploy. Useful for one-time initialization tasks such as seeding a database or downloading files to a persistent disk." and "This command runs only once per service instance (including in preview environments). For commands that should run before _every_ deploy, use `preDeployCommand` instead." `afterFirstDeployCommand` is "a deprecated alias for this field."
- **bex today:** `PreDeployStepCommand` (`lego/types/v1alpha1/predeploy.go:28`) selects the hook in place of `preDeployCommand` until a release has run it to success, and `reconcilePreDeploy` (`lego/operator/internal/controller/app_controller.go:7365`) runs it as the pre-deploy Job. So the hook runs before the first rollout, the first release never runs `preDeployCommand`, and a failing hook fails the first deploy. The Job mounts only the `/etc/secrets` projection (`lego/operator/internal/predeploy/predeploy.go:138`), not the disk. `afterFirstDeployCommand` appears nowhere in `lego/` or the pinned schema, so a file using it is refused as an unknown field. ADR018's gap-backlog row for `initialDeployHook` records "Bex runs the hook as a pre-deploy Job, so it precedes the first rollout rather than following it" with no user sign-off, and w8/m63 kept that timing on purpose ("do not silently claim exact post-deploy timing").
- **Goal linkage:** ADR008 Render parity first; ADR049 D2 (equivalent behavior or an explicit refusal); ADR004 (pre-deploy gating); ADR082 (disks).
- **Expected outcome:** a Render template that migrates before each deploy and seeds once after the first one deploys unmodified, and its hook can fill the disk. Render's own `render-examples/dify-render-template` declares a hook and a disk on one web service.
- **Why now:** it is a recorded divergence with no sign-off, and it breaks a first deploy outright (a seed that runs in place of the migration it depends on) rather than degrading it.
- **Decision state:** running the hook after the first release is live, and beside rather than instead of `preDeployCommand`, is what Render documents. Two parts are this proposal's recommendation and not a Render capture or a user decision: a failed hook leaves the deploy live and is retried after the next deploy goes live (Render documents no failure rule), and the hook runs once per service rather than once per instance. t001 records both as bex's choices; if a Render workspace is available it captures Render's failure behavior instead.
- **Render parity:** included: the Blueprint field's behavior and the deploy log are tenant-facing on REST, GraphQL, MCP and the dashboard.

## Render parity (t008, 2026-11-16)

Compared against render.com/docs/blueprint-spec#initialdeployhook, re-read 2026-11-16. The comparison was made through the services and their adapters in tests, not on a running stack (t007 could not run), so the wire-level walk is the production replay's.

| Surface | What it does now | How it was checked |
| --- | --- | --- |
| Blueprint validate, plan, create, sync, approve on REST, GraphQL and MCP | one compile path (`ValidateBlueprint`, `parseStack`): `initialDeployHook` and its alias `afterFirstDeployCommand` are accepted on web and private services and background workers and refused on cron jobs and static sites; both spellings together are refused at the alias's path; a plan carries one warning notice at the alias | `TestBlueprintAfterFirstDeployCommand…` (five tests) |
| Dashboard Blueprint flow | the same validation result; no client-side field list exists for the hook | read of `dashboard/src` (no reference to either spelling) |
| Blueprint and service reads | `initialDeployHook` is echoed from the App's annotation under its canonical name (`lego/backend/internal/apps/render.go`) | unchanged code |
| Generate Blueprint | writes neither spelling: it has never exported the hook (`blueprint_generate.go` writes `preDeployCommand` only). t005's step 4 assumed it did | read of the generator |
| Deploy record on REST, GraphQL, MCP | `status` never reflects the hook; `initialDeployHookStatus` and `initialDeployHookFinishedAt` are bex extras, on all three from one view | `TestAFailedInitialDeployHookChangesNoDeployStatus`; MCP renders REST's shape, and `rendertest.MCPResultExtensions` lists the two keys |
| Deploy log on REST `GET /v1/logs`, GraphQL `logs`, MCP `list_logs` | the hook pod's output as `type=build` (durable) and `type=predeploy` (live), and the platform lines after the live line | `lego/backend/internal/logs/initial_deploy_hook_test.go` |
| Dashboard deploy log | the window stays open while the hook runs and ends where it ended | `hook-window.test.ts`, `deploy-detail-page.test.tsx`, `use-deploy.test.ts` |

Differences from Render, each recorded in ADR018's Blueprint row:

- **A failed hook leaves the deploy live and is retried after the next deploy goes live.** Render documents no failure rule. bex's choice; capture filed as w33/004.
- **A hook added to a service that has already deployed runs after its next deploy.** Render's sentence reads as "never". Behavior from before this milestone; capture filed as w33/004.
- **A 30-minute bound and one attempt per release.** Render documents none. bex's choice; w33/004.
- **Once per service.** Render says "once per service instance (including in preview environments)"; previews are a recorded non-goal, so the two agree wherever bex has the feature.
- **The dashboard's deploy header shows no hook line.** The outcome is in the deploy's log and on the record's `initialDeployHookStatus`. Render's dashboard was not captured (w33/004).
- **Generate Blueprint does not export the hook**, before and after this milestone. Whether Render's does is in w33/004.

## Local-cluster walk (t007): not run, 2026-11-16

The local CAPD cluster cannot run on this host, so none of the definition of done was observed on a kubelet. Nothing was rebuilt, scaled or deleted.

- **State found:** the shared `bex` cluster had been half up for 3 days 16 hours: control plane `NotReady`, `coredns` Pending, no CNI, no bex components, and both MachineDeployments replacing their one Machine every few minutes.
- **Cause:** `fs.inotify.max_user_instances` is the kernel default, 128, and Docker's node containers all run as the host's root. The mgmt node held 63 instances and the workload control plane 36, so each new worker's containerd fails `failed to create CRI service: … failed to create fsnotify watcher: too many open files`, CAPD's `crictl ps` times out, and the Machine is replaced. A reprovision would stop at the same point.
- **Why it is a gate:** raising the limit needs root on the host (`sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576`), and `sudo` asks for a password. Writing the host's sysctl from a privileged container would go around that, so it was not done. w33 also has no `dev-N` stack (`scripts/dev-env.sh` takes N 1–10), so there is no local bex-api to apply a Blueprint through. Filed as w33/005 so the script names the cause.
- **What stands in for it:** the operator's order, live gate, failure rule, retry, disk mount and carry-over run against a real API server in envtest ("Initial deploy hook (kubernetes runtime)"), and the admission envtest submits the disk-sharing hook Job to the deployed `bex-operator-workloads` policy under every container policy and both serving-pool tiers. The alias and the log and deploy surfaces are covered through the services (§ Render parity).
- **What no test shows:** a kubelet mounting the `ReadWriteOnce` volume in a second pod on the instance's node and the service reading the hook's file; the hook pod scheduling beside the instance on a packed node; the hook's output arriving in the log store. The production replay (t011) is therefore the first run on a kubelet, not a confirmation. To make it a confirmation, raise the host limit, run `bash scripts/mock-cluster.sh reprovision` and walk t007 first.

## Blocked (2026-11-16)

Two gates. Neither needs a product decision.

1. **Release gate (clears by itself).** Production runs the operator, bex-api and dashboard images of the newest `chore(deploy): pin platform images to <sha>` commit on `main`. When that pin contains `c513acc1b`, the later of this milestone's two commits, t011 can run. Check:

   ```bash
   git fetch origin main
   pin=$(git log origin/main --grep='pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
   git merge-base --is-ancestor c513acc1b "$pin" && echo "pinned: replay the definition of done"
   ```

   Then walk items 1–5 in `bex-canary` as t011 says (a paid web service with a disk, a `preDeployCommand` and an `initialDeployHook`, each leaving its own marker), delete the fixtures, record the conclusions here, and close the milestone.

2. **Local-cluster walk, t007 (needs the machine's owner).** The local CAPD cluster cannot start its worker nodes while the host's `fs.inotify.max_user_instances` is 128 (§ Local-cluster walk; filed as w33/005). Either of these clears it:
   - on the host, `sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576`, then `bash scripts/mock-cluster.sh reprovision`, and a worker walks t007 before t011; or
   - `/unblock w33` records that the production replay stands in for the local walk, and t007 is closed as superseded by t011.

   Until one of them, the production replay is the first time the hook runs on a kubelet. What that leaves unproven is listed under § Local-cluster walk: the second mount of the `ReadWriteOnce` volume on the instance's node, scheduling beside the instance on a packed node, and the hook's output reaching the log store.

## Unblocked in part (2026-11-17)

**Source: verified evidence (loopx w33), no user decision involved. Affects t011 only.** Gate 1, the release gate, cleared. The newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` is `7a8203c3b` (`… to 58ef086ab21d`, 2026-11-17 04:37 UTC), and `git merge-base --is-ancestor` succeeds for both `d098bb097` and `c513acc1b` against `58ef086ab21d`. Next: t011 replays definition-of-done items 1–5 in `bex-canary` as the QA user, deletes the fixtures (by-id `GET` 404), and records the conclusions here.

Gate 2 is unchanged. Nobody has raised the host's inotify limit, and no `/unblock w33` record says the production replay stands in for t007. This record decides neither. While t007 is open the milestone cannot close, so after the replay it goes back to `blocked/` with t007 as its only gate.

## Production replay (2026-11-17, t011)

Pin `58ef086ab21d`, QA user, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). REST and GraphQL were called with the QA session, MCP through `https://api.bex.co/mcp` with `workspaceId`. The dashboard's deploy pages were read in headless Chrome (playwright-core, because the Playwright MCP was down). Three owned Starter web services ran the image `docker.io/library/python:3.13-alpine`, and each had a `preDeployCommand` that echoes its own marker:

- **A**, `qa-20261117-m435-hook` (`srv-dbtubbuu6dls73ehcig0`): a 1 GB disk at `/var/data`, served by `python3 -m http.server $PORT --directory /var/data`. Its hook appends one line to `/var/data/hook-runs.txt`. It was applied with a `repo`, which registered Blueprint `exs-dbtubbuu6dls73ehcihg` (auto-sync off).
- **B**, `qa-20261117-m435-fail` (`srv-dbtubd6u6dls73ehcijg`): no disk. Its hook exits 3 unless the manifest's `QA_HOOK_OK` is `1`.
- **C**, `qa-20261117-m435-alias` (`srv-dbtubdfok1cs73a3mttg`): A's shape, with the hook spelled `afterFirstDeployCommand`.

Before the run the workspace held `hello-go` and another hunt's `qa-m333-disk`. Neither was touched.

1. **First deploy (holds).** A was applied at 05:27:11Z. Its first deploy's log has the same lines in the same order on REST `GET /v1/logs` (`type=build`), on GraphQL `logs` with `deployId`, and on the dashboard deploy page: the pre-deploy marker at 05:27:16Z, `==> Your service is live 🎉` at 05:28:23.94Z, `==> Running the initial deploy hook`, the hook's own marker at 05:28:24.9Z, and `==> Initial deploy hook succeeded`. The deploy record reads `status: live`, `preDeployStatus: succeeded`, `initialDeployHookStatus: succeeded` and `initialDeployHookFinishedAt: 05:28:28Z` on REST, GraphQL and MCP `get_deploy`. The running instance served `https://qa-20261117-m435-hook.onbex.co/hook-runs.txt` from its disk, and the file held the hook's one line. So the hook pod mounted the `ReadWriteOnce` volume on the instance's node, and its output reached the log store. Those are two of the three things t007 was meant to show first.
2. **Second deploy and re-sync (hold).** A manual deploy at 05:29:49Z ran the pre-deploy command (marker at 05:29:51Z) and went live with no hook line and no `initialDeployHookStatus`. `POST /v1/blueprints/{id}/sync` with the unchanged file planned nothing and started no deploy. A second sync added one env var at 05:31:30Z (validate planned an `update` of `envVars`). It rolled a `blueprint_sync` deploy, which ran the pre-deploy command (05:31:32Z) and no hook. At 05:36Z `hook-runs.txt` still held only the line from 05:28:24Z.
3. **A failing hook (holds).** B's first deploy went live at 05:28:00Z and kept serving (200). Its hook printed its marker and exited 3. The log read `==> Initial deploy hook failed: the initial deploy hook exited with code 3` and `==> The deploy stays live. The hook runs again after the next deploy goes live.`, and the record read `status: live`, `initialDeployHookStatus: failed`. REST refuses an env-var `PUT` on a manifest-owned key (`ENV_VAR_MANIFEST_MANAGED`), so the file was re-applied with `QA_HOOK_OK: "1"`. That rolled the next deploy: pre-deploy, live at 05:30:14Z, then the hook succeeded at 05:30:18Z, on REST, GraphQL, MCP and the dashboard. A third, manual deploy at 05:31:18Z ran the pre-deploy command and no hook. Once the retry ran, the first deploy's record reads no hook status, and its log keeps the hook pod's own output but not the two failure lines. ADR004 records that limit.
4. **The alias (holds).** C's file answered `valid: true` with a create plan on REST, GraphQL and MCP `validate_bex_yml`. C was created with `initialDeployHook` carrying the alias's command and behaved as A did: pre-deploy at 05:27:22Z, live at 05:28:28.1Z, then the hook, which `succeeded` at 05:28:31Z, and the instance served the hook's line from `/var/data/hook-runs.txt`. A file that declares both spellings was refused on all three surfaces with `BLUEPRINT_ALIAS_CONFLICT` at `services[0].afterFirstDeployCommand` (line 10). Validate shows no deprecation notice: t005 puts that notice on a planned sync, and no repository carrying the alias was available to plan from.
5. **Services from before this milestone (no live subject).** No service in the three workspaces the QA user can read (`bex`, `tian-personal`, `bex-canary`; 20 services) declares an `initialDeployHook`, so none of them has a completed hook that could run again. Reading other tenants' Apps needs the production kubeconfig, and `scripts/fetch-app-kubeconfig.sh` refuses while the repository's `.env` is group-readable (mode 664). The mode was left unchanged. Item 5 therefore rests on the controller tests t006 names: `TestInitialDeployHookCompletedByEitherMarkerNeverRuns` and the three legacy-step tests.
6. **Suites (hold).** On `58ef086ab21d`, CI `test (backend)` (run 44016267997) and `test (dashboard)` (run 44016268008) passed. `test (operator)` (`make test`) and `go lint` (`make lint`, all four modules) passed on `d098bb097` (runs 44010377466 and 44010377467) and on `e8e450fc1`, the pin's child commit (runs 44016353118 and 44016353159). On the pin itself, `go lint` was cancelled when the next push superseded it.

**Found outside the DoD.** A deploy-scoped GraphQL `logs` read with `type: "build"`, over a window wider than the deploy, returned the other deploys' pre-deploy output: A's first deploy listed the pre-deploy markers of deploys 2 and 3. Hook pods were left out, as they should be. The cause predates this milestone (w21/m324) and is filed as [w33/008](../../008.md). Each dashboard deploy page reads its own deploy's window, and each showed only its own lines.

**Cleanup.** At 05:36:30Z Blueprint `exs-dbtubbuu6dls73ehcihg` was disconnected (204) and the three services were deleted (204 each). By-id `GET` answers 404 for all four, the three `onbex.co` hosts answer 404, and the workspace lists no Blueprint and no disk. The QA session was logged out (`qa-login.sh --logout`, then a 401 on the API), and its local files were removed.

## Blocked (2026-11-17)

One gate remains, and only the user can clear it. It is not a product decision, and no Render behavior depends on it.

**What t007 is for now.** t007's local walk was meant to run before the production replay, so that the replay would confirm the mechanism instead of being its first run on a kubelet. The replay has now run. It showed on a kubelet two of the three things only a cluster can show (§ Local-cluster walk): the hook mounting the `ReadWriteOnce` volume a second time while the service reads the hook's file, and the hook's output arriving in the log store. It did not test the third: a hook scheduling beside its instance on a packed node. Choose one through `/unblock w33`:

- **Accept the production replay in t007's place.** t007 closes as superseded by t011, recorded as such and not as a walk. A worker then closes the milestone with no further live work.
- **Keep the local walk.** Run `sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576` on the host, then `bash scripts/mock-cluster.sh reprovision` (w33/005). A worker then walks t007 and closes the milestone.

## Unblocked (2026-11-17): t007 superseded

**Source: explicit user decision, 2026-11-17. Affects t007 and t011.** The `/loopx w33` orchestrator put the two options of § Blocked (2026-11-17) to the user. Option 1, accepting the production replay in place of the local-cluster walk, was marked as the recommended option. Option 2 was raising the host's inotify limit and having a worker walk t007 locally. The user replied with the single word "approve". The orchestrator read that as option 1 and reported that reading back to the user.

- **Chosen:** the 2026-11-17 production replay (§ Production replay) stands in for t007's local walk. t007 closes as superseded by t011. It is recorded as such and not as a walk: none of the definition of done was observed on the local cluster.
- **Rationale:** the replay ran on a kubelet and showed two of the three things only a cluster can show: the hook pod mounting the `ReadWriteOnce` volume beside the running instance, and the hook's output reaching the log store. No Render behavior depends on this choice. It only decides which run verifies the mechanism.
- **Left unobserved:** a hook scheduling beside its instance on a packed node. The replay did not exercise it, and § Local-cluster walk already lists it among what no test shows. The user accepted that gap with this decision.
- **Remaining gates:** none. The host's inotify limit stays at 128. That is w33/005's subject, which shipped as a fast failure in `scripts/mock-cluster.sh`, and it no longer gates this milestone.
- **Next:** t011 step 3 closes the milestone with no further live work.

## Closed (2026-11-17)

**Item 5, checked across the whole of production.** At 06:59Z the production app cluster's kubeconfig was fetched to a scratch path outside the repository with `bash scripts/fetch-app-kubeconfig.sh <path>`, and then deleted. `.env` is now mode 600, so the script no longer refuses. `kubectl get apps.app.bex.co -A` listed 29 Apps across every tenant namespace. None of them declares `spec.initialDeployHook`, carries a `bex.co/initial-deploy-hook*` annotation, or has `status.initialDeployHookCompleted` or `status.initialDeployHook`. So no service in production completed a hook before this milestone shipped, and none can run one again. Item 5 holds there with no subject. Its behavior rests on t006's controller tests: `TestInitialDeployHookCompletedByEitherMarkerNeverRuns` and the three legacy-step tests.

**Definition of done.** Items 1–4 and 6 held on production on pin `58ef086ab21d` (§ Production replay). Item 5 holds as above. The fixtures were deleted, and by-id `GET` answers 404. t007 is closed as superseded (§ Unblocked (2026-11-17): t007 superseded). t011's step 3 moved every task to `done/` and the milestone to `w33/done/m435/`.
