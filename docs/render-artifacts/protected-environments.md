# Render protected-environments dashboard contract

**Captured:** 2026-07-15, live from an authenticated Render dashboard session plus current official documentation. The dashboard inspection was read-only: no environment protection, isolation, or IP rule was changed.

This is the UI source for `w5/m31`'s Environment settings and protected-action flows. Transient screenshots are stored under the gitignored `.playwright-mcp/` directory.

## Environment controls

An Environment card's `•••` menu exposes both a direct **Block cross-environment connections** toggle and **All settings**. The latter opens an Environment settings page whose **Permissions** section presents:

- a default state where all team members can delete and suspend resources, move services, and manage database networking;
- **Protected**, labeled “Best for production”; and
- cross-environment connections as **Allowed** or **Blocked**.

Render's documentation confirms that protection is role enforcement: Admins can designate an Environment as protected, after which only Admins can perform the listed destructive or sensitive actions. The restricted set includes deleting resources, creating/moving resources, suspending or resuming services, changing datastore access-control IPs, and accessing secrets or shells. Blueprint updates are a documented exception for non-Admin members. Network isolation affects only private traffic; it does not block public endpoints or SSH. See [Projects and Environments](https://render.com/docs/projects) and [workspace roles](https://render.com/docs/team-members).

Evidence:

- `.playwright-mcp/render-protected-environment-menu.png`
- `.playwright-mcp/render-protected-environment-settings.png`

## Typed destructive confirmations

Render's ordinary destructive dialogs use a resource-type-specific `sudo` phrase, independent of the protected-environment role gate. Two live dialogs showed:

- deleting the web service `beancount-cms-v2`: `sudo delete web service beancount-cms-v2`
- suspending the static site `beancount-cms`: `sudo suspend static site beancount-cms`

The action remains disabled until the entire phrase matches. Render protection itself does not add a second typed-confirmation bypass for a non-Admin; the role lacks permission.

Evidence:

- `.playwright-mcp/render-delete-service-confirmation.png`
- `.playwright-mcp/render-suspend-service-confirmation.png`

## Environment inbound IP rules

Environment-level rules live under **Networking → Inbound IP Restrictions** on eligible Scale/Enterprise plans. The editor is a replaceable list: edit an existing CIDR, **Add source**, delete with the trash control, then **Save**. Render seeds `0.0.0.0/0` as the allow-all default. Deleting every rule means deny-all, and a source must pass every applicable workspace, Environment, and service rule. Only IPv4 CIDRs are supported. See [Inbound IP Rules](https://render.com/docs/inbound-ip-rules).

The captured editor uses two columns, **Source** and **Description**. The description is editable and optional; the public API exposes the same pair as `{cidrBlock, description}`. Render's official documentation was rechecked on 2026-07-16 and still specifies the same edit/add/delete/save flow for the shared service, Environment, and workspace editor.

Evidence: `.playwright-mcp/render-environment-ip-rules.png` (current official dashboard image captured through the docs page).

## Public API contract

**Captured:** 2026-07-15 from Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json).

- `POST /v1/environments` requires `name` and `projectId` and optionally accepts `protectedStatus`, `networkIsolationEnabled`, and `ipAllowList` as `{cidrBlock, description}` objects.
- The Environment response object contains `id`, `name`, `projectId`, `databasesIds`, `redisIds`, `serviceIds`, `envGroupIds`, `protectedStatus`, `networkIsolationEnabled`, and optional `ipAllowList`. It does **not** contain `ownerId`, `createdAt`, or `updatedAt`; bex's GraphQL-only `ownerId`/`createdAt` fields are explicit product extensions.
- `GET /v1/environments` returns `[{environment, cursor}]`. Its list parameters are `name`, required `projectId`, `createdBefore`, `createdAfter`, `updatedBefore`, `updatedAfter`, `ownerId`, `environmentId`, `cursor`, and `limit` (default 20, range 1–100). The list-valued filters use form/explode-false semantics, so comma-separated and repeated values are OR alternatives.
- bex can evaluate `name`, `projectId`, created-time, `ownerId`, and `environmentId` from its current view. It cannot evaluate updated-time filters because the Environment view/store contract has no `updatedAt`; those parameters therefore return a named 400 instead of being silently ignored.

## bex implementation decisions and drift

`w5/m31` mirrors the discoverability of Render's Environment card: **All settings** opens protection, cross-environment isolation, and a full-replace CIDR editor. It also exposes Environment-group membership in the existing **Manage resources** dialog and reuses one Project/Environment selector across service, Postgres, and Key Value creation.

`w5/m38` completes the Environment editor's rule fidelity: its GraphQL read/write path uses `ipAllowListEntries {cidrBlock, description}`, existing rules expose editable CIDR and optional-description fields, and new rules accept both values. Description-less legacy entries normalize to an empty optional description instead of disappearing. Add, edit, delete, save, and empty-list deny-all behavior now match the captured Render editor. The established product divergences remain explicit: bex supports IPv6 CIDRs and seeded existing environments with both `0.0.0.0/0` and `::/0`, whereas Render documents IPv4-only rules with a single IPv4 default.

For protected actions, bex follows its already-shipped backend contract rather than pretending Render has the same authorization model. A first delete, suspend, or Blueprint sync that would override a protected resource returns the authoritative bex phrase (`sudo <verb> service <name>`, `sudo <verb> database <name>`, or `sudo <verb> key value <name>`). The dashboard displays that exact phrase, requires an exact match, and retries with it; it never synthesizes the phrase or turns a generic error into a bypass. Unprotected actions retain their existing confirmation flow.

**w6/m37 (2026-07-17)** extended this guard from Apps to Postgres and Key Value members using their `app.bex.co/environment-id` CR label, covering delete and suspend over REST/GraphQL, suspend over MCP, and every dashboard action placement. Resume remains deliberately unguarded because it restores availability. Render's official MCP server has no datastore delete tools; bex adds none. `suspend_keyvalue` is a documented bex lifecycle extension.

`w4/m28` (2026-07-15) closed the enforcement/default drift this capture originally filed as `.pm/w4/018.md`: environment rules now apply to every eligible public member (web services + static sites via a chained HTTP middleware, datastores via a chained MiddlewareTCP — a source must pass both the environment's and the resource's own layer, Render's intersection semantics), an empty list means deny-all, and existing empty-list environments were migrated seeded-open (`0.0.0.0/0` plus the `::/0` IPv6 twin, a documented bex extension) so no tenant was locked out. The workspace rule layer remains an explicit deferral (bex has no workspace-level IP surface anywhere). `w4/m32` (2026-07-16) then closed the three lifecycle-correctness gaps m28's own consistency review found: a service leaving its project (not its environment) no longer strands a frozen `spec.environmentIPAllowList`, deleting a project fans the layer clear out to every cascaded environment's members, and a one-shot idempotent backfill (`api environments-backfill`) repairs pre-m32 drift and stamps environments whose rules predate m28. Design + mechanics: docs/ADR032-environments.md § Inbound IP rules.
