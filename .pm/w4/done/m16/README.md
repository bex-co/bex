# w4 · m16 — OAuth consent screen for third-party agent clients

**Worker:** worker4 **Goal:** A freshly self-registered (DCR) OAuth client completes authorization end-to-end with **no operator action** — the logged-in user sees a consent page naming the client and its requested scopes, approves or denies it themselves; the headless auto-accept path for trusted/skippable clients stays byte-identical. **Status:** done — all 9 tasks complete; e2e green (`scripts/auth-oauth21-e2e.sh`, exit 0, 12/12 legs)

## Tasks (in order)

| id   | title                                                                                                                                                          | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Consent route: unknown client + live session ⇒ render a consent page (client name, requested scopes/audience) instead of the flat 403; trusted path unchanged — **DONE** | 40m | —          |
| t002 | Approve/deny server actions → Hydra admin accept (granted scopes, `remember: true`) / reject — **DONE** | 35m | t001       |
| t003 | Hardening: challenge-bound CSRF, subject↔session binding, no open redirect, deny-by-default on any failure, unauthenticated ⇒ login-first then back to consent — **DONE** | 30m | t002       |
| t004 | E2E: extend `scripts/auth-oauth21-e2e.sh` — DCR client → login → consent approve → token works at `/mcp`; deny path asserts rejection — **DONE** | 30m | t002       |
| t005 | Docs: ADR012 §7 + rewrite ADR025's "consent denied" troubleshooting (operator blessing becomes optional, not required) — **DONE** | 20m | t004       |
| t006 | Render parity — superset check: update ADR018's "bex ahead of Render" section (Render has no OAuth provider); cross-surface consistency of the consent policy — **DONE** | 15m | t003, t005 |
| t007 | Simplify — run `/simplify` over the code this milestone changed — **DONE** | 20m | t006       |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE** | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/` — **DONE** | 10m | t008       |

## Definition of done

A freshly DCR-registered OAuth client completes authorization end-to-end with no operator action: the logged-in user sees a consent screen naming the client and requested scopes, approves once, gets a working token at `/mcp`, and a second authorization within the remember window skips consent; the deny path is honored (Hydra reject, agent sees an OAuth error, no token); the trusted-client headless path (Hydra `skip`/`skip_consent`/`OAUTH_TRUSTED_CLIENTS`) behaves byte-identically to today, including the PKCE-required check; the flow is scripted (extension of `scripts/auth-oauth21-e2e.sh`, exit 0 = pass).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w4` 2026-07-12 (Proposal 1); ADR012 §7 ("a real consent UI is future work"); ADR025 § Troubleshooting ("consent denied … the platform operator needs to bless the client once"); follow-on to `w4/m9`.
- **Goal linkage:** pillar 4 (AI-native — agents hold their own revocable, user-consented credentials); removes the last manual operator step in docs/ADR025-connect-an-agent.md's connect recipe.
- **Expected outcome:** "connect Claude Code to bex" works self-serve in one browser round-trip — no Hydra admin `PATCH`, no `OAUTH_TRUSTED_CLIENTS` edit.
- **Why now:** w4 is otherwise drained (only m11's manual WebAuthn check remains) and this is the sole remaining IAM gap in the agent-connect path; shipping pre-tenants settles the consent policy before anyone depends on the operator-blessing workaround. **Anti-goal check (resolved at brainstorm):** `DO_NOT_DO.md`'s "at most a headless consent acceptor is needed" targets re-implementing the *login* provider Kratos ships natively; a consent UI is the one piece Ory deliberately leaves to the app (Hydra has no consent screen), and this extends the existing blessed `/auth/consent` route rather than adding a provider app. User accepted the milestone with that reading 2026-07-12.
- **Render parity:** included as a **superset** check (t006) — Render has no OAuth provider at all, so the comparison target is ADR018's "bex ahead of Render" section and bex's own cross-surface consistency, not a Render shape.
