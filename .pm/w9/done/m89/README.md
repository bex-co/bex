# w9 · m89 — Public auth-surface bugfixes from live dashboard hunt

**Worker:** worker9 **Goal:** fix the real bugs Playwright found on unauthenticated `https://dashboard.bex.co/` auth pages so signed-out users get honest logout, branded device errors, and matching verification copy. **Status:** done

## Tasks (in order)

| id   | title                                                                 | est | depends_on                   |
| ---- | --------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | Signed-out logout: treat no-session as already signed out             | 35m | —                            — **DONE** |
| t002 | `/auth/device` without code: branded error, not raw `text/plain` 400  | 40m | —                            — **DONE** |
| t003 | Align verification (+ forgot) hero copy with the Ory step shown       | 25m | —                            — **DONE** |
| t004 | Login form hygiene: empty submit feedback + duplicate `id=password`   | 40m | —                            — **DONE** |
| t005 | Render parity                                                         | 20m | [w9/m89/t001, w9/m89/t002, w9/m89/t003, w9/m89/t004] — **DONE** |
| t006 | Simplify                                                              | 20m | [w9/m89/t005]                — **DONE** |
| t007 | Test coverage                                                         | 40m | [w9/m89/t005]                — **DONE** |
| t008 | Closeout                                                              | 15m | [w9/m89/t007]                — **DONE** |

## Definition of done

On live (or equivalent local) unauthenticated dashboard:

- Visiting `/auth/logout` and confirming **Sign out** with **no** Kratos session lands on `/auth/login` (or an explicit already-signed-out success), **never** the red "Sign-out failed / You may still be signed in" state.
- `GET /auth/device` with no `user_code` returns dashboard chrome (AuthPageShell or equivalent) explaining the code is missing/expired — **not** a bare `text/plain` body `missing or expired device code` with an empty document title. Consent's missing-challenge → home degradation is the pattern to match in spirit (human-readable, not a raw API error).
- `/auth/verification` hero subtitle matches the first Ory step shown (email identifier today), not "enter the code we sent" while the card asks for email. Forgot-password hero/card drift fixed in the same pass if still mismatched.
- Login empty submit either blocks with browser/`required` validation or shows a visible identity error; the password field and show-password control no longer share `id="password"` when an override is available.
- Dashboard unit tests cover the no-session logout path and the device missing-code response shape; `yarn typecheck && yarn lint && yarn test` green for touched dashboard files.

## Source + Goal linkage

- **Source:** Playwright live hunt of `https://dashboard.bex.co/` (2026-08-19 session; captures under `.playwright-mcp/hunt*.png` / `hunt2.json` / `hunt4.json`). User asked to hand the fixes to `/pm` for w9.
- **Goal linkage:** first-party auth UX / Render-parity dashboard trust (ADR012 sessions + ADR008 human surface). Public auth pages are the front door; lying about logout or dumping raw Hydra errors breaks that.
- **Expected outcome:** signed-out visitors no longer see a false "still signed in" failure, CLI device deep-links without a code look like the rest of the app, and verification copy stops contradicting the form.
- **Why now:** bugs were just reproduced on production with screenshots; they are small, unauthenticated, and block confidence in the auth chrome before any authenticated product hunt. No dependency on an open milestone (w9 queue empty after m88).
- **Render parity — included:** auth UI / device-verification browser chrome vs Render's human-facing login/device flows. No REST/GraphQL/MCP surface change expected; note any deliberate divergence (Kratos/Ory Elements vs Render's IdP).

## Closeout notes (t005 parity)

- **Logout / device / verification / login hygiene:** dashboard-only; no bex-api surface change. Device GET without `user_code` now returns `null` so the existing AuthPageShell expired chrome renders (HTTP 200 document) instead of `text/plain` 400 — deliberate human UX, not an API contract.
- **Deliberate IdP divergence:** bex stays on Kratos + Ory Elements (ADR012); Render's IdP copy/casing (e.g. "GitHub") remains in `w9/051` residuals.
- **Shipped:** `endBrowserSession` 401/403 → clear local state; `handleDeviceVerification` missing code → `null`; locale subtitles; `oryAuthFormOverrides` (unique password id + `required` email + Form.Root without `noValidate`).
