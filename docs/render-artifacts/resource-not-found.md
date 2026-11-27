# Dead resource links: Render's not-found, deleted, no-access and load-error states

**Captured:** 2026-11-27, from Render's public dashboard bundle read without sign-in (`index-D5V4ZgbE.js` and the chunks it imports). No signed-in Render session was available, so the capture is bundle-only: strings and rules are quoted from the code, not from screenshots.  
**Implementation:** w3/m706  
**Supersedes:** w9/m55's redirect home with a "That resource doesn't exist or was deleted." toast (user decision 2026-07-17, Render unverified then, no ADR018 sign-off).

Render never redirects or toasts for a dead resource link: the URL stays, and the page draws one of the states below in place, inside the dashboard shell (a `Wrapper` with `mt-12 mb-20` around a ghost `EmptyState`: a 500 px column, the title as `type-heading-03`, the body as `type-body-02`, the buttons under them). A toast appears only after the user's own delete, which then navigates to the list.

## The states

Find each one in the bundle by the strings quoted here.

### Resource not found / deleted (`ResourceNotFound-CqxL2YY9.js`)

Props `resourceType`, `resourceId`, `resourceLabel`, `name`, `variant`, `links`, `title`, `renderBody`.

| Part | Unknown id (no cached name) | Deleted (the cache still holds its name) |
| --- | --- | --- |
| Title | `` `${resourceLabel} not found` `` | `` `${resourceLabel} deleted` `` |
| Body | "We couldn’t find `<id>`. It may have been deleted, or the link may be incorrect." | "**`<name>`** (`<id>`) was deleted." |

- The id is an inline `CodeSnippet` (a code chip); the name is `text-strong`.
- The title is chosen by whether `name` is set; `variant` (`unknown`, `deleted`, `preview`, `pull_request_preview`) only feeds analytics and the service's preview bodies. `title` and `renderBody` override the defaults.
- **Actions:** at most two buttons. The first link is the primary button (`links[0]`, or **Return home** when there is none). **Return home** (`/`) follows as a `secondary` button unless the primary already is Return home. The link list is built by `r(context, extra)`: the caller's own links, then **Go to project** (`/project/<prj-id>`) when the cached resource's `environment.project.id` is set, then Return home. So a deleted service in a project reads Go to project, then Return home; one in no project reads Return home alone; a link the caller adds first (View deploys) pushes Go to project out of the two buttons.
- **Analytics:** "Resource Not Found Viewed" on mount and "Resource Not Found Action Clicked" with the action (`go_to_project`, `go_to_dashboard`, `view_deploys`, `view_previews`).
- **Deleted vs unknown:** each caller reads a `missing…Context` fragment from Apollo's cache with `useFragment` (`missingServerContext`, `missingCronJobContext`, `missingSandboxGroupContext`, `missingWorkflowContext`: `id`, `name`, `environment { id name project { id name } }`). A query that comes back empty leaves the normalized entity in the cache, so a resource the dashboard had already read (its page open, or a list that showed it) reads as deleted, and a link to an id the dashboard never saw reads as not found.

### Deploy or build not found (`DeployDetailsHeader-BsF-c7WS.js`)

`ResourceNotFound` with `resourceLabel` `Deploy` or `Build`, `variant: unknown`, `links: [View deploys, Return home]` and its own body: "We couldn’t find `<id>` for **`<service name>`**. The link may be incorrect." The title is "Deploy not found" or "Build not found". It draws inside the service's own layout.

### Page not found (`Error404-CCmfI9sy.js`)

Title "Page not found", body "`<id>` doesn’t exist." (the id an inline code snippet; no body without an id), one primary button: `actionText` (default "Return home") to `actionUrl` (default `/`).

### Access denied (`Error403-jHswJleF.js`)

Title "Access denied", body "You don’t have access to `<id>`." ("You don’t have access to this resource." without an id), one primary button: `actionText` (default "Return home") to `actionUrl` (default `/`).

With a `joinHint` (`extensions.joinHint.workspaceId` on the GraphQL error, read by `errors-Bc7Ku8FH.js`), the page looks the workspace up among the caller's organizations (`query joinHintLookup { organizations { id name workspaces(currentMember: false) { id name orgPrivacy } } }`) and, when it finds it, adds a second paragraph:

- a public workspace: "It belongs to **`<workspace>`**, a workspace in **`<organization>`** that you can join." with a **Join workspace** button (`joinTeam(teamId)`, then a reload; a failure toasts "We weren’t able to add you to this workspace. Try again?");
- a private one: "It belongs to **`<workspace>`**, a private workspace in **`<organization>`**. Ask an admin for an invite." with Return home.

The error classifiers: a permission error is any GraphQL error with `extensions.code === 403`; not found is an error whose message includes `not found`.

### An error occurred (`lH` in `index-D5V4ZgbE.js`)

Title "An error occurred", body "`<id>` could not be loaded.", one primary button: `actionText` (default "Return home") to `actionUrl` (default `/`).

## Which page draws which state

Render's order on every page: loading → its skeleton; data → the page; then the rows below. "Other error" is a failure that is neither 403 nor not found.

