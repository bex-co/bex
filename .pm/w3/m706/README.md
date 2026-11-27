# w3 · m706 — A dead resource link shows Render's "not found" or "deleted" page in place

**Worker:** worker3 **Goal:** a dashboard URL naming a resource that does not exist, or no longer exists, keeps its URL and draws Render's page in place ("`<Label>` not found" or "`<Label>` deleted", the id, Go to project when known, Return home), with Render's separate no-access and load-error states, instead of a toast and a redirect home. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's not-found, deleted, no-access and load-error states, and which page uses which | 30m | — |
| [t002](t002.md) | A shared `ResourceNotFound` draws Render's title, body, id and actions, with its head title and every locale | 1h | t001 |
| [t003](t003.md) | A resource that disappears while open reads "`<Label>` deleted" with its name, from the Apollo cache | 45m | t002 |
| [t004](t004.md) | Every `useNotFoundRedirect` call site draws the page in place; a dead deploy or build reads Render's "for `<service>`" with View deploys | 1h30m | t002 |
| [t005](t005.md) | A resource in a workspace the user cannot see shows Render's no-access state, and a failed load its error state | 45m | t004 |
| [t006](t006.md) | ADR018 records dead resource links, and w9/m55's redirect is marked superseded | 20m | t004 |
| [t007](t007.md) | Walk every resource type in a real browser at 1440 and 390, pending and ready, locally and on production | 45m | t003, t005, t006 |
| [t008](t008.md) | Render parity | 25m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008 |
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
