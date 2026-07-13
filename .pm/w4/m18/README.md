# w4 · m18 — Account access control: connected agents + active sessions

**Worker:** worker4 **Goal:** A signed-in user can see and revoke everything that can currently act as them — the OAuth clients they've authorized (Hydra consent sessions; revoking kills the client's tokens) and their active browser sessions (Kratos; sign-out-everywhere) — as two sibling cards under Settings → Security & Compliance. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                       | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Server-fns beside `hydra-consent.ts`: list + revoke a subject's Hydra consent sessions (admin API, server-only env)                          | 30m | —            |
| t002 | Settings → Security & Compliance: **Connected agents** card — client name, scopes, grant date, revoke with confirm                          | 35m | t001         |
| t003 | **Active sessions** card (`006`): Kratos `/sessions` list + "sign out other sessions"                                                        | 40m | —            |
| t004 | e2e: extend `auth-oauth21-e2e.sh` — authorize → list shows the client → revoke → the agent's token stops working                             | 30m | t002, t003   |
| t005 | Render parity — superset check: no Render shape (no OAuth provider; session mgmt Kratos-owned); ADR018 "bex ahead" revocation evidence       | 15m | t004         |
| t006 | Simplify — run `/simplify` over the code this milestone changed                                                                              | 20m | t005         |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped                                                                     | 30m | t005         |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                                               | 10m | t007         |

## Definition of done

A signed-in user sees, under Settings → Security & Compliance, every OAuth client they've authorized (name, scopes, grant date) and every active Kratos session; revoking a client kills its tokens — proven by an `auth-oauth21-e2e.sh` leg (exit 0) that authorizes a client, sees it listed, revokes it, and asserts the client's token is rejected (introspection inactive / 401 at `/mcp`); "sign out other sessions" terminates the user's other Kratos sessions while keeping the current one; no bex-api surface is added (dashboard SSR only, by design) and no OAuth token ever reaches browser-readable storage.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4` 2026-07-12 — direct follow-on to `w4/m16` (remembered consent shipped with no revocation surface); folds inbox `w4/006` (account session management, filed 2026-07-09 to ride the next Settings→Security milestone — m11, its original ride-along, is closing).
- **Goal linkage:** roadmap #5/#7 account security, plus the pillar-4 agent story's revocability half — "revocable, scoped token" is the advertised property of the agent-token design (ADR018 § bex ahead of Render); until a revocation surface exists that claim is only half true.
- **Expected outcome:** an authorized agent is no longer a permanent grant — one user story ("what can act as me, and how do I cut it off") covering both OAuth grants and browser sessions.
- **Why now:** m16 just shipped remembered consent and the tokens it mints outlive the consent screen; Hydra's admin API already has the verbs (`GET`/`DELETE /admin/oauth2/auth/sessions/consent`) and the dashboard SSR side already holds `HYDRA_ADMIN_URL` (`hydra-consent.ts`) — this is assembly, not new machinery. The m15 Security & Compliance grouping was built for exactly these cards.
- **Render parity:** included as a **superset** check (t005), following m16's precedent — Render has no OAuth provider, and its session surface is not a dashboard feature bex mirrors (`w4/006` already made this call: Kratos-owned, like login/MFA). The comparison target is ADR018's "bex ahead" section + bex's own cross-surface consistency (deliberately **no** REST/GraphQL/MCP surface — account-scoped, dashboard-SSR only).
- **Anti-goal check:** stays on the right side of both auth anti-goals — the dashboard keeps its Kratos session (HttpOnly cookie; no tokens in the browser), and nothing here re-implements a login/consent provider (t001 calls Hydra admin verbs, exactly the `hydra-consent.ts` pattern).
