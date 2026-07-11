# w4 · m11 — MFA: TOTP + passkeys via Kratos

**Worker:** worker4 **Goal:** A dashboard user enrolls TOTP and/or a security key from Settings and gets a second-factor challenge on login — Kratos `totp` + `webauthn` **as second factor** (`passwordless: false` — passkey/first-factor mode cannot satisfy the aal2 challenge) enabled in values, Ory Elements rendering the flows, lookup-secret recovery codes so MFA + lost device ≠ lockout. All Kratos-native; no custom auth code. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Kratos values: enable `totp`, `webauthn` (second factor, `passwordless: false`), `lookup_secret` + `highest_available` AAL — incl. prod RP ID `bex.co`/origins (base values ARE prod) | 40m | —          |
| t002 | Dashboard: settings-flow enroll/unenroll rendering (Ory Elements) + the `aal2` login challenge step                        | 35m | t001       |
| t003 | E2E on mock: enroll TOTP (otplib codes) → logout → login challenges; recovery codes work; scripted exit-0 check            | 35m | t002       |
| t004 | Docs + verification: rendered-config check (RP ID landed in t001), `docs/auth.md` MFA section, scripted prod smoke         | 20m | t003       |
| t007 | Render parity — MFA surface check vs Render's 2FA; update the parity matrix row (retrofit 2026-07-09)                      | 15m | t004       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                | 20m | t007       |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                   | 30m | t007       |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                       | 10m | t006       |

## Definition of done

On the mock cluster: a user enrolls TOTP in Settings, and the next login requires the code; lookup-secret recovery codes unlock a lost-device account; WebAuthn enrollment works in a real browser against the local stack; the live prod config (base values — see the 2026-07-11 note) carries WebAuthn RP ID `bex.co` + origins `[https://dashboard.bex.co]`, verified in the rendered config and by a scripted prod smoke; the flow is scripted (extension of `scripts/auth-e2e.sh` or a sibling, exit 0 = pass). No custom auth code — configuration + Ory Elements rendering only.

_2026-07-11: two rebuild-era corrections. (1) Mock precondition — the m19 platform-pool sweep pins the whole auth stack to `bex.co/pool=platform`, a label no CAPD mock node carries and the local overlay does not neutralize, so a fresh local sync leaves Kratos Pending; the mock-cluster half of this DoD is blocked on the `w1` inbox note on mock pool-labels, or retarget verification at prod. (2) There is no prod Kratos overlay — `deploy/gitops/base/values/kratos.values.yaml` IS the live prod config, auto-synced from main with prune+selfHeal, and Kratos refuses to start with webauthn enabled but no RP config; the RP ID/origins therefore land in t001's base edit (deferring them to t004 is a crashloop/stranded-enrollment window). WebAuthn ships as second factor (`passwordless: false`) — passkey mode (`passwordless: true`) and Kratos's separate `passkey` method are first-factor-only and cannot satisfy aal2._

## Source + Goal linkage

- **Source:** promotion of inbox `w4/002` (parked 2026-07-06 with the condition "revisit after w1/m2 lands the tenants/accounts model" — now met: w1/m2 implemented, one acceptance task open); `/pm-brainstorm tasks for w4` 2026-07-08.
- **Goal linkage:** roadmap #5/#7 — account security for a PaaS holding tenants' credentials and databases.
- **Expected outcome:** bex accounts are protectable by a second factor before any real tenant exists.
- **Why now:** w1/m9 (real tenant onboarding) is queued — shipping MFA **before** tenants arrive means day-one protection; after, it means forcing enrollment migrations on live users.
