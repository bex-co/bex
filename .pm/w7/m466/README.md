# w7 · m466 — An organization can require two-factor authentication and Google login in every one of its workspaces, as a Render org can

**Worker:** worker7 **Goal:** an Org Owner turns on "Require two-factor authentication" and "Require Google login" on the org Settings page and on GraphQL. While a requirement is on, every workspace in the org enforces it beside the workspace's own setting, and so do the org's own pages. A member, an API key or an SSH key that does not meet it gets the coded 403 the workspace settings already give, on REST, GraphQL, MCP and the SSH gateway. Each change is recorded in the org audit log as Render's `ChangeOrg2FAEnforcementEvent` or `ChangeOrgAllowedLoginMethodsEvent`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's org login settings and record the design in ADR012, ADR115, ADR018 and ADR030](t001.md) | 45m | — |
| t002 | [Store: the org's two settings, read by the gate for every org workspace](t002.md) | 1h | t001 |
| t003 | [The org setting: Owner-only GraphQL mutations, plan gate, lock-out guards and the two org audit events](t003.md) | 1h30m | t002 |
| t004 | [The gate: every org workspace and the org's own pages refuse a caller who does not meet the org's requirement](t004.md) | 1h30m | t002 |
| t005 | [Dashboard: the org Settings Security card, the locked workspace switches and the blocked page naming the org](t005.md) | 1h30m | t003, t004 |
| t006 | [Production: org 2FA on a Scale fixture org](t006.md) | 1h | t005 |
| t007 | [Render parity](t007.md) | 45m | t005 |
| t008 | [Simplify](t008.md) | 30m | t007 |
| t009 | [Test coverage](t009.md) | 1h | t007 |
| t010 | [Closeout](t010.md) | 10m | t006, t008, t009 |

## Definition of done

- An Org Owner turns "Require two-factor authentication" and "Require Google login" on and off on the org Settings page (a **Security** section, where Render puts "Require Login Method") and on GraphQL. Members and Guests see each setting's state but cannot change it. The surface rule follows the workspace settings: t001 records whether the org settings also get an MCP mutation.
- While org 2FA is on, a person without a second factor is refused with `403 WORKSPACE_2FA_REQUIRED` in **every** workspace of the org, whatever that workspace's own setting is. The refusal covers every REST, GraphQL and MCP route and every SSH gateway path, keeps the exempt reads the workspace setting has, and covers their API and SSH keys. The same holds for org Google login with `403 WORKSPACE_GOOGLE_LOGIN_REQUIRED`. The org's own pages (Members, Workspaces, Settings, the audit log) refuse such a person the same way.
- A workspace in an enforcing org shows its own switch as on and locked, "Required by <org>". Turning the org setting off leaves each workspace's own setting as it was.
- An Owner who does not meet a requirement cannot turn it on (409, the lock-out guard). A member of an org that requires Google login cannot change their email or unlink Google, and the Kratos settings guard names the org.
- Each change writes one org audit row. 2FA writes `ChangeOrg2FAEnforcementEvent` and Google login writes `ChangeOrgAllowedLoginMethodsEvent`, readable on `GET /v1/organizations/{orgId}/audit-logs` and GraphQL `organizationAuditLogs`. A no-op writes nothing. `renderOrganizationEvents` in `lego/backend/internal/audit/render_inventory_test.go` maps both events instead of listing them as `notYet`.
- ADR012 (a §9c beside §9a and §9b), ADR115 (D7 and a new decision), ADR018 (the Workspace-enforced 2FA row and its Google sibling) and `docs/render-artifacts/login-settings.md` state the shipped rule. Each Render inference is marked as one.
- On production, t006's walk passes on a Scale fixture org. The Google half is walked on production once production has Google sign-in (w7/m465 t008's gate); until then it is walked on dev-7, and that check is recorded as the open one.

## Source + Goal linkage

