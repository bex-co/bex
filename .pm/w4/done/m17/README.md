# w4 · m17 — Login-flow correctness: signed-in OAuth authorize + aal2 step-up altitude

**Worker:** worker4 **Goal:** A user already signed into the dashboard can connect an agent end-to-end — the OAuth login redirect no longer dead-ends on Kratos's `200 null` flow response — and the login hook learns the aal2 step-up need from the session fetch instead of rediscovering it by trial-and-error. **Status:** done — all 8 tasks complete; e2e green (`scripts/auth-oauth21-e2e.sh`, exit 0, 13/13 legs) and the recipe driven end-to-end in a real browser

## Tasks (in order)

| id   | title                                                                                                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce `010` in a real browser: sign into the dashboard, connect an agent, observe the login page against the local e2e stack — **DONE** (see findings below)   | 25m | —          |
| t002 | Fix: handle a `200 null` flow carrying a `login_challenge` in `use-ory-flow.ts` — navigate the browser to Kratos's HTML endpoint (303 → Hydra continue) — **DONE** | 30m | t001       |
| t003 | aal2 altitude lift (`009`): surface `session_aal2_required` from `fetchSession`, propagate through `requireAuth`, redirect with explicit `aal=aal2` — **DONE**     | 45m | t002       |
| t004 | e2e: add a reused-cookie-jar leg to `scripts/auth-oauth21-e2e.sh` — a signed-in authorization completes without re-login — **DONE** (leg 13)                       | 25m | t003       |
| t005 | Render parity — superset check: no Render shape (Render has no OAuth provider); ADR012 §7 / ADR025 accuracy + ADR018 "bex ahead" evidence update — **DONE**        | 15m | t004       |
| t006 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                                                                                         | 20m | t005       |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE**                                                                                | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                                                                          | 10m | t007       |

## Definition of done

With a live Kratos session in the browser, connecting an agent (the `docs/ADR025-connect-an-agent.md` recipe) completes the full authorize → consent → token flow without dead-ending on the login page; `scripts/auth-oauth21-e2e.sh` exits 0 including a new leg that drives a second authorization from a reused cookie jar (signed-in browser); the login hook no longer probes for the aal2 step-up by minting a trial flow — `fetchSession` surfaces `session_aal2_required`, `requireAuth` redirects with an explicit `aal=aal2` param, and the hook renders the step-up flow directly; the six m11 fixture-driven `use-ory-flow` tests are updated alongside and the dashboard suite is green.

**Met.** Driven in a real browser (Playwright, against the e2e stack): signed in → agent authorize → login page short-circuits with no UI → consent → Approve → `code` at the agent's callback → token exchange → `GET /v1/services` with the Bearer = **200**. `scripts/auth-oauth21-e2e.sh` exits 0 across 13 legs (leg 13 is the reused-jar one). Dashboard suite green: 109 files / 692 tests, typecheck + eslint clean.

## t001 findings (the reproduction, and what it corrected)

Inbox `010` predicted the dead-end but got the trigger half-right. What Kratos v1.3.1 + Hydra v2.2.0 actually do, measured (both a curl harness and a real browser):

- **A signed-in browser sees one of _two_ shapes**, depending on whether **Hydra** — not Kratos — already has a login session for it:
  - **First authorization from that browser** (Hydra has no session ⇒ its login request is not skippable): Kratos returns a **re-authentication flow** — the password form, identifier prefilled, "confirm that it is you". Not a dead end; the user confirms once. This is what a real browser hits first, and it is why the bug hid: the obvious manual test passes.
  - **Every subsequent authorization** (Hydra now has a login session ⇒ `skip: true`): Kratos accepts the challenge against the session and answers the page's AJAX call with **HTTP 200 and a body of literally `null`** — no `ui.nodes`, no `Location`, no error. `@ory/client-fetch`'s `LoginFlowFromJSON(null)` returns `null`, so `createBrowserLoginFlow` _resolves_ with a null flow, `setFlow(null)`, and the login page renders its loading skeleton **forever**. Confirmed exactly as `010` described.
