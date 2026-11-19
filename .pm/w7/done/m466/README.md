# w7 · m466 — An organization can require two-factor authentication and Google login in every one of its workspaces, as a Render org can

**Worker:** worker7 **Goal:** an Org Owner turns on "Require two-factor authentication" and "Require Google login" on the org Settings page and on GraphQL. While a requirement is on, every workspace in the org enforces it beside the workspace's own setting, and so do the org's own pages. A member, an API key or an SSH key that does not meet it gets the coded 403 the workspace settings already give, on REST, GraphQL, MCP and the SSH gateway. Each change is recorded in the org audit log as Render's `ChangeOrg2FAEnforcementEvent` or `ChangeOrgAllowedLoginMethodsEvent`. **Status:** done (2026-11-18; the production Google walk stays the open check until production has Google sign-in, w7/m465 t008's user gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's org login settings and record the design in ADR012, ADR115, ADR018 and ADR030](done/t001.md) — **DONE** | 45m | — |
| t002 | [Store: the org's two settings, read by the gate for every org workspace](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [The org setting: Owner-only GraphQL mutations, plan gate, lock-out guards and the two org audit events](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [The gate: every org workspace and the org's own pages refuse a caller who does not meet the org's requirement](done/t004.md) — **DONE** | 1h30m | t002 |
| t005 | [Dashboard: the org Settings Security card, the locked workspace switches and the blocked page naming the org](done/t005.md) — **DONE** | 1h30m | t003, t004 |
| t006 | [Production: org 2FA on a Scale fixture org](done/t006.md) — **DONE** | 1h | t005 |
| t007 | [Render parity](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Simplify](done/t008.md) — **DONE** | 30m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 1h | t007 |
| t010 | [Closeout](done/t010.md) — **DONE** | 10m | t006, t008, t009 |

## Definition of done

- An Org Owner turns "Require two-factor authentication" and "Require Google login" on and off on the org Settings page (a **Security** section, where Render puts "Require Login Method") and on GraphQL. Members and Guests see each setting's state but cannot change it. The surface rule follows the workspace settings: t001 records whether the org settings also get an MCP mutation.
- While org 2FA is on, a person without a second factor is refused with `403 WORKSPACE_2FA_REQUIRED` in **every** workspace of the org, whatever that workspace's own setting is. The refusal covers every REST, GraphQL and MCP route and every SSH gateway path, keeps the exempt reads the workspace setting has, and covers their API and SSH keys. The same holds for org Google login with `403 WORKSPACE_GOOGLE_LOGIN_REQUIRED`. The org's own pages (Members, Workspaces, Settings, the audit log) refuse such a person the same way.
- A workspace in an enforcing org shows its own switch as on and locked, "Required by <org>". Turning the org setting off leaves each workspace's own setting as it was.
- An Owner who does not meet a requirement cannot turn it on (409, the lock-out guard). A member of an org that requires Google login cannot change their email or unlink Google, and the Kratos settings guard names the org.
- Each change writes one org audit row. 2FA writes `ChangeOrg2FAEnforcementEvent` and Google login writes `ChangeOrgAllowedLoginMethodsEvent`, readable on `GET /v1/organizations/{orgId}/audit-logs` and GraphQL `organizationAuditLogs`. A no-op writes nothing. `renderOrganizationEvents` in `lego/backend/internal/audit/render_inventory_test.go` maps both events instead of listing them as `notYet`.
- ADR012 (a §9c beside §9a and §9b), ADR115 (D7 and a new decision), ADR018 (the Workspace-enforced 2FA row and its Google sibling) and `docs/render-artifacts/login-settings.md` state the shipped rule. Each Render inference is marked as one.
- On production, t006's walk passes on a Scale fixture org. The Google half is walked on production once production has Google sign-in (w7/m465 t008's gate); until then it is walked on dev-7, and that check is recorded as the open one.

## Source + Goal linkage

