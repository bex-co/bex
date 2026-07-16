# w6 · m36 — Read-path correctness chores: legacy-timestamp filter + registry-cred batch lookup

**Worker:** worker6 **Goal:** two verified read-path defects land as one chores round (the w1/m30 grouped pattern): env groups with no persisted `createdAt`/`updatedAt` stop silently vanishing from time-filtered lists, and `restServices` stops paying one registry-credential DB query per bound service on a page it already batch-resolves owners for. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | `matchesTimeWindow`: legacy groups with missing/unparseable timestamps stop being silently dropped | 30m | —            |
| t002 | Batch registry-credential name resolution in `restServices` (one `IN (…)` lookup per page)         | 30m | —            |
| t003 | Simplify — `/simplify` over the milestone's diff                                                   | 20m | t001, t002   |
| t004 | Test coverage — meaningful tests for the shipped behavior                                          | 30m | t001, t002   |
| t005 | Closeout — move to `done/` when the DoD holds                                                      | 15m | t004         |

## Definition of done

A pre-metadata env group appears in `createdBefore/After`/`updatedBefore/After`-filtered lists (or its exclusion is an explicit, documented, tested decision — not a parse-error fallthrough); a page of N credential-bound services costs one credential-name query, not N; both locked by tests.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 21, 2026-07-15 — shipped-diff mine over `4627caaf..65a979b6` finding 4 (`envgroups/service.go:215-224`: `matchesTimeWindow` returns `false` on any `time.Parse` error, so legacy groups with empty timestamps are silently excluded under any time filter — the accepted-param-silent-drop class this codebase keeps paying to remove) + `w6/019` (filed round 17, re-verified at HEAD today: `apps/render.go:355-360` calls `RegistryCredentialName` — one `GetRegistryCredential` query — per item inside the loop that batch-resolves owners one line above).
- **Goal linkage:** API correctness/efficiency on Render-shaped list surfaces (`docs/ADR006-bex-api.md`); the silent-drop class violates the board's standing "never silently ignored" rule.
- **Expected outcome:** two latent read-path defects closed before they're user-visible; the env-group filter semantics for legacy rows becomes an explicit decision.
- **Why now:** finding (a) shipped yesterday (w6/m35) — cheapest to fix while the filter code is warm; w6's queue is empty. Grouped per the sizing rule (each sub-hour alone).
- **Render parity:** omitted — no wire-shape change (the filters are REST-only by m35's documented dialect decision; the credential summary shape is untouched). Noted here per the canon.