| Render page | bex call site | 403 | Not found | Other error | Empty result, no error |
| --- | --- | --- | --- | --- | --- |
| Service (`/web`, `/worker`, `/pserv`, `/static/:id`) | `features/services/components/service-detail-layout.tsx` | Access denied + join hint | "Service not found" / "Service deleted" | redirect home | "Service not found" / "Service deleted" |
| Cron job (`/cron/:id`) | the same layout, for a cron job | Access denied + join hint | "Cron job not found" / "Cron job deleted" | redirect home | "Cron job not found" / "Cron job deleted" |
| Deploy (`…/deploys/:deployId`) | `features/deploys/components/deploy-detail-page.tsx` | Access denied | "Deploy not found" for `<service>`, View deploys | inline alert "We’re sorry. We weren’t able to load this deploy. Refresh and try again?" | skeleton |
| Cron job build | `features/build-sources/components/build-detail-page.tsx` (bex's build of a build source; Render opens those in a drawer, so its cron build page is the nearest) | "Build not found" | "Build not found" for `<service>`, View deploys | "Build not found" | skeleton |
| Build Source (`/build-sources/:id`) | `features/build-sources/components/build-source-layout.tsx` | Access denied, no id | Page not found | Page not found | Page not found |
| Postgres (`/d/:id`) | `routes/databases.$databaseId.tsx` | Access denied + join hint | Page not found (when the error's path is `database`) | blank | blank |
| Key Value (`/r/:id`) | `routes/keyvalue.$keyValueId.tsx` | Access denied + join hint | Page not found | Page not found | blank |
| Environment group (`/env-group/:id`) | `routes/env-groups_.$groupId.tsx` | Access denied + join hint | Page not found | redirect home | blank |
| Blueprint (`/iac/:id`) | `routes/blueprints.$blueprintId.tsx` | Page not found | Page not found | Page not found | Page not found |
| Webhook (`/webhooks/:id`) | `routes/webhook.$webhookId.tsx` | Access denied, **Go to Webhooks** | Page not found, **Go to Webhooks** | redirect home | blank |
| Project (`/project/:id`) | `routes/project.$projectId.tsx` | Access denied + join hint | Page not found | Page not found | Page not found |
| Sandbox group (`/sandbox-group/:id`) | `routes/sandbox-groups_.$sandboxGroupId.tsx` | Access denied + join hint | An error occurred | An error occurred | "Sandbox group not found" / "Sandbox group deleted" |
| Workspace (`/w/:id/…`) | `routes/w.$.tsx` | not read from the bundle: `WorkspaceSettings` waits on the workspace's read and switches to its owner | — | — | — |
| `/resource/<id>` (`ResourceRedirector-DwoXTM_f.js`) | `routes/resource.$.tsx` (w3/m705) | Access denied + join hint | "`<Label>` not found" by the id's prefix (Service, Cron job, Database, Key Value instance, Environment group, Sandbox group, Blueprint, Webhook, …; "Resource" otherwise), Return home only | redirect home | the same not found |

Pages outside bex's call sites draw the same components: Dedicated IPs ("Go to Dedicated IPs") and Private Links ("Go to Private Links") use Page not found, and the workflow pages use all four. Workflows and preview environments are non-goals here.

## What bex draws

w3/m706 replaced `useNotFoundRedirect` at every call site above (`dashboard/src/common/components/resource-not-found.tsx`, `dashboard/src/common/lib/resource-read-state.ts`):

- **Not found** draws the row's own state: "`<Label>` not found" / "`<Label>` deleted" with Go to project and Return home on a service, cron job and sandbox group, the deploy and build wording with View deploys and View builds, and Page not found (with Go to Webhooks on a webhook) on the others. A sandbox group's not-found answer draws "Sandbox group not found", where Render draws An error occurred for any error: bex-api reports a dead id as a `NOT_FOUND` error where Render's API answers an empty result. The deleted variant reads the resource's name and project from the Apollo cache, as Render's `missing…Context` fragments do. The dashboard's own delete keeps its own landing and success toast (w4/645).
- **No access** (bex-api's `FORBIDDEN`, which it answers for a resource in a workspace the caller is not in) draws Access denied with the id on every page but the build source, which names no id as Render's does (Render's Blueprint draws Page not found for it instead), and `/w/<id>` for a workspace the caller is not in draws it with the workspace id. Apollo caches a refused read's null without its error, so the route loaders keep a `forbidden` state the pages read. bex-api sends no join hint, so the second paragraph and Join workspace are not drawn (w3/158).
- **Other errors** on a resource page draw An error occurred with the id and Return home on every page, where Render redirects home on some pages and draws a blank page or Page not found on others. A deploy or a build keeps its own in-page load error, under the live service or build source. An expired session keeps bex's Sign in state (w3/m80).
- The document title is the drawn title: "Service not found ・ bex Dashboard", "Access denied ・ bex Dashboard", and so on.
- `/resource/<id>` for a known prefix opens the type's page, which draws that page's state. Render draws its own "`<Label>` not found" at `/resource/<id>` instead (w3/159).
