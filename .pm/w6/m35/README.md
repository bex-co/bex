# w6 · m35 — Env-groups list filters: implement or explicitly reject

**Worker:** worker6 **Goal:** every filter param Render documents on `GET /v1/env-groups` either works or returns an explicit 400 — none silently ignored. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin Render's env-groups list-filter semantics from the OpenAPI snapshot; decide implement-vs-explicit-reject per param | 30m | —          |
| t002 | Implement the accepted filters in `ListEnvGroups` + REST (repeated/comma `name`, repeated `ownerId`, `environmentId`, timestamps) | 45m | t001       |
| t003 | GraphQL/MCP list dialect decision + wiring or documented divergence (environments-list precedent)                 | 30m | t002       |
| t004 | Render parity — filter semantics/error shapes vs Render across all touched surfaces                               | 30m | t003       |
| t005 | Simplify — `/simplify` over the changed code                                                                       | 20m | t004       |
| t006 | Test coverage — each filter narrows the page; any rejected param 400s explicitly                                   | 40m | t004       |
| t007 | Closeout — verify DoD, sync status, move to done                                                                   | 15m | t006       |

## Definition of done

For every filter param Render's public OpenAPI documents on `GET /v1/env-groups` (`name` repeated/comma-separated, repeated `ownerId`, `environmentId`, `createdBefore`/`createdAfter`/`updatedBefore`/`updatedAfter`): passing it either narrows the result set correctly or returns an explicit 400 naming the unsupported param. Tests prove both behaviors; no param is silently ignored.

## Source + Goal linkage

- **Source:** promotes inbox note `w6/018` (filed by w6/m32's closeout), verified live in `/pm-brainstorm` round 18, 2026-07-15: `lego/backend/internal/envgroups/rest.go:51-57` reads only a single `ownerId` + paging — `name`, `environmentId`, and all four timestamp filters are silently ignored.
- **Goal linkage:** Render REST parity (`docs/ADR006-bex-api.md`) and the standing filter-contract invariant — nothing accepted may be silently ignored (the same class the projects/custom-domains/environments milestones eliminated).
- **Expected outcome:** Render clients filtering env-group lists get correct subsets or an honest error; the env-groups list matches the environments-list precedent shipped in w6/m33 (implemented filters + explicit 400 for unsupported ones).
- **Why now:** w6/m32 shipped the paging half and filed this as the known remainder; w6's queue is down to one open milestone (m34), making this its natural refill. Render parity task included: REST change, with a GraphQL/MCP dialect decision in scope.
