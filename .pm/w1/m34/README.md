# w1 · m34 — Build filters: `buildFilter.paths` + `ignoredPaths`

**Worker:** worker1 **Goal:** Render's `buildFilter` (glob `paths`/`ignoredPaths` deciding whether a push triggers an auto-deploy) exists end-to-end: CRD field, webhook glob matching, settable/readable on REST/GraphQL/MCP + dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                      | est | depends_on |
| ---- | ---------------------------------------------------------- | --- | ---------- |
| t001 | CRD: `spec.buildFilter{paths[],ignoredPaths[]}` + codegen  | 30m | —          |
| t002 | Webhook: glob matching in the push path-filter             | 45m | t001       |
| t003 | REST: create + `PATCH` + read-back (Render shape)          | 40m | t001       |
| t004 | GraphQL `setBuildFilter` + create arg; MCP tool + args     | 40m | t003       |
| t005 | Dashboard: Build & Deploy rows for paths/ignoredPaths      | 45m | t004       |
| t006 | Render parity                                              | 30m | t002, t005 |
| t007 | Simplify                                                   | 30m | t006       |
| t008 | Test coverage                                              | 45m | t006       |
| t009 | Closeout                                                   | 15m | t008       |

## Definition of done

A push touching only `ignoredPaths`-matched files does not open a deploy; a push matching `paths` does (empty `paths` = everything, Render's semantics). `buildFilter` is settable at create and via update, and readable back, on REST, GraphQL, MCP, and the dashboard Build & Deploy section — field shape verified against Render's OpenAPI.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 3); `docs/ADR018-render-parity.md` "Remaining `PATCH` field `buildFilter` is not editable — ◐, low" (understated: zero hits in `lego/` — the field is entirely unbuilt).
- **Goal linkage:** GOAL #3 (git push to deploy) precision + Render parity; the general form of w1/m18's rootDir webhook path-scoping.
- **Expected outcome:** monorepo users stop getting spurious deploys from unrelated paths; a named ◐ ledger row closes across all four surfaces.
- **Why now:** w1 is down to m33; this is the natural sibling of m18's `rootDirMatches` machinery while it's still fresh. Render parity task included — all-surface change.
