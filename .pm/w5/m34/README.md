# w5 · m34 — Lift aal2 step-up detection into the session fetch

**Worker:** worker5 **Goal:** The dashboard's MFA step-up stops being discovered by trial-and-error on the login page: `fetchSession` surfaces `session_aal2_required` itself and the redirect carries an explicit `aal=aal2` param — one deterministic hop to the step-up screen, per the refactor the w4/m11 closeout specced. **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Surface `session_aal2_required` from `fetchSession`          | 45m | —          |
| t002 | Explicit `aal=aal2` redirect; remove the trial-and-error path | 45m | t001       |
| t003 | Exercise the TOTP/WebAuthn step-up flow end-to-end            | 30m | t002       |
| t004 | Simplify                                                       | 20m | t003       |
| t005 | Test coverage                                                  | 30m | t003       |
| t006 | Closeout                                                       | 15m | t005       |

## Definition of done

Logging in as an MFA-enrolled identity lands on the step-up screen via a single redirect carrying `aal=aal2`; the browser network log shows no failed probe/retry requests; the login page contains no aal2 rediscovery logic; both TOTP and WebAuthn step-up complete end-to-end against local Kratos.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — closeout residual, unowned per board grep: `.pm/w4/done/m11/README.md:47` "consider lifting aal2 detection out of the login page into the session fetch (the t005 altitude finding) — surface `session_aal2_required` from `fetchSession` and redirect with an explicit `aal=aal2` param, so the hook stops rediscovering the step-up need by trial-and-error."
- **Goal linkage:** auth robustness (docs/ADR012-auth.md) + dashboard quality (w5's charter); dashboard code, so it lands in w5 though the spec came from w4/m11.
- **Expected outcome:** MFA users stop paying a failed-request round-trip on every login; the step-up path becomes deterministic and debuggable.
- **Why now:** the refactor is already specced in the closeout; cheapest while the m11 session/auth context is recent.
- **Render parity:** omitted — internal dashboard auth flow; no REST/GraphQL/MCP change and no Render-comparable UI surface (Render's login is not part of the parity ledger).
