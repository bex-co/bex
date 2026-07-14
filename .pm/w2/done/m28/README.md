# w2 · m28 — Typed error codes: general envelope + plan-limit as first consumer

**Worker:** worker2 **Goal:** The invite-dialog's plan-limit CTA renders based on a machine-readable error code, not a substring match on English prose; changing the backend's error message text has zero effect on whether the CTA shows; the CTA's copy is localizable (reads from structured params, not the raw backend string). Establishes a general, reusable error-code envelope (code + params) that every REST/GraphQL/MCP surface can key on, mirroring the one existing ad-hoc precedent (`RATE_LIMITED`) but as a shared mechanism instead of a one-off — a machine-readable error contract is exactly the kind of thing an agent client benefits from as much as the dashboard does. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: a general `core` error envelope — a `core.CodedError` (or similar) wrapping an existing sentinel (e.g. `ErrBadRequest`) with `{code string, params map[string]any}`; REST `WriteErr` and the GraphQL error formatter both read it, mirroring the existing ad-hoc `RATE_LIMITED` shape in `ratelimit.go` but as a reusable mechanism instead of a one-off | 45m | —          | — **DONE** |
| t002 | Implement `core.ErrPlanLimit` sentinel carrying `{code: "PLAN_LIMIT", plan, limit}`, per `w6/014`'s exact design                                                                                                | 30m | t001       | — **DONE** |
| t003 | Wire both `members` guards (seat-cap refusal, role-gate refusal) to return `ErrPlanLimit` instead of a plain error string                                                                                      | 40m | t002       | — **DONE** |
| t004 | REST `WriteErr` + GraphQL error formatter both surface the code+params consistently (mirroring the `RATE_LIMITED` envelope shape already proven in production)                                                | 40m | t001       | — **DONE** |
| t005 | Dashboard: `use-invite-member.ts` keys on the `PLAN_LIMIT` code instead of substring-matching `"plan"`; render localized copy from the params (fixes the `zh`-shows-English-string wart as a side effect)     | 45m | t003, t004 | — **DONE** |
| t006 | Regression tests: replace/extend `TestInvitePlanRefusalsNameThePlan` to assert the code+params shape (not just that the string contains "plan"); a dashboard test asserting the CTA renders off the code, not string-matching | 40m | t005       | — **DONE** |
| t007 | Live/functional verification: reword the backend's plan-limit message text and confirm the CTA still renders correctly (proving the fix actually decouples UI branching from prose)                          | 20m | t006       | — **DONE** |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                                                                                                                    | 15m | t007       | — **DONE** |
| t009 | Test coverage — final gap sweep (direct test for the general `CodedError` envelope, independent of the plan-limit case)                                                                                       | 15m | t007       | — **DONE** |
| t010 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                                                                                  | 10m | t008, t009 | — **DONE** |

## Definition of done

The invite-dialog's plan-limit CTA renders based on a machine-readable error code, not a substring match on English prose; changing the backend's error message text has zero effect on whether the CTA shows; the CTA's copy is localizable (reads from structured params, not the raw backend string).

## Source + Goal linkage

- **Source:** promotion of inbox `w6/014` (filed by `w6/m15`'s `/simplify` altitude pass, 2026-07-13). Materialized under `w2` per explicit user direction (`/pm-brainstorm` proposed it for `w6`, where the plan-limit case originates; the user redirected it to `w2` — noted here for provenance, not organic topical fit).
- **Goal linkage:** reliability/correctness — closes a fragility class `w6/m15` itself flagged as load-bearing (a copy edit to the plan-limit message today would silently break dashboard behavior, papered over only by a test pinning the exact English wording) and fixes a real i18n bug (non-English users see raw English error text) as a side effect. The general envelope this establishes is also a genuine `w2` (AI-native surface) asset: a machine-readable `{code, params}` error contract is exactly the kind of thing an agent client can branch on reliably, the same problem class the dashboard has today.
- **Expected outcome:** a reusable error-code envelope exists across REST/GraphQL (mirroring the one working precedent, `RATE_LIMITED`), with the plan-limit case as its first real consumer; the dashboard's CTA logic is decoupled from backend prose.
- **Why now:** `w6/m15` made the substring-matching load-bearing just this session; the only thing standing between "safe" and "silently regresses on the next copy edit" is a test asserting an exact English string never changes. The fix is small and the design is already worked out in the source note.
- **Render parity closing task: not expected** — this is bex's own internal error-envelope mechanism; no Render capability to compare against.
