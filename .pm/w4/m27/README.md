# w4 · m27 — Official Render CLI browser login via Hydra device flow

**Worker:** worker4 **Goal:** Make the official, unmodified `render login` work against production bex with Render's fixed public client and wire protocol: one permanent platform OAuth client, short-lived access tokens, automatic refresh, human tenancy, and per-user logout that never deletes the shared client. **Status:** todo (local implementation + E2E green; production rollout pending)

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Seed the one permanent public Render CLI OAuth client in every environment                              | 30m | —          |
| t002 | Render wire adapters: device grant/token/refresh plus the public-route boundary                         | 60m | t001       |
| t003 | Dashboard device verification/authorization through the existing Kratos + Hydra flow                   | 60m | t002       |
| t004 | Human tenancy, token identity, and logout/revocation semantics                                          | 60m | t003       |
| t005 | Official unmodified CLI E2E: login, forced refresh, logout, and two-user isolation                      | 45m | t004       |
| t006 | Render parity — update the CLI checklist/ledger from real protocol evidence                             | 30m | t005       |
| t007 | Simplify — `/simplify` over the milestone diff                                                          | 20m | t006       |
| t008 | Test coverage — adapter, auth-boundary, refresh, revocation, and shared-client regression cases         | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                          | 10m | t007, t008 |

## Definition of done

With `RENDER_API_KEY` unset and a clean isolated CLI config, the pinned official CLI completes `render login` in a browser, `render services` uses the signed-in human's workspace, an artificially expired stored access token refreshes automatically on the next command, and `render logout` makes both the old access token and its refresh chain unusable. The single Render CLI client remains registered and a second user's session remains valid. Production bootstrap is idempotent, no client secret exists, no token is made unexpired, and the Settings → API Keys list never exposes this platform client.

## Source + Goal linkage

- **Source:** user request 2026-07-15 after investigating whether Render CLI uses a long-lived API key. The pinned CLI (`c23438e`) instead uses Render's fixed public client `429024F5E608930E2A65EF92591A25CC`, device authorization, expiring access tokens, refresh tokens, and `/token/refresh/`. Existing evidence had falsely graded browser login/logout by setting `RENDER_API_KEY`, which makes login short-circuit and cannot exercise refresh.
- **Goal linkage:** Render parity (pillar 1) + w4 identity ownership. Hydra v26.2 already supports device authorization and refresh; bex is missing the fixed-client bootstrap, Render-shaped compatibility routes, browser approval bridge, and correct per-user revocation.
- **Expected outcome:** interactive users run the official `render login` with no wrapper or API-key credentials. CI may continue using the separate API-key client-credentials flow.
- **Why now:** the simplified production setup exposed that API-key access tokens expire every 15 minutes while the CLI cannot refresh environment-provided `RENDER_API_KEY`; the real interactive flow closes that UX gap without weakening token TTLs.
- **Render parity closing task: included** (t006) — this adds a user-facing CLI authentication surface and corrects false checklist evidence.

## Guardrails

- The permanent object is the **public client registration**, not an immortal bearer token. Access tokens stay expiring; refresh tokens rotate/revoke through Hydra.
- This client is platform configuration, not a tenant-created "bex API key": no secret, no one-client-per-key behavior, and no listing in Settings → API Keys.
- Reuse the Kratos-native login bridge and Hydra consent machinery from m9/m16/m17. Do not build a second identity provider or fork/patch the Render CLI.
- `RENDER_API_KEY` remains a separate, non-refreshable machine-auth override. Redesigning API keys or making their access tokens unexpired is out of scope.

## Evidence

- [Local implementation, bootstrap, CLI E2E, regression suites, and production-gate audit — 2026-07-15](evidence/local-2026-07-15.md)
