# ADR115 — Organizations: Scale and Enterprise workspaces belong to an org, as on Render

**Status:** accepted and implemented (2026-11-17, w7/m464): the store, the OpenFGA model, the GraphQL and MCP verbs, the org audit log at `GET /v1/organizations/{orgId}/audit-logs`, the support transfer command and the plan-table dispositions. The production walk waits for the image pin (w7/m464 t013).

**Basis:** a recommendation under [ADR008 § Render parity first](ADR008-vision.md#render-parity-first) (user decision 2026-10-17, restated 2026-10-20), not a user decision. ADR018 used to record that bex had no organization scope, with no sign-off, so parity applies. The user can still sign off "no organizations" as a divergence through `/unblock w7`, which would retire this ADR. Render's behavior is captured, with quotes and the read date, in [render-artifacts/organizations.md](render-artifacts/organizations.md).

## Context

Render's Scale and Enterprise plans put workspaces in an organization: "An org can contain unlimited workspaces at no additional cost". Upgrading a workspace to Scale creates the org, makes that workspace the primary and its upgrader the Org Owner. The org has Owner, Member and Guest, an org audit log at `GET /v1/organizations/{orgId}/audit-logs`, and a downgrade that deletes it once Render's preconditions hold.

bex had no organization. `deploy/gitops/authz/model.fga` declared an `org` type with `admin from org` on `workspace`, but nothing wrote an org tuple, and that relation would have made an org admin an implicit admin of every workspace, which Render rules out ("Org Owners are not _automatically_ added to any workspaces"). Each Scale or Enterprise workspace stood alone with its own plan, and `list-organization-audit-logs` was an omission.

## Decisions

### D1 — Model

- `organizations (id, name, primary_workspace_id, created_at, audit_visible_from)`. The id is `org-<xid>`, minted by `id.New(id.Organization)`; Render documents no org id prefix, so the prefix is bex's own. `primary_workspace_id` is unique and cascades: deleting the primary deletes the org.
- `organization_members (organization_id, subject, role, created_at)`, role `owner | member | guest`. A person has one org role.
- `tenants.organization_id` (nullable; `NULL` is a standalone workspace) and `tenants.privacy` (`invite_only` by default, or `public`).
- `organization_invites (id, organization_id, email, role, token_hash, invited_by, created_at, expires_at, accepted_at)`, role `owner | member`, ids `inv-` like workspace invites, token stored as its sha256 as workspace invites are (w1/041).
- `tenants.plan` stays the one column every feature gate reads. The org has no plan column: its plan is its primary's, and every write keeps the secondaries equal to it.

### D2 — Creation, plan and fee

- Changing a standalone workspace to Scale or Enterprise creates the org in the same transaction as the plan write. Its name is the workspace's name, the workspace is the primary, the caller becomes Owner, and every other person in the workspace becomes a Member ("all other workspace members become org members"). A caller who is not a person in the workspace (an API key) leaves every current admin as Owner instead.
- On the primary, a change between Scale and Enterprise writes every org workspace's plan in one transaction. A secondary's plan cannot be changed by itself: `400 ORGANIZATION_PLAN_ON_PRIMARY` with `params {organizationId, primaryWorkspaceId}`.
- The Scale or Enterprise fee is listed once per organization, on the primary. A workspace created in the org shows "Included in your organization's plan" and no price. Usage stays billed per workspace, each with its own Billing page and payment method, as on Render. bex charges no licensed plan fee today ([ADR030 §1](ADR030-pricing.md)); a future licensed SKU attaches once per org, to the primary's subscription.
- `organizations.audit_visible_from` is the moment the org was created, so the org log starts there ("Retention does not begin until you upgrade").

### D3 — Workspaces in the org

- An Owner creates a workspace in the org with a name and a privacy setting, Invite-only by default. It takes the org's plan, asks for no plan and no payment method (Render's org create flow asks for neither), and the creator becomes its first admin. The per-user Hobby cap does not apply: org workspaces are on a paid plan, where team-members allows unlimited workspaces.
- A Public workspace may be joined by any Member, who gets the Developer role (audit-logs, `JoinTeamEvent`). An Owner may join any org workspace as admin. A Guest may join none.
- A workspace admin changes its privacy (Render: "Workspace admins can change a workspace's privacy setting").
- Deleting a secondary works as any workspace delete does. The primary cannot be deleted while secondaries exist (`409 ORGANIZATION_PRIMARY_HAS_WORKSPACES`). Deleting a primary that is alone deletes the org.
- **Support transfer.** Render: "Existing workspaces can be transferred into an org by contacting Render support". bex's support path is the operator command `api move-workspace-to-organization <tea-id> <org-id> [--apply]`, run in the bex-api pod like `revoke-oauth-grant`. It attaches a standalone workspace (refusing one already in an org), sets its plan to the org's, makes its people Members of the org who were not already in it ("Each member of each transferred workspace becomes a member of your org"), and writes the tuples. Without `--apply` it prints what it would do.

