# w22 · m402 — Workspace admins can require 2FA, as on Render: a member without it is blocked, API keys and SSH included

**Worker:** worker22 **Goal:** a workspace admin can turn on "require two-factor authentication" in Workspace Settings. While it is on, a member who has not enrolled TOTP or a passkey cannot use the workspace's resources or settings from the dashboard, REST, GraphQL, MCP or SSH, and neither can an API key or SSH key that member created. They are sent to enroll, and regain access the moment they do. Turning the setting on and off is audited. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record: where the setting lives, what "has 2FA" means, which principals and routes it covers, the admin lock-out guard and the check's cache; amend ADR012, ADR024 and ADR018 | 45m | — |
| t002 | The setting: a workspace column and migration, an admin-only GraphQL mutation and read field, and an audit event named as Render names it | 45m | t001 |
| t003 | Enforcement in bex-api: a member without 2FA gets a named 403 on every REST, GraphQL and MCP route of the workspace, except the exempt ones | 75m | t002 |
| t004 | Enforcement at the SSH gateway and the ticket mints: service SSH, the browser shell, agent-session attach and sandbox connect refuse a member without 2FA, and a live session is re-checked | 45m | t003 |
| t005 | Dashboard: the admin toggle in Workspace Settings, the blocked-member page that leads to enrollment, and the member row's required/not-enabled state | 75m | t002, t003 |
| t006 | Live check on `dev-N` with three kinds of principal: an enrolled member, a member without 2FA (session, API key, SSH) and an admin enabling it | 45m | t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- An admin of any workspace, on any plan, can turn the setting on and off in Workspace Settings. A non-admin cannot (named 403 on the mutation, no control in the UI).
- With it on, for a member who has no TOTP and no passkey enrolled: a dashboard session, a REST/GraphQL/MCP call carrying their session, an API key they created, and an SSH or shell ticket mint for a service in the workspace all get the named 403 `WORKSPACE_2FA_REQUIRED` (or the gateway's refusal), with the workspace id in its details. The dashboard shows a page that explains why and leads to Kratos enrollment in account settings. The member's other workspaces are unaffected.
- After the member enrolls, access returns within the check's cache lifetime (recorded in the ADR). A member who removes their last second factor while it is on is blocked again within the same bound, and a live SSH or shell session is closed at the gateway's next revalidation.
- The admin who enables it must have 2FA themselves, so the setting cannot lock out its only actor (a bex safety guard, recorded in the ADR as a recommendation of this milestone, not a Render fact).
- Enabling and disabling write an audit-log entry that REST `GET /v1/owners/{ownerId}/audit-logs` and GraphQL return under Render's event type `ChangeTeam2FAEnforcementEvent`.
- Every path above has an automated test, and the live check on `dev-N` is recorded in this README with the observed codes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`.
  - **Render** ([render.com/docs/login-settings](https://render.com/docs/login-settings), read 2026-11-14): "Enforce two-factor authentication (2FA) from your **Workspace Settings** page"; "Only workspace admins can configure these enforcements"; "your team members can't access the workspace's resources or settings until they enable 2FA for their Render account"; "Team members with SSH or API keys can't use these keys to access workspace resources until they enable 2FA". The page lists no plan requirement for 2FA enforcement (the plan requirement it states is for SAML SSO).
  - **Pinned Render OpenAPI** (`lego/backend/internal/api/openapi/render-public-api-1.json`): the audit-log event type enum contains `ChangeOrg2FAEnforcementEvent` and `ChangeTeam2FAEnforcementEvent`.
  - **bex today:** Kratos has TOTP, WebAuthn and recovery codes with a `highest_available` AAL policy (ADR018 MFA row, w4/m11; ADR012 §9). `lego/backend/internal/workspaces/kratos.go` (~lines 191–199) derives `MFAEnabled` from the identity's `totp`/`webauthn` credentials, and `members/service.go` and `dashboard/src/features/team/components/member-row.tsx` only display it as a badge. There is no workspace setting, no refusal and no audit event. `grep -rniE "2fa.?enforc|WORKSPACE_2FA|Team2FA|Org2FA|secure login" .pm docs lego dashboard/src` finds only the OpenAPI enum.
- **Goal linkage:** ADR008 § Render parity first (Identity, workspaces and account); pillar 1, Render-compatible REST and GraphQL.
- **Expected outcome:** a workspace holding production credentials can require every member, and every key and SSH session a member holds, to be second-factor protected, as on Render.
- **Why now:** bex has paying workspaces (`.pm/w2/blocked/058.md` records 9 live subscriptions) whose members hold production credentials, and the second-factor foundation is shipped; the missing piece is only the enforcement layer. SSO/SAML/SCIM stay a non-goal (DO_NOT_DO): this is not SSO, and Render lists no plan gate for it, so it applies to every workspace.
- **Open recommendations, not user decisions** (this milestone's tasks implement them; the ADR in t001 records them as recommendations): "has 2FA" means an enrolled TOTP or passkey rather than an AAL2 session (Kratos already forces AAL2 on login for an enrolled user); the admin lock-out guard; exempting non-human platform principals. Nothing here diverges from Render's observable behavior, so no ADR018 divergence sign-off is needed. The sibling Render setting on the same page, allowed login methods (`ChangeTeamAllowedLoginMethodsEvent`), is not part of this milestone.
- **Render parity included:** the milestone adds a GraphQL field and mutation, a dashboard setting and an audit event, so t007 compares each surface with Render.
