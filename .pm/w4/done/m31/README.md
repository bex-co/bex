# w4 · m31 — Auth-gate hardening: revocation epoch + device-route rate limiting

**Worker:** worker4 **Goal:** A CLI logout no longer stalls every in-flight cache-miss introspection process-wide, and the three credential-less device-flow routes stop being unmetered Hydra amplification — the two design-laden findings w4/m27's simplify pass deferred to `w4/023`, both re-verified live in code on 2026-07-15. **Status:** DONE 2026-07-16

## Tasks (in order)

| id   | title                                                                                                     | est  | depends_on | status |
| ---- | ---------------------------------------------------------------------------------------------------------- | ---- | ---------- | --- |
| t001 | Atomic revocation epoch: lock-free Hydra RTT, conditional cache `Put`, invariant-preserving               | 2h   | —          | DONE |
| t002 | IP-keyed token bucket on the three public device-flow routes, OAuth-dialect 429s                          | 1.5h | —          | DONE |
| t003 | Render parity                                                                                               | 20m  | t001, t002 | DONE (CLI ceremony not live-verified — no cluster access) |
| t004 | Simplify                                                                                                    | 15m  | t003       | DONE |
| t005 | Test coverage                                                                                               | 45m  | t003       | DONE |
| t006 | Closeout                                                                                                    | 15m  | t005       | DONE |

## Definition of done

A concurrency test demonstrates that an `invalidate` arriving during in-flight upstream introspections neither blocks unrelated new cache-miss auth nor lets an older in-flight positive result resurrect a revoked entry; anonymous floods of `POST /v1/device-grant|device-token|token/refresh/` are shed with OAuth-shaped 429s before reaching Hydra, while the official Render CLI's full login ceremony (including its legitimate `/v1/device-token` polling cadence) completes unthrottled against a rate-limited build.

## Source + Goal linkage

- **Source:** promoted from `.pm/w4/023.md`, filed by `w4/m27/t007`'s simplify pass 2026-07-15; both findings re-verified live at promotion time (round-19 lesson): `introspectUpstream` holds `revocationMu.RLock()` across the entire Hydra HTTP round trip (`lego/backend/internal/api/auth.go:227-240`), and the three device routes mount outside the rate limiter with no metering of their own (`lego/backend/internal/cliauth/service.go:79-81`, `internal/api/CLAUDE.md` always-public inventory). Declined-with-reasons items from the note (Identity.Human derivability, RevokeHandler collapse, script cosmetics, NewOryClient consolidation) stay declined — recorded in `w4/done/m27/done/t007.md`.
- **Goal linkage:** multi-tenant security/robustness (w4's charter, docs/ADR012-auth.md) on the hottest path in the binary — every authenticated request crosses this gate.
- **Expected outcome:** logout ceases to be a caller-triggerable process-wide auth stall (lock hold time drops from network-RTT to a map write); the public device routes get abuse bounds without breaking the official CLI.
- **Why now:** w4/m27 just made the CLI login flow production-real, which makes both the poller traffic and the logout-triggered stall reachable by any user at will.
- **Render parity:** included, narrowly — no wire-shape change intended; the check is that the official CLI's device flow still completes against the rate-limited routes and any 429 body stays OAuth-dialect (`writeOAuthError`), never Render-REST-dialect (these are OAuth protocol routes, the one deliberate dialect exception — `w9/done/m39`).
