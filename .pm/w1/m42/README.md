# w1 · m42 — Projects list: real cursor pagination

**Worker:** worker1 **Goal:** `GET /v1/projects` honors `cursor`/`limit` per Render's contract — today it emits per-item cursors it then ignores, so a compliant paging client loops on identical full pages forever — and the absent-`limit` convention for the remaining unpaginated lists is decided once, here. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide + document the absent-`limit` convention for the remaining unpaginated lists       | 20m | —          |
| t002 | Implement `cursor`/`limit` in projects REST; check GraphQL/MCP project lists              | 45m | t001       |
| t003 | Re-verify the CLI `projects` row against a populated list; update checklist + ADR018      | 25m | t002       |
| t004 | Render parity                                                                              | 25m | t003       |
| t005 | Simplify                                                                                   | 20m | t004       |
| t006 | Test coverage                                                                              | 30m | t004       |
| t007 | Closeout                                                                                   | 15m | t006       |

## Definition of done

Paging `GET /v1/projects` by echoing the returned cursor terminates and yields every project exactly once (test-asserted); `limit` is honored with the documented default; the CLI checklist `projects` row carries evidence captured against a **populated** list; the absent-`limit` convention decision is written down where the w6/m32 and w7/m38 pagination milestones can inherit it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — mechanical-consistency mining, verified in code: `lego/backend/internal/projects/rest.go:103-124` builds `renderProjectWithCursor` from the complete list with no `core.PageParams`/`core.Page` call; sibling `environments/rest.go:142-143` does paginate. The CLI checklist `projects` row (docs/cli-compatibility-checklist.md:53) was only ever verified against an empty list.
- **Goal linkage:** Render parity core (docs/ADR018-render-parity.md; docs/ADR032-environments.md layers on `internal/projects`) — emit-but-ignore cursors is a contract bug, not just a missing feature.
- **Expected outcome:** the one list endpoint that actively lies to paging clients is fixed; a single documented convention (recommend: the services convention — default limit 20, `StablePage` from commit 89c42936) exists for the remaining lists.
- **Why now:** any compliant Render client (including the official CLI) infinite-loops on a populated projects list today; t001's convention decision is a dependency of w6/m32 and w7/m38.
- **Render parity:** included — REST/GraphQL/MCP list surface change.
