# Unverified email and no bound card still get a working account

Why: Product rule is that a user must verify their email AND bind a card before using bex. Production enforces neither, so unverified, card-less identities get a workspace and full API access — and any OIDC relying party on `oauth.bex.co` (e.g. the forum.bex.co Discourse client) receives tokens for unverified emails.

- **Severity / attribution:** major / bex-api onboarding + prod config + Kratos flow. Policy bypass in production; real accounts affected (below).
- **Estimate:** likely > 1h across several tasks (gate + dashboard wall + prod flip + decision on existing accounts) — promote to a milestone before starting. Source: forum.bex.co OIDC setup, 2026-10-05. Goal: ADR075 D8 (verification), ADR040 billing / paid-intent gate.
- **Versions:** bex2 checkout `9c67eaec9`; prod `hetzner-prod` as of 2026-10-05 ~19:30 PDT.

## Evidence

**1. Email verification is not a gate after sign-up.** `deploy/gitops/base/values/kratos.values.yaml`:

- `registration.after.password.hooks` = `show_verification_ui`, then `session` — sign-up mints a live session _before_ the address is verified (ADR075 D8 as revised 2026-08-20). The verification UI is shown, but nothing stops the user from leaving it and continuing with that session.
- `login.after.password.hooks` = `require_verified_address` — only a _later password login_ is refused. A session that already exists (the sign-up one) is never re-checked.
- The OIDC (social) mirror lives in `scripts/auth-secrets.sh` `oidc_fragment` (`require_verified_address` at line 127); same shape.

**2. bex-api onboards unverified humans anyway.** `lego/backend/internal/api/auth.go:343` calls `EnsureTenant(ctx, subject, email, emailVerified)` for every human caller; `internal/api/tenancy.go:156` mints a personal workspace regardless of `emailVerified`. The only verified-email check is invite redemption (`acceptInvites`, `BEX_REQUIRE_VERIFIED_INVITE_EMAIL`, `tenancy.go:104`). `internal/router/service.go:264` is the only other consumer.

**3. The card gate is off in production.** `kubectl -n bex-system get deploy bex-api` env: `BEX_REQUIRE_PAYMENT_METHOD=` (empty). Per `lego/backend/AGENTS.md`, `1` = paid-intent gate and `all` = includes free plans + agent sessions and drives the dashboard sign-up wall (`paymentMethodOnboardingRequired`). Empty = no gate at all.

**4. Production data** (read-only, `BEGIN READ ONLY … ROLLBACK` on `bex-db`, Kratos admin `GET /admin/identities`; "verified" = the verifiable address matching the `email` trait has `verified: true`):

| measure                                                        | count               |
| -------------------------------------------------------------- | ------------------- |
| Kratos identities                                              | 69                  |
| … email NOT verified                                           | 41                  |
| unverified identities that are members of ≥1 workspace         | 15                  |
| workspaces those 15 belong to                                  | 16                  |
| … of which have a live card (`billing_provider_mappings`)      | 1                   |
| apps in those 16 workspaces                                    | 6                   |
| workspaces owning apps, across the platform                    | 4                   |
| … of which have NO live card bound                             | 1                   |
| newest workspace with an unverified member                     | 2026-09-24 18:39:06 |

Not checked: whether the 15 are QA/operator accounts vs. real customers, and when each app was created relative to any earlier gate.

**5. Downstream exposure (OIDC).** Hydra login reuses an existing Kratos session, so the sign-up session should complete an `oauth.bex.co` authorization-code flow without hitting `require_verified_address`. Inferred from config, **not yet reproduced** — verify on a dev-N stack. The forum.bex.co Discourse client (`bex-forum`) mitigates by sending real `email_verified` in the id_token (bex6 consent `OAUTH_IDENTITY_CLIENTS`), so Discourse won't link accounts by email for unverified users. Other relying parties may not.

## Expected

A human identity can't use bex (dashboard, REST/GraphQL/MCP, CLI, OAuth relying parties) until its email is verified **and** a card is bound to its workspace. Exceptions (operator/ops, bootstrap, invited members of a carded workspace, …) should be explicit.

## Suggested direction (to be decided at promotion)

1. Gate in bex-api's auth middleware: refuse (a coded error the dashboard can route) for humans whose `EmailVerified` is false, before `EnsureTenant` — one choke point for every API surface.
2. Gate OAuth consent the same way for non-platform clients (dashboard `hydra-consent.ts`): reject unverified subjects.
3. Turn the card gate on in prod (`BEX_REQUIRE_PAYMENT_METHOD=all`), after confirming the dashboard sign-up wall and the Stripe live-mode wiring.
4. Decide what happens to the existing 15 unverified-member identities and the 1 card-less workspace that owns apps (grace period, email, suspend).
5. Test: sign up on dev-N, skip verification, then call `/v1/services`, open the dashboard, and run an OAuth code flow — all three must be refused until verify + card.

## Triage (2026-10-05, read-only against `hetzner-prod`)

**Verdict: mostly working as designed (ADR075 D7/D8); no w2 fix is filed until the user decides on the D8 question below.**

- **#3 is wrong.** `BEX_REQUIRE_PAYMENT_METHOD` reaches the Deployment through `secretKeyRef` (`bex-stripe`, `lego/operator/config/api/deployment.yaml:230`), so `kubectl get deploy` can't show its value. The live Secret value is `all`, which matches ADR075's status line.
- **The card wall holds.** Of the 4 app-owning workspaces, 3 have a card bound. The 4th (`tea-da1eg9gbiuuc73bd8uag`) is `billing_excluded` (first-party Mode-A), and its only app predates the gate (2026-08-17). The 1 carded workspace with an unverified member is `tea-d98210cbbpdc73dcrkvg`: its owner is verified, and the unverified admin was added 2026-07-24, before D8 existed.
- **#1 and #2 are by design.** ADR075 D8 (user decision 2026-08-20) chose the hybrid: sign-up mints a session, and the login refusal is only a backstop. Option B (a bex-api `EMAIL_VERIFICATION_REQUIRED` gate) is a recorded deferral, with D7's card gate as the operative usage gate. The 41 unverified identities and 15 workspace members follow from that design. None of them can create resources without a card.
- **Real residue.** D8 claims that "an unverified identity cannot outlive its registration session". That's inaccurate: the session slides (`lifespan: 168h`, `earliest_possible_extend: 24h`), so an active sign-up session never expires. Unverified sessions therefore persist indefinitely and can complete OAuth flows for `skip_consent` clients (`bex-forum`, `bex-desktop`, `bex-mobile`, `bex-obs`). `bex-obs` is ops-gated, and desktop/mobile are first-party, the same exposure as the dashboard. `bex-forum`'s `email_verified` claim is still uncommitted work in the `bex6` checkout (`OAUTH_IDENTITY_CLIENTS`), so it should land before the forum goes live.
- **Open decision (user).** One option keeps D8 as-is, corrects the ADR075 sentence and closes this note. The other un-defers Option B (a per-request bex-api verification gate, plus consent refusal for unverified subjects), which reverses a recorded decision.
