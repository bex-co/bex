# w4 · m652 — A persistent disk can be added while creating a service, from the dashboard and GraphQL

**Worker:** worker4 **Goal:** a service that needs a disk gets it on its first deploy from the create page or GraphQL, as Render's create form offers, instead of a create, then the Disk page, then a Recreate restart. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | GraphQL `createService(disk:)` with REST's validation and error codes | 1h | — |
| [t002](t002.md) | The create form's Advanced section adds a disk | 1h15m | t001, w4/m651/t003 |
| [t003](t003.md) | dev-4: the first deploy mounts the disk, and data survives a restart | 40m | t002 |
| [t004](t004.md) | ADR018 records disk-at-create | 20m | t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 4h35m**.

## Definition of done

GraphQL `createService(disk:)` creates a paid web service, private service or background worker with its disk and refuses a static site, a cron job, a Free plan, more than one instance and autoscaling with the codes REST's `serviceDetails.disk` answers. The create page's Advanced section has Add Disk, hidden for static sites and cron jobs and disabled on Free with its reason. On dev-4, a paid web service created with a 1 GB disk at `/var/data` mounts it on its first deploy, a file written there survives a restart, and the service has exactly one deploy and no Recreate for the disk. ADR018's "Persistent disks" row records create-time disks on REST, GraphQL and the dashboard, with MCP unchanged as on Render.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w4`, 2026-11-25 (the aspect: creating a service from a repository, as Render's create docs describe), item 3. Found by the service-types research agent and confirmed by the Git-create agent; the Render quotes and the bex code were re-checked by hand the same day. Filed by `/pmx them all to w4` at `origin/main` `c196617a8b`.
- **Evidence (2026-11-25):** [render.com/docs/disks](https://render.com/docs/disks): "You can do so during service creation (click *Advanced* at the bottom of the creation form), or any time _after_ creation from your service's *Disk* page"; [render.com/docs/web-services](https://render.com/docs/web-services) step 5: "add a persistent disk". bex: REST create takes `serviceDetails.disk` (`lego/backend/internal/apps/rest.go:309`), validated by `validateCreateDisk` (`lego/backend/internal/apps/disks.go:798`), but GraphQL `createService` (`graphql.go:1574`) has no disk argument and the dashboard's create form has no disk control, so a disk is added after create from the Disk page.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and persistent disks ([ADR082](../../../docs/ADR082-persistent-disks.md), reopened 2026-08-22).
- **Expected outcome:** a service that needs a disk skips the extra round of create, add a disk, Recreate restart.
- **Why now:** disks have shipped and the create form is the remaining gap; the validation is reusable. It follows w4/m651, which reshapes the same Advanced section (t002 depends on w4/m651 t003).
- **Render parity:** included: GraphQL and the dashboard change, and REST and MCP must keep agreeing with them and with Render. No user decision is open.
