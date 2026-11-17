# w33 · m435 — A Blueprint's `initialDeployHook` runs once after the service's first successful deploy, beside `preDeployCommand`, as Render documents

**Worker:** worker33 **Goal:** the one-time hook stops standing in for the pre-deploy command: a first release runs `preDeployCommand`, goes live, and then runs the hook once with the service's disk; a hook failure never fails the deploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record the hook's mechanism in ADR004 and ADR049 and amend the ADR018 backlog row | 40m | — |
| t002 | Operator: the first release runs `preDeployCommand`, and the hook runs once after that release is live | 60m | t001 |
| t003 | Operator: the hook sees the service's disk at its mount path | 50m | t002 |
| t004 | The hook's outcome on tenant surfaces: deploy log, status, and bex-api's awaiting-pre-deploy read | 45m | t002 |
| t005 | Blueprint: `afterFirstDeployCommand` is accepted as the deprecated alias | 30m | — |
| t006 | Services created under the old mechanism carry over | 30m | t002 |
| t007 | Local-cluster walk: a migrate pre-deploy, a seed hook and a disk | 40m | t003, t004, t005, t006 |
| t008 | Render parity | 25m | t007 |
| t009 | Simplify | 15m | t008 |
| t010 | Test coverage | 45m | t008 |
| t011 | Closeout (live replay on production) | 45m | t009, t010 |

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
