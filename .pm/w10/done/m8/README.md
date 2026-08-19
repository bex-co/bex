# w10 · m8 — Auth-flow consistency round 2: error states, logout page, CLI client branding

**Worker:** worker10 **Goal:** every path through the browser auth workflow — including the failure paths m7 didn't reach — renders inside `AuthPageShell`, and no user-visible surface anywhere (consent hero, Connected Agents) still says "Render CLI" **Status:** done

**Resolution (2026-08-19):** every device/consent document-GET/POST failure now carries a typed error code (`DeviceErrorCode`/`ConsentErrorCode`) through the existing loader-context pipeline instead of returning a bare `text/plain` `Response` — a POST failure redirects to its own GET endpoint with the code (mirroring the consent route's pre-existing retry pattern) so it renders inside `AuthPageShell` too; the round-14 scope-refusal's developer-facing detail survives as a `requiredScopes` array instead of response-body text. The logout page's hand-rolled full-screen layout is now `AuthPageShell` + `Card`, matching every other auth page. `scripts/auth-bootstrap-client.sh`'s Hydra client is registered as `"bex CLI"` (client id and `bex.co/platform-client` marker unchanged — the upsert is in place, never a recreate). `yarn typecheck && yarn lint && yarn test` green (339 files / 2341 tests, up from 338/2326). **Live dev/prod verification (screenshots of the shell-rendered states, re-running the bootstrap against a live Hydra) could not be completed** — the shared `bex` mock cluster was unreachable this session; blocker + resolution steps recorded in `w10/003`, not faked. The code itself is safe and idempotent regardless.

## Tasks (in order)

| id   | title                                                                                | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------------- | --- |
| t001 | Route `/auth/device` document-GET error states through AuthPageShell                 | 45m | —                | — **DONE** |
| t002 | Route `/auth/consent` refusal/error states through AuthPageShell                     | 45m | t001             | — **DONE** |
| t003 | Move the logout page onto AuthPageShell + fix the reset-password document title      | 40m | —                | — **DONE** |
| t004 | Rename the Hydra CLI client to "bex CLI" (bootstrap script + re-run, identifier tidy) | 30m | —                | — **DONE** |
| t005 | Render parity — auth-surface consistency check                                        | 20m | t002, t003, t004 | — **DONE** |
| t006 | Simplify — /simplify over the changed code                                            | 20m | t005             | — **DONE** |
| t007 | Test coverage — error-state pages, logout shell, bootstrap rename                     | 30m | t005             | — **DONE** |
| t008 | Closeout                                                                              | 15m | t007             | — **DONE** |

## Definition of done

Navigating to `/auth/device` with an expired/invalid/missing device code, or to `/auth/consent` with a refused challenge (PKCE refusal, scope refusal, unconfigured provider, accept-failed, cross-user), renders a themed `AuthPageShell` error page in both locales — never a bare `text/plain` body; `/auth/logout` renders inside `AuthPageShell` with the same chrome as the rest of the auth flow; the reset-password route's document title no longer says "Forgot password"; after re-running `scripts/auth-bootstrap-client.sh`, the consent hero and Settings → Connected Agents show "bex CLI", not "Render CLI (bex platform)"; the device/consent Hydra bridge endpoints remain byte-compatible (codex-security #9 same-origin session-bound POST semantics preserved); `yarn typecheck && yarn lint && yarn test` green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-18 auth-flow mine — the direct continuation of w10/m7 (user report 2026-08-18). m7 fixed the confirm page's happy path; the mine found its siblings untouched: every device/consent failure still returns raw `text/plain` on a full-page navigation (`hydra-device.ts:21-26` `error()` helper; `hydra-consent.ts` `pkceRefusal()` :184, `scopeRefusal()` :250, unconfigured/accept-failed/cross-user paths), the logout page hand-rolls its own layout (`logout-page/index.tsx:66-160`), and the mis-branding the user reported survives at its source — `scripts/auth-bootstrap-client.sh:170` registers the Hydra client as `"Render CLI (bex platform)"`, which surfaces verbatim on the consent hero and in Connected Agents.
- **Goal linkage:** the device/consent flow is the human face of `bex login` (ADR012 §7/§8a) and of every MCP agent connect (ADR025); a branded happy path that dumps users into unstyled text on the _common_ failure (an expired device code) undoes m7's point.
- **Expected outcome:** the whole authorize flow — success and failure — reads as one product, in the user's theme and language, with "bex CLI" branding on every surface including Settings → Connected Agents.
- **Why now:** the error paths and the client name are the two halves of the original 2026-08-18 user report that m7 didn't reach; the device/consent code context is freshest immediately after m7 shipped (`080b7993`).
- **Render parity note:** included (t005) — this is a fix touching the dashboard UI and the consent/device browser surface; the check confirms the Hydra bridge endpoints stay byte-compatible and no REST/GraphQL/MCP shape changes.
