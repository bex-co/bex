# w6 · m548 — A service with a disk keeps the default shutdown delay, as on Render

**Worker:** worker6 **Goal:** as Render documents ("You can't change the shutdown delay for a service with a persistent disk"), every surface refuses a custom `maxShutdownDelaySeconds` on a service with a disk, attaching a disk returns a custom delay to the 30 s default, and the dashboard says why the setting is unavailable. **Status:** todo (t001–t003, t005, t006, t008, t009 done; t004 step 3, t007 and t010 wait for production)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's rule and decide what attaching a disk does to a custom delay](done/t001.md) — **DONE** | 30m | — |
| t002 | [REST, GraphQL and MCP refuse a shutdown delay on a service with a disk](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Blueprint validate, plan and apply refuse a disk with a shutdown delay](done/t003.md) — **DONE** | 30m | t002 |
| t004 | [Attaching a disk returns a custom delay to 30 s, and the operator applies 30 s to a disk-bearing App](t004.md) | 45m | t002 |
| t005 | [The dashboard disables Maximum shutdown delay on a service with a disk, with Render's reason](done/t005.md) — **DONE** | 30m | t002 |
| t006 | [Record the rule in ADR082 and ADR018](done/t006.md) — **DONE** | 15m | t003, t004, t005 |
| t007 | [Render parity](t007.md) | 20m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, setting `maxShutdownDelaySeconds` on a service with a disk is refused with a `400` that names the disk on REST `PATCH /v1/services/{id}`, GraphQL `setMaxShutdownDelay` and `updateService`, MCP `update_service`, a create carrying both, and Blueprint validate.
- Attaching a disk to a service whose delay is 120 s, through REST `POST /v1/disks`, reads back 30 s on every surface, the pod's `terminationGracePeriodSeconds` is 30, and the change appears as a setting change.
- The dashboard's Maximum shutdown delay control is disabled on a service with a disk, with Render's reason in en, zh and ja; on a service without a disk it is unchanged.
- A service without a disk accepts 1–300 s as before on every surface.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-20 (item 3, persistent disks), approved by the user's `/pmx them to w6`. Evidence:
  - [Render — Deploys](https://render.com/docs/deploys): "You can't customize the shutdown delay for a service with a persistent disk." [Render — Persistent Disks](https://render.com/docs/disks): "You can't change the shutdown delay for a service with a persistent disk."
  - bex has no such rule: `SetMaxShutdownDelay` (`lego/backend/internal/apps/service.go:5568`) and `validateMaxShutdownDelaySeconds` (`:4079`) check only the service type; the CRD's CEL rules (`lego/types/v1alpha1/app_types.go:849-858`) bound the value only; Blueprint applies it on its own (`blueprint_plan.go:81,270`); `AddDisk` (`disks.go:104`), GraphQL `addDisk` (`disks.go:619`) and MCP `add_disk` (`disks.go:722`) leave a custom delay in place. Other surfaces: REST `deploy.go:412`, GraphQL `graphql.go:834,1887`, MCP `mcp.go:106,213,925`, dashboard `services.$serviceId.settings.tsx:173` and `features/services/components/max-shutdown-delay-row.tsx`.
  - The pod's grace is `terminationGracePeriodSeconds(shutdownDelay, drain)` with drain 0 for a disk App (`deployment_projection.go:608`), so under `Recreate` a 300 s delay with an app that ignores `SIGTERM` is 300 s of downtime.
- **Goal linkage:** ADR008 § Render parity first; it also bounds the stop side of m547's downtime.
- **Expected outcome:** no setting can lengthen a disk service's deploy downtime, and every surface says so the same way.
- **Why now:** pairs with m547 (that bounds the start side, this the stop side); independent of it, so both can run in either order.
- **Open decision (recommendation, not a user decision):** Render does not document what attaching a disk does to an existing custom delay. The tasks implement the proposal's recommendation, resting on ADR008 (parity work is never parked on an optional decision): refuse while a disk is attached, return a custom value to 30 s on attach as a recorded setting change, report 30 s. Unverified against Render; t001 records it as such and t007 re-checks.
- **Render parity task included:** REST, GraphQL, MCP, Blueprint and dashboard all change.
