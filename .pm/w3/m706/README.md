# w3 · m706 — A dead resource link shows Render's "not found" or "deleted" page in place

**Worker:** worker3 **Goal:** a dashboard URL naming a resource that does not exist, or no longer exists, keeps its URL and draws Render's page in place ("`<Label>` not found" or "`<Label>` deleted", the id, Go to project when known, Return home), with Render's separate no-access and load-error states, instead of a toast and a redirect home. **Status:** todo (t001–t006 and t008–t010 done; t007 walked locally, its production walk and t011 left)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's not-found, deleted, no-access and load-error states, and which page uses which — **DONE** | 30m | — |
| [t002](done/t002.md) | A shared `ResourceNotFound` draws Render's title, body, id and actions, with its head title and every locale — **DONE** | 1h | t001 |
| [t003](done/t003.md) | A resource that disappears while open reads "`<Label>` deleted" with its name, from the Apollo cache — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Every `useNotFoundRedirect` call site draws the page in place; a dead deploy or build reads Render's "for `<service>`" with View deploys — **DONE** | 1h30m | t002 |
| [t005](done/t005.md) | A resource in a workspace the user cannot see shows Render's no-access state, and a failed load its error state — **DONE** | 45m | t004 |
| [t006](done/t006.md) | ADR018 records dead resource links, and w9/m55's redirect is marked superseded — **DONE** | 20m | t004 |
| [t007](t007.md) | Walk every resource type in a real browser at 1440 and 390, pending and ready, locally and on production | 45m | t003, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 7h10m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-26 without sign-in: `ResourceNotFound-CqxL2YY9.js` and its callers in `index-D5V4ZgbE.js`, `CronJob-*.js`, `DeployDetailsHeader-*.js`, `ResourceRedirector-*.js`. Find them by the strings quoted here.
- **`ResourceNotFound`** (props `resourceType`, `resourceId`, `resourceLabel`, `name`, `variant`, `links`, `title`, `renderBody`): a ghost empty state (`mt-12 mb-20`). Title `${label} not found`, or `${label} deleted` when `variant` is `deleted`. Body "We couldn’t find `<id>`. It may have been deleted, or the link may be incorrect.", or "`<name>` (`<id>`) was deleted." for a deleted one; the id is an inline code snippet. Actions: the first link as the primary button, then **Go to project** (when the resource's environment has a project) and **Return home** (`/`) as ghost buttons. Analytics events "Resource Not Found Viewed" and "Resource Not Found Action Clicked". The URL does not change.
- **Deleted vs unknown:** each caller reads a `missing…Context` fragment from the Apollo cache (`name`, `environment { project { id name } }`); a cached name means `deleted`, none means `unknown`.
- **Callers seen:** Service (`resourceLabel: Service`; preview variants are a non-goal), Cron job, Sandbox group, Deploy and Build (`DeployDetailsHeader`: "We couldn’t find `<id>` for `<service>`…", links **View deploys** then Return home), and `/resource/<id>` with an unrecognised id ("Resource not found"). Workflow is a non-goal.
- **Other states on the same routes:** a permission error draws a no-access state (`ih`, with `entityId` and a `joinHint` from the error); any other error draws "An error occurred" with "`<id>` could not be loaded." and Return home (`lH`).
- **No toast and no redirect** on any of these paths; a toast appears only after the user's own delete (for example "Webhook deleted", then navigation to the list).

## bex today

Verified 2026-11-26 on main `76dfb1e0e2`.

- `dashboard/src/common/hooks/use-not-found-redirect.ts`: on `notFound`, unless `ownDeleteActive()` (the user's own delete, w4/645), it toasts `common.resourceNotFoundToast` and navigates to `/` (or a passed target) with `replace`.
- 12 call sites: `features/services/components/service-detail-layout.tsx`, `features/deploys/components/deploy-detail-page.tsx` (to the service's Deploys tab), `features/build-sources/components/build-source-layout.tsx`, `build-detail-page.tsx`, and the routes `databases.$databaseId.tsx`, `keyvalue.$keyValueId.tsx`, `env-groups_.$groupId.tsx`, `blueprints.$blueprintId.tsx`, `webhook.$webhookId.tsx`, `project.$projectId.tsx`, `sandbox-groups_.$sandboxGroupId.tsx`, `w.$.tsx`.
- History: w9/m55 (`.pm/w9/done/m55/`) chose redirect-home-with-toast as a user decision on 2026-07-17; its t005 recorded Render's behavior as unverified. ADR018 has no row for it, so no signed-off divergence exists.

## What shipped

Recorded 2026-11-27 (t001–t006, t008–t010). The capture is [resource-not-found.md](../../../docs/render-artifacts/resource-not-found.md); ADR018's row is in [deployment-sources-iac.md](../../../docs/ADR018/deployment-sources-iac.md).

- **Deviations from the brief, from the bundle:** Render draws `ResourceNotFound` only on a service, cron job, sandbox group, deploy, build and `/resource/<id>`. Postgres, Key Value, environment groups, Blueprints, webhooks, projects and build sources draw `Error404`, "Page not found", "`<id>` doesn’t exist.", with Go to Webhooks on a webhook, so bex draws that there. `ResourceNotFound` has at most two buttons (the first link, then Return home), so View deploys pushes Go to project out. Render's deleted variant reads its project from the cached environment; bex's Server query now spreads `MissingServiceContext` (name, type, project, environment) so the cache holds them.
- **Components:** `common/components/resource-not-found.tsx` (`ResourceNotFound`, `PageNotFound`, `AccessDenied`, `ResourceLoadFailed`, `ResourceStateView`, `ResourceStatePage`), `common/lib/resource-read-state.ts` (`resourceReadState`; `use-not-found-redirect.ts` and its toast are gone), `common/hooks/use-missing-resource-context.ts`, `common/hooks/use-resource-state-links.ts`. Every locale carries the strings; the tab's title is the drawn title.
- **No access:** bex-api answers a resource in a workspace the caller is not in with `FORBIDDEN`, which Apollo caches as a bare null, so the route loaders gained a `forbidden` state (`routeResourceFailure`) that the pages honor. `/w/<tea-id>` for a workspace the caller is not in draws Access denied with the id.
- **Own delete:** `resourceReadState` stands down on not found while a dashboard delete is in flight or just done (w4/645), and the service layout keeps its own delete flag (w4/340).
- **Follow-ups:** w3/158 (Render's join hint and Join workspace), w3/159 (Render's own not-found at `/resource/<id>`).

## Walk

**Locally, 2026-11-27** (t007 step 1): dev-3 (`bash scripts/dev-env.sh 3 status` all green), the dashboard from the tree on `:50030`, headless Chromium at 1440×900 and 390×844, signed in as a dev-3 user created for the walk, with a second dev-3 user's workspace as the foreign one (both disposable). Every row drew its state with the URL kept, no toast and no horizontal scroll: a mistyped service id ("Service not found", Return home); a dead deploy under a live service ("Deploy not found", "…for m706-orders", View deploys then Return home); mistyped Postgres, Key Value, environment group, Blueprint, project and build source ids (Page not found, Return home) and webhook (Go to Webhooks); the other user's Postgres and `/w/<their tea-id>/settings` (Access denied with the id); a service in a project, a service and a cron job deleted through the API while their pages were open ("Service deleted" with the name, Go to project then Return home; "Cron job deleted" with the name, Return home); a forced 502 on a service and a Postgres read ("An error occurred", "`<id>` could not be loaded."). A direct load renders the state on the server, so its first paint is the state itself; a client navigation shows the resource's skeleton while its loader runs, then the state. dev-3 serves no sandbox groups, so a mistyped sandbox group read drew "An error occurred" there; production's walk covers the sandbox group states. The disposable Postgres, project and services were deleted afterwards.

**Production:** waits for a pin carrying this milestone's commit (t007 step 2).

## Definition of done

1. Opening a dead URL for any of the 12 call sites keeps the URL and draws the not-found page in place, with no toast: the right label, the id, Go to project when the resource's project is known, Return home; a dead deploy or build reads Render's wording with View deploys.
2. A resource deleted elsewhere while its page is open reads "`<Label>` deleted" with its name; the user's own delete still behaves as w4/645 made it.
3. A resource in a workspace the user cannot see shows the no-access state; a failed load shows the error state with Return home.
4. The head title and every locale carry the strings; the pending frame and the not-found page match at 1440 and 390.
5. ADR018 has a dead-resource-links row, and w9/m55's README notes that this milestone superseded its redirect under the Render-parity-first rule.
6. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary`, with resources created and deleted for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 3, verified against Render's bundle the same day. The brainstorm flagged that this reverses w9/m55's 2026-07-17 user decision; the user filed it with `/pm them all to w3` the same day, without choosing to sign the redirect off as a divergence.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)): an older decision that conflicts with Render is amended unless ADR018 records the user's sign-off, and w9/m55's has none.
- **Expected outcome:** a stale or mistyped link tells the user what was missing and where to go, as Render does, instead of dropping them on the home page with a toast they may miss.
- **Why now:** w3/m705 adds `/resource/<id>` and many Render aliases whose unknown ids land in this handling, so the page they reach should be Render's. The 12 call sites are all on one hook, so the swap is contained.
- **Render parity task:** included (t008): the milestone changes what every resource page shows for a dead id.