- **Source:** promoted from [w7/081](../done/081.md) on 2026-11-17. w7/m464 t014 found it, and ADR115 D7 lists both events as follow-ups.
- **Render's behavior** (read 2026-11-17; there is no Render account, `w10/blocked/002`, so only the docs were read):
  - [audit-logs](https://render.com/docs/audit-logs): "`ChangeOrgAllowedLoginMethodsEvent`: The org's set of allowed login methods was changed." "`ChangeOrg2FAEnforcementEvent`: Two-factor authentication enforcement was enabled or disabled for the org." These events appear "_only_ in audit logs exported from your org's **Settings** page", and org audit logs are "available on the Scale plan or higher".
  - [saml-sso](https://render.com/docs/saml-sso): the org setting lives at "**Settings** in the left pane … **Require Login Method** under **Security**". You "select SAML SSO as a required login method", and only an Org Owner can do it. When it is enforced, "Each listed member will lose access to the org until they log in via SSO" and "Each listed API key will be invalidated until its owner logs in via SSO".
  - [login-settings](https://render.com/docs/login-settings) describes only the workspace settings. On a workspace, the Google-login requirement is what `ChangeTeamAllowedLoginMethodsEvent` records (w7/m465).
  - **Inferences, recorded as such by t001:**
    - The org's "set of allowed login methods" is the org form of the workspace's Google-login requirement, with SAML SSO as one more choice.
    - Org 2FA enforcement binds every workspace in the org, as SSO enforcement binds "access to the org".
    - The confirmation lists the members and keys who would be blocked, as SSO enforcement's does.
- **bex today:**
  - `organizations` (migration `0293_organizations`) has no login settings.
  - The gate is `core.Base.requireTwoFactor` → `twofactor.Gate.Check` (`lego/backend/internal/twofactor/gate.go`). It reads `PGStore.LoginRequirements(workspaceID)` (`internal/store/workspaces.go:208`), which selects `tenants.require_two_factor, require_google_login` alone and is cached 60 s per workspace.
  - The workspace setters are `workspaces.Service.SetRequireTwoFactor` and `SetRequireGoogleLogin` (`internal/workspaces/service.go:1215`). They share `setLoginRequirement`, the plan gates `core.FeatureTwoFactorEnforcement` and `core.FeatureGoogleLoginEnforcement`, and the lock-out guards.
  - Org mutations live in `internal/workspaces/organizations*.go` (`renameOrganization` is GraphQL only) and `internal/members/organizations*.go`. The org log maps verbs in `internal/audit/service.go` (`AuditVerbOrgRenamed` → `ChangeOrgNameEvent`).
  - The Kratos settings guard asks `GoogleLoginWorkspaces` (`internal/store/workspaces.go:225`), which reads the workspace setting alone.
  - The dashboard org pages are `dashboard/src/routes/org.$orgId.*.tsx` with `dashboard/src/features/organizations/`. The workspace cards are `dashboard/src/features/workspaces/components/workspace-login-requirement-card*`, and the blocked pages are `workspace-two-factor-blocked-page.tsx` and `workspace-google-login-blocked-page.tsx`.
- **Premise check:** it holds. `render_inventory_test.go:158-159` lists both events as `notYet`, and nothing in `lego/` stores an org login setting.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)), organizations ([ADR115](../../../docs/ADR115-organizations.md)), workspace login settings ([ADR012 §9a](../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), [§9b](../../../docs/ADR012-auth.md#9b-workspace-enforced-google-login-w7m465)).
- **Expected outcome:** an org on Scale or Enterprise sets its members' login rules once for all of its workspaces, as on Render, instead of setting them workspace by workspace. The org log's two remaining non-IdP events get recorded.
- **Why now:** m464 shipped organizations and m465 shipped the Google-login gate on 2026-11-17. The shared gate, its caches, the SSH verb and the org log are fresh, and these two events are the only org events left that are not IdP ones.
- **No user decision gates the code.** Under Render parity first, building both settings is the parity choice. SAML SSO stays out of "Require Login Method" by `.pm/DO_NOT_DO.md` (Enterprise surfaces: SSO/SAML · SCIM). Production's walk needs a pin carrying m464 and this milestone. Its Google half also needs production's Google OAuth client, which is w7/m465's user gate.
- **Out of scope:** SAML SSO, SCIM and domain capture (`DO_NOT_DO.md`). The org invite token (w7/083). Enterprise seat billing (w7/082).
- **Render parity included:** the change touches GraphQL, the refusals on REST, GraphQL and MCP, the SSH gateway and the dashboard.