- The same URL fetched **without** `Accept: application/json` answers **303 → Hydra's `login_verifier` continue URL**. That is the fix (t002): hand the challenge back to Kratos as a browser navigation.
- ADR012 §7's claim that an existing session short-circuits with `browser_location_change_required` is **wrong for this path** — live Kratos does not raise it here. Corrected in t005. (The branch is kept in the hook: Ory Elements' own submits still provoke it, and older Kratos versions do name a destination here.)

**Bug found in the real browser, outside the m17 diff but blocking its DoD — the consent page never worked in a browser.** Its Approve/Deny buttons carry the decision (`name="decision"`), and `onSubmit` set `disabled` on them. The browser builds a form's entry list _after_ the submit handler returns and excludes disabled controls, so React's synchronous re-render dropped the submitter's own field: every real approve/deny POSTed a challenge and a CSRF token with **no decision**, and got `400 malformed consent decision`. m16's e2e can't see it — curl posts the three fields explicitly. Fixed (guard the double-submit with `preventDefault` + `pointer-events-none`, never by disabling the controls) and pinned by a test that asserts the submitted entry list, mutation-checked to fail when `disabled` returns.

## What shipped

- `use-ory-flow.ts` — `bootstrapViaKratos` (hand the flow back to Kratos as a browser navigation) covers both the `200 null` short-circuit and the unknown-failure escape hatch; the escape hatch now **keeps the `login_challenge`**, which it used to drop (silently abandoning the authorization). The aal2 probe is gone: the hook asks for the step-up flow first, because the caller told it to.
- `session.ts` — `fetchSession` returns `{ session, aal2Required }`, reading `session_aal2_required` off whoami's 403. Only this call can tell a step-up from a sign-in.
- `requireAuth` + the consent route's login-first bounce — both redirect to `/auth/login?aal=aal2` when a factor is owed. Two guards remain because not every Kratos answer is a challenge: an aal2 flow with no second-factor node (identity that has none) navigates on rather than render an empty card; `session_aal1_required` (stale step-up link) falls back to a first-factor login.
- `scripts/auth-oauth21-e2e.sh` — leg 13 reuses the signed-in jar and asserts the **path** (`signed-in-shortcircuit`), not just the outcome: it fails if the browser is made to re-authenticate.
- Docs: ADR012 §7 (both signed-in shapes, replacing the wrong `browser_location_change_required` claim) + § MFA (the lift), ADR025 troubleshooting (re-auth prompt; skeleton symptom), ADR018 "bex ahead" (leg-13 evidence).
- `/simplify` (t006): extracted the duplicated Ory error-envelope parser to `common/lib/ory/errors.ts`; `createFlow` takes a named `FlowAsk` bag so each retry says which param it drops; `AuthContext` narrowed from `RouterContext` instead of restated; `session_already_available` resolved once instead of parsed twice.

## Source + Goal linkage

- **Source:** promotion of inbox `w4/010` (found in m16 t004 while writing `scripts/auth-oauth21-e2e.sh`, 2026-07-12) + folded inbox `w4/009` (the m11 `/simplify` altitude finding) — both live in `use-ory-flow.ts` + the session plumbing and share the same fixture test suite; `/pm-brainstorm for w4` 2026-07-12.
- **Goal linkage:** pillar 4 (AI-native, agents as first-class clients) — the OAuth 2.1 connect-an-agent recipe (`docs/ADR025-connect-an-agent.md`) is the agent front door; this was the last known break in it.
- **Outcome:** the most common connect-an-agent path — a user already signed into the dashboard — works end-to-end, proven both by an e2e leg that reuses a signed-in cookie jar and by a real browser driven through the whole recipe.
- **Render parity:** superset check (t005) — Render has no OAuth provider at all, so there is no Render shape to mirror; the targets were ADR018's "bex ahead of Render" evidence and ADR012/ADR025 accuracy. The login surface itself is Kratos-owned (ADR018 login row `—/—/—/✅`), no REST/GraphQL/MCP change.
