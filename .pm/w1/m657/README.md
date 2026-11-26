# w1 · m657 — An Org Owner mints audit-log API keys, and the org Members page shows each member's login methods and 2FA, as on Render

**Worker:** worker1 **Goal:** an Org Owner creates, lists and revokes organization API keys that read the audit logs of the org and of every workspace in it, and nothing else; and the org Members page's **View details** shows a member's workspaces, the login methods they have signed in with and whether 2FA is on, as Render's does. **Status:** todo (t001–t006 and t008–t010 done, shipped in `3926ec646c`; t007 walked on dev-1 and on production, where two loading frames and a refusal code were fixed; their production check next, then t011)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's two surfaces in `organizations.md` and settle the details the bundle leaves open — **DONE** | 30m | — |
| [t002](done/t002.md) | Organization audit-log keys: store, mint, list and revoke on GraphQL, for Org Owners — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | An organization audit-log key reads the org's and its workspaces' audit logs, and is refused everywhere else — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | org Settings → Audit Logs gets Render's **Audit Log API Keys** section — **DONE** | 1h15m | t002 |
| [t005](done/t005.md) | GraphQL `orgUserAuthStatus`: a member's login methods and 2FA, for Org Owners — **DONE** | 1h | t001 |
| [t006](done/t006.md) | org Members: **View details** with Workspaces and **Login & Authentication Status** — **DONE** | 1h | t005 |
| [t007](t007.md) | Verify in a real browser and over the API, on dev-1 and on production | 45m | t003, t004, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 9h10m**.

## What Render does

Read 2026-11-26 from Render's public dashboard bundle (the `Settings`, `Members`, `user`, `ApiTokenHeader` and `frontendFlag` chunks; Rollup renames chunk hashes every deploy, so find them by the strings below) and from render.com/docs. Not walked signed in.

- **Audit Log API Keys** (org Settings → Audit Logs, under Export CSV).
  - Heading "Audit Log API Keys": "Create API keys with read-only access to all Audit Logs across this organization and its workspaces." A **Create API key** button opens a dialog with a name field, **Generate API key**, then the key shown once with **Done**. The list is the account API key table without its workspace and permission columns (Name, Created, Last used, an overflow menu with Revoke); empty, it reads "No provisioned API keys.".
  - Revoke asks "“<label>” will no longer be able to access your organization's Audit Logs." with **Revoke API key**. Errors: "Unable to load your organization's API keys." with Retry, "Unable to create your API key. Please try again.", "Unable to revoke your API key. Please try again.", and a rate-limited create "You have created too many keys. Please try again later.".
  - Without the org's `manageOrganization` permission the button is disabled and the section reads "You do not have permission to manage this organization's API keys.".
  - Operations: `query organizationAPITokens($organizationId)` reads `organization { organizationAPITokens { id createdAt label lastUsedAt expiresAt type } }`; `mutation provisionOrganizationAuditLogToken($organizationId, $label) { token }`; `mutation revokeOrganizationAPIToken($organizationId, $tokenId)` answers a Boolean.
  - **Flag:** the section renders only when the frontend flag `scoped_access_tokens` is on (the `ApiTokenHeader` chunk's hook, which reads `allFrontendFlags`), the same flag as Render's scoped account keys. **Docs:** [render.com/docs/api-keys](https://render.com/docs/api-keys) documents the feature with no beta wording: "Org Owners can create an audit log API key" from the org's Settings → Audit Logs → Audit Log API Keys → Create API key; "Audit log API keys have read-only access to audit logs for your organization and all of its workspaces."; "It's shown in full only once.". So the flag is a staged rollout of a documented feature, as `artifact_reuse` is for Build Sources, and the gap stands (the w1/149 rule, not w1/148's).
  - The keys read Render's public API: `GET /v1/organizations/{orgId}/audit-logs` and `GET /v1/owners/{ownerId}/audit-logs` (both in bex's pinned OpenAPI). Render's public API has no route that manages org keys.
- **A member's login status** (org Members → a member row's overflow menu → **View details**).
  - Not behind any flag. The dialog's header is the member's avatar and name (or email) with "Owner" for an Org Owner. It has a **Workspaces** section listing the member's workspaces and, when the viewer holds the org permission `viewOrgUserAuthStatus`, a **Login & Authentication Status** section, then a **Close** button.
  - Each login method is a row with "Signed in" (success) or "Not signed in" (warning), and a last row "Two-factor Authentication" reads "Enabled" or "Not enabled". Method labels: `pwd` → "Password", `saml-sso-…` → "SAML SSO", anything else capitalized (`google` → "Google").
  - Operation: `query orgUserAuthStatus($userId, $organizationId) { orgUserAuthStatus(userID, orgID) { loginMethodStatuses { method signedIn } is2FAEnabled } }`.
  - **Docs:** render.com/docs has no page for this dialog (login-settings, saml-sso, organizations and team-members read 2026-11-26). The nearest is saml-sso's enforcement dialog, "a report of your org members who have yet to log in via SSO", which is what "Not signed in" means.