### D4 — Roles

- **Owner** manages the org: its name, members and invites, creates workspaces, and reads the org audit log. An Owner is not a member of any workspace until they join one.
- **Member** has no org-level permission ("Other org members do not have an organization-level role"). They see the org and its workspaces, join Public ones, and are invited to Invite-only ones.
- **Guest** reaches only the one workspace they belong to: they see that workspace and the org's name, not its other workspaces or its members, and cannot join any workspace.
- **How people enter the org.** bex has no SSO (`.pm/DO_NOT_DO.md`, Enterprise surfaces). Render without SSO: "workspace admins can invite any Render user to any org-managed workspace. These users automatically become standard members of the org." bex does exactly that: redeeming a workspace invite in an org workspace makes a new person a Member. An Owner can also invite people to the org itself, as Owner or Member, by email; the invite redeems by its emailed token (`/invite#org-invite=<token>`, GraphQL `organizationInvitePreview` and `acceptOrganizationInvite`, MCP `preview_organization_invite` and `accept_organization_invite`, w7/083) for whoever opens it signed in, whatever their email, or at the invitee's sign-in with the invited email. A used link answers `INVITE_ALREADY_ACCEPTED`, a lapsed one `INVITE_EXPIRED`, as a workspace invite's does. An Owner makes someone a Guest by changing their org role, which is allowed while they belong to at most one org workspace; a workspace invite to a Guest's email for a second workspace is refused `409 ORGANIZATION_GUEST_SINGLE_WORKSPACE`, telling the admin to make them a Member first.
- Removing a person from the org removes them from every org workspace, through the same teardown a workspace removal runs (API keys, agent sessions, tuples). The last Owner cannot be removed or demoted (`409 ORGANIZATION_LAST_OWNER`). A removal that would leave an org workspace with no admin is refused (`409 ORGANIZATION_MEMBER_LAST_ADMIN`, naming the workspace).
- **Billing cap.** At most two distinct people hold the Billing role across an org's workspaces, counting pending Billing invites: a third is refused `409 ORGANIZATION_BILLING_LIMIT` with `params {limit: 2}`, on REST `PATCH /v1/owners/{ownerId}/members/{userId}`, GraphQL and MCP alike.

### D5 — Authorization (OpenFGA)

