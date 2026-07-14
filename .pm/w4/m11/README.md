# w4 · m11 — MFA: TOTP + passkeys via Kratos

**Worker:** worker4 **Goal:** A dashboard user enrolls TOTP and/or a security key from Settings and gets a second-factor challenge on login — Kratos `totp` + `webauthn` **as second factor** (`passwordless: false` — passkey/first-factor mode cannot satisfy the aal2 challenge) enabled in values, Ory Elements rendering the flows, lookup-secret recovery codes so MFA + lost device ≠ lockout. All Kratos-native; no custom auth code. **Status:** TOTP + recovery codes shipped, deployed, and prod-verified (`auth-mfa-e2e.sh` exit 0 against `auth.bex.co`). WebAuthn/passkey was live-attempted 2026-07-14 (Playwright + a CDP virtual authenticator against prod) and found **actively broken by two real bugs** — a dashboard CSP blocking Kratos's WebAuthn script, and Kratos's identity schema missing the `webauthn` identifier trait. Both root-caused and fixed in source (see the 2026-07-14 note below) but **unshipped** — t008 stays open until deployed and re-verified live

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on | status |
| ---- | ------------------------------------------------------------------------------------------------------------------------ | --- | ---------- | ------ |
| t001 | Kratos values: enable `totp`, `webauthn` (second factor, `passwordless: false`), `lookup_secret` + `highest_available` AAL — incl. prod RP ID `bex.co`/origins (base values ARE prod) | 40m | —          | — **DONE** |
| t002 | Dashboard: settings-flow enroll/unenroll rendering (Ory Elements) + the `aal2` login challenge step                        | 35m | t001       | code + tests **DONE**; TOTP/aal2 proven on prod; browser passkey the one manual check |
| t003 | E2E on mock: enroll TOTP (otplib codes) → logout → login challenges; recovery codes work; scripted exit-0 check            | 35m | t002       | — **DONE** (ran green against prod `auth.bex.co`, exit 0) |
| t004 | Docs + verification: rendered-config check (RP ID landed in t001), `docs/ADR012-auth.md` MFA section, scripted prod smoke         | 20m | t003       | — **DONE** (rendered-config + docs + prod smoke green) |
| t007 | Render parity — MFA surface check vs Render's 2FA; update the parity matrix row (retrofit 2026-07-09)                      | 15m | t004       | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                | 20m | t007       | — **DONE** |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                   | 30m | t007       | — **DONE** |
| t009 | Fix WebAuthn — dashboard CSP + Kratos identity-schema bugs found via live browser reproduction (2026-07-14)                | 1h  | t002       | fixes in source, unshipped — awaits `/ship` + live re-verification |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                       | 10m | t006, t009 | open — awaits live DoD |

## Definition of done

On the mock cluster: a user enrolls TOTP in Settings, and the next login requires the code; lookup-secret recovery codes unlock a lost-device account; WebAuthn enrollment works in a real browser against the local stack; the live prod config (base values — see the 2026-07-11 note) carries WebAuthn RP ID `bex.co` + origins `[https://dashboard.bex.co]`, verified in the rendered config and by a scripted prod smoke; the flow is scripted (extension of `scripts/auth-e2e.sh` or a sibling, exit 0 = pass). No custom auth code — configuration + Ory Elements rendering only.

_2026-07-11: two rebuild-era corrections. (1) Mock precondition — the m19 platform-pool sweep pins the whole auth stack to `bex.co/pool=platform`, a label no CAPD mock node carries and the local overlay does not neutralize, so a fresh local sync leaves Kratos Pending; the mock-cluster half of this DoD is blocked on the `w1` inbox note on mock pool-labels, or retarget verification at prod. (2) There is no prod Kratos overlay — `deploy/gitops/base/values/kratos.values.yaml` IS the live prod config, auto-synced from main with prune+selfHeal, and Kratos refuses to start with webauthn enabled but no RP config; the RP ID/origins therefore land in t001's base edit (deferring them to t004 is a crashloop/stranded-enrollment window). WebAuthn ships as second factor (`passwordless: false`) — passkey mode (`passwordless: true`) and Kratos's separate `passkey` method are first-factor-only and cannot satisfy aal2._

