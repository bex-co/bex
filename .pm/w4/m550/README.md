# w4 · m550 — Services suspend and resume in bulk from the service list, and ungrouped services move in bulk, as on Render

**Worker:** worker4 **Goal:** a member selects several services in a project environment or in the workspace's ungrouped list and suspends, resumes or moves them in one action, as Render's service list does; rows the caller may not act on are disabled with the reason the service page already gives. **Status:** todo (t001–t004, t006–t008 done; t005 and t009 open)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's service-list bulk actions from its public dashboard bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | The workspace's ungrouped list selects rows and moves them in bulk — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Suspend and Resume in the bulk toolbar, with per-row outcomes — **DONE** | 1h30m | t001 |
| [t004](done/t004.md) | Role and protected-environment gating for the bulk actions — **DONE** | 45m | t003 |
| [t005](t005.md) | Live check on dev-4 at 1440 and 390 | 30m | t002, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h10m**.

## Definition of done

On dev-4, then on production once the pin carries the change:

- In a project environment, selecting a running web service, a suspended background worker and a static site and choosing **Suspend** or **Resume** changes each row as t001 records Render doing; a row whose action fails stays selected and says why, and the others complete.
- The workspace home's ungrouped list has row checkboxes and the same toolbar; **Move** puts the selected resources in the chosen environment, and the home page and the project page agree afterwards.
- A Developer in a protected environment sees Suspend and Resume disabled with a reason, never a 403 after clicking; an Admin there is asked for the protected confirmation once; a Viewer gets no enabled lifecycle action.
- REST and MCP are unchanged, and GraphQL too unless t001 shows Render's dashboard calls a batch operation; ADR018's Suspend / Resume row names the bulk path.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-20, item 1. The aspect is Render features announced only in its dashboard and changelog, which none of bex's API pins (OpenAPI, MCP, Blueprint schema, CLI) can see. Filed by `/pmx them to w4`.
- **Render:** changelog 2025-08-27, "Suspend and resume services in bulk from the Render Dashboard": "Click the checkbox next to each service you want to suspend, then click **Suspend** in the action menu that appears"; Resume works the same way. [render.com/docs/projects](https://render.com/docs/projects): bulk Move is "by selecting them in your workspace's service list and then clicking **Move**", ungrouped services included; in a protected environment only Admins may suspend or resume a service.
- **bex today:** selection and a Move-only toolbar exist only in `dashboard/src/features/environments/components/environment-card.tsx` (`projects.moveSelected`). The home page's ungrouped list (`dashboard/src/routes/index.tsx` → `dashboard/src/features/projects/components/resource-table.tsx`) has per-row actions and no selection. No board item tracked bulk suspend or resume.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); [ADR018](../../../docs/ADR018-render-parity.md) Services & lifecycle, Suspend / Resume (✅ per service, absent in bulk).
- **Expected outcome:** a member suspends or resumes a batch of services from the list in one action, and moves ungrouped services in bulk, instead of opening each service.
- **Why now:** Render has shipped it for 15 months with no board owner; bex's bulk-Move selection already exists, so this is its cheapest moment.
- **Render parity:** included (t006): a dashboard surface; REST and MCP need nothing because Render has no bulk API.
- **Open point, settled by t001 (2026-11-21, [capture](../../../docs/render-artifacts/service-list-bulk-actions.md)):** Render's toolbar acts on services of every type, Postgres and Key Value. Resume shows while a selected one is suspended by the user, and Suspend while one is running. Each dialog lists only the rows it changes, and Suspend asks for `sudo suspend <N> services[ and <M> databases]`. One existing mutation runs per resource, 10 at a time, with no batch operation anywhere, so GraphQL is unchanged too. A refusing resource disables the action with _This action is limited to admins._ rather than dropping rows.

## Progress (2026-11-21)

- **Shipped (t001–t004, t006–t008):** Render's bundle is captured in `docs/render-artifacts/service-list-bulk-actions.md`. Both lists share `useResourceSelection`, `useBulkMove` and `BulkResourceToolbar` (`dashboard/src/features/projects/`). The Overview's ungrouped list has checkboxes and moves into any workspace environment, labelled `project / environment`. Resume and Suspend run through `BulkLifecycleDialog`, and both skeletons reserve the toolbar row. ADR018's Suspend / Resume row names the bulk path and what bex builds differently.
- **Deviation, protected confirmation (t004 step 2):** bex-api no longer issues a protected-environment phrase (w8/187, `lego/backend/internal/core/confirmation.go` `ProtectedVerbRestricted`). Only Admins may suspend or resume there, as on Render. So an Admin's one confirmation for the whole selection is the bulk dialog itself, with Render's typed phrase on Suspend, and there is no per-call phrase to pass. A Developer sees both actions disabled with Render's copy. The grant is read fresh (`viewerCapabilities(fresh: true)`) before anything is sent.
- **Deviation, gating source (t004 step 1):** the toolbar uses the workspace grant (`can_operate`, and `can_manage` in a protected environment) instead of each row's `serverActions`. `serverActions` reports a protected member as allowed plus `protected_confirmation_required` even for a Developer (`lego/backend/internal/apps/actioncaps.go` `suspendPrecondition`), and datastores have no per-row projection.
- **Order:** t006 (parity) ran before t005 (live check), because t005 cannot run yet.
- **Open:** t005, the dev-4 replay. On 2026-11-21 the local Docker daemon did not answer `docker info`, and the kind/CAPD API servers refused connections (`infra/local/bex.kubeconfig`, 127.0.0.1:55009), so dev-4 could not come up. t009 then needs the production replay once the pin carries the change.
