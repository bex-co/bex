# w6 · m547 — A disk-bearing service is unavailable for a few seconds on a deploy, as on Render

**Worker:** worker6 **Goal:** a deploy, restart or plan change of a service with a disk (`Recreate`) leaves it unavailable for a few seconds plus the app's own stop and start time, as Render documents, instead of the tens of seconds to minutes bex's probe cadence, node choice and image pull can add today. **Status:** blocked (t001–t008 and t010 done; t011 re-measures once w6/161 and w6/162 ship)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure a disk service's unavailable window on production, phase by phase](done/t001.md) — **DONE** | 60m | — |
| t002 | [The new instance starts on the volume's node](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [A `Recreate` rollout probes startup and readiness every second](done/t003.md) — **DONE** (startup only; readiness not needed, t001) | 45m | t001 |
| t004 | [The new image is on the node before the old instance stops](done/t004.md) — **DONE** (not needed, t001; large images are w6/160) | 75m | t002 |
| t010 | [Tenant kubelets sync node status every 2 s](done/t010.md) — **DONE** | 30m | t001 |
| t005 | [Ship and re-measure on production](done/t005.md) — **DONE** | 45m | t003, t004, t010 |
| t006 | [Record the measured bound in ADR082, ADR004 and ADR018](done/t006.md) — **DONE** | 20m | t005 |
| t011 | [Re-measure once the same-node detach race and the port rediscovery are fixed](t011.md) | 45m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008, t011 |

## Definition of done

- On production, a disk-bearing Starter fixture in `bex-canary` whose server listens within 2 s of start is unavailable for at most about 5 s plus its own exit time on each of three code deploys and one restart, measured by a 250 ms probe through the public edge.
- The new instance runs on the node the volume was attached to in each of those, unless that node was unschedulable (then recorded).
- ADR082 states the measured bound and what it does not cover (the app's own start, a node that is gone).
- If t001 already measures the bound, t002–t004 close as not needed with t001's numbers, and t005 is the re-measure only.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-20 (item 2, persistent disks), approved by the user's `/pmx them to w6`. Evidence (inferred; never measured for a disk service):
  - `lego/operator/internal/controller/deployment_projection.go:102` runs the startup and liveness probes every 10 s (`healthCheckPeriodSeconds`; startup threshold derived from the 15-minute budget at `:140`), and readiness every 5 s. In a rolling update the old pod serves meanwhile; under `Recreate` (`lego/operator/internal/controller/disk.go:96-107`) every second until the first passing probe is downtime.
  - `disk.go` sets `Recreate` and nothing about placement: the new pod may land on another node, which adds a Hetzner volume detach/attach and an image pull (ADR060 records one 2.5 GB pull at 3 m 39 s).
  - `drainSecondsFor` (`deployment_projection.go:608`) already gives disk apps a 0 s preStop drain, so the stop side is the app's own exit.
  - w2/m169 recorded a `Recreate` downtime of 08:55–08:57Z when it attached a disk to a Discourse service (`.pm/w2/done/m169/README.md`), a slow-starting app, so not a platform-only figure.
  - [Render — Persistent Disks](https://render.com/docs/disks): "Render stops the old instance before starting the new one, which makes the service unavailable for a few seconds."
- **Goal linkage:** ADR008 § Render parity first; ADR082's deploy behavior matches Render's.
- **Expected outcome:** each deploy of a disk service costs seconds of downtime, not tens of seconds or minutes.
- **Why now:** it touches every deploy, restart and plan change of every disk service; t001 measures first, so the fixes ship only where the numbers ask for them. Independent of m548, which bounds the stop side.
- **Render parity task omitted:** operator mechanism only; no REST, GraphQL, MCP or dashboard change.

## BLOCKED — 2026-11-21: t005 waits for the production pin

- **Shipped:**
  - `e0b731eba` makes a disk-bearing App's next instance prefer its volume's node and run its startup probe every 1 s, with the same 15-minute budget. Both are adopted lazily, so the upgrade Recreates no disk service.
  - The same commit sets the four tenant KubeadmConfigTemplates to sync node status every 2 s. Existing nodes keep 10 s until Cluster API replaces them. No roll was forced, because a forced roll would Recreate every disk service and other workers' live fixtures.
- **Measured before (t001, production, 2026-11-21):** 15.8–23.3 s on the same node and 39.8–46.8 s across nodes. The time went to node moves (27–32 s each), the 10 s startup period (7–9 s) and the kubelet's 10 s sync (1.7–8.6 s on the same node). LUKS `cryptsetup open` (2.1–2.3 s) is a floor the operator cannot move.
- **Expected after:** about 6–9 s on the volume's node: stop 0.5 s, a sync of at most 2 s, LUKS 2.2 s, mount 0.8 s, pull 0.6–2.2 s, start 0.3 s, a 1 s probe and Ready 0.4 s. That is above the definition of done's "about 5 s". t005 names the excess phase if the measurement agrees, and t006 records the measured bound.
- **Gate (clears by itself): pin must carry `e0b731eba`.** Then t005 runs its steps: a tenant node at 2 s (roll `bex-tenant-0` with `rolloutAfter` if none), the fixture, and the runs.
- **Filed on the way:** [w6/159](../../159.md) (a plan change after an `imageUrl` deploy redeploys the create-time image) and [w6/160](../../160.md) (pre-pull for large images).

## UNBLOCKED — 2026-11-21: production pin `cce6c749f6ff` carries `e0b731eba`

- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `b32b3ff43` (`cce6c749f6ff`), and `git merge-base --is-ancestor e0b731eba cce6c749f6ff` succeeds.
- **Next:** t005 finds a tenant node whose kubelet `/configz` reads `nodeStatusUpdateFrequency: 2s` (rolling `bex-tenant-0` with `rolloutAfter` if none does), then re-measures as t001 did. t006 records the bound, and t009 closes.

## BLOCKED — 2026-11-21: the definition of done needs w6/161 and w6/162

- **Rolled:** `82d55f40d` set `rolloutAfter` on `bex-tenant-0`. All six of its new nodes sync node status every 2 s. The first surge waited 6 min on Hetzner's shared-core limit (78 of 80 in use).
- **Measured (t005, production pin `cce6c749f6ff`):** with a named port, same-node restarts took 5.74–6.25 s and code deploys 7.25–9.0 s, against 15.8–23.3 s in t001. A move took 34.24 s, against 39.8–46.8 s. All ten unforced runs stayed on the volume's node. The startup probe passed 0.5–1.0 s after start, and the kubelet's sync gate was at most 1.96 s.
- **Why it does not hold:**
  - One same-node swap in ten (D1, 31.75 s) detached and reattached the volume, because the node's 2 s sync reported it free before the new pod was bound: [w6/161](../../done/161.md).
  - A web image service that names no port waited 25 s after Ready on each image deploy (37.0 and 44.0 s): [w6/162](../162.md).
  - The rest of the excess over "about 5 s" is the code deploy's pull (0.6–2.2 s, [w6/160](../../160.md)) and the publish's LUKS open (about 2.2 s).
- **Gate:** w6/161 and w6/162 ship. Then t011 re-measures on production once it runs their commits, and t009 closes.
