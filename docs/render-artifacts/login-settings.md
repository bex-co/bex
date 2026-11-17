# Render: workspace login settings (2FA and Google login enforcement)

Captured 2026-11-17 for `w7/m465` from Render's public documentation. No live Render workspace was used: a capture needs a Render account, which bex does not have yet (`w10/blocked/002`). bex's design built on these sentences is [ADR012 §9a](../ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402) (2FA) and [§9b](../ADR012-auth.md#9b-workspace-enforced-google-login-w7m465) (Google login).

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