## Completion note (2026-07-11)

Implementation is complete and **verified against prod** — the mock-cluster block was routed around exactly as the DoD's 2026-07-11 note allows ("or retarget verification at prod"). Shipped as `e5c449a`; Argo auto-synced the kratos Application to it and Kratos rolled out `1/1 Running`, 0 restarts — **no crashloop** (the RP config shipped in the same base edit, so WebAuthn-enabled Kratos started cleanly). `scripts/auth-mfa-e2e.sh` with `KRATOS_PUBLIC_URL=https://auth.bex.co` **exits 0**: register → enroll TOTP + recovery codes → password-only login is aal1/`whoami` 403 → wrong TOTP rejected / right upgrades to aal2 → recovery code unlocks + single-use. The only outstanding DoD item is the manual browser WebAuthn ceremony (can't be scripted over curl).

Prod verification also corrected two script assumptions that never fired until a live Kratos exercised them: (1) an aal1 privileged session **can** open settings and enroll credentials — `required_aal: highest_available` gates privileged submits by session recency, not settings-flow creation, and the user-visible gate is the *next login* (`whoami` 403), so recovery codes now enroll on the aal1 session with no mid-flow step-up; (2) recovery codes are 8-char alphanumeric read from the structured `lookup_secret_codes` node, and `mapfile` → a `while read` loop for stock-bash-3.2 compatibility. `docs/ADR012-auth.md` §9 updated to match.

**Shipped + verified in-environment:**

- **t001** — `totp` (issuer `bex`), `webauthn` (`passwordless: false`, RP `bex.co` / origins `[https://dashboard.bex.co]`), `lookup_secret`, and `highest_available` AAL on whoami + settings, all in [`base/values/kratos.values.yaml`](../../../deploy/gitops/base/values/kratos.values.yaml); the [local overlay](../../../deploy/gitops/overlays/local/values/kratos.values.yaml) deep-merges an `rp.id: localhost` / `origins: [http://localhost:5173]` override. **Verified by `helm template`:** prod renders RP `bex.co`, local renders `localhost`, `passwordless:false` + TOTP + lookup_secret carry through the merge, and the m19 platform-pool `nodeSelector`/tolerations are intact.
- **t002** — dashboard `aal=aal2` step-up wired in [`use-ory-flow.ts`](../../../dashboard/src/common/hooks/use-ory-flow.ts) (mints a second-factor flow on `session_already_available`, renders it only when it carries a second-factor node, otherwise navigates on); settings/login rendering is Ory Elements, unchanged. Settings subtitle i18n updated (en/zh). `yarn typecheck` + `yarn lint` clean.
- **t003** — [`scripts/auth-mfa-e2e.sh`](../../../scripts/auth-mfa-e2e.sh): register → enroll TOTP → aal2 step-up → enroll recovery codes → assert password-only login is aal1/403 → wrong TOTP rejected / right one upgrades → single-use recovery code. Zero-dep TOTP (pure `node` crypto) **cross-checked against a Python reference**; `bash -n` clean; parameterized for prod via `KRATOS_PUBLIC_URL`.
- **t004** — `docs/ADR012-auth.md` §9 (methods, AAL policy, recovery codes, RP-ID-is-forever, `passwordless:false` decision, dashboard step-up); prettier-clean.
- **t006** — 6 new fixture-driven `use-ory-flow` tests (aal2 step-up renders for totp/webauthn/lookup_secret; navigates on when no second factor / when step-up refused; non-login flows never step up). Full suite 602/602 green.
- **t007** — `docs/ADR018-render-parity.md` MFA row ✖ → ✅ with evidence (superset: WebAuthn security keys).
- **t005** — `/simplify` (4 parallel review agents): applied the script dedup (route `whoami_aal`/`step_up`/`login_password` through the existing `kratos()`/`jqf()`/`rel()` helpers, behavior-preserving); reuse/efficiency clean; **skipped** the altitude finding (propagate `aal2_required` from `fetchSession`→`requireAuth`→explicit `aal=aal2` param) as an out-of-diff refactor touching `session.ts`/`auth.ts`/route schema — a reasonable follow-up, filed below.

