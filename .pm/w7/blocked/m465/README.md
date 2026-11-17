# w7 · m465 — A workspace can require its members to sign in with Google, as a Render workspace can

**Worker:** worker7 **Goal:** members can sign in to bex with Google, and a workspace admin on Pro or higher can require it. While the requirement is on, a member who did not sign in with Google, and an API key not created in a Google session, gets the same coded 403 the 2FA requirement gives on REST, GraphQL, MCP and the SSH gateway. A member of an enforcing workspace cannot change their account email or disconnect Google, and the change is audited as Render's `ChangeTeamAllowedLoginMethodsEvent`. **Status:** blocked (t001–t007 and t009–t011 done; t008 waits on production's Google OAuth client and a pin carrying the milestone, then t012 closes it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's Google-login enforcement and record the design in ADR012, ADR018, ADR027 and ADR030](done/t001.md) — **DONE** | 1h | — |
| t002 | [Sign in with Google: a second Kratos OIDC provider beside GitHub](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Each caller carries how it signed in: Kratos sessions, OAuth tokens, and the API and SSH keys a Google session created](done/t003.md) — **DONE** | 1h30m | t001 |
| t004 | [The workspace setting: store, plan gate, GraphQL, the lock-out guard and `ChangeTeamAllowedLoginMethodsEvent`](done/t004.md) — **DONE** | 1h30m | t001 |
| t005 | [The gate: a caller not signed in with Google gets the coded 403 on REST, GraphQL, MCP and the SSH gateway](done/t005.md) — **DONE** | 1h30m | t003, t004 |
| t006 | [Kratos refuses an email change and a Google unlink for a member of an enforcing workspace](done/t006.md) — **DONE** | 1h30m | t002, t004 |
| t007 | [Dashboard: the Workspace Settings switch, the blocked-member page and the member status](done/t007.md) — **DONE** | 1h30m | t004, t005, t006 |
| t008 | [Production: Google sign-in and the requirement on a Pro fixture workspace](t008.md) | 1h | t007 |
| t009 | [Render parity](done/t009.md) — **DONE** | 45m | t007 |
| t010 | [Simplify](done/t010.md) — **DONE** | 30m | t009 |
| t011 | [Test coverage](done/t011.md) — **DONE** | 1h | t009 |
| t012 | [Closeout](t012.md) | 10m | t008, t010, t011 |

## Definition of done

