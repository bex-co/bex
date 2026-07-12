# w4 · m7 — Email flows: Kratos courier + recovery/verification live

**Worker:** worker4 **Goal:** the identity lifecycle stops dead-ending — Kratos's courier sends real mail (SendGrid SMTP relay in prod, secret out-of-band; Mailpit catcher locally), so the dashboard's already-shipped forgot-password/reset-password pages work end-to-end and signup triggers a verification email. Resolves the "no SMTP configured" consequence in `docs/ADR012-auth.md` and the TODO in `base/values/kratos.values.yaml`. **Status:** done (2026-07-12; recovery + verification + both negatives proven end-to-end by `scripts/auth-mail-e2e.sh`, exit 0. Per the 2026-07-11 mock-cluster pool-label precondition, the scripted proof runs as a self-contained Docker harness — real Kratos with the courier enabled + `code` recovery/verification wired exactly as `base/values/kratos.values.yaml`, plus a Mailpit catcher — the same shape the social-login milestone used; base values verified via `helm template` (courier StatefulSet platform-pinned, `COURIER_SMTP_CONNECTION_URI` from the Secret key `smtpConnectionURI`, placeholder stripped from the ConfigMap), Mailpit confined to the local overlay via kustomize render. Missing `auth.verification` dashboard page filed as inbox note `w4/008`.)

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | SendGrid SMTP relay + secret plumbing (`.env` → `auth-secrets.sh` → gh-secrets) for both consumers (Kratos courier + bex-api `BEX_SMTP_*`), recorded in `docs/ADR012-auth.md` — **DONE** | 30m | —          |
| t002 | Mailpit in the local overlay (Argo app, mail catcher; control-plane pin + PSA per local quirks) — **DONE**     | 30m | t001       |
| t003 | Kratos values: enable courier from secret + verification flow; local overlay targets Mailpit — **DONE**        | 35m | t002       |
| t004 | E2E on mock: recovery end-to-end via Mailpit API + verification email on signup — **DONE**                     | 40m | t003       |
| t005 | Prod wiring + docs: `deploy.yml` step, `BEX_SMTP_*` into the bex-api deployment, `docs/ADR012-auth.md` consequences updated, reset-password page verified — **DONE** | 30m | t004       |
| t006 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                                     | 20m | t005       |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE**                            | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09) — **DONE**                                | 10m | t007       |

## Definition of done

On the local mock cluster with Mailpit deployed: a dashboard user completes forgot-password end-to-end — the recovery email lands in Mailpit, its code/link completes the reset, and the new password logs in; signup triggers a verification email (proven via the API flow, t004 — see the 2026-07-11 note below on the missing dashboard page); both flows are scripted (extension of `scripts/auth-e2e.sh` or a sibling, exit 0 = pass); in prod values the courier reads its SendGrid SMTP URI from the out-of-band Secret, and the same relay feeds bex-api's invite mailer (`BEX_SMTP_ADDR/FROM/USERNAME/PASSWORD`, w4/m12) — all var names mirrored in `.env.example`/`.env.template` (mechanism provider-agnostic for self-hosters); the "no SMTP" consequence in `docs/ADR012-auth.md` is replaced with the shipped design.

_2026-07-11: mock precondition — the m19 platform-pool sweep pinned the auth substrate (kratos/hydra deployments+jobs, CNPG auth DBs) to `bex.co/pool=platform`, a label no CAPD mock node carries and the local overlay does not neutralize, so a fresh local sync leaves the whole stack Pending. The mock-cluster half of this DoD is blocked on the `w1` inbox note on mock pool-labels (label the mock nodes or sweep the local overlay values); alternatively retarget verification at prod._

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (w4 sweep); gap flagged in `docs/ADR012-auth.md` §Consequences and the courier TODO in `deploy/gitops/base/values/kratos.values.yaml`.
- **Goal linkage:** roadmap #1 (multi-tenant control plane) — self-service identity is not real if account recovery dead-ends; pillar 1's API-first claim extends to the identity lifecycle.
- **Expected outcome:** the two dashboard pages that currently dead-end (forgot-password, verification) work end-to-end, observable in Mailpit locally and a real inbox in prod.
- **Why now:** the dead end already shipped — the dashboard advertises recovery it cannot deliver; every real-tenant signup before this lands risks a permanent lockout. Independent of m4–m6, so it parallelizes cleanly.
- **Render parity closing task: omitted** (retrofit note 2026-07-09) — courier configuration only; no REST/GraphQL/MCP surface changes. _2026-07-11: the claim that "the recovery/verification dashboard pages already exist unchanged" is half-false — `dashboard/src/routes/` has `auth.forgot-password.tsx`/`auth.reset-password.tsx` but **no** `auth.verification` route, while both kratos values files point `verification.ui_url` at `/auth/verification`, so a verification email's link dead-ends on the catch-all. This milestone proves verification via the API flow only (t004); file the missing Ory Elements verification page (mirror of `auth.reset-password.tsx`) as a follow-up inbox note at closeout — do not close with the README asserting the page exists._
