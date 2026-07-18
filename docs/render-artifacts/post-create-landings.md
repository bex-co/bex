# Dashboard post-create landing contract

**Captured:** 2026-07-17  
**Scope:** every resource, child-record, credential, export, or deploy creation action reachable from the current bex dashboard  
**Implementation:** w5/m45

## Contract

Post-create behavior is resource-specific, not a blanket redirect:

1. A standalone resource with a detail page opens the most useful detail state immediately from the mutation's returned immutable id. Provisioning continues on that page; the dashboard does not wait for `available` or derive a route from a display name.
2. A contextual child record remains on its owning resource and refreshes there. If it creates an independently inspectable resource, an explicit View action or automatic landing uses the returned id.
3. A server-minted one-time secret remains visible until explicit acknowledgement. Navigation must never destroy the only copy.
4. A failed mutation never navigates, closes the interaction, or clears recoverable form state.
5. URL-owned dialogs finish their close/search replacement before starting the final detail navigation, preventing the close from racing and overwriting the destination.

This matches Render's documented resource-specific outcomes. Render redirects a newly created Project to its page, shows new Key Value instances progressing to Available in the dashboard, creates PITR as a separate Postgres instance, and shows an API key in full only at creation ([Projects](https://render.com/docs/projects), [Key Value](https://render.com/docs/key-value), [Postgres recovery](https://render.com/docs/postgresql-backups), [API keys](https://render.com/docs/api)). The live Postgres-create capture that initiated w5/m45 landed on `/d/dpg-d9dhmev7f7vs738lrf5g-a` while provisioning.

## Creation matrix

### Standalone resources

