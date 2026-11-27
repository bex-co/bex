# Render's project page: one section per environment

Read 2026-11-26 for w1/m682 from Render's public dashboard bundle (entry `index-D5V4ZgbE.js`, read without sign-in and never executed), and from [render.com/docs/projects](https://render.com/docs/projects). It is the record w1/m682 (the project page) built from, and the one w1/m683 (environment settings), w1/m684 (the New menu) and w1/m685 (Move Services) start from. It supersedes the July walk's "one selected Environment" ([dashboard-walk/workspace.md](dashboard-walk/workspace.md#populated-project-comparison--2026-07-17)): Render has since replaced that layout.

**Evidence.** Chunk hashes change with every Render deploy, so find the code by the strings quoted here: the `Project-*.js` chunk (the page, its sections, the New menu, the Move Services modal, the environment's ••• menu), `page-templates-*.js` (the header and its paywall), `urls-*.js` (the URL builders) and the entry `index-*.js` (the route table and the environment settings page). Neither `Project-*.js` nor the header imports Render's `featureflags` hooks: every Render workspace has this page.

## The page

- `/project/:projId` draws Render's header over `x.environments.map(…)`: **every environment of the project, in the project's order, each a `section` of its own**. There is no environment picker and no project-wide resource table.
- A project with exactly one environment shows an **Add environment** card (a wide button with a `New` icon) under it, which opens the new-environment modal.
- After creating an environment the page outlines the new section for two seconds (`onCreate` sets the focused id, then clears it after `$=2e3`).

### The header (`page-templates`)

- A `Project` eyebrow (`type-mono-01-caps text-faint`).
- The project's name as the `h1`, inside a link to `${projectSettings(id)}?edit=name` with a `Settings` (gear) icon and `<span class="sr-only">Edit project name</span>`.
- **Add environment** (`kind: secondary`, a `New` icon) on the right. It opens the new-environment modal, unless:
  - the caller lacks `createEnvironment`: the button is disabled, with the permission's message in a tooltip;
  - the project is at the plan's environment limit (`environments.length >= limit`, the tier's `projectEnvironments`, unlimited from Pro): the button carries a count badge `<count>/<limit>` and opens Render's paywall popover instead: title `Get more environments with <tier 2 title>`, description `Upgrade to get unlimited environments and all <tier 2 title> plan features.`, a Pro badge, and the buttons `Upgrade` (to `/billing/update-plan`) and `Learn more` (to `/docs/projects`).
- Project Settings opens its Name card editing for `?edit=name` ("Project Name — A unique name for your project").

### One environment's section (`Project`)

- `<section>` whose header is the environment's name as an `h2`, followed for a protected environment by the `ProtectedEnvironment` icon (`aria-hidden`), and, under it, the description `Protected environment`.
- One row: the counted switcher on the left; on the right the static-IP badge link (only when the environment has dedicated static IPs) and the environment's ••• menu.
- **The switcher** (`ContentSwitcher`, size small) has three options: `All (<n>)`, `Services (<n>)` and `Env Groups (<n>)`. Services counts `services`, `databases`, `redises`, `workflowServices` and `sandboxGroups`; Env Groups counts `envGroups`; All is their sum. Counts are the section's own, before search. The choice is kept **per environment** in `localStorage`, as one JSON object from environment id to choice; an unreadable or unknown stored value falls back to All.
- **The search** under it: placeholder `Search resources in <env>`, matching each row's name (and its id when the query looks like one). It filters only its own section.
- With All or Services chosen:
  - when the environment has resources, its resource table (`ServicesForUserList`, with selection and Move), then a trailing `New` row that opens the New menu when the caller may create here;
  - when it has none, the empty state (below).
- With Env Groups chosen and no env groups: `This environment has no env groups.`, with a create link.
- With All or Env Groups chosen and env groups present: the Env Groups list table with its `environment` column hidden (Name, Env Vars, Secret Files, Updated).
- A search that matches nothing shows the search's no-results state.

### The empty state

- Title `<env> is empty` (an `h3`).
- Body `Kickstart your environment by creating a new service or by moving an existing one.` when the workspace has any resource (services, Postgres, Key Value, workflows, sandbox groups), else `Kickstart your environment by creating a new service.`
- Buttons `Create new service` (primary, to `/project/<id>/environment/<envId>/new`) and, only when the workspace has a resource, `Move existing services` (secondary, opens the Move Services modal). Each is disabled with the permission's message (`createEnvironmentResource`, `moveEnvironmentResources`) for a caller who may not.
- While the workspace's resources load, a spinner holds the state's height (`min-h-[19.75rem]`).

### `/project/:projId/environment/:envId`

- The same page. The named section scrolls into view (`scrollIntoView({behavior: "smooth", block: "start"})`) and is outlined (`outline outline-1 outline-offset-8 outline-status-highlight`, fading with `motion-safe:transition duration-500`).
- After two seconds the outline clears, and the user's next scroll replaces the address with `/project/:projId` (`history.replaceState`, only while the path is still the environment's).
- An id that names no section outlines nothing.

## For w1/m683: the environment's ••• menu and settings page

Read in full for w1/m683 (2026-11-27) from the same bundle: the menu in the `Project` chunk, the page (`eV`'s environment branch) and its cards in the entry chunk.

- **The ••• menu** (`OverflowMenu`, bottom-right), on every section:
  - `Block cross-environment connections` with `Prevent private network traffic from crossing the environment boundary.` and a toggle, shown only when `updateEnvironmentIsolated` is allowed or denied for the tier. Below Pro the toggle is disabled, with a Pro badge whose popover reads `Upgrade to unlock this feature` (`Upgrade` links to the change-plan page). Toggling opens the isolation dialog below; on success it toasts `Cross-environment connections allowed|blocked for <env>.`, and a refusal stays in the dialog.
  - `All settings`, to `/project/<id>/environment/<envId>/settings`.
- **The route** is `/project/:projectId/environment/:environmentId/:environmentPage`: `settings` draws the page, any other page navigates to `/project/<id>`. The title is `<project> / <env> / Settings`, the breadcrumbs Project › Environment, the back link `Environment` (to the environment's URL), with the sidebar's Overview open.
- **The page** (`Environment settings`, sections in a side list; the anchors are the card titles' slugs: `#permissions`, `#cross-environment-connections`, `#dedicated-ips`, `#networking`):
  - **Name** — `A unique name for your environment`; Edit (disabled with the permission's message for a member who may not), then Cancel / Save (Save off until the name changes); `Environment name is required`, `Name is already used by another environment in this project`, else `An error occurred while updating the environment. Please try again.`
  - **Permissions** — `By default, all team members can delete and suspend resources, move services, and manage database networking.` and `If marked as a protected environment, only admins can delete and suspend resources, move services, and manage database networking.`; radio `Default` (`For staging and development`) / `Protected` (`Best for production`), Edit / Cancel / Save. Saving while the saved value is Protected first asks `Remove restrictions`: `Removing restrictions for the <env> environment will allow all team members to perform destructive actions. Are you sure you want to do this?`, `Yes, remove restrictions`. Below Pro the card is disabled with `Protected environments and team collaboration are available on the Pro plan.` and Upgrade. Shown only when `updateEnvironmentProtected` is allowed or denied for the tier.
  - **Cross-Environment Connections** (side-list title `Connections`) — `Prevent private network traffic from crossing the environment boundary, while allowing traffic between services in this environment.` and `Services in this environment will not be able to communicate with Render services outside of it.`; radio `Allowed` (`Default for new environments`) / `Blocked` (`Recommended for production environments`). Save always confirms in the isolation dialog: title `Restrict connections` or `Remove restrictions`, description `Confirm you want to isolate connections for <project / env>`; restricting says `Block private network traffic to and from your Render services outside this environment. This may disrupt services that rely on cross-boundary traffic.`, `Existing open network connections will not be affected. Restart services in this environment to ensure all existing connections are closed.` and `Are you sure you want to do this?`; removing says `Private network traffic to and from your Render services outside this environment will no longer be blocked.` and the question; the buttons are Cancel and `Restrict connections` / `Yes, remove restrictions`. No toast. Below Pro: disabled with `Block cross-environment connections by upgrading to the Pro plan.` and Upgrade.
  - **Environment Groups** — `New Env Group` (to `…/environment/<envId>/new/env-group`, disabled with the permission's message), a `Search Environment Groups` search, `No environment groups found`, and the env groups table without its Environment column.
  - **Dedicated IPs** — the environment's dedicated static IPs (not on Hobby).
  - **Networking** — the shared inbound IP rules editor, or a `Networking` card with the plan's message below the plan.
  - **Delete Environment** — `This environment, including its services, will be permanently deleted. This action cannot be undone.` and `Delete environment`; for a project's only environment, an informative notice `This environment cannot be deleted as projects require at least one environment. Consider deleting the project instead.` (`project` links to Project Settings) and the button disabled. The sudo modal is titled `Delete environment`, asks for `sudo delete environment <name>` and says `This environment will be permanently deleted, along with all of its services. To keep any services, please move them out of the environment first.` On success: `Deleted environment <env> successfully.` and the browser goes to Project Settings. A server error matching `project contains only this environment` reads `Cannot delete the last environment in a project. You can still delete the entire project.`; any other `Couldn't delete environment. Please try again in a bit. Sorry!`. A member who may not delete it sees an `Environment Deletion` card with the permission's message instead.

## For w1/m684: the New menu and the environment create URLs

- The menu (`Add services to environment` when unlabelled, `New` on the trailing row) lists the create types under a `Create new` header, then `Env Group` (to `…/environment/<envId>/new/env-group`), `Move existing services` and, when the environment holds resources, `Generate Blueprint` (navigating with `source: "project environment menu"`). Without `moveEnvironmentResources` each item is disabled with a `Protected` icon. At the plan's service limit the menu sits behind the paywall `Create more services with a Pro plan`.
- Routes in the entry chunk: `/project/:projId/environment/:envId/new`, `…/new/:serviceTypeSlug`, `…/:serviceTypeSlug/new` and `…/new/env-group`.

## For w1/m685: the Move Services modal

- Title `Move Services`, description `Move the following services into my <environment badge> environment.`, a `Services` multi-select (placeholder `Search services to move…`) grouped by project (`Ungrouped services` first), each option's tooltip `From <project> / <environment>` or `Ungrouped`, and `No available services` / `No services matching "<query>"`.
- The submit reads `Move service` or `Move <n> services`; an error shows `An error occurred.` with `Error: <message>`.

## What bex does (w1/m682)

The dashboard's project page draws this page, with these differences:

- **Unassigned.** bex keeps resources that are in a project but in no environment (Render requires an environment). They have a section of their own after the environments, `Unassigned`, while it has rows or a resource list failed to load, so none becomes unreachable. An old `?env=unassigned` link lands on it.
- **Interim actions.** Until w1/m684, the switcher row's right holds bex's New Service link and Manage resources before Render's ••• menu (w1/m683), instead of Render's trailing New row; Move existing services opens Manage resources until w1/m685.
- **Selection.** bex's env groups table keeps row selection, so an env group still moves with the section's Move toolbar (w4/564).
- **Plan.** bex's environment limit is Hobby's two (ADR030); the paywall's Upgrade opens the change-plan dialog and Learn more opens bex's projects and environments docs.
- **Old links.** `?env=<id>` redirects to the environment's URL, and `?q=` and `?kind=` are dropped.

## What bex does (w1/m683)

The section's ••• menu and `/project/<id>/environment/<envId>/settings` are Render's, as read above, with these differences:

- **Who sees what.** Render's permission levels become bex roles: Permissions, Connections and Networking (and the menu's toggle) show only to an Admin, the one role bex-api lets change them; Name and the delete card follow `can_create`, and deleting a protected, isolated or IP-restricted environment needs an Admin.
- **Plans.** Isolation is Pro and higher and inbound IP rules Scale and higher (ADR030); an already isolated environment can always turn it off, and below Scale the rules stay listed with Render's reset to allow all. Protection has no plan gate on bex (w1/180).
- **No Dedicated IPs card**: dedicated IPs are a recorded non-goal.
- **New Env Group** opens the create dialog with the environment chosen, until w1/m684 brings Render's create page.
- **Delete.** The dialog adds that the environment's env groups are kept (bex detaches them). bex-api does not refuse deleting a project's last environment; the dashboard does (w1/181).
- **Other environment pages**: one path segment, as Render's `:environmentPage`; a deeper path gets the not-found page.
