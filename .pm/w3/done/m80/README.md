# w3 · m80 — Session-expiry experience: auth-aware 401 handling, sliding sessions, lossless re-auth

**Worker:** worker3 **Goal:** a mid-session Kratos-session expiry never renders as a platform outage — the dashboard detects the 401, sends the user through sign-in with `next=` back to the exact page, active users rarely hit expiry at all (sliding sessions), and unsaved work survives the round-trip. **Status:** done (2026-09-07). t001–t008 shipped and deterministically tested; t009's destructive live E2E ran on prod `dashboard.bex.co` and passed all four DoD behaviors — mounted-page 401 → `/auth/login?next=…` (not the "bex-api failed" card, network-log confirmed), the 168-hour sliding Kratos window, one-sign-in return to the exact page, and unsaved env-editor edits restored across the round-trip ("Restored unsaved changes" banner). Full reproduction + evidence in [t009](done/t009.md). The re-auth was completed by minting a fresh Kratos session through `scripts/qa-login.sh` so the QA password never entered the browser.

## Tasks (in order)

| id   | title                                                                 | est | depends_on          |
| ---- | --------------------------------------------------------------------- | --- | ------------------- |
| t001 | Centralized 401 handling in the Apollo link chain                     | 45m | — — **DONE**        |
| t002 | Auth-aware error taxonomy: session-expired card, never "api failed"   | 45m | t001 — **DONE**     |
| t003 | Kratos session policy: explicit lifespan + sliding extension          | 30m | — — **DONE**        |
| t004 | Proactive expiry: surface `expires_at`, kill the stale-session window | 45m | t001 — **DONE**     |
| t005 | Lossless re-auth: preserve unsaved env-editor work across sign-in     | 60m | t002 — **DONE**     |
| t006 | Render parity: compare render.com session-expiry behavior             | 30m | t003, t005 — **DONE** |
| t007 | Simplify: run /simplify over the changed code                         | 30m | t006 — **DONE**     |
| t008 | Test coverage: 401 classification, redirect, draft round-trip         | 45m | t006 — **DONE**     |
| t009 | Closeout                                                              | 15m | t008 — **DONE**     |

## Definition of done

With a signed-in dashboard session that is then expired server-side (revoke via Kratos), an already-mounted service page's next Apollo request produces a redirect to `/auth/login?next=<that page>` (or an explicit "Your session has expired — Sign in" card) — **never** the "Couldn't load service / The request to bex-api failed" network-error card — and after one sign-in the user lands back on the same page. Kratos config declares an explicit session lifespan with sliding extension (active users are not logged out on the old fixed 24h default). Unsaved environment-variable edits survive the re-auth round-trip. Tests lock the 401-vs-network classification and the redirect target.

## Source + Goal linkage

- **Source:** live prod screenshot 2026-08-26 (`eden-cms-v2` → "Couldn't load service · The request to bex-api failed" after an expired session) + PM research hand-off the same day: codebase audit (no Apollo error link — `dashboard/src/common/apollo/factory.client.ts`; retry link skips 401 — `retry-link.ts:21-34`; error card lumps auth with outages — `service-detail-layout.tsx:56-58`; no Kratos `lifespan`/`earliest_possible_extend` configured — `deploy/gitops/base/values/kratos.values.yaml`) + industry best practice (OWASP Session Management Cheat Sheet: idle + absolute timeouts, sliding extension; WCAG 2.2.5/G105: preserve work across re-auth; Apollo error-link 401 interception with returnTo).
- **Goal linkage:** hosted-platform trust (a false "bex-api failed" signal makes the platform look down when it isn't — perceived reliability is the product) + Render-parity dashboard UX (Render never shows a service-load failure for an expired session). Stays inside the ADR012 auth architecture: Kratos cookie sessions, no browser-held tokens (`.pm/DO_NOT_DO.md` line 15 respected — this is session *handling*, not an auth re-architecture).
- **Expected outcome:** zero 401s rendered as network-error cards; forced mid-session logouts become rare (sliding sessions) and lossless (returnTo + draft preservation) when they do happen.
- **Why now:** reproduced on prod today by the operator's own daily use; every session older than 24h hits it, so QA walks and demos routinely open on a false outage card. Render parity task included because the change touches the dashboard user surface (no REST/GraphQL/MCP contract change — the 401 shape is untouched).
