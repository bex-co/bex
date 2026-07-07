# w4 · m7 — Email flows: Kratos courier + recovery/verification live

**Worker:** worker4 **Goal:** the identity lifecycle stops dead-ending — Kratos's courier sends real mail (SendGrid SMTP relay in prod, secret out-of-band; Mailpit catcher locally), so the dashboard's already-shipped forgot-password/reset-password pages work end-to-end and signup triggers a verification email. Resolves the "no SMTP configured" consequence in `docs/auth.md` and the TODO in `base/values/kratos.values.yaml`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | SendGrid SMTP relay + secret plumbing (`.env` → `auth-secrets.sh` → gh-secrets), recorded in `docs/auth.md`   | 25m | —          |
| t002 | Mailpit in the local overlay (Argo app, mail catcher; control-plane pin + PSA per local quirks)                | 30m | t001       |
| t003 | Kratos values: enable courier from secret + verification flow; local overlay targets Mailpit                   | 35m | t002       |
| t004 | E2E on mock: recovery end-to-end via Mailpit API + verification email on signup                                | 40m | t003       |
| t005 | Prod wiring + docs: `deploy.yml` step, `docs/auth.md` consequences updated, reset-password page verified       | 25m | t004       |
| t006 | Simplify — run `/simplify` over the code this milestone changed                                                | 20m | t005       |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped                                       | 30m | t005       |

## Definition of done

On the local mock cluster with Mailpit deployed: a dashboard user completes forgot-password end-to-end — the recovery email lands in Mailpit, its code/link completes the reset, and the new password logs in; signup triggers a verification email; both flows are scripted (extension of `scripts/auth-e2e.sh` or a sibling, exit 0 = pass); in prod values the courier reads its SendGrid SMTP URI from the out-of-band Secret with var names mirrored in `.env.example`/`.env.template` (mechanism provider-agnostic for self-hosters); the "no SMTP" consequence in `docs/auth.md` is replaced with the shipped design.

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (w4 sweep); gap flagged in `docs/auth.md` §Consequences and the courier TODO in `deploy/gitops/base/values/kratos.values.yaml`.
- **Goal linkage:** roadmap #1 (multi-tenant control plane) — self-service identity is not real if account recovery dead-ends; pillar 1's API-first claim extends to the identity lifecycle.
- **Expected outcome:** the two dashboard pages that currently dead-end (forgot-password, verification) work end-to-end, observable in Mailpit locally and a real inbox in prod.
- **Why now:** the dead end already shipped — the dashboard advertises recovery it cannot deliver; every real-tenant signup before this lands risks a permanent lockout. Independent of m4–m6, so it parallelizes cleanly.
