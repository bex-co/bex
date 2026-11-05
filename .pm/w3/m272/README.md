# w3 · m272 — Every REST response in the test suite is validated against the pinned Render schema

**Worker:** worker3 **Goal:** a Render-shape drift in any `/v1` response fails CI instead of being found by live QA **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Response-validating wrapper in the API test handler | 1.5h | — |
| t002 | Triage violations: fix each, or allowlist with an ADR018 row | 2h | t001 |
| t003 | Conformance fixtures across all service kinds | 1h | t001 |
| t004 | Hook internal/apps httptest suites into the same validator | 1h | t001 |
| t005 | Render parity | 30m | t002, t003, t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 40m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- Every `/v1` response produced by the backend API test suites that matches a Render operation is schema-validated
- Suites green; remaining divergences are in a shrink-only allowlist, each citing an ADR018 row
- A deliberately wrong field in a response fails CI (demonstrated)

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-05 (Render-parity-filtered round, item 3); code-verified at `158cb45d0`; w4 QA-loop response-shape findings 413, 420, 421, 470, 472, m242
- **Goal linkage:** Render parity first (ADR008); bex-api is Render-API-compatible (ADR006)
- **Expected outcome:** response-shape drift is caught in CI rather than by live QA one bug at a time
- **Why now:** the live QA loop keeps finding the same class of bug; a systemic check stops new ones landing while parity work (m270, m271, m274) adds routes
- **Render parity:** included — a fix-class change on the REST surface; the parity task checks fixed drifts against GraphQL/MCP twins
