# Render: workspace and organization login settings (2FA and Google login enforcement)

Captured 2026-11-17 for `w7/m465` (workspaces) and `w7/m466` ([Organizations](#organizations-w7m466)) from Render's public documentation. No live Render workspace was used: a capture needs a Render account, which bex does not have yet (`w10/blocked/002`). bex's design built on these sentences is [ADR012 §9a](../ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402) (2FA) and [§9b](../ADR012-auth.md#9b-workspace-enforced-google-login-w7m465) (Google login).

## Sources

| Source | Read | What it settles |
| --- | --- | --- |
| [render.com/docs/login-settings](https://render.com/docs/login-settings) | 2026-11-17 | Who configures it, what a blocked member loses, API keys, email change, disconnecting Google |
| [render.com/pricing](https://render.com/pricing) ([pricing.md](pricing.md), row "Enforce 2FA & Google login") | 2026-11-16 | `— \| ✓ \| ✓ \| ✓`: Pro and higher |
| The pinned OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`, `auditLog.event` enum) | 2026-11-17 | `ChangeTeam2FAEnforcementEvent` and `ChangeTeamAllowedLoginMethodsEvent` |

## Render's sentences

Enforcing secure login:

- "Your workspace can require its members to use any combination of the following login practices:" two-factor authentication, Google account login, and SAML SSO ("requires a Scale or Enterprise plan").
- "Only workspace admins can configure these enforcements."

Two-factor authentication:

- "Enforce two-factor authentication (2FA) from your Workspace Settings page".
- "If you enforce 2FA, your team members can't access the workspace's resources or settings until they enable 2FA for their Render account."
- "Team members with SSH or API keys can't use these keys to access workspace resources until they enable 2FA."

Google account login:

- "Enforce Google-account-based login from your Workspace Settings page".
- "Team members can't access the workspace's resources or settings if they log in using any method besides their Google account (such as username and password)."
- "Team members can't change their Render account's associated email address."
- "Team members must sign in with the Google account that matches the invited email address." The page's example: an invitation for `person@example.com` "must be accepted by signing in with that specific Google account, not a personal Gmail address."
- "API keys must be created while signed in via Google account to access resources of a workspace that enables this feature." "API keys created before May 1, 2024 are not subject to this restriction."
- "You can't disconnect your Google account if you belong to a workspace that enforces Google-account-based login." "First, leave any such workspaces."

## What the page does not say

- **SSH keys.** For 2FA the page names SSH keys; for Google login it names only API keys. bex applies the API-key rule to SSH keys (an inference, ADR012 §9b).
- **The plan.** The page names no plan for 2FA or Google login enforcement. The pricing table's one row, "Enforce 2FA & Google login", is Pro and higher.
- **May 1, 2024.** The page does not say why that date. It is when Render shipped the setting, so the exemption covers keys older than the feature itself.
- **Error codes and wording.** Render documents no API error for a member refused by either setting. bex's codes are its own vocabulary.
- **The audit event's metadata.** The OpenAPI names `ChangeTeamAllowedLoginMethodsEvent` but no metadata keys for it.

## Organizations (w7/m466)

Captured 2026-11-17 for `w7/m466`, again from the public documentation only (no Render account, `w10/blocked/002`). bex's design is [ADR012 §9c](../ADR012-auth.md#9c-organization-enforced-two-factor-authentication-and-google-login-w7m466) and [ADR115 D10](../ADR115-organizations.md#d10--org-login-settings).

| Source | Read | What it settles |
| --- | --- | --- |
| [render.com/docs/audit-logs](https://render.com/docs/audit-logs) | 2026-11-17 | The two org events, that they are org-log events, and the org log's plan |
| [render.com/docs/saml-sso](https://render.com/docs/saml-sso) | 2026-11-17 | Where the org setting lives, who changes it, what enforcing it does to members and API keys |
| [render.com/docs/organizations](https://render.com/docs/organizations) | 2026-11-17 | An org is Scale or Enterprise; Owners manage org settings |
| [render.com/docs/login-settings](https://render.com/docs/login-settings) | 2026-11-17 | The workspace settings only (above) |

Render's sentences:

- audit-logs: "`ChangeOrgAllowedLoginMethodsEvent`: The org's set of allowed login methods was changed." "`ChangeOrg2FAEnforcementEvent`: Two-factor authentication enforcement was enabled or disabled for the org." These events appear "_only_ in audit logs exported from your org's **Settings** page", and org audit logs are "available on the Scale plan or higher".
- saml-sso: the org setting is under "**Settings** in the left pane", at "**Require Login Method** under **Security**", where you "select SAML SSO as a required login method". Only an Org Owner can do it. When SSO is enforced, "Each listed member will lose access to the org until they log in via SSO" and "Each listed API key will be invalidated until its owner logs in via SSO".

What the pages do not say (each is a bex **inference**, recorded in ADR012 §9c):

- **The org's Google choice.** No page lists the org's "allowed login methods" beyond SAML SSO. bex reads the org's set as the org form of the workspace settings: Google login (what `ChangeTeamAllowedLoginMethodsEvent` records on a workspace), with SAML SSO as one more choice bex does not build (`.pm/DO_NOT_DO.md`).
- **Where org 2FA lives.** No page shows the org 2FA control. bex puts it beside "Require Login Method" in the org Settings page's Security section.
- **Scope.** No page says whether org enforcement binds every workspace. SSO enforcement binds "access to the org", so bex makes org 2FA and Google login bind every org workspace, beside each workspace's own setting, and the org's own pages.
- **The confirmation's list.** SSO enforcement lists the members and keys that lose access. No page says the 2FA or Google confirmation does; bex lists them for both requirements, on a workspace and on an org (w7/m471), from the enforcement gate's own checks, with SSH keys as bex's addition because its gate blocks them too.
- **Plan.** An org is always Scale or Enterprise, above the pricing table's Pro floor for "Enforce 2FA & Google login", so the org setting adds no plan row.
- **Error codes, metadata.** Render documents no API refusal and no metadata keys for the two events. bex reuses its workspace codes and `metadata.to`.

## Account providers (w7/m468)

Read 2026-11-17 from [render.com/docs/login-settings](https://render.com/docs/login-settings) for `w7/m468` (sign in with GitLab and Bitbucket). bex's design is [ADR012 §10](../ADR012-auth.md#10-social-login-sign-in-with-github-google-gitlab-and-bitbucket-kratos-oidc-w4003-w7m465-w7m468).

- The login methods, in this order: "Any of the following account providers:" Google, GitHub, GitLab, Bitbucket; then "Email and password"; then "SAML SSO (requires a Scale or Enterprise plan)".
- In Account Settings, the Account Security section lets you "Add or remove connected login methods."
- "Multiple Render accounts can't use the same provider account to log in."
- Its login and sign-up pages (dashboard.render.com/login and /register, read again 2026-11-18 for `w7/091`) lead with the form: Email, Password, "Forgot password?", "Sign in" (or "Create account"), then a divider reading "Or continue with", then GitHub · Google (· SSO on login) and "More options".
- "You can use a Git provider for deployment without using it for login (or vice versa)." "If you connect a Git provider for both login and deployment, you must use the same account for both."

What the page does not say:

- **Which GitLab and Bitbucket.** It names the providers, not self-managed GitLab or Bitbucket Data Center. bex offers gitlab.com and Bitbucket Cloud.
- **The address a provider sign-up gets.** bex takes the provider's verified email: GitLab's `email_verified`, and Bitbucket's primary confirmed address (the only one Dex's connector accepts).
- **The same account for login and deployment.** bex's Git connections are GitHub only (`.pm/DO_NOT_DO.md` rejects GitLab and Bitbucket Git connections), so the rule can only concern GitHub, which `w7/m468` does not change.
