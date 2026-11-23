# w4 · m550 — Services suspend and resume in bulk from the service list, and ungrouped services move in bulk, as on Render

**Worker:** worker4 **Goal:** a member selects several services in a project environment or in the workspace's ungrouped list and suspends, resumes or moves them in one action, as Render's service list does; rows the caller may not act on are disabled with the reason the service page already gives. **Status:** done (2026-11-22: the DoD held on dev-4 and on production pin `19fdb46f1758`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's service-list bulk actions from its public dashboard bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | The workspace's ungrouped list selects rows and moves them in bulk — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Suspend and Resume in the bulk toolbar, with per-row outcomes — **DONE** | 1h30m | t001 |
| [t004](done/t004.md) | Role and protected-environment gating for the bulk actions — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Live check on dev-4 at 1440 and 390 — **DONE** | 30m | t002, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

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
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); [ADR018](../../../../docs/ADR018-render-parity.md) Services & lifecycle, Suspend / Resume (✅ per service, absent in bulk).
- **Expected outcome:** a member suspends or resumes a batch of services from the list in one action, and moves ungrouped services in bulk, instead of opening each service.
- **Why now:** Render has shipped it for 15 months with no board owner; bex's bulk-Move selection already exists, so this is its cheapest moment.
- **Render parity:** included (t006): a dashboard surface; REST and MCP need nothing because Render has no bulk API.
- **Open point, settled by t001 (2026-11-21, [capture](../../../../docs/render-artifacts/service-list-bulk-actions.md)):** Render's toolbar acts on services of every type, Postgres and Key Value. Resume shows while a selected one is suspended by the user, and Suspend while one is running. Each dialog lists only the rows it changes, and Suspend asks for `sudo suspend <N> services[ and <M> databases]`. One existing mutation runs per resource, 10 at a time, with no batch operation anywhere, so GraphQL is unchanged too. A refusing resource disables the action with _This action is limited to admins._ rather than dropping rows.

## Progress (2026-11-21)

- **Shipped (t001–t004, t006–t008):** Render's bundle is captured in `docs/render-artifacts/service-list-bulk-actions.md`. Both lists share `useResourceSelection`, `useBulkMove` and `BulkResourceToolbar` (`dashboard/src/features/projects/`). The Overview's ungrouped list has checkboxes and moves into any workspace environment, labelled `project / environment`. Resume and Suspend run through `BulkLifecycleDialog`, and both skeletons reserve the toolbar row. ADR018's Suspend / Resume row names the bulk path and what bex builds differently.
- **Deviation, protected confirmation (t004 step 2):** bex-api no longer issues a protected-environment phrase (w8/187, `lego/backend/internal/core/confirmation.go` `ProtectedVerbRestricted`). Only Admins may suspend or resume there, as on Render. So an Admin's one confirmation for the whole selection is the bulk dialog itself, with Render's typed phrase on Suspend, and there is no per-call phrase to pass. A Developer sees both actions disabled with Render's copy. The grant is read fresh (`viewerCapabilities(fresh: true)`) before anything is sent.
- **Deviation, gating source (t004 step 1):** the toolbar uses the workspace grant (`can_operate`, and `can_manage` in a protected environment) instead of each row's `serverActions`. `serverActions` reports a protected member as allowed plus `protected_confirmation_required` even for a Developer (`lego/backend/internal/apps/actioncaps.go` `suspendPrecondition`), and datastores have no per-row projection.
- **Order:** t006 (parity) ran before t005 (live check), because t005 cannot run yet.
- **Open:** t005, the dev-4 replay. On 2026-11-21 the local Docker daemon did not answer `docker info`, and the kind/CAPD API servers refused connections (`infra/local/bex.kubeconfig`, 127.0.0.1:55009), so dev-4 could not come up. t009 then needs the production replay once the pin carries the change.
- **Parked 2026-11-21:** the feature shipped in `afed68992c`. Once production's pin carries it, replay the DoD on production. Replay it on dev-4 as well, as an Admin, a Developer and a Viewer at 1440 and 390 (t005), if its Docker daemon answers by then. Then close with t009.
- **Unblocked 2026-11-22 (`/pm unblock w4/m550`, verified evidence):** production's pin `19fdb46f1758` (`chore(deploy): pin platform images to 19fdb46f1758`, `5330be9f54`) carries `afed68992c`: `git merge-base --is-ancestor afed68992c 19fdb46f1758` holds. dev-4 answers again (`bash scripts/dev-env.sh 4 status`: verification inventory all green, bex-api on :54040). No gate remains. Next: t005 replays the DoD on dev-4 as an Admin, a Developer and a Viewer at 1440 and 390, the same journeys run on production, and t009 closes the milestone if they hold.

## Live check (2026-11-22)

