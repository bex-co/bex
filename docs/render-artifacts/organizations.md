# Render organizations — what Render documents, read 2026-11-17

Captured for w7/m464 t001 from Render's own pages, fetched as markdown (`curl https://render.com/docs/<page>.md`) on **2026-11-17**. Every fact [ADR115](../ADR115-organizations.md) relies on is quoted here with its page. The last section lists what Render does not say, and the choice bex made in each case.

## Sources

| Page | URL |
| --- | --- |
| Organizations | https://render.com/docs/organizations |
| Audit Logs | https://render.com/docs/audit-logs |
| Workspaces, Members, and Roles | https://render.com/docs/team-members |
| Pricing table rows | [pricing.md](pricing.md) lines 189-197 and 233 (captured earlier) |
| Pinned public API | `lego/backend/internal/api/openapi/render-public-api-1.json`, operation `list-organization-audit-logs` |

## What an organization is

- organizations: "With a Render _Scale_ or _Enterprise_ plan, you can manage all of your team's users, workspaces, and services in a single _organization_ (or _org_)".
- organizations: "An org can contain unlimited workspaces at no additional cost. Each org member can belong to any combination of workspaces, based on the services they need to access. You can also add guests, who receive access to a single workspace."
- organizations, glossary: the primary workspace is "The original workspace that was upgraded to the _Scale_ plan. Your primary workspace manages the billing for the organization."
- team-members: "With a _Scale_ or _Enterprise_ plan, you can manage multiple workspaces and team members in a unified organization."
- pricing.md:189-197: `Workspaces: 1 | 1 | Unlimited | Unlimited`, `Organization user roles: — | — | Owner, Member, Guest | Owner, Member, Guest`, `Audit logs: — | Workspace | Workspace and organization | …`.

## Creating an org

- organizations: "When you sign up for the _Scale_ plan, Render automatically creates an organization with a workspace".
- organizations: "If you upgrade an existing workspace to _Scale_, Render creates an org and designates that workspace as the org's _primary workspace_. The team member who performs the upgrade becomes the Org Owner, and all other workspace members become org members."
- organizations: "With an _Enterprise_ plan, the Render team works with you directly to create your organization."
- organizations: "If you have existing Render workspaces, you can transfer them into your org for centralized management. Each member of each transferred workspace becomes a member of your org."
- organizations, Transferring: "To transfer an existing workspace into your org, reach out to our support team in the Render Dashboard."

## Adding workspaces