- `org`: `owner: [user]`, `member: [user] or owner`, `guest: [user]`; `can_manage: owner` (Render's "Org Owner" permissions), `can_create: owner` (create a workspace in the org), `can_view: member or guest`. The org type reuses the existing relation names, so its decisions run through the same audited, capability-scoped funnel as every workspace decision.
- `workspace`: `org: [org]`, written for every org workspace; `public_org: [org]`, written only while the workspace is Public. `can_join: member from public_org` and `can_join_as_admin: owner from org` are two new relations (`core.RelCanJoin`, `core.RelCanJoinAsAdmin`). `admin from org` is removed: an Owner holds no workspace relation until they join.
- The store is the source of truth and the tuples follow every org write. A one-shot reconcile at bex-api start writes the tuples of every org (idempotent), which covers the migration's backfill and a tuple write lost to a crash.

### D6 — Downgrade

Changing the primary below Scale is refused with one `400 ORGANIZATION_DOWNGRADE_BLOCKED` listing every unmet precondition in `params.preconditions`, each with its kind and what to clear:

- `secondary_workspaces`: the org still has workspaces besides the primary;
- `member_roles` and `invite_roles`: people or pending invites in the primary with the Billing, Viewer or Contributor role (Pro and Hobby offer neither, which bex's existing plan-capacity check already refused). Both keep Render's meaning on either target, so a Developer is never listed here;
- `member_limit`, on a Hobby target only: Hobby seats one member, so every other member and every pending invite is listed, to remove. The Owner making the change keeps the seat (w7/087);
- `inbound_ip_rules`: restricting inbound IP rules on the workspace, its environments or its web services and static sites. Rules on Postgres and Key Value are not affected, as on Render.

Each person is an `email:role` item, the email they are listed by on the Members page; a member whose email cannot be read is named by their sign-in id (w7/087).

Once they hold, the downgrade deletes the org, its members, its invites and its tuples in the same transaction as the plan write, and the primary carries on as a standalone workspace with its own members unchanged. The org's audit rows age out with the retention sweep.

### D7 — The org audit log

- Org events are audit rows whose resource is `org:<org-id>`. `audit_events.workspace_id` is the id part of the resource, so an org row is keyed by the org id and never appears in a workspace log, and no workspace row appears in the org log.
- `GET /v1/organizations/{orgId}/audit-logs` answers Render's `auditLogWithCursor` rows with the workspace log's parameters (`startTime`, `endTime`, `direction`, `cursor`, `limit` up to 1000, through `audit.FilterOf`). An Owner reads it (`can_manage` on the org), and so does one of the org's audit-log API keys (D11); anyone else gets 403, and so does an org that does not exist or was dissolved, so an org id reveals nothing (`audit.Service.ListOrganization`; seen on production 2026-11-17, w7/m464 t013). History is floored at the org's `audit_visible_from`. GraphQL `organizationAuditLogs(organizationId, …)` reads the same rows, and the org Settings page's **Export as CSV** pages the REST route, at most 90 days per export. Render's MCP has no audit-log tool, so bex adds none, as for workspaces.
- Events bex records: `InviteToOrgEvent`, `AcceptOrgInviteEvent`, `AddOrgMemberEvent` (a person added other than by an org invite: a workspace invite that made them a Member, a support transfer), `RemoveOrgMemberEvent`, `JoinTeamEvent`, `ChangeOrgRoleEvent`, `ChangeOrgNameEvent`, `CreateWorkspaceEvent` and `DeleteWorkspaceEvent` (for workspaces created in or deleted from the org), `ChangeWorkspacePrivacyEvent`, and, since w7/m466 (D10), `ChangeOrg2FAEnforcementEvent` and `ChangeOrgAllowedLoginMethodsEvent` with `metadata.to`. An org created by an upgrade records no Render event; the workspace's own plan change stays in its own log.
- Non-goals: the IdP events (`CreateOrgDomainEvent` … `RevokeOrganizationSCIMToken`), because SSO and SCIM are out (`.pm/DO_NOT_DO.md`). No other Render org event is left unrecorded (`renderOrganizationEvents`).

### D8 — Surfaces

- REST keeps Render's shapes. Render's public API has no org read besides the audit log, so `GET /v1/owners` and `GET /v1/owners/{ownerId}` stay byte-identical, and MCP `list_workspaces` keeps Render's owner objects.
- GraphQL: `organizations`, `organization(id)` (id, name, plan, primary workspace, the caller's role, its workspaces with privacy and primary, members for Owners and Members), `Workspace.organization`, `Workspace.primary`, `Workspace.privacy`, and the org mutations.
- MCP: `list_organizations` and `get_organization` with GraphQL's fields, plus the org member, join and privacy tools, as bex's members tools already go beyond Render's MCP.
- Dashboard: the org sits in the workspace switcher, with Workspaces, Members and Settings pages; plan surfaces say the org's plan is managed on the primary. An org invite's link opens the `/invite` page, which reviews it (the org, the role, the inviter, the signed-in account) and joins on request, then opens the org's Workspaces page.
- Since w1/m657: GraphQL `organization` also takes Render's `organizationId` argument, `Organization.organizationAPITokens` and the key mutations of D11, `OrganizationMember.workspaceRoles`, and Render's `orgUserAuthStatus(userID, orgID)`: for an Org Owner, a member's login methods (`pwd`, `google`, `github`) with whether they have signed in with each, and `is2FAEnabled` (D12). The org Members page's row menu holds **View details** and, for an Owner, **Remove member**; the Settings page's Audit Logs section shows every org member Render's section, with its no-permission texts for anyone but an Owner.

### D9 — Backfill

Migration `0293_organizations` makes each existing Scale and Enterprise workspace the primary of its own org, named after it, with its current user admins as Owners and its other people as Members. bex never recorded who upgraded a workspace, and an admin already controls it. Two such workspaces held by one admin become two orgs; joining them is the support transfer, run only on that customer's request.

### D10 — Org login settings

An Org Owner can require two-factor authentication and Google login in every workspace of the org (w7/m466), Render's "**Require Login Method** under **Security**" without SAML SSO, and its org 2FA enforcement. While a setting is on, each org workspace enforces it beside its own setting (the OR of the two), and so do the org's own objects; turning it off leaves each workspace's own setting as it was. Owners change it on the org Settings page's Security card and GraphQL; every person in the org reads it. Refusals reuse the workspace codes with the org named; each change is one org audit row (D7). The full rule, with which parts are inferred from Render's pages, is [ADR012 §9c](ADR012-auth.md#9c-organization-enforced-two-factor-authentication-and-google-login-w7m466).

### D11 — Org audit-log API keys (w1/m657)

Render: "Org Owners can create an audit log API key" from the org's Settings → Audit Logs → Audit Log API Keys; "Audit log API keys have read-only access to audit logs for your organization and all of its workspaces"; "It's shown in full only once" (render.com/docs/api-keys). Render shows the section behind its frontend flag `scoped_access_tokens`, a staged rollout of a documented feature ([render-artifacts/organizations.md](render-artifacts/organizations.md#audit-log-api-keys)).

- **The key.** A bex API key like any other (a Hydra client stamped with its creator and sign-in, sent as `Authorization: Bearer bex_<id>_<secret>`), whose `organization_api_keys` row (migration 0349) makes it the org's: bound to no workspace, holding no OpenFGA tuple. Its grant is `core.OrganizationKeyGrant`: the dispatch gate passes it to exactly `GET /v1/organizations/{orgId}/audit-logs` and `GET /v1/owners/{ownerId}/audit-logs` (`organizationKeyOps`) and refuses every other REST route, GraphQL field and MCP tool with a scoped key's refusal, an operation added later included. It mints no ticket, so the SSH gateway and the shells never see it. The two verbs authorize through `core.Base.AuthorizeOrganizationAuditLog` and `AuthorizeWorkspaceAuditLog`, which read the key's row and the workspace's org uncached on each request: the key reads its own org's log and the log of every workspace the org holds when it asks, a workspace created later included, one that left not. [ADR012 §8g](ADR012-auth.md#8g-organization-audit-log-api-keys-w1m657) has the authorization rule.
- **Who manages them.** Org Owners (`can_manage` on the org, then the Owner role in the store), Render's `manageOrganization`: GraphQL `organization(organizationId) { organizationAPITokens { id createdAt label lastUsedAt expiresAt type } }`, `provisionOrganizationAuditLogToken(organizationId, label) { token }` and `revokeOrganizationAPIToken(organizationId, tokenId): Boolean`, Render's operations. A Member or Guest is refused 403 `FORBIDDEN`, as `orgUserAuthStatus` refuses them; someone outside the org is told the org was not found (`ORGANIZATION_NOT_FOUND`). Production first told a Member the org was not found too (w1/m657 t007), because the `can_manage` check came first; `can_view` now decides who is told not found. Minting is a person's own act (`AuthorizeMintClass`). No REST or MCP surface: Render's public API and MCP have none.
- **Lifecycle.** The key is the org's, not its creator's (Render's list names no creator): it outlives its creator's demotion and departure from the org, and any Owner revokes it. A revoke marks the row revoked, which stops the key on its next request on every replica, then deletes its Hydra client. The org's end (a downgrade, or deleting its primary) leaves every key reading nothing, since its reads need the org, and then revokes them. Deleting the creator's account revokes the key, as every key a person made ([ADR086](ADR086-account-deletion.md)); the org's list stops showing it. The row stays as a tombstone in every case, so a key never falls back to a bare unbound key.
- **Login requirements.** The org's login requirements (D10), and a workspace's when its log is read, hold the key as they hold any key: by its creator's second factor and the sign-in it was minted in ([ADR012 §9a–9c](ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402)).
- **Limits.** At most 20 live keys per org (the workspace quota), and the workspace keys' creation rate per org; either answers `ORGANIZATION_API_KEY_LIMIT`, which the dashboard shows as Render's "You have created too many keys. Please try again later.". `expiresAt` is always null: the docs give the keys no expiry. `type` is `AUDIT_LOG`.
- **Audit rows.** Minting and revoking write no row to the org's log: Render's org event list has no event for them, and the org log answers only Render's events (`renderOrganizationEvents`). A refused attempt is still recorded.

### D12 — A member's login status (w1/m657)

Render's org Members page opens **View details** on a member: their workspaces with their role in each and, for a viewer with `viewOrgUserAuthStatus`, **Login & Authentication Status**, one row per login method ("Signed in" or "Not signed in") and "Two-factor Authentication" ("Enabled" or "Not enabled"). bex gives that permission to Org Owners, as every other org setting. GraphQL `orgUserAuthStatus(userID, orgID)` takes the member's `userId` (`usr-…`, Render's `user.id`) or subject, and lists `pwd`, `google` and `github` in Render's order, every method bex offers whether or not the org's login requirements allow it; `signedIn` is whether the member's Kratos identity holds that credential (a password set, the provider linked), which signing in with it the first time creates. `is2FAEnabled` is the 2FA gate's rule (a TOTP credential or a WebAuthn key). A Member or Guest asking, an Owner asking about someone the org does not hold, and an unknown user get the same 403; someone outside the org is told it was not found; a Kratos read that fails is an error, never a status that reads "Not signed in". SAML SSO rows stay out (`.pm/DO_NOT_DO.md`).

## Consequences

- A Scale or Enterprise team runs unlimited workspaces under one org and one plan, with Render's org roles and audit log. The pricing table's `Workspaces`, `Organization user roles` and `Org-level audit logs` rows get a checked disposition (`plan_table_test.go`).
- `admin from org` is gone from the model; no code ever wrote the tuple it read.
- SSO, SCIM and domain capture stay out (`.pm/DO_NOT_DO.md`). Enterprise seat-based billing is a follow-up; org-level login settings shipped as D10 (w7/m466).
