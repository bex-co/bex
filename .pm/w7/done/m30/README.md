# w7 · m30 — Render OpenAPI contract-conformance suite in CI

**Worker:** worker7 **Goal:** bex's actual REST responses are validated against a pinned copy of Render's OpenAPI spec in CI, with an explicit divergence allowlist — so new parity drift fails a build instead of waiting for the next manual audit. **Status:** DONE

## Tasks (in order)

| id   | title                                                      | est | depends_on      |
| ---- | ---------------------------------------------------------- | --- | --------------- |
| t001 | Pin Render's OpenAPI spec + choose a validator             | 40m | —               | — **DONE** |
| t002 | Conformance harness over the CI Postgres/OpenFGA setup     | 60m | t001            | — **DONE** |
| t003 | Per-family conformance cases (services/deploys/pg/kv/…)    | 60m | t002            | — **DONE** |
| t004 | Divergence allowlist keyed to ADR018 rows                  | 40m | t003            | — **DONE** |
| t005 | Wire into `backend-test.yml` + document the pin-update flow| 30m | t003            | — **DONE** |
| t006 | Simplify                                                   | 30m | t004, t005      | — **DONE** |
| t007 | Test coverage                                              | 30m | t004, t005      | — **DONE** |
| t008 | Closeout                                                   | 15m | t006, t007      | — **DONE** |

## Definition of done

`backend-test.yml` runs a conformance suite that validates bex's live responses for the core endpoint families (services, deploys, postgres, key-value, env vars/groups, custom domains, events) against the pinned Render OpenAPI response schemas; a deliberate divergence is silenced only via an allowlist entry pointing at its ADR018 row; introducing an unlisted response-shape divergence turns the job red locally and in CI. The pin-update workflow (bump the spec, re-triage) is documented.

## Delivered

- `lego/backend/internal/api/testdata/render-openapi.json` — pinned Render OpenAPI 3.0.3
  response-schema subset (11 paths, 14 schemas).
- `lego/backend/internal/api/conformance_schema_test.go` — ~150-line stdlib JSON Schema
  validator (no new deps); supports object/array/primitive/nullable/$ref/oneOf/anyOf.
- `lego/backend/internal/api/conformance_allowlist_test.go` — divergence allowlist with
  ADR018 citations; 2 active entries (postgres list, kv list flat-array divergences).
- `lego/backend/internal/api/conformance_test.go` — `TestRenderConformance` (11 subtests)
  + `TestConformanceAllowlistEntries` (negative guard: stale allowlist entries fail).
- `.github/workflows/backend-test.yml` — pin-update workflow documented inline.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 4, 2026-07-14; the parity ledger's manual "verified field-by-field vs Render's OpenAPI" method has no automated guard, and both sides move (bex ships hourly; Render's API evolves).
- **Goal linkage:** w7's CI-guard charter (gitleaks, trivy, structural guards, `w1/m28` test-gating) + the whole `docs/ADR018-render-parity.md` investment — mechanizing its central claim.
- **Expected outcome:** parity becomes a standing CI guarantee, not a periodic audit; new divergences are caught at PR time with a pointer to decide keep-or-fix.
- **Why now:** the gap-well is dry — the remaining risk is drift, not absence, so the highest-leverage parity work is now a guard rather than a feature. w7 owns CI guards and has capacity.
- **Render parity closing task OMITTED:** the milestone *is* the parity check, mechanized — there is no product surface to re-verify against Render (the suite does that continuously). Pure test/CI infra.
