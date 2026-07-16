# w1 · m42 — Projects list: real cursor pagination

**Worker:** worker1 **Goal:** `GET /v1/projects` honors `cursor`/`limit` per Render's contract — today it emits per-item cursors it then ignores, so a compliant paging client loops on identical full pages forever — and the absent-`limit` convention for the remaining unpaginated lists is decided once, here. **Status:** done (2026-07-15)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide + document the absent-`limit` convention for the remaining unpaginated lists — **DONE** | 20m | —          |
| t002 | Implement `cursor`/`limit` in projects REST; check GraphQL/MCP project lists — **DONE**     | 45m | t001       |
| t003 | Re-verify the CLI `projects` row against a populated list; update checklist + ADR018 — **DONE** | 25m | t002       |
| t004 | Render parity — **DONE**                                                                   | 25m | t003       |
| t005 | Simplify — **DONE**                                                                        | 20m | t004       |
| t006 | Test coverage — **DONE**                                                                   | 30m | t004       |
| t007 | Closeout — **DONE**                                                                        | 15m | t006       |

## Definition of done

Paging `GET /v1/projects` by echoing the returned cursor terminates and yields every project exactly once (test-asserted); `limit` is honored with the documented default; the CLI checklist `projects` row carries evidence captured against a **populated** list; the absent-`limit` convention decision is written down where the w6/m32 and w7/m38 pagination milestones can inherit it.

## Closeout evidence

- Render's current official Projects reference confirms `limit` defaults to 20 and is bounded to 1–100. The repository's reduced pinned response-schema fixture does not contain the Projects path, so ADR006 cites the canonical reference rather than claiming evidence absent from the fixture.
- The REST walk test seeds 41 projects, verifies 20 + 20 + 1 exclusive pages, checks every id exactly once, and confirms the final cursor returns an empty tail.
- The unmodified official Render CLI v2.21.0 walked 101 populated projects through the bex projects handler in exactly two requests (100 + 1) and terminated.
- GraphQL and MCP share stable explicit project-id pages; their omitted-argument complete-list compatibility behavior is documented, tested, and avoids truncating existing dashboard/agent callers.
- After the integration pull landed w6/m32 concurrently, its env-group REST handler was aligned to this milestone's default-20 convention and given an omitted-limit regression test; its GraphQL/MCP compatibility defaults remain intentional extensions.
- `go build ./...` and `go test ./...` passed from `lego/backend/` on 2026-07-15.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — mechanical-consistency mining, verified in code: `lego/backend/internal/projects/rest.go:103-124` builds `renderProjectWithCursor` from the complete list with no `core.PageParams`/`core.Page` call; sibling `environments/rest.go:142-143` does paginate. The CLI checklist `projects` row (docs/cli-compatibility-checklist.md:53) was only ever verified against an empty list.
- **Goal linkage:** Render parity core (docs/ADR018-render-parity.md; docs/ADR032-environments.md layers on `internal/projects`) — emit-but-ignore cursors is a contract bug, not just a missing feature.
- **Expected outcome:** the one list endpoint that actively lies to paging clients is fixed; a single documented convention (recommend: the services convention — default limit 20, `StablePage` from commit 89c42936) exists for the remaining lists.
- **Why now:** any compliant Render client (including the official CLI) infinite-loops on a populated projects list today; t001's convention decision is a dependency of w6/m32 and w7/m38.
- **Render parity:** included — REST/GraphQL/MCP list surface change.
