# w4 · m11 — MFA: TOTP + passkeys via Kratos

**Worker:** worker4 **Goal:** A dashboard user enrolls TOTP and/or a passkey from Settings and gets a second-factor challenge on login — Kratos `totp` + `webauthn` methods enabled in values, Ory Elements rendering the flows, lookup-secret recovery codes so MFA + lost device ≠ lockout. All Kratos-native; no custom auth code. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Kratos values: enable `totp`, `webauthn` (passkey mode), `lookup_secret` + `highest_available` AAL for settings/session    | 30m | —          |
| t002 | Dashboard: settings-flow enroll/unenroll rendering (Ory Elements) + the `aal2` login challenge step                        | 35m | t001       |
| t003 | E2E on mock: enroll TOTP (otplib codes) → logout → login challenges; recovery codes work; scripted exit-0 check            | 35m | t002       |
| t004 | Prod values + docs: WebAuthn RP ID/origin for `dashboard.bex.co`; `docs/auth.md` consequences updated                      | 20m | t003       |
| t007 | Render parity — MFA surface check vs Render's 2FA; update the parity matrix row (retrofit 2026-07-09)                      | 15m | t004       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                | 20m | t007       |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                   | 30m | t007       |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                       | 10m | t006       |

## Definition of done

On the mock cluster: a user enrolls TOTP in Settings, and the next login requires the code; lookup-secret recovery codes unlock a lost-device account; passkey enrollment works in a real browser against the local stack; prod values carry the correct WebAuthn RP ID/origin for `dashboard.bex.co`; the flow is scripted (extension of `scripts/auth-e2e.sh` or a sibling, exit 0 = pass). No custom auth code — configuration + Ory Elements rendering only.

## Source + Goal linkage

- **Source:** promotion of inbox `w4/002` (parked 2026-07-06 with the condition "revisit after w1/m2 lands the tenants/accounts model" — now met: w1/m2 implemented, one acceptance task open); `/pm-brainstorm tasks for w4` 2026-07-08.
- **Goal linkage:** roadmap #5/#7 — account security for a PaaS holding tenants' credentials and databases.
- **Expected outcome:** bex accounts are protectable by a second factor before any real tenant exists.
- **Why now:** w1/m9 (real tenant onboarding) is queued — shipping MFA **before** tenants arrive means day-one protection; after, it means forcing enrollment migrations on live users.
