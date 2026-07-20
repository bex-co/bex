# w1 · m53 — Security-review remediation: close the 2026-07-19 audit findings

**Worker:** worker1 **Goal:** close the concrete, verified problems found by the 2026-07-19 security review — a fail-open internal API, a tenant-triggerable SSRF, an incomplete authz-guard safety net, an invite-redemption trust assumption, and a batch of defense-in-depth gaps — so bex's multi-tenant boundary holds under the newer surface (SSH/web-shell, static sites, control plane, GitHub app) added since ADR028. **Status:** done

## Tasks (in order)

| id   | title                                                                  | est  | depends_on   | status        |
| ---- | ---------------------------------------------------------------------- | ---- | ------------ | ------------- |
| t001 | M3: fail closed on the internal control-plane API (`:8091`)            | 1h   | —            | — **DONE**    |
| t002 | M1: close the admission-webhook SSRF (boundary + dial guard + `Image`) | 1h30 | —            | — **DONE**    |
| t003 | M2: authz "every verb guarded" sweep covers every wired service        | 45m  | —            | — **DONE**    |
| t004 | V1: verify + document Kratos email-verification for invite redemption  | 45m  | —            | — **DONE**    |
| t005 | LOW backend batch (L1, L2, L6, L9, L10)                                | 1h30 | —            | — **DONE**    |
| t006 | LOW operator/infra batch (L3, L4, L5)                                  | 1h30 | —            | — **DONE**    |
| t007 | LOW SSH/ticket batch (L7, L8)                                          | 1h   | —            | — **DONE**    |
| t008 | Render parity: `Image` validation + deploy-hook across REST/GraphQL/MCP | 30m  | [t002, t005] | — **DONE**    |
| t009 | Simplify the code this milestone changed                               | 30m  | [t008]       | — **DONE**    |
| t010 | Test coverage for the shipped fixes                                    | 1h   | [t008]       | — **DONE**    |
| t011 | Closeout                                                               | 15m  | [t010]       | — **DONE**    |

**Deferred (recorded with rationale, per DoD):** L6 invite-token hashing (`w1/041`), L4 namespace fail-closed admission + L7 shared-nonce + L8 ticket-off-URL (`w1/042`), and `BEX_REQUIRE_VERIFIED_INVITE_EMAIL` flip after the verification UX ships (`w1/040`).

## Definition of done

- Internal CP API (`:8091`) refuses to serve unauthenticated: `BEX_CP_TOKEN` empty + CP listener enabled is a fatal startup error (or the listener binds loopback / the NetworkPolicy is port-scoped to `:8090`); `BEX_CP_TOKEN` is present in the deployment manifest and mirrored (name only) in `.env.example` + `.env.template`.
- The admission webhook matches the registry with a `/` boundary, the operator's shared HTTP client dials through `netutil.SafeDialContext`, and `Image` is rejected at the API boundary (+ CRD `pattern`/`maxLength`) — a tenant-supplied `image: zot.bex-registry.svc:5000.attacker.tld/x` no longer causes an operator-side fetch to an attacker host (test-proven).
- A test fails when `sweepableServices` diverges from `Server.features()`; the 5 omitted services (keyvalue, usage, events, notifications, webhooks) are in the sweep and `TestAuthzGuardsEveryVerb` is green over all of them.
- The Kratos email-verification dependency for login-time invite redemption is confirmed (schema enforces verification / `whoami` returns only verified addresses) and documented in `docs/ADR024-members.md`; if it is NOT enforced, the gap is escalated and a hardening fix filed.
- The LOW items are either fixed or, where a fix is deliberately deferred, recorded with rationale. No behavior change ships without a test asserting the new failure mode.

## Source + Goal linkage

- **Source:** 2026-07-19 security review (six parallel focused audits: authz/multi-tenancy, SSH/web-shell + shell tickets, injection/path/SSRF, secrets/credentials, operator privilege/admission, bex-api internal surface). Findings verified in-code before filing. Extends `docs/ADR028-security-review.md` (the 2026-07-11 audit), which predates the SSH gateway, browser web shell, static sites, cron jobs, members/invites, device flow, and GitHub App.
- **Goal linkage:** ADR008 vision — a trustworthy multi-tenant PaaS; ADR022 tenant isolation; ADR012 auth; ADR035 SSH. Keeps the tenant/control-plane boundary sound as new surface lands.
- **Expected outcome:** the internal API can't ship auth-disabled; the operator can't be steered into SSRF by tenant image input; the CI authz safety net covers 100% of wired services; the invite-redemption trust assumption is proven or fixed; the defense-in-depth backlog from the review is burned down.
- **Why now:** M3 is a fail-open on a privilege-granting API that is unauthenticated in prod today (bounded only by NetworkPolicy); M1 is tenant-triggerable when image signing is enabled; M2 is the safety net the repo's own conventions rely on. These are cheap, high-confidence fixes whose absence is a standing risk.
- **Render parity included:** yes (narrow) — t002 adds `Image` validation to the create surface and t005 touches deploy-hook `imgURL` behavior; both must stay consistent across REST/GraphQL/MCP and match Render's error shapes.