- organizations: "_Org Owners_ can create a new workspace": "open the workspace dropdown at the top of the left pane and select your org", "From your org's _Workspaces_ page, click _+ New Workspace_", "Provide a name for the workspace", "Set the workspace's privacy setting to _Public_ or _Invite-only_", "Click _Create Workspace_".
- organizations: "_Public_: Any org member can add themselves to the workspace." "_Invite-only_: Only admins of the workspace can invite other org members."
- organizations: "Render creates the workspace and adds you as its first admin. You can immediately start creating services and inviting members."
- The org create flow asks for a name and a privacy setting only: no plan and no payment method (contrast team-members' standalone create: "specifying a plan type and payment method").

## Roles

- organizations, The Org Owner role: "On the _Scale_ plan, the Org Owner is the team member who upgrades their workspace." Org Owners can "Manage all org-level settings, such as integrating your IdP for SSO and SCIM", "Create new workspaces in the org", "Invite or remove other org members (including other Org Owners)", "Add or remove the Org Owner role from other org members", "Add themselves to any workspace as an admin". "Org Owners are not _automatically_ added to any workspaces in the org."
- organizations: "Other org members do not have an organization-level role or associated permissions. Workspace-level permissions depend on a member's role within each workspace."
- organizations, Member types: "If you _haven't_ enabled SSO, workspace admins can invite any Render user to any org-managed workspace. These users automatically become standard members of the org."
- organizations, Member types (with SSO): a _Standard member_ is "Any user with an email address managed by your IdP … After joining, they can then add themselves to any public workspace in the org and receive invitations to invite-only workspaces." A _Guest_ is "Any user with an email address that _isn't_ managed by your IdP … Workspace admins can invite guests to individual workspaces in the org. Guests can't access any org resources _except_ those in the single workspace they're invited to."
- organizations, Per-workspace access: "Newly added workspaces are invite-only by default. Workspace admins can change a workspace's privacy setting from the workspace's Settings page." "Standard org members can add themselves to any public workspace (guests cannot)." "When you add an org member to a workspace, you can assign them any member role."
- audit-logs, `JoinTeamEvent`: "Org Owners can add themselves to any workspace as an admin. Other org members can add themselves to public workspaces only (they receive the Developer role)."
- team-members: "On a _Scale_ or _Enterprise_ plan with multiple workspaces, org members can have a different role in each workspace."

## The Billing role

- organizations: "When you add an org member to an individual workspace, you can assign them the _Billing_ role". "Each org supports up to two total members with the _Billing_ role."
- organizations: on Enterprise orgs with seat-based billing, a member who holds Billing in every org workspace they belong to is not charged a seat (custom terms; out of scope here).

## Downgrading

- organizations: "Downgrading removes your organization and access to org-level features." "When you downgrade from _Scale_, Render deletes the org and its configuration, including SSO, SCIM, and domain capture."
- organizations, "Before you can downgrade, you must": "Delete all secondary workspaces (only the primary workspace can remain)." "Reassign all team members and pending invites with the _Billing_, _Viewer_, or _Contributor_ role to _Developer_ or _Admin_." "Delete any inbound IP rules for services, environments, and workspaces. Rules for managed datastores are not affected."
- organizations: "Your _primary workspace_ continues as a standalone workspace on the new plan." "An Org Owner can downgrade from _Scale_ to _Pro_ or _Hobby_ from the org's primary workspace": the org's Workspaces page, "the _•••_ menu next to the workspace labeled _Primary_", "_Workspace settings_, then go to _Billing_", "Select a new plan".

## Org audit logs

- audit-logs: "With a _Scale_ or _Enterprise_ plan, you can also export a _separate_ audit log of organization events for your org." "Org-level audit logs are available on the Scale plan or higher."
- audit-logs, Organization events tab: "Only org members with the Org Owner role can export these audit logs." "open the workspace dropdown at the top of the left pane and select your org. Click _Settings_ in the left pane and scroll down to the _Audit Logs_ section. Select a start and end date, then click _Export as CSV_."
- audit-logs: "Each audit log export can span a maximum of 90 days." "Render retains at least the last 90 days of audit log events for your workspace and organization. Retention does not begin until you upgrade to an eligible plan."
- audit-logs: org events "appear _only_ in audit logs exported from your org's _Settings_ page (not in audit logs exported for an individual workspace)."
- audit-logs, Member management: `InviteToOrgEvent` ("A user was invited to the org"), `AcceptOrgInviteEvent` ("An invitation to join the org was accepted"), `AddOrgMemberEvent` ("A user was added to the org"), `RemoveOrgMemberEvent` ("A user was removed from the org"), `JoinTeamEvent` ("An org member added themselves to a workspace in the org"), `ChangeOrgRoleEvent` ("An org member's role was changed … a member's org-level role (such as Org Owner), not their role within a particular workspace"), `ChangeOrgAllowedLoginMethodsEvent`, `ChangeOrg2FAEnforcementEvent`.
- audit-logs, Workspace management: `CreateWorkspaceEvent` ("A workspace was created in the org"), `DeleteWorkspaceEvent` ("A workspace in the org was deleted"), `ChangeWorkspacePrivacyEvent` ("The access setting for a workspace in the org was changed").
- audit-logs, IdP management: `CreateOrgDomainEvent`, `VerifyOrgDomainEvent`, `DeleteOrgDomainEvent`, `CreateSSOConnectionEvent`, `UpdateSSOConnectionEvent`, `DeleteSSOConnectionEvent`, `ProvisionOrganizationSCIMToken`, `RevokeOrganizationSCIMToken`.
- Pinned OpenAPI: `GET /organizations/{orgId}/audit-logs` (`list-organization-audit-logs`) takes `startTime`, `endTime`, `direction`, `cursor` and `limit` (`auditLogLimitParam`), and answers an array of `auditLogWithCursor`. Its event enum also carries `ChangeOrgNameEvent` and `UpdateOrgPaymentMethodEvent`, which the docs page does not list.

## Audit log API keys

Read 2026-11-26 for w1/m657 from Render's public dashboard bundle (the `Settings`, `ApiTokenHeader` and `frontendFlag` chunks, found by the strings below; Rollup renames chunk hashes every deploy) and from render.com/docs/api-keys. Not walked signed in.

- docs, api-keys: "Org Owners can create an audit log API key" from the org's _Settings_ → _Audit Logs_ → _Audit Log API Keys_ → _Create API key_. "Audit log API keys have read-only access to audit logs for your organization and all of its workspaces." "It's shown in full only once." No beta wording and no expiry.
- Bundle: the section sits in the org Settings page's _Audit Logs_ section ("Render logs material actions taken by members of your organization."), under _Export CSV_ ("Download Audit Logs for a selected date range."), and renders only while the frontend flag `scoped_access_tokens` is on (the hook reads `allFrontendFlags`), the flag of Render's scoped account keys. The docs describe the feature, so the flag is a staged rollout, as `artifact_reuse` is for Build Sources.
- Heading "Audit Log API Keys", then "Create API keys with read-only access to all Audit Logs across this organization and its workspaces." and a secondary **Create API key** button. Without the org's `manageOrganization` permission the button is disabled and the section reads `permissions.manageOrganization.message`, else "You do not have permission to manage this organization's API keys.", and the list is not asked for. A non-manager still sees the whole _Audit Logs_ section, whose export reads "You do not have permission to perform this action." without `readOrgAuditLogs`.
- The list: "Loading API keys…" while it loads; "Unable to load your organization's API keys." with **Retry** when it fails; the account keys' table without its workspace and permission columns (Name, Key as the id's first characters then "…", Created, Last used, "Never" for an unused key, an overflow menu with **Revoke**), sorted by last use; "No provisioned API keys." when empty.
- Create dialog: the title "Create an API key" with the org's avatar and name above it, "Give your API key a unique name, like “agent” or “laptop”.", a name field (label "API key name", placeholder "name", at most 32 characters with an `n/32` counter), **Close** and **Generate API key**. Created: "API key created" with a success icon, the key in a copyable field, "Save this API key now. You won’t be able to see it again." and **Done**. Errors: "You have created too many keys. Please try again later." for a rate-limited create, else the server's message or "Unable to create your API key. Please try again.".
- Revoke dialog: "Revoke API key", "“<label>” will no longer be able to access your organization's Audit Logs.", **Cancel** and **Revoke API key**; a failure reads "Unable to revoke your API key. Please try again.".
- Operations: `query organizationAPITokens($organizationId) { organization(organizationId: $organizationId) { id organizationAPITokens { id createdAt label lastUsedAt expiresAt type } } }`, `mutation provisionOrganizationAuditLogToken($organizationId, $label) { provisionOrganizationAuditLogToken(organizationId, label) { token } }` and `mutation revokeOrganizationAPIToken($organizationId, $tokenId) { revokeOrganizationAPIToken(organizationId, tokenId) }`, a Boolean. The list's type is `APIToken` with an `APITokenType` enum; the bundle names no value for these keys.
- The keys read Render's public API: `GET /organizations/{orgId}/audit-logs` and `GET /owners/{ownerId}/audit-logs`. The public API has no route that manages org keys, and Render's MCP has neither the keys nor audit logs.

