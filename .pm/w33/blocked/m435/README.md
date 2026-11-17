# w33 · m435 — A Blueprint's `initialDeployHook` runs once after the service's first successful deploy, beside `preDeployCommand`, as Render documents

**Worker:** worker33 **Goal:** the one-time hook stops standing in for the pre-deploy command: a first release runs `preDeployCommand`, goes live, and then runs the hook once with the service's disk; a hook failure never fails the deploy. **Status:** blocked — release gate: t001–t006 and t008–t010 are done and shipped in `d098bb097` (operator, bex-api, ADRs) and `c513acc1b` (dashboard). [t011](t011.md) replays the definition of done on production once a "pin platform images to" commit contains both; that clears by itself. [t007](t007.md), the local-cluster walk, could not run on this host and needs its owner (§ Blocked).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record the hook's mechanism in ADR004 and ADR049 and amend the ADR018 backlog row — **DONE** | 40m | — |
| [t002](done/t002.md) | Operator: the first release runs `preDeployCommand`, and the hook runs once after that release is live — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Operator: the hook sees the service's disk at its mount path — **DONE** | 50m | t002 |
| [t004](done/t004.md) | The hook's outcome on tenant surfaces: deploy log, status, and bex-api's awaiting-pre-deploy read — **DONE** | 45m | t002 |
| [t005](done/t005.md) | Blueprint: `afterFirstDeployCommand` is accepted as the deprecated alias — **DONE** | 30m | — |
| [t006](done/t006.md) | Services created under the old mechanism carry over — **DONE** | 30m | t002 |
| [t007](t007.md) | Local-cluster walk: a migrate pre-deploy, a seed hook and a disk — **BLOCKED** (the host cannot hold the local cluster; see § Local-cluster walk) | 40m | t003, t004, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout (live replay on production) — **BLOCKED** (release gate) | 45m | t009, t010 |

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