- bex offers "Sign in with Google" on the login, sign-up and account settings pages wherever the installation sets a Google OAuth client, as it offers GitHub today. On production, a person signs up and signs in with a Google account.
- A workspace admin on Pro or higher turns "Require Google login" on and off in Workspace Settings and on GraphQL, where the 2FA setting lives. Like that one, the setting has no REST or MCP mutation, because Render has no REST workspace-mutation API. Below Pro, turning it on answers 400 `PLAN_FEATURE_REQUIRED`, and an admin who is not signed in with Google cannot lock themselves out (409).
- While it is on, a member whose current sign-in was not Google gets the coded 403 on every REST, GraphQL and MCP route of the workspace and on every SSH gateway path (native SSH, web shell, agent attach, sandbox exec and files), with the same exempt reads as the 2FA requirement. An API key that was not created in a Google session is refused too, apart from the exemption t001 records for keys that predate the setting.
- A member of an enforcing workspace cannot change their account email or unlink Google, through the dashboard or Kratos's own settings API. The refusal names the reason.
- Turning the setting on or off writes `ChangeTeamAllowedLoginMethodsEvent`, readable on `GET /v1/owners/{ownerId}/audit-logs` and GraphQL. `render_inventory_test.go` no longer lists the event as a non-goal.
- ADR012, ADR018 (the Workspace-enforced 2FA row's sibling), ADR027 and ADR030 §9 state the shipped rule.
- On production, t008's walk passes on a Pro fixture workspace.

## State (2026-11-17)

Every item but the production ones holds in code, tests and on dev-7 (t001–t007, t009–t011). Open: Google sign-up and sign-in on production, and t008's walk there, need production's Google OAuth client, which only someone with access to bex's Google Cloud project can create ([t008](t008.md#gate-2026-11-17)), and a production pin carrying the milestone. Follow-ups: [w7/084](../../084.md) (GitLab and Bitbucket sign-in), [w7/085](../../085.md) (a GitHub unlink is refused too while Google is linked).

## Source + Goal linkage

- **Source:** promoted from [w7/078](../../done/078.md) on 2026-11-17. w7/070 found it while gating the 2FA half of Render's "Enforce 2FA & Google login" row to Pro.
- **Render's behavior** ([login-settings](https://render.com/docs/login-settings), re-read 2026-11-17):
  - Only workspace admins configure it, on the Workspace Settings page. A workspace can require any combination of 2FA and Google login.
  - Members "can't access the workspace's resources or settings" if they log in using any method besides their Google account. The check is on how the member signed in, not on whether a Google account is linked.
  - Members must sign in with the Google account that matches the invited email address.
  - "API keys must be created while signed in via Google account". Keys created before May 1, 2024, when Render shipped the setting, are exempt.
  - Members can't change their account's email address, and can't disconnect Google while they belong to an enforcing workspace. They must leave those workspaces first.
  - The page does not say whether SSH access is affected. For 2FA it says SSH and API keys are blocked.
  - The pricing table's "Enforce 2FA & Google login" row is `— | ✓ | ✓ | ✓` (`docs/render-artifacts/pricing.md`), so Pro and higher. The pinned OpenAPI's audit-log enum carries `ChangeTeamAllowedLoginMethodsEvent`.
  - Render's sign-in providers are Google, GitHub, GitLab and Bitbucket, plus email and password.
- **bex today:**
  - There is no Google sign-in. `scripts/auth-secrets.sh` `oidc_fragment` emits one provider, `id: github`, and `.env.example` names only `BEX_GITHUB_OIDC_*`. ADR027 §1 says Google would be "the same shape", but nothing configures it.
  - The 2FA half: `tenants.require_two_factor` (migration 0261), `core.TwoFactorGate` on `core.Base` (`lego/backend/internal/core/twofactor.go`), its policy in `lego/backend/internal/twofactor/gate.go`, the SSH gateway verb (`internal/agentsession/two_factor.go`, `internal/sshgateway/twofactor.go`), `setWorkspaceRequireTwoFactor` (`internal/workspaces/graphql.go:308`), `core.FeatureTwoFactorEnforcement` (`internal/core/plan_gate.go`), the audit verb mapped to `ChangeTeam2FAEnforcementEvent` (`internal/audit/service.go:219`) and the dashboard's `workspace-two-factor-card.tsx`.
  - `core.Identity` does not record how a caller signed in: no Kratos `authentication_methods` and no Hydra `amr` reach it.
  - `lego/backend/internal/audit/render_inventory_test.go:44` lists `ChangeTeamAllowedLoginMethodsEvent` as `nonGoal: "bex has no allowed-login-methods setting"`.
- **Premise corrections:** the note said "Google sign-in exists through Kratos (ADR027)"; it does not, so t002 adds it. The note also proposed asking whether the member's identity has a Google OIDC credential. Render checks how the member signed in and when an API key was created, so t003 carries the sign-in method instead.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)), auth ([ADR012 §9a](../../../../docs/ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402)), social login ([ADR027](../../../../docs/ADR027-sso.md)), the plan table ([ADR030 §9](../../../../docs/ADR030-pricing.md)).
- **Expected outcome:** a team on Pro or higher can make Google its members' only way in, as on Render. The last open half of the "Enforce 2FA & Google login" row gets a checked disposition.
- **Why now:** w7/070 shipped the 2FA half and its plan gate, so the shared hook, caches and SSH verb are fresh. ADR018 and ADR030 both name this as the one unbuilt half of the row.
- **No user decision gates the code.** Under Render parity first, building the setting is the parity choice. The one external step is production's Google OAuth client: someone with access to bex's Google Cloud project creates it and sets `BEX_GOOGLE_OIDC_CLIENT_ID` and `BEX_GOOGLE_OIDC_CLIENT_SECRET`. t002 ships the code with the provider off until those exist. If they are still missing at t008, that task is the gate.
- **Out of scope:** org-level allowed login methods (`ChangeOrgAllowedLoginMethodsEvent`) is w7/081. GitLab and Bitbucket sign-in can be filed as follow-ups by t009. SAML SSO stays a non-goal (`.pm/DO_NOT_DO.md`).
- **Render parity included:** the change touches the GraphQL setting, the refusals on REST, GraphQL and MCP, the SSH gateway and the dashboard.
