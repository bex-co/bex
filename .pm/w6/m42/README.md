# w6 · m42 — Open-signup launch gates: email verification at the door + payment `all` mode (ADR075 D7+D8)

**Worker:** worker6 **Goal:** both ADR075 launch gates enforced end to end — sign-up requires email verification before any session exists (D8), and every resource use (free tier included) requires a bound payment method (D7 `all` mode) — so open signup can launch without a mid-flight policy change. **Status:** in progress (t001–t006 done 2026-08-20; t012 OTP auto-submit + paste and t013 branded Kratos emails done 2026-08-21, all verified end to end on dev-6; t007 production enablement + closing tasks open)

## Implementation notes (2026-08-20)

- **D8 revised same-day to the HYBRID (user decision, after walking the first implemented flow):** the at-the-door shape forced every new user back to a login form right after entering the code — a restart at the most conversion-critical moment, buying ~zero security (invites gated independently, usage gated by D7's card wall). Now: registration keeps `session` + adds `show_verification_ui` (verify **while signed in**, continue seamlessly to the deep link), and `require_verified_address` stays on both login methods as the backstop so a stale unverified account cannot start a new session. t007's session-revocation step deleted accordingly. ADR075 D8/D3/D2 carry the revision.
- D8 needed a hook the ADR draft missed: **`show_verification_ui`** on `registration.after.<method>.hooks` is what emits the verification `continue_with` (verified empirically on dev-6; with the after-hooks merely emptied Kratos falls back to a bare `redirect_browser_to`); both config files carry it.
- Backstop verified live on dev-6 (during the interim at-the-door build): unverified login refused with `session_verified_address_required` + `show_verification_ui` continue_with (Elements redirects into verification). Hybrid happy path E2E'd via Playwright: sign-up (`?next=/usage`) → verification page **signed in** (code + working **Resend code** button) → straight to the target, no re-login.
- t006 invariant PROVEN on Kratos v26.2.0: recovery of an unverified identity marks the address verified while minting the session — evidence + re-probe commands in `done/t006.md`, re-probe note pinned in kratos.values.yaml.
- Pre-existing, unrelated: two `webhook-deliveries-card` dashboard tests fail when a stray `VITE_API_URL` env leaks into vitest (absolute hrefs); all 2337 tests pass with it cleared. Not introduced here.

## Tasks (in order)

| id   | title                                                                                    | est | depends_on                         |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------------------------------- |
| t001 | D7: `BEX_REQUIRE_PAYMENT_METHOD=all` gate mode in the backend seam — **DONE**            | 45m | —                                  |
| t002 | D7: synchronous card check at agent-session admission — **DONE**                         | 30m | t001                               |
| t003 | D8: Kratos config — hook both login methods, remove both registration session hooks — **DONE** | 45m | —                             |
| t004 | D8: dashboard auth-flow handling + guarded `next` threading — **DONE**                   | 60m | t003                               |
| t005 | D8: verification resend affordance + courier-health alert — **DONE**                     | 30m | t004                               |
| t006 | D8: recovery-flow invariant check (recovery marks the address verified) — **DONE**       | 30m | t003                               |
| t007 | Enablement rollout: comp first-party workspaces, flip `all`, apply the D8 hybrid config  | 45m | t001, t002, t003, t004, t005, t006 |
| t012 | Verification OTP input: auto-submit on last digit + page-wide paste capture — **DONE**   | 45m | t004                               |
| t013 | Brand the Kratos courier emails with the bex email layout — **DONE**                     | 90m | t003                               |
| t008 | Render parity: widened 402 + auth flows consistent across REST/GraphQL/MCP/dashboard     | 30m | t007, t012, t013                   |
| t009 | Simplify: `/simplify` over the changed code                                              | 30m | t008                               |
| t010 | Test coverage: gate-mode, admission, and auth-flow regressions                           | 45m | t008                               |
| t011 | Closeout                                                                                 | 15m | t010                               |

## Definition of done

Observable on production (and reproducible on a `dev-6` stack):

- A fresh password sign-up **holds its session** and lands on the verification flow; entering the code continues straight into the product (deep link or `/`) with **no re-login anywhere in the happy path** (D8 hybrid, revised 2026-08-20). A **stale** unverified identity's login attempt (password **or** GitHub OIDC with a provider-unverified address) is refused into the verification flow — never a 404 or a dead end — so an unverified identity cannot outlive its registration session.
- With `BEX_REQUIRE_PAYMENT_METHOD=all`, a cardless workspace's **free-tier** Service/Postgres/KeyValue create, Blueprint apply, and **agent-session create/steer** are refused **synchronously** with the coded 402/`PAYMENT_REQUIRED` across REST, GraphQL, and MCP, and the dashboard's w1/m62 interception dialog resolves it inline (cancel offers the D6 self-host exit where that dialog already carries it). `1` keeps today's paid-intent-only behavior byte-identical; unset keeps the gate off.
- Existing first-party workspaces are unaffected at flip time (comped Mode-B or card-bound as part of t007 — no surprise refusals).
- The guarded `next` param survives sign-up → verification → login; `/auth/verification` has a working resend affordance and the courier has a health alert.
- All three test suites green; `.env.example` + CLAUDE.md env table mirror the new `all` value.

## Source + Goal linkage

- **Source:** `docs/ADR075-user-onboarding.md` D7+D8 (user decisions 2026-08-19, corrected 2026-08-20 by the PM/principal-engineer review — the review found the earlier draft's OIDC seam wrong in two places and the agent-session admission gap; both are in scope here). Materialized per user direction 2026-08-20: the launch gates are their own milestone, split from the D1–D6 dashboard work by risk profile.
- **Goal linkage:** open-signup readiness (ADR075 Consequences: both gates are launch preconditions, same "before open signup" class as `.pm/DO_NOT_DO.md` #PSL); revenue integrity — no unbillable metered usage (ADR046 §0's egress/build-seconds problem, now closed for the free tier too).
- **Expected outcome:** activation is the ordered pair verify-email → bind-card → first deploy, enforced by Kratos config + the existing `PaymentGate` seam; enabling open signup later requires no policy change on real users.
- **Why now:** with zero external users there is nothing to grandfather — the cheapest moment to build the wall (ADR075 Context § Positioning). D8 also neutralizes the w1/m53 invite-redemption attack at the door.
- **Render parity included:** D7 widens the 402 scope across REST/GraphQL/MCP and D8 restructures user-facing auth flows — both are tenant-facing surface changes (and both are deliberate, documented divergences from Render's softer walls; parity here means internal three-surface consistency + recording the divergence, per ADR075).
