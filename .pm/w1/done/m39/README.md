# w1 · m39 — Dependency security: clear the alert spike

**Worker:** worker1 **Goal:** The Dependabot spike introduced by the 2026-07-14/15 push wave (4 → 18 alerts initially, then 31 as the same `x/crypto` advisories attached to the operator manifest) is triaged and cleared: every critical/high either fixed and verified on GitHub or filed with a written reason. **Status:** done (2026-07-15)

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Triage all live alerts (31 at triage): real dependency vs vendored-test-fixture vs dev-only; record the classification — **DONE** | 45m | —          |
| t002 | Apply the safe batch (patch/minor bumps); all three test suites green — **DONE**                      | 45m | t001       |
| t003 | Breaking upgrades: majors required by any critical, one at a time, gated by the affected feature's tests — **DONE** | 60m | t002       |
| t004 | Residuals filed with reasons (the m30 pattern); confirm the GitHub alert count actually dropped — **DONE** | 20m | t003       |
| t005 | Simplify — `/simplify` over any upgrade shims/code changes — **DONE**                                  | 20m | t004       |
| t006 | Test coverage — regression tests where an upgrade changed behavior (else record explicitly none needed) — **DONE** | 20m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/` — **DONE**                                              | 10m | t006       |

## Definition of done

Zero unaddressed critical/high Dependabot alerts on the default branch: each one either resolved (verified on the repo's Dependabot page, count observed to drop) or recorded as a residual with a written reason and a revisit condition; `make test`, backend `go test ./...`, and dashboard `yarn test` all green after every bump.

## Closeout evidence

- Dependency commit `a2f1ae9ea8caacff2ff56124f48bf74177d52af6` is on the default branch.
- GitHub's live Dependabot API returned zero open alerts at `2026-07-16T01:31:02Z` (31 → 0; no dismissals or security residuals).
- Operator, backend, and dashboard validation passed; dashboard production client, SSR, and Nitro builds also passed.

## Source + Goal linkage

- **Source:** observed during the round-9 `/ship` (2026-07-15): the push banner went from 4 alerts (3 moderate, 1 low) to 18 (7 critical, 3 high) within the hour the SSH-gateway wave landed, then to 31 when GitHub attached the same `x/crypto` advisory set to the operator manifest. Triage found no vendored CLI manifest. Precedent: `w1/006` → `m23` → `m30` (triage → safe batch → residual watch).
- **Goal linkage:** GOAL.md #7 security posture; supply-chain hygiene (docs/ADR028-security-review.md follow-up register).
- **Expected outcome:** the alert page is green or explainable; no critical CVE sits silently on a multi-tenant platform's default branch.
- **Why now:** 14 criticals appeared this week from freshly-added dependency surface — the cheapest moment to fix is before anything builds on them.
- **Render parity closing task: omitted** — dependency maintenance; no REST/GraphQL/MCP/UI surface change.