## A member's login status

- Bundle (the `Members` and `user` chunks): org Members → a member row's overflow menu → **View details**, the first item, before **Promote to owner** or **Demote from owner** and **Remove member**; the member's email in the row opens it too. Not behind any flag.
- The dialog's header is the member's avatar and name (or email), with "Owner" under it for an Org Owner. A **Workspaces** section lists the member's workspaces (avatar, name, an "Invite-only" lock, their role), or "No workspaces" and "This member has not been invited to a workspace.". When the viewer holds the org permission `viewOrgUserAuthStatus`, a **Login & Authentication Status** section follows: one row per login method with "Signed in" (success) or "Not signed in" (warning), then "Two-factor Authentication" with "Enabled" or "Not enabled". Then **Close**.
- Method labels: `pwd` is "Password", `saml-sso-…` "SAML SSO", anything else its name capitalized (`google` is "Google", `github` "Github"). Render's login method list, in its order: `pwd`, `google`, `saml-sso`, `github`, `gitlab`, `bitbucket`.
- Operation: `query orgUserAuthStatus($userId, $organizationId) { orgUserAuthStatus(userID: $userId, orgID: $organizationId) { loginMethodStatuses { method signedIn } is2FAEnabled } }`, with the row's `user.id`.
- Docs: render.com/docs has no page for this dialog (login-settings, saml-sso, organizations and team-members read 2026-11-26). The nearest is saml-sso's enforcement dialog, "a report of your org members who have yet to log in via SSO", which is what "Not signed in" means.