| Dashboard action | Successful result used | Required successful outcome | Failure outcome | Evidence / decision |
| --- | --- | --- | --- | --- |
| Create Service — web, private, worker, cron, or static | `createService.id`; `latestDeployId` when the control-plane deploy row exists | `/services/{srv-id}/deploys/{dep-id}` when a deploy id exists; otherwise `/services/{srv-id}` | Stay on `/services/new` with the form and inline/toast error | `routes/services.new.tsx`; [new-service-wizard.md](new-service-wizard.md). All five types share one mutation and landing branch. |
| Create Postgres | `createDatabase.id` | `/databases/{dpg-id}` immediately in the creating state | Keep `/?new=database` and the populated dialog open | `routes/index.tsx`; `create-database-dialog.tsx`; live Render `/d/{dpg-id}` capture. w5/m45 fixes the previous list-only refetch. |
| Recover Postgres with PITR | `recoverDatabase.id` | `/databases/{new-dpg-id}` for the independently provisioned recovery instance | Keep the source database's recovery dialog populated | `recovery-panel.tsx`; Render documents PITR as a new database instance whose status advances through Recovery/Creating/Available. w5/m45 fixes the previous boolean-only hook result. |
| Create Key Value | `createKeyValue.id` | `/keyvalue/{red-id}` immediately in the creating state | Stay on `/keyvalue/new` with the form populated | `routes/keyvalue.new.tsx`; [key-value.md](key-value.md); [Render Key Value docs](https://render.com/docs/key-value). |
| Create Project | `createProject.id` | `/project/{prj-id}` after the URL-owned dialog closes | Keep `/?new=project` and the populated dialog open | `routes/index.tsx`; [Render Projects docs](https://render.com/docs/projects). |
| Create Environment Group from the workspace page | `createEnvGroup.id` | `/env-groups/{evg-id}` | Keep `/env-groups` and the populated dialog open | `routes/env-groups.tsx`; [env-group-create.md](env-group-create.md). |
| Create Workspace | returned workspace `id` | Select the returned `tea-*` workspace, then `/`; refetch the switcher list in the background | Stay on `/new/workspace`; do not change workspace selection | `routes/new.workspace.tsx`. There is no separate workspace-detail route; the selected workspace's home is its operating context. |

### Contextual children and one-time credentials

| Dashboard action | Successful result used | Required successful outcome | Failure outcome | Evidence / decision |
| --- | --- | --- | --- | --- |
| Create Environment inside a Project | `createEnvironment.id` confirms success; the named query refetches | Remain on the owning Project and refresh its Environment selector/cards | Keep the dialog and name | `new-environment-dialog.tsx`; an Environment has no standalone detail route in bex. |
| Create Environment Group from a Service Environment page | `createEnvGroup.id` is retained by the create hook; caller refetches linked/available groups | Remain on the Service Environment page with the current service pre-linked | Keep the dialog and staged contents | `services/components/env-groups-panel.tsx`; workspace-destructive group management intentionally stays out of the service context. |
| Create outbound webhook | endpoint `id` plus mint-once signing `secret` | Stay on the create page's secret step; explicit **View webhook** opens `/webhook/{whk-id}` | Keep the create form | `routes/webhooks_.new.tsx`; [webhooks-ui.md](webhooks-ui.md). Bex's mint-once secret is a documented divergence from Render's retrievable secret. |
| Create API key | key `id` plus mint-once `secret` | Keep the modal open on the secret/copy step; refresh the key list; **Done** returns to Account Settings | Keep the name step | `api-keys/components/create-api-key-dialog.tsx`; [Render API docs](https://render.com/docs/api). The hook uses `no-cache`, so dismissal destroys the only browser copy. |
| Add registry credential | success plus returned metadata at the API layer; caller supplied the token | Close the modal and refresh Account Settings → Registry Credentials | Keep all fields and the modal open | `create-registry-credential-dialog.tsx`; the secret is caller-supplied and never returned by reads. |
| Add SSH public key | mutation success; query returns typed `ssk-*` metadata | Stay in Account Settings and refresh the SSH-key list | Keep the form and current account context | `ssh-keys-panel.tsx` / `use-ssh-keys.ts`; there is no key-detail route. |
| Invite workspace member | invite success; APIs mint an invite id/token | Close the modal and refresh Workspace Settings → Team pending invitations | Keep email/role and show the plan or mutation error | `invite-member-dialog.tsx`; the invite is operated from its owning workspace. |
| Create Postgres user | returned one-time password, keyed by username within the database | Stay on database Access Control and show the password until the page is left | Keep the username input; do not show a false credential | `access-control-panel.tsx`; the password is not recoverable later. |
| Create logical Postgres export | returned export record `id` and status | Stay on database Recovery, refetch/poll the exports table, then expose the signed download | Stay on Recovery and show an error toast | `use-recovery.ts` / `recovery-panel.tsx`; [Render Postgres recovery docs](https://render.com/docs/postgresql-backups). |
| Add custom domain | returned primary domain and optional auto-paired sibling | Stay in Service Settings and replace the form with DNS instructions for every added host | Keep the domain form and error state | `custom-domains-section.tsx`; [custom-domain-dns-instructions.md](custom-domain-dns-instructions.md). |
| Manual deploy, restart, or rollback | returned `dep-*` id | `/services/{srv-id}/deploys/{new-dep-id}` | Keep the current service/deploy page; no missing-id or rejected mutation navigates | `manual-deploy-button.tsx`, `deploy-actions.tsx`; ADR018 **Trigger a deploy** / **Rollback**. A rollback creates a new deploy rather than rewriting history. |
| Add/update Service env vars or secret files | coherent patch result; optional deploy id for requested rollout | Remain on `/services/{srv-id}/env`, clear the committed draft, and show rollout state without discarding masked secrets | Preserve the draft and retry state | `service-environment-editor.tsx`; [service-environment-page.md](service-environment-page.md). These are configuration children, not standalone resources. |
| Add/update Environment Group variables or secret files | mutation success within `evg-*` | Remain on the Environment Group detail and refresh its contents | Preserve the editor/input | `env-groups_.$groupId.tsx`; group contents have no independent route. |

Authentication/identity flows, settings updates, deletes, links/assignments, rotations such as deploy-hook regeneration, and external GitHub App installation are not resource-create actions in this matrix. The source audit covered every dashboard GraphQL mutation; this boundary distinguishes create outcomes from mutations that update an existing owner or credential.

## API identity audit

| Resource family | REST create identity | GraphQL create identity | MCP create identity | Dashboard dependency |
| --- | --- | --- | --- | --- |
| Service / deploy | service object `id`; deploy envelope where supported | `createService.id` + `latestDeployId`; trigger/rollback `dep-*` | service view `id`; deploy result for deploy tools | Requires `srv-*`; optionally `dep-*` |
| Postgres, including PITR | Postgres view `id` | `createDatabase.id`; `recoverDatabase.id` | Postgres view `id` | Requires returned `dpg-*` |
| Key Value | Key Value view `id` | `createKeyValue.id` | Key Value view `id` | Requires returned `red-*` |
| Project / Environment / Environment Group | created view `id` | created view `id` | created view `id` | Uses returned `prj-*`/`env-*`/`evg-*` where a destination exists |
| Workspace | Render exposes no workspace write REST API | returned workspace `id` | Render's MCP workspace tools are read-only; bex mirrors that | Selects returned `tea-*` |
| Webhook / API key | created object `id`; mint secret only in create response | created object `id`; mint secret only in create response | created object `id`; mint secret only in create response | Retains both id and secret locally until acknowledgement |

The dashboard hooks must treat a missing required id as a failed create. Falling back to the submitted display name is invalid because bex resource ids are typed opaque identities and remain stable across rename ([ADR020](../ADR020-identifiers.md)). w5/m45 removes the former name fallback from Service, Postgres, and Key Value create hooks.

## Surfaces without a direct dashboard create action

| Resource | Current dashboard behavior | Classification |
| --- | --- | --- |
| Blueprint | `/blueprints` lists registered Blueprints and detail pages read, validate, and sync. A Blueprint is auto-registered when a deploy includes a repo plus `bex.yml`. | Not a post-create landing defect because no direct Create Blueprint action exists. A dedicated creation/import UI would be separately sized feature work. |
| Preview / workflow / task / one-off-job resources | No dashboard create action | Deliberate non-goals in `.pm/DO_NOT_DO.md`, not omissions from this contract. |

## Automated and browser evidence

- Route-level suites cover successful immutable-id destinations and non-navigation on failure for Service, Postgres, Key Value, Project, Environment Group, and Workspace creation. Hook/component suites reject missing ids for the standalone resources, Projects, Environments, PITR, and rollback instead of reporting false success.
- Contextual coverage pins Environment refresh/retention, webhook/API-key secret retention, database-user password reveal, custom-domain DNS continuation, registry-credential retention on failure, invitations, and SSH-key list refresh.
- **Browser walk, 2026-07-17:** local-bex plus the real Vite dashboard in headless Chrome created Postgres at `/databases/dpg-localmrq1igj2` on a 1440×900 viewport and Key Value at `/keyvalue/red-localmrq1ilzi` on a 390×844 viewport. Both detail pages visibly rendered `Creating` immediately and polling converged to `Available`.
- The same run added `m45.example.test` while remaining on `/services/eden-cms-v2/settings`, showed a newly minted API key only inside the `/settings` acknowledgement dialog and removed it after **Done**, and injected a `CreateDatabase` GraphQL rejection that remained at `/?new=database` with `m45-failure` still populated. No blocking browser-console errors occurred in the final run.
- Screenshots are gitignored under `.playwright-mcp/`: `m45-postgres-create.png`, `m45-keyvalue-create-narrow.png`, `m45-custom-domain-context.png`, `m45-api-key-secret.png`, and `m45-create-failure.png`. The local fixture was in-memory and both fixture/Vite processes were terminated after the run, leaving no shared or production resource to clean up.
- Quality gates on 2026-07-18 after rebasing onto current `origin/main`: `yarn typecheck` passed, `yarn lint` passed, the focused landing suite passed 110/110 tests in 15 files, and the full dashboard suite passed 1526/1526 tests in 244 files. Exact closeout commands are also recorded in the completed w5/m45 milestone README.
