# w4 · m8 — API keys in the dashboard (settings surface)

**Worker:** worker4 **Goal:** the m3 API-key lifecycle becomes self-service for humans: a logged-in dashboard user (Kratos session) mints, lists, and revokes keys from Settings — secret shown exactly once — through the existing GraphQL adapter, so handing an agent a credential no longer requires `curl`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Session-caller path: confirm/extend GraphQL api-key ops for Kratos-session users (interplay with m4's checker)      | 30m | —          |
| t002 | Dashboard settings: API Keys section — list (created/last fields, never secrets) + revoke with confirmation          | 40m | t001       |
| t003 | Dashboard: mint flow — name input, secret shown exactly once with copy affordance, unretrievable after dismiss       | 35m | t002       |
| t004 | Dashboard tests per `dashboard/CLAUDE.md` conventions + docs cross-links                                             | 30m | t003       |
| t005 | Simplify — run `/simplify` over the code this milestone changed                                                      | 20m | t004       |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                             | 30m | t004       |

## Definition of done

A logged-in dashboard user (Kratos session) opens Settings → API Keys, mints a named key whose secret is displayed exactly once, sees it in the list without the secret, and revokes it — after which the key's token no longer authenticates against bex-api (verifiable with the existing `scripts/auth-e2e.sh` revoke check); all of it through the existing GraphQL adapter with no new REST surface; dashboard tests cover mint-once-visibility and revoke.

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (w4 sweep); gap = m3 shipped mint/list/revoke over REST/GraphQL/MCP with no human surface (`grep` of `dashboard/src` finds no API-key UI; the settings page is Kratos profile+password only).
- **Goal linkage:** pillar 4 (deploy-from-chat) — the human-hands-a-credential-to-an-agent loop is the on-ramp; pillar 1's "no dashboard-only features" cuts both ways: no API-only features humans need daily.
- **Expected outcome:** a tenant can provision and revoke agent credentials without `curl`; the m3 lifecycle becomes self-service.
- **Why now:** m3's verbs are fresh and GraphQL parity already exists, so this is pure surface work; doing it before m4's enforcement flips means the session-caller authorization path (t001) is designed against the checker rather than retrofitted.