### What the bundle and docs leave open, and bex's choice (w1/m657 t001)

| Open | bex's choice | Basis |
| --- | --- | --- |
| Which login methods a member's status lists | `pwd`, `google`, `github`, in Render's order, whether or not the org's login requirements allow them. No SAML (`.pm/DO_NOT_DO.md`), GitLab or Bitbucket row: bex has no such login. | Every method bex offers; the dialog is how an Owner sees who is ready before turning a requirement on. |
| What `signedIn` means | The member's Kratos identity holds that credential: a password set, the provider linked. Signing in with a method the first time creates it. `is2FAEnabled` is the 2FA gate's rule, a TOTP credential or a WebAuthn key. | The nearest observable fact; Render's saml-sso dialog reports who "ha[s] yet to log in". |
| Who holds `viewOrgUserAuthStatus` | Org Owners. | Every other org setting is an Owner's, and `manageOrganization`, which gates the keys, is. |
| What `userID` takes | The member's `userId` (`usr-…`, Render's `user.id`) or their subject. | The dashboard holds both for a member. |
| Who manages the keys | Org Owners, Render's `manageOrganization`. | The docs: "Org Owners can create an audit log API key". |
| Whether a key outlives its creator's demotion or departure | Yes: the key is the org's, and any Owner revokes it. | Render's list names no creator. |
| The creator's account deletion | Revokes the key, as every key a person made ([ADR086](../ADR086-account-deletion.md)); the list stops showing it. | The login requirements hold a key by its creator, which a deleted account no longer answers for. |
| The org's login requirements | Hold the key by its creator's second factor and the sign-in it was minted in, as every key ([ADR012 §9a–9c](../ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402)). | The nearest bex rule. |
| The org's end (a downgrade or its primary deleted) | Its keys read nothing at once and are revoked. | The milestone's definition of done. |
| `expiresAt` | Always null. | The docs give the keys no expiry. |
| `type` | `AUDIT_LOG`. | The bundle names no value. |
| The quota, and the "too many keys" error | At most 20 live keys per org (the workspace quota) and the workspace keys' creation rate per org; either is `ORGANIZATION_API_KEY_LIMIT`, which the dashboard shows as Render's rate-limit text. | bex's workspace key limits; Render's one sentence for a refused create. |
| Audit rows for minting and revoking | None. A refused attempt is still recorded. | Render's org event list (render.com/docs/audit-logs) has no event for them, and the org log answers only Render's events. |

## What Render does not say, and bex's choice

| Unsaid | bex's choice (ADR115) |
| --- | --- |
| Render's org id prefix. Neither the pinned OpenAPI (no example), `render-oss/cli`'s generated client (`orgId string`) nor `team-members.graphql` shows one. | `org-`, minted by `id.New(id.Organization)`. The prefix is bex's own. |
| How a person becomes a Guest without SSO. Render ties Guest to an email outside the IdP, and without SSO every invitee "automatically become[s] a standard member". | bex has no SSO (`.pm/DO_NOT_DO.md`), so a workspace invite makes a standard Member, as Render does without SSO. An Owner makes someone a Guest by changing their org role, which bex allows while they belong to at most one org workspace. |
| How an Owner invites someone to the org itself, without SSO. | An org invite by email, as Owner or Member, redeemed by token like a workspace invite (`InviteToOrgEvent`, `AcceptOrgInviteEvent`). |
| Whether a new org workspace needs its own payment method. | No payment step at creation, as Render's org create flow asks for none. Each workspace keeps its own billing, usage and payment method, so its paid usage follows the workspace's existing payment gate. |
| The privacy of the primary workspace when an upgrade creates the org. | Invite-only, Render's default for new workspaces. |
| Whether the primary can be deleted while secondaries exist. | Refused (`ORGANIZATION_PRIMARY_HAS_WORKSPACES`): the primary holds the org's plan, and Render's downgrade requires deleting the secondaries first. Deleting a primary with no secondaries deletes the org. |
| Who becomes Owner when an existing Scale or Enterprise workspace predates organizations. | Every current admin of the workspace (bex never recorded who upgraded it, and an admin already controls the workspace); every other member becomes a Member. |
| Whether removing a member from the org removes their workspace memberships. | Yes. A former org member keeps no access to org workspaces, as a Guest's access ends with the org's. |
| Whether the org can be renamed. | Yes, by an Owner, recording `ChangeOrgNameEvent` (in the pinned enum). |
