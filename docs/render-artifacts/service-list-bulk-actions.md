# Service-list bulk actions — Render's dashboard bundle, 2026-11-21

## Sources

Read without signing in, on 2026-11-21, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-BR2q8yex.js` and the chunks it imports), for w4/m550. The hashes change with every Render deploy, so find the code by the strings quoted below:

- `index-BR2q8yex.js`: the selection provider (`selectedResources`, `isSuspendModalOpen`, `resetSelectedResources`), the floating action bar (`data-testid: "action-bar"`), its toolbar (`Move`, `Generate Blueprint`, `Resume`, `Suspend`), the **Suspend services** and **Resume services** modals, and the bulk-suspend hook (`Bulk Service Suspension Completed`).
- `bulkResume-Cr00qjEY.js`: the bulk-resume hook (`Bulk Service Resumption Completed`) and the per-kind resume permission check.
- `suspension-BEDZ7oXL.js`: which resources are suspendable and which count as suspended by the user.
- `service-TN0K5kvW.js`, `database-BZ7Tvd7O.js`, `redis-CJe-1R61.js`: the GraphQL mutations the bulk hooks call.
- `ServicesForUserList-DqyVV6wL.js`: the Overview page, whose **Ungrouped Services** table selects rows through the same provider.

The 2025-08-27 changelog entry ("Suspend and resume services in bulk from the Render Dashboard") and [render.com/docs/projects](https://render.com/docs/projects) describe the same feature in prose.

## Render's behaviour

### Where rows can be selected

- One selection provider wraps the whole signed-in dashboard. Every service table (the Overview's **Ungrouped Services**, each project environment's table) has row checkboxes, and each table shows the selected rows that are its own.
- The selection empties when the workspace changes and on navigation. A Blueprint page mounts the provider with Move turned off.
- A row checkbox exists for every resource the table lists: services of every type, Postgres databases, Key Value instances, and sandbox groups.

### The action bar

- A bar floating at the bottom centre of the page appears while anything is selected: `<N> service selected:` or `<N> services selected:`, then the actions, then a close button (`aria-label` **Close actions**). Escape clears the selection too.
- **Move**: shown unless the provider turned it off; disabled without the `moveResource` permission. It opens a project and environment picker, with **New project** and **New environment** entries.
- **Generate Blueprint**: shown while a resource is selected; disabled without `createBlueprint`.
- **Resume** (a ghost button): shown only when a selected suspendable resource is suspended by the user.
- **Suspend** (a critical ghost button): shown only when a selected suspendable resource is not suspended.
- A mixed selection shows both Resume and Suspend.

### Which resources Suspend and Resume act on

- Suspendable means anything but a workflow (`__typename !== "WorkflowService"`): a `Server` (web service, private service, background worker, static site), a `CronJob`, a `Database` (Postgres) and a `Redis` (Key Value).
- Suspended by the user means its `suspenders` include `User` (or the legacy `Suspended`). A resource the platform suspended for billing, abuse or a crash loop is not offered to Resume.
- **Suspend** lists the selected suspendable resources that are not suspended. Those already suspended are left out, not reported.
- **Resume** lists the selected suspendable resources that are suspended by the user. Running ones are left out.

### Permission gating

- Suspend is enabled only when every selected suspendable resource allows it: `suspendServer`, `suspendCronJob`, `suspendDatabase` or `suspendRedis` at `permissionLevel ALLOWED`. Resume is the same over `resumeServer`, `resumeCronJob`, `resumeDatabase` and `resumeRedis`.
- When any selected resource refuses, the button stays visible and disabled, with the tooltip **This action is limited to admins.** One refusing resource disables the action for the whole selection; rows are not dropped from it.
- The docs' rule behind it: in a protected environment, only Admins may suspend or resume a service.

### The Suspend services modal

- Title **Suspend services**. Body: _"The following services will be suspended and stop working immediately. This action can be undone by resuming the services."_
- Then the list, capped at 12.5rem and scrolling: each row is the resource's icon and name, with its state on the right. Running shows nothing, suspending a spinner, suspended a check mark, and an error a small **Retry** button.
- A typed confirmation: `sudo suspend <N> service[s]`, or `sudo suspend <N> service[s] and <M> database[s]` when Postgres is selected. Key Value instances count as services.
- Buttons **Cancel** and **Suspend services** (critical). Once every row is suspended, the footer is a single **Close**.
- An outside click does not close it.

### The Resume services modal

- Title **Resume services**. Body: _"The following services will be resumed:"_, then the same list. Suspended shows nothing, resuming a spinner, resumed a check mark, and an error **Retry**.
- No typed confirmation. Buttons **Cancel** and **Resume services** (primary), then a single **Close** once every row is running.

### What submitting does

- One GraphQL mutation per resource, never a batch operation: `suspendService(id)` / `resumeService(id)` for a `Server` or `CronJob`, `suspendDatabase(id)` / `resumeDatabase(id)`, and `suspendRedis(id)` / `resumeRedis(id)`.
- At most 10 run at once (`Math.min(10, n)` workers drain one queue).
- A resolved call marks its row suspended (or active), and a rejected one marks it error. The modal shows no error message: the row's Retry runs that one call again.
- When the batch settles, the dashboard refetches `servicesForOwnerOverview` and logs an analytics event with the count that succeeded. It shows no toast.
- Closing the modal after every row succeeded clears the selection. Closing it with a failure left keeps the whole selection.

### Not present in the bundle

- No bulk delete, bulk restart or bulk deploy.
- No batch mutation: Render's public REST API and MCP server have no bulk suspend or resume either.

## What bex had before w4/m550

- A project environment's card had row checkboxes and an inline toolbar (`Selected: <N>`, an environment picker, **Move**), with no Suspend or Resume.
- The Overview's ungrouped table had per-row menus and no checkboxes.

## What bex has since w4/m550

- Both lists share one selection (`useResourceSelection`) and one toolbar (`BulkResourceToolbar`).
- The Overview's ungrouped list moves its selection into any of the workspace's environments, listed as `<project> / <environment>`.
- Move is disabled for a role without `can_create` (a Viewer), with the role's reason, as Render disables it without `moveResource`. bex-api refuses that role's placement write.
- The toolbar offers Resume and Suspend under Render's rules: when they show, the kinds they act on, the modals' copy, Render's typed phrase on Suspend, one call per resource with 10 at a time, per-row states and Retry, and **This action is limited to admins.** for a non-Admin in a protected environment.
- What bex builds differently is recorded in [ADR018](../ADR018-render-parity.md)'s Suspend / Resume row.
