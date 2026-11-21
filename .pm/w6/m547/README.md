# w6 · m547 — A disk-bearing service is unavailable for a few seconds on a deploy, as on Render

**Worker:** worker6 **Goal:** a deploy, restart or plan change of a service with a disk (`Recreate`) leaves it unavailable for a few seconds plus the app's own stop and start time, as Render documents, instead of the tens of seconds to minutes bex's probe cadence, node choice and image pull can add today. **Status:** todo (t001–t004, t007, t008 and t010 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure a disk service's unavailable window on production, phase by phase](done/t001.md) — **DONE** | 60m | — |
| t002 | [The new instance starts on the volume's node](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [A `Recreate` rollout probes startup and readiness every second](done/t003.md) — **DONE** (startup only; readiness not needed, t001) | 45m | t001 |
| t004 | [The new image is on the node before the old instance stops](done/t004.md) — **DONE** (not needed, t001; large images are w6/160) | 75m | t002 |
| t010 | [Tenant kubelets sync node status every 2 s](done/t010.md) — **DONE** | 30m | t001 |
| t005 | [Ship and re-measure on production](t005.md) | 45m | t003, t004, t010 |
| t006 | [Record the measured bound in ADR082, ADR004 and ADR018](t006.md) | 20m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

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
