# w4 · m652 — A persistent disk can be added while creating a service, from the dashboard and GraphQL

**Worker:** worker4 **Goal:** a service that needs a disk gets it on its first deploy from the create page or GraphQL, as Render's create form offers, instead of a create, then the Disk page, then a Recreate restart. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | GraphQL `createService(disk:)` with REST's validation and error codes — **DONE** | 1h | — |
| [t002](done/t002.md) | The create form's Advanced section adds a disk — **DONE** | 1h15m | t001, w4/m651/t003 |
| [t003](done/t003.md) | dev-4: the first deploy mounts the disk, and data survives a restart — **DONE** | 40m | t002 |
| [t004](done/t004.md) | ADR018 records disk-at-create — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 4h35m**.

## Definition of done

GraphQL `createService(disk:)` creates a paid web service, private service or background worker with its disk and refuses a static site, a cron job, a Free plan, more than one instance and autoscaling with the codes REST's `serviceDetails.disk` answers. The create page's Advanced section has Add Disk, hidden for static sites, cron jobs and Free, as Render's create form hides it (amended 2026-11-26, see § Record). On dev-4, a paid web service created with a 1 GB disk at `/var/data` mounts it on its first deploy, a file written there survives a restart, and the service has exactly one deploy and no Recreate for the disk. ADR018's "Persistent disks" row records create-time disks on REST, GraphQL and the dashboard, with MCP unchanged as on Render.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w4`, 2026-11-25 (the aspect: creating a service from a repository, as Render's create docs describe), item 3. Found by the service-types research agent and confirmed by the Git-create agent; the Render quotes and the bex code were re-checked by hand the same day. Filed by `/pmx them all to w4` at `origin/main` `c196617a8b`.
- **Evidence (2026-11-25):** [render.com/docs/disks](https://render.com/docs/disks): "You can do so during service creation (click *Advanced* at the bottom of the creation form), or any time _after_ creation from your service's *Disk* page"; [render.com/docs/web-services](https://render.com/docs/web-services) step 5: "add a persistent disk". bex: REST create takes `serviceDetails.disk` (`lego/backend/internal/apps/rest.go:309`), validated by `validateCreateDisk` (`lego/backend/internal/apps/disks.go:798`), but GraphQL `createService` (`graphql.go:1574`) has no disk argument and the dashboard's create form has no disk control, so a disk is added after create from the Disk page.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and persistent disks ([ADR082](../../../../docs/ADR082-persistent-disks.md), reopened 2026-08-22).
- **Expected outcome:** a service that needs a disk skips the extra round of create, add a disk, Recreate restart.
- **Why now:** disks have shipped and the create form is the remaining gap; the validation is reusable. It follows w4/m651, which reshapes the same Advanced section (t002 depends on w4/m651 t003).
- **Render parity:** included: GraphQL and the dashboard change, and REST and MCP must keep agreeing with them and with Render. No user decision is open.

## Record — 2026-11-26

**Triage: work.** GraphQL `createService` (`graphql.go:1574`) had no disk argument and `gqlCreateRequest` never set `CreateRequest.Disk`; the dashboard's create page had no disk control. REST's `serviceDetails.disk` and `validateCreateDisk` (`disks.go:798`) were there to reuse. w4/m651 t003 shipped in `9a9d882da5`.

**Render's create form, read from its public bundle** (`index-D5V4ZgbE.js`, 2026-11-26; recorded in [render-artifacts/new-service-wizard.md](../../../../docs/render-artifacts/new-service-wizard.md#the-advanced-section-read-2026-11-25-w4m651-t001)). The Disk field renders only when the type is neither a static site nor a cron job **and the plan is not Free**, and an effect empties the drafted disk whenever that stops holding. It has no name field and sends `name: "disk"`, as its Disk page does. Its size is a 1–1000 GB stepper defaulting to 10.

**Decision: hidden on Free, not disabled.** The definition of done said "disabled on Free with its reason". Render's form hides Disk on Free and drops a drafted disk, so bex does the same (ADR008 § Render parity first), and the DoD above is amended to match. Both dashboard forms name the disk `disk`, Render's name (the Disk tab named it `data` before).

**What shipped.**

- GraphQL `createService(disk: ServiceDiskInput)` (`{name!, mountPath!, sizeGB}`) reaches `Create` through the same `validateCreateDisk` as REST. `TestGraphQLCreateServiceRefusesADiskAsRESTDoes` sends a static site, a cron job, Free and two instances to both and requires the same 400 message from each. GraphQL's `createService` takes no autoscaling, so that refusal is REST's alone. `TestGraphQLCreateServiceAttachesADisk` creates a web service, a private service and a worker with a disk and reads it back from `createService { disk }`, `server { disk }` and the disk row.
- The create page's Advanced card shows Disk after Secret Files for a paid web service, private service or background worker of any source. Add Disk shows the Disk tab's five notes, Mount path (Required) and the 1/5/10/50/100 GB chips; Remove disk drops it. An empty path holds Deploy with "Enter a valid disk mount path and size to continue."; a path or size bex-api would refuse is marked under its field. The disk rules, sizes and the `disk` name now live in `dashboard/src/features/services/lib/disk.ts` for both forms, and the reserved-path list gained `/bex-health`, which bex-api already refused.
- Render parity (t005): REST and GraphQL accept and refuse the same disks; the dashboard offers one exactly where they accept it (the form sends one instance and no autoscaling). MCP is unchanged, as Render's `create_web_service` has no disk argument. The one drift found, Render's 1000 GB dashboard cap against bex's 10 000 GB, is filed as [w4/732](../../732.md).
- Simplify (t006): the shared disk controls moved to `disk-fields.tsx` so the create page does not load the Disk tab's metrics charts; `diskAllowed` moved into `lib/disk.ts`; the GraphQL reader uses `gqlutil.Int`.
- Test coverage (t007): reverting the GraphQL `Disk:` mapping fails the attach, default-size and refusal tests and the transport guard; reverting the paid-plan gate, the draft drop or the focus handoff each fails a create-page test.

**dev-4 (t003).** bex-api rebuilt from this tree (`dev-env.sh 4 up`), the dashboard from this tree on `:50040`, a disposable verified Kratos identity and its workspace (disposable), and headless Chrome on `/web/new`. Removed afterwards: the service, then the account (the identity answers 404).

- **Free first.** The web service opened on Free and the Advanced card had no Add Disk. Choosing Starter showed it.
- **One create.** Public Git URL `https://github.com/bex-co/bex`, branch `main`, Root Directory `examples/hello-go`, runtime Docker, Starter, Add Disk, Mount path `/var/data`, size 1 GB. Deploy was held with the path empty and enabled once it was filled. Service `bexm652-walk` (disposable).
- **Read back.** GraphQL `server { plan disk { name mountPath sizeGB } }` answered `starter`, `disk`, `/var/data`, 1; `GET /v1/disks?serviceId=` listed the one `dsk-` row.
- **First deploy mounts it.** The first deploy went `build_in_progress` → `update_in_progress` → `live` (09:01:32–09:02:26 UTC), and `GET /v1/services/{id}/deploys` listed exactly one deploy. The pod's `disk` volume was the claim `disk-<workspace>-bexm652-walk` (1Gi, Bound), mounted at `/var/data`, and the Deployment's strategy was already `Recreate`. Events: `deploy_started` (`firstBuild: true`), `build_started`, `build_ended`, `deploy_ended`, with no second deploy and no restart for the disk.
- **Data survives a restart.** A marker written to `/var/data/m652.txt` through the instance's shell (`kubectl exec`) read back from the new pod after `POST /v1/services/{id}/restart`. The restart itself opened its own deploy, as every bex restart does (w1/m148).