- **Source:** promoted from [w7/081](../081.md) on 2026-11-17. w7/m464 t014 found it, and ADR115 D7 lists both events as follow-ups.
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
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)), organizations ([ADR115](../../../../docs/ADR115-organizations.md)), workspace login settings ([ADR012 §9a](../../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402), [§9b](../../../../docs/ADR012-auth.md#9b-workspace-enforced-google-login-w7m465)).
- **Expected outcome:** an org on Scale or Enterprise sets its members' login rules once for all of its workspaces, as on Render, instead of setting them workspace by workspace. The org log's two remaining non-IdP events get recorded.
- **Why now:** m464 shipped organizations and m465 shipped the Google-login gate on 2026-11-17. The shared gate, its caches, the SSH verb and the org log are fresh, and these two events are the only org events left that are not IdP ones.
- **No user decision gates the code.** Under Render parity first, building both settings is the parity choice. SAML SSO stays out of "Require Login Method" by `.pm/DO_NOT_DO.md` (Enterprise surfaces: SSO/SAML · SCIM). Production's walk needs a pin carrying m464 and this milestone. Its Google half also needs production's Google OAuth client, which is w7/m465's user gate.
- **Out of scope:** SAML SSO, SCIM and domain capture (`DO_NOT_DO.md`). The org invite token (w7/083). Enterprise seat billing (w7/082).
- **Render parity included:** the change touches GraphQL, the refusals on REST, GraphQL and MCP, the SSH gateway and the dashboard.

## Park record (2026-11-17)

- **Done and shipped:** t001–t005 and t007–t009. The org's two settings (migration 0295), the Owner-only GraphQL mutations with the plan check and lock-out 409s, the two org audit events, the gate holding every org workspace and the org's own objects to the org's setting (refusals naming the org on REST, GraphQL, MCP and the SSH verb), the settings guard naming the org, the dashboard's Security card, locked workspace switches and blocked states, and ADR012 §9c, ADR115 D10, ADR018's new row and the Render capture. Follow-up filed: [w7/086](../086.md).
- **Gate (clears by itself, no user action):** t006's walk needs a production pin that carries this milestone (and m464). Check `git merge-base --is-ancestor "$(git log -1 --format=%H --grep='w7/m466' origin/main)" <the sha in the newest "pin platform images to" commit on main>`. Then run t006 on a Scale fixture org: the 2FA half on production; the Google half on dev-7 (`bash scripts/dev-env.sh 7 up`) until production has Google sign-in (w7/m465 t008's user gate), recorded as the open check; include the org Settings page's pending-vs-ready look t005 owed (no Playwright server in this session). Close with t010.

## Unblock record (2026-11-17)

- **Affected:** t006 and t010.
- **Source:** verified evidence, no user decision. The newest pin on origin/main, `359aee37d` (`chore(deploy): pin platform images to 14eaa71b98e5`), pins this milestone's own commit: `git merge-base --is-ancestor 14eaa71b9 14eaa71b98e5` exits 0. w7/m464 t013's walk on the same pin read bex-db at schema 295, which is this milestone's migration.
- **Behavior and scope:** unchanged. The park record's gate is cleared. The production Google half still needs production's Google OAuth client (w7/m465 t008's user gate), so it is walked on dev-7 and recorded as the open check, as the definition of done allows.
- **Next steps:** run t006 (the 2FA half and the org Settings pending-vs-ready look on production, the Google half on dev-7), then close with t010 if the definition of done holds.

## Park record (2026-11-17, second)

- **t006 walked on production** at pin `14eaa71b98e5` ([t006](done/t006.md)): the lock-out, the Owner-only toggle and the no-op, the Member refused naming the org in both workspaces on REST, GraphQL, MCP, the API key, the shell ticket and native SSH, the blocked Owner on the org pages, the dashboard's blocked page and locked switch, the org log on REST, GraphQL and CSV, and the way back with each workspace's own setting unchanged. The Google half was walked on dev-7, and the production Google walk is the open check the definition of done allows.
- **Defect found and fixed here:** the 2FA-exempt workspaces list handed a Member the org blocks the inbound IP rules (and, on GraphQL, the overlapping deploy policy) of an org workspace whose own setting is off. `WorkspaceView.requiresLogin` now asks the gate when the org may require a login. Regression tests over real Postgres and the real gate; ADR012 §9c records it. Follow-up filed: [w7/090](../../090.md) (the SSH banner does not name the org).
- **Gate (clears by itself, no user action):** a production pin carrying the fix. Check `git merge-base --is-ancestor "$(git log -1 --format=%H --grep='w7/m466' origin/main)" <the sha in the newest "pin platform images to" commit on main>`. Then repeat the steps in [t006](done/t006.md)'s remaining gate (the exempt list under org 2FA, and the Owner's org Settings pending frame at 390, where w4/706's fix ships in the same pin) and close with t010.

## Unblock record (2026-11-18)

- **Affected:** t006 and t010.
- **Source:** verified evidence, no user decision. The newest pin on origin/main, `826d32e42` (`chore(deploy): pin platform images to 7af50a28168d`, 2026-11-19T03:48Z), carries `a7817736f`, the newest commit naming w7/m466 and the fix t006 found: `git merge-base --is-ancestor a7817736f 7af50a28168d` exits 0. w4/706's fix is in the same pin (`git merge-base --is-ancestor ef5ecb56f 7af50a28168d` exits 0). Both bex-api replicas run `bex-operator@sha256:7ce6ae1e…` and the dashboard runs `bex-dashboard@sha256:56f8b1d4…`, the pin's digests.
- **Behavior and scope:** unchanged. The second park record's gate is cleared. The production Google walk still needs production's Google OAuth client (w7/m465 t008's user gate) and stays the open check the definition of done allows.
- **Next steps:** repeat [t006](done/t006.md)'s remaining gate on production (the exempt list under org 2FA, and the Owner's org Settings pending frame at 390), delete the fixtures, then close with t010 if the definition of done holds.

## Closeout (2026-11-18)

- **t006's re-check passed on production** at pin `7af50a28168d` ([t006](done/t006.md)). With org 2FA on and the Public secondary's own setting off, a Member without a second factor got the secondary's `ipAllowList` as `null` on session and key REST `/v1/owners` and MCP `list_workspaces`, and `WORKSPACE_2FA_REQUIRED` naming the org on GraphQL `ipAllowList` and `overlappingDeployPolicy`, on every probe across both replicas. With it off, the rules came back from the first probe. The Owner's org Settings pending frame at 390 matched the ready one (w4/706 holds live). The fixtures are deleted.
- **Open check:** the production Google walk, until production has Google sign-in (w7/m465 t008's user gate). The Google half passed on dev-7 on 2026-11-17.