## bex today

Verified 2026-11-26 on main `bea7a8664a`.

- **Org audit log:** `GET /v1/organizations/{orgId}/audit-logs` and GraphQL `organizationAuditLogs` (w7/m464, [ADR115 D7](../../../../docs/ADR115-organizations.md#d7--the-org-audit-log)) answer an Owner's own session or key. `audit.Service.ListOrganization` (`lego/backend/internal/audit/organizations.go`) checks `can_manage` on the org and then the caller's org role. A workspace's log is `audit.Service.List` (`audit/service.go`).
- **No organization key exists.** Every API key is a Hydra client bound to one workspace (`lego/backend/internal/apikeys/service.go`: `CreateAPIKey`, the binding's `TenantForKey`, the active-key quota, the `bex.co/last-used` stamp). The bearer form is `bex_<id>_<secret>` (`apikeys/bearerkey.go`), resolved in `lego/backend/internal/api/auth_apikey.go`. What a scoped key may call is `lego/backend/internal/api/key_permission_ops.go` and `scope_matrix_ops.go` (w9/m496).
- **No per-member login status.** GraphQL `OrganizationMember` has `subject`, `email`, `role`, `workspaces`, `isSelf`, `createdAt`, `avatarDownload` and no auth fields. The org enforcement gate reads 2FA enrollment and the Google account through `workspaces.KratosIdentities` (`MFAEnrolled`, `GoogleAccountMatches` in `lego/backend/internal/workspaces/kratos_verification.go`), and w1/m647 lists the members a requirement locks out, but no surface shows one member's methods.
- **Dashboard:** `dashboard/src/features/organizations/components/organization-audit-logs-card.tsx` (org Settings, Export as CSV only) and `organization-members-card.tsx` (a table with Remove; no row menu and no details dialog). Pending frames live in `organization-route-skeletons.tsx`.
- **Ledger:** rows `organizationAPITokens`, `provisionOrganizationAuditLogToken`, `revokeOrganizationAPIToken` and `orgUserAuthStatus` are `gap` on this milestone, and the `scoped_access_tokens` flag row names it, in [dashboard-operations.md](../../../../docs/render-artifacts/dashboard-operations.md#workspaces-members-and-organizations).

## Definition of done

1. An Org Owner creates an organization audit-log key with a label on GraphQL and in org Settings → Audit Logs → Audit Log API Keys; the key is shown once, then listed with its label, created and last-used times; revoking it stops it at once. A Member or Guest cannot list, create or revoke one, and sees Render's no-permission text. Too many keys answer Render's "too many keys" refusal.
2. The key, sent as a bearer, reads `GET /v1/organizations/{orgId}/audit-logs` and `GET /v1/owners/{ownerId}/audit-logs` for the org and each workspace in it, a workspace created after the key included, with Render's response shapes. It is refused on every other REST route, every other GraphQL field, MCP, and the SSH and shell paths, and on another org's logs and a workspace outside the org.
3. Dissolving the org revokes its keys. Whatever t001 settles for an Owner's removal and for the org's login requirements holds and is tested.
4. An Org Owner opens **View details** on any org member and sees the member's workspaces, a row per login method with Signed in or Not signed in, and Two-factor Authentication Enabled or Not enabled, matching what that member's account actually has. GraphQL `orgUserAuthStatus` answers the same, and refuses a caller without the permission and a subject outside the org without telling the two apart.
5. Each new section's pending frame matches its ready state at 1440 and 390, and the dialogs fit at 390 with no sideways scroll.
6. The four ledger rows and the flag row in `dashboard-operations.md`, `docs/render-artifacts/organizations.md`, ADR115 and ADR018's identity rows say what bex does.
7. Walked on dev-1 and, once a production pin carries the milestone, on production as the QA user on a Scale fixture org (disposable), as w7/m466 t006 built one.

## Source + Goal linkage

- **Source:** promoted from [w1/151](../../done/151.md) on 2026-11-26 by `/loopx w1`; the note was filed from w1/m645's dashboard ledger ([done/m645](../../done/m645/README.md)). Its own estimate already said "more than an hour: promote to a milestone".
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). Both are customer-facing organization behavior that Render shows every Org Owner: the keys behind a staged rollout of a documented feature, the login status with no flag at all. An Org Owner who needs audit logs in a SIEM, or to check who is ready before turning on Require Login Method (ADR115 D10), has no way to do it in bex today.
- **Expected outcome:** an org's audit logs can be read by a key that can do nothing else, and an Owner sees each member's sign-in methods and 2FA before or after enforcing them.
- **Why now:** the ledger just made both gaps visible and checkable, the org login settings (w7/m466) and their lock-out lists (w1/m647) are fresh, and an Owner's own key is today the only way to read the org log over the API, which hands a log reader everything the Owner can do.
- **Triage notes (2026-11-26):** the note's "no flag" was half wrong. The keys sit behind `scoped_access_tokens` (as the w1/148 worker reported), but render.com/docs/api-keys documents them, so the gap stands. The login status is unflagged and gated only by `viewOrgUserAuthStatus`; it lives in the row's **View details** dialog, not in a column as the note said. SAML SSO rows are out (`.pm/DO_NOT_DO.md`, Enterprise surfaces).
- **Render parity task:** included (t008), because the milestone adds GraphQL fields, REST authorization and dashboard UI.

## Unblock (2026-11-26)

- **Affects:** t007 step 4, then t011.
- **Source:** verified evidence. Production's newest pin, `chore(deploy): pin platform images to 410850e80281` (`918667a83f`), carries `3926ec646c` (`git merge-base --is-ancestor 3926ec646c 410850e80281` exits 0), so the gate `pin must carry 3926ec646c` holds.
- **Next:** a worker walks t007 step 4 on production as the QA user on a Scale fixture org (disposable) with a second identity as a Member, deletes the fixture, and closes with t011. No user action.

## Production walk (t007, 2026-11-26, pin `410850e80281`)

Walked as the QA user on a Scale fixture org (disposable) with a second identity as a Member, then deleted; the record is in [t007](t007.md).

- **DoD 1–4 hold**: minting, listing, using and revoking keys over GraphQL and in the browser, the quota and rate refusals with Render's text, the Member's disabled section, the key's reads and refusals on REST, GraphQL and MCP, the org's end revoking its keys, and View details against each identity's credentials, TOTP included.
- **Fixed here**: the Audit Log API Keys section and View details each showed a one-line loading text while their own read was in flight, so the card and the dialog grew when the answer landed (DoD 5); both now draw the ready state's frame. A Member was told the org was not found on the key operations; they now get 403 `FORBIDDEN`, as ADR115 D11 says.
- **Filed**: [w1/169](../169.md), the "needs: none" wording of an org key's refusal on a route that needs no permission.
