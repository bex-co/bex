# w4 · m17 — Login-flow correctness: signed-in OAuth authorize + aal2 step-up altitude

**Worker:** worker4 **Goal:** A user already signed into the dashboard can connect an agent end-to-end — the OAuth login redirect no longer dead-ends on Kratos's `200 null` flow response — and the login hook learns the aal2 step-up need from the session fetch instead of rediscovering it by trial-and-error. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce `010` in a real browser: sign into the dashboard, connect an agent, observe the login page against the local e2e stack                     | 25m | —          |
| t002 | Fix: handle a `200 null` flow carrying a `login_challenge` in `use-ory-flow.ts` — navigate the browser to Kratos's HTML endpoint (303 → Hydra continue) | 30m | t001       |
| t003 | aal2 altitude lift (`009`): surface `session_aal2_required` from `fetchSession`, propagate through `requireAuth`, redirect with explicit `aal=aal2`   | 45m | t002       |
| t004 | e2e: add a reused-cookie-jar leg to `scripts/auth-oauth21-e2e.sh` — a signed-in authorization completes without re-login                             | 25m | t003       |
| t005 | Render parity — superset check: no Render shape (Render has no OAuth provider); ADR012 §7 / ADR025 accuracy + ADR018 "bex ahead" evidence update      | 15m | t004       |
| t006 | Simplify — run `/simplify` over the code this milestone changed                                                                                      | 20m | t005       |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped                                                                             | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                                                       | 10m | t007       |

## Definition of done

With a live Kratos session in the browser, connecting an agent (the `docs/ADR025-connect-an-agent.md` recipe) completes the full authorize → consent → token flow without dead-ending on the login page; `scripts/auth-oauth21-e2e.sh` exits 0 including a new leg that drives a second authorization from a reused cookie jar (signed-in browser); the login hook no longer probes for the aal2 step-up by minting a trial flow — `fetchSession` surfaces `session_aal2_required`, `requireAuth` redirects with an explicit `aal=aal2` param, and the hook renders the step-up flow directly; the six m11 fixture-driven `use-ory-flow` tests are updated alongside and the dashboard suite is green.

**Contingency (t001 gates t002):** if a real browser does *not* reproduce the dead-end, record the findings in this README, keep t002 as a robustness fix only if the `200 null` response is still observed (it was, against Kratos v1.3.1 + Hydra v2.2.0 in the e2e harness), and re-scope with the user before proceeding.

## Source + Goal linkage

- **Source:** promotion of inbox `w4/010` (found in m16 t004 while writing `scripts/auth-oauth21-e2e.sh`, 2026-07-12) + folded inbox `w4/009` (the m11 `/simplify` altitude finding) — both live in `use-ory-flow.ts` + the session plumbing and share the same fixture test suite; `/pm-brainstorm for w4` 2026-07-12.
- **Goal linkage:** pillar 4 (AI-native, agents as first-class clients) — the OAuth 2.1 connect-an-agent recipe (`docs/ADR025-connect-an-agent.md`) is the agent front door; this is the last known break in it.
- **Expected outcome:** the most common connect-an-agent path — a user already signed into the dashboard — works end-to-end, proven by an e2e leg that reuses a signed-in cookie jar.
- **Why now:** m16 just completed the consent half of the flow; its e2e passes only because every leg uses a fresh cookie jar, so the signed-in path is silently broken for exactly the users most likely to try it. `009` rides along because splitting it would churn the same hook and tests twice.
- **Render parity:** included as a **superset** check (t005), following m16's precedent — Render has no OAuth provider at all, so the comparison target is ADR018's "bex ahead of Render" section, ADR012 §7's documented short-circuit behavior (currently describes `browser_location_change_required`, which live Kratos v1.3.1 does not throw here), and ADR025's troubleshooting accuracy — not a Render shape. The login surface itself is Kratos-owned (ADR018 login row `—/—/—/✅`), no REST/GraphQL/MCP change.