**dev-4 (t005).** bex-api from main `3e9e7a7c8a`, the dashboard from this tree (`yarn dev --port 50040`). dev-4's bex-api normally runs fail-open, which would make every member an Admin. So it ran against an isolated OpenFGA seeded with `deploy/gitops/authz/model.json` (`DEV_OPENFGA_URL`), and a Developer's and a Viewer's `POST …/resume` answered 403. A throwaway script made three verified Kratos identities. The Admin moved their workspace to Scale and invited a Developer and a Viewer, who joined at their first request. The fixtures were a project with `staging` and a protected `production`, three services in staging (a web service, a background worker and a static site), two in production, two ungrouped ones, and one more service per width for a real failure. Headless Chromium drove the dashboard at 1440 and 390:

| DoD line | Result, at 1440 and at 390 |
| --- | --- |
| 1. Staging, Admin: running web + suspended worker + static site, plus a fourth service | Both actions shown. **Suspend services** lists only the three running rows and asks for `sudo suspend 3 services`. The fourth service, deleted after the dialog opened, reads "Not found" with Retry, and the other two read Suspended (REST agrees). **Resume services** has no phrase, resumes all three and clears the selection on Close. |
| 1. A failed row stays selected | One row's call was pointed at a missing id in the browser. It read "Not found" with Retry, and the other row completed. After Close only that row was still checked, with Suspend still offered. Its retry suspended it and the selection emptied. |
| 2. Overview Move, Admin | The Overview's ungrouped list has checkboxes and the toolbar, with targets `m550-bulk / staging` and `m550-bulk / production`. Move showed "Services for "staging" updated.", and the ungrouped section emptied. The project's staging card read 5 resources, the two moved ones among them, and REST's `environmentId` was staging. |
| 3. Production (protected), Developer | Suspend and Resume shown but disabled, with the tooltip "This action is limited to admins.". No lifecycle call was sent. In staging the Developer suspended and resumed. |
| 3. Production (protected), Admin | One dialog per action. Suspend asked for `sudo suspend 1 service`. Every row changed, and no other prompt appeared. |
| 3. Viewer, both environments | Suspend and Resume disabled with "Your role can only view this service. Ask a workspace admin to make changes.". No lifecycle call was sent. |
| Pending against ready | Overview: the toolbar is at y 314 (1440) and 408 (390), 36px tall, with the table right under it, the same in both states. Environment card: at 1440 the toolbar is 36px in both states. At 390 the ready toolbar wraps Move to a second row (76px), where the skeleton reserved 36px. |

**Fixed by the replay, in the same commit as this closeout:**

- A Viewer could choose a target on the Overview and click Move, then read "Forbidden": the Overview passed the toolbar no Move reason. bex-api gates placement on `can_create`, and Render disables Move without `moveResource`. So `BulkResourceToolbar` now disables the picker and Move for any role without `can_create`, with "Your role can’t make this change. Ask a workspace admin." The fix covers both lists. A toolbar test for a Viewer and a Developer fails without it.
- The two skeletons now share `SelectionToolbarSkeleton`, at the ready parts' sizes (73px, 192px and 62px) and wrapping as the toolbar does. At 390 the card's toolbar is now 76px in both states, and at 1440 36px. `yarn lint` and `yarn test` (10192 tests) passed.

**Production (pin `19fdb46f1758`, carrying `afed68992c`).** Signed in as the QA user with `bash scripts/qa-login.sh`, in `bex-canary`, where the QA user is an Admin. Fixtures were project `qa-20261122-m550` (disposable) with `staging` and a protected `production`. Staging held a free web service, a starter background worker (suspended first) and a static site, production a web service and a static site. Two more services were ungrouped. Headless Chromium ran at 1440 and 390:

- DoD 1 held as on dev-4. Suspend listed only the two running rows (`sudo suspend 2 services`). One call per resource was sent, 10 calls in all per width. The row whose call was pointed at a missing id read "Not found" with Retry and stayed selected after Close, with the non-target worker. The static site was suspended. Its retry, then Resume of all three, left REST at `not_suspended` everywhere.
- DoD 2: Move put the two ungrouped services in `qa-20261122-m550 / staging`. The Overview's list then held only resources this run had not made, the project's staging card read 5 resources, and REST agreed.
- DoD 3, Admin: in the protected environment, one dialog per action with the typed phrase on Suspend, and both rows changed. Production has one QA identity, an Admin, so the Developer and Viewer lines ran on dev-4 only.
- Pending against ready matched dev-4 before the fix: the Overview was identical in both states, and the card's toolbar at 390 was 36px pending and 76px ready. This commit's two fixes reach production with the next pin.
- Cleanup: all seven services and the project were deleted (each by-id `GET` answers 404), the session was logged out (`--logout`: `ok logged-out`), and its jar and state file were removed. The two other ungrouped resources in `bex-canary` were not touched.

**dev-4 cleanup:** the fixtures, project and three identities were deleted, the OpenFGA container removed, and bex-api restarted with its default settings.

**Filed:** [w4/714](../../done/714.md): `DELETE /v1/environments/{id}/resources` leaves a service in its project with no environment, so no dashboard list shows it. [w4/715](../../done/715.md): at 390 the environment card's header crushes the name, and its skeleton's header and filters do not match, a pre-m550 offset of 16px at 1440 and 64px at 390.
