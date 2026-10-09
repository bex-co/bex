# w8 · m54 — Accept one-sided CLI build filters

**Worker:** worker8 **Goal:** the unchanged pinned CLI accepts include-only and ignore-only repo-backed service configuration **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add operation-scoped compatibility for null filter lists | 45m | — |
| t002 | Verify replacement, response arrays and shared callers | 20m | t001 |
| t003 | Verify actual CLI create/update and runtime | 20m | t002 |
| t004 | Render parity across filter surfaces | 15m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 20m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Actual Bex and unmodified same-pin Render against Bex each exit 0 on include-only and ignore-only creates/updates. Fresh equivalent owned free fixtures serve a harmless marker with HTTP 200, then are deleted and absent from detail/list/runtime.
- The unused list reads as []; the requested list is exact. One-sided updates replace the whole filter; omitted flags preserve it; both-list and all-empty behavior stays intact.
- Compatibility is scoped to create-service/update-service. Embedded artifact and shared response schemas stay non-null; missing required fields and malformed lists/items retain named 400s. Request bytes are preserved.
- Meaningful local coverage checks five repo-backed service kinds and existing GraphQL/MCP/UI/Blueprint/matcher behavior. Preserve rootDir composition and image-backed refusals. Distinguish this coverage from live web evidence.
- Meaningful regressions, applicable full backend checks and lint pass; statuses/evidence match the actual result.

## Source + Goal linkage

- **Source:** [CLI QA finding](finding.md), w8 sweep 23, 2026-10-09 UTC; Bex v0.3.2, pinned Render v2.27.0, exact-source gate diagnostics and empty-array controls.
- **Goal linkage:** ADR008 reliable hosting and ADR018/ADR006 unchanged CLI/API compatibility. w1/m34 already supplies filter domain behavior.
- **Expected outcome:** ordinary one-sided filter configuration works without requiring an unrelated second filter.
- **Why now:** the shared upstream builder breaks create and update. Scoped schema cloning, consumer checks and actual CLI acceptance exceed one hour: 7 tasks, 2h20m.
- **Parity:** included for user-facing REST input. Document serializer/OpenAPI drift; authenticated Render-production behavior remains unverified. Existing GraphQL/MCP/UI semantics must remain consistent.

Scheduled only; no product fix or ship was performed by this hunt.