**Verified on prod (2026-07-11):** deploy (Argo → Kratos `1/1`, no crashloop), scripted smoke (`auth-mfa-e2e.sh` exit 0 against `auth.bex.co`), TOTP enroll → next-login-requires-code, single-use recovery codes.

**Outstanding (the one manual check keeping t008 open):**

- Browser WebAuthn enroll/challenge — a real browser + authenticator ceremony (can't be scripted over curl). Everything else in the DoD is verified.

**Follow-up (non-blocking):** consider lifting aal2 detection out of the login page into the session fetch (the t005 altitude finding) — surface `session_aal2_required` from `fetchSession` and redirect with an explicit `aal=aal2` param, so the hook stops rediscovering the step-up need by trial-and-error.

## Investigation note (2026-07-14) — t009: WebAuthn is broken, not just unverified

Attempted the outstanding manual check with Playwright driving a real Chrome session against **prod** (`dashboard.bex.co`), with a CDP `WebAuthn.addVirtualAuthenticator` standing in for a physical security key (Chrome's own WebAuthn testing API — the standard way to automate this ceremony without hardware). Registered a throwaway account (`mfa-webauthn-verify-1752522000@bex.co`, left in place — no self-service account deletion exists, mirroring `auth-mfa-e2e.sh`'s own prod throwaway accounts) and opened Settings → "Add security key."

**Found two real, independent bugs, both now fixed in source (uncommitted):**

1. **Dashboard CSP silently blocked Kratos's WebAuthn glue script.** `dashboard/vite.config.ts`'s `script-src 'self' 'unsafe-inline'` has no allowance for `auth.bex.co`, so the `<script src="https://auth.bex.co/.well-known/ory/webauthn.js">` node Ory Elements injects (defining `window.__oryWebAuthnRegistration`/`__oryWebAuthnLogin`) never loads — confirmed via a CSP violation in the browser console, and via `@ory/elements-react`'s own source (`triggerToWindowCall` polls 10s for the global function, then throws "Unable to load Ory's WebAuthn script. Is it being blocked...?"). **Fixed:** `script-src` now includes the Kratos public origin (derived from `VITE_KRATOS_PUBLIC_URL`, same fallback `src/common/lib/ory/config.ts` uses).
2. **Kratos's identity schema never marked `email` as a WebAuthn identifier.** `deploy/gitops/base/values/kratos.values.yaml`'s `identity.schema.json` had `"credentials": { "password": { "identifier": true } }` only. Manually completing a real WebAuthn ceremony (bypassing bug 1 by injecting the fetched script content inline, then POSTing the real browser-produced attestation straight to Kratos's settings-flow API) returned Kratos error `4000009`: "Could not find any login identifiers. Did you forget to set them? This could also be caused by a server misconfiguration." **Fixed:** added `"webauthn": { "identifier": true }` alongside `password`; `helm template`/`helm lint` against the pinned chart (`ory/kratos@0.62.1`) confirm it renders correctly, and the local overlay inherits the fix unmodified (no shadowing `identitySchemas` block there).

**Why WebAuthn was never caught broken until now:** every prior verification pass (t002's original work, the 2026-07-11 prod smoke) exercised TOTP/recovery-codes end-to-end but explicitly deferred WebAuthn as "can't be scripted over curl" — nobody had driven an actual browser through the ceremony against the real deployed stack until this session.

**Status:** both fixes are source-only, **not deployed**. Per this repo's rules only `/ship` commits/pushes, so getting them live (and re-verifying the full enroll → logout → aal2-challenge ceremony in a real browser) is the one thing left. See `t009` for the full writeup.

## Source + Goal linkage

- **Source:** promotion of inbox `w4/002` (parked 2026-07-06 with the condition "revisit after w1/m2 lands the tenants/accounts model" — now met: w1/m2 implemented, one acceptance task open); `/pm-brainstorm tasks for w4` 2026-07-08.
- **Goal linkage:** roadmap #5/#7 — account security for a PaaS holding tenants' credentials and databases.
- **Expected outcome:** bex accounts are protectable by a second factor before any real tenant exists.
- **Why now:** w1/m9 (real tenant onboarding) is queued — shipping MFA **before** tenants arrive means day-one protection; after, it means forcing enrollment migrations on live users.
