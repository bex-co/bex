# w7 · m40 — Custom-domains list parity: GraphQL + MCP filters & pagination

**Status:** DONE 2026-07-16

## Problem

`GET /v1/services/{id}/custom-domains` gained `cursor`/`limit` pagination and
`verificationStatus`/`domainType` filters in w7/m38, but the other two adapters
were not updated: GraphQL `customDomains` still accepted only `id`, and MCP
`list_custom_domains` still accepted only `serviceId` — a one-day-old violation
of the three-adapter rule.

## What shipped

### `apps/graphql.go`

- `customDomains(id:…)` now accepts `cursor`, `limit`, `verificationStatus`
  (`pending|verified`), and `domainType` (`apex|subdomain`) args.
- Resolver filters and paginates with `core.StablePage` (cursor = domain name),
  identical semantics to REST. Unknown enum values return an error matching
  REST's 400 behavior.
- Full-list compatibility preserved: absent `cursor` + `limit` → all results.

### `apps/mcp.go`

- New `listCustomDomainsArgs` struct adds `cursor`, `limit`,
  `verificationStatus`, `domainType` to the existing `serviceId`.
- `list_custom_domains` switched from `serviceArgs` to `listCustomDomainsArgs`;
  handler applies the same filter + pagination logic.
- `domainListResult` gains `Cursor string` (the last item's domain name).
- `fmt` import added (needed for error formatting).
- Tool description updated to name the new args.

### `apps/domains_test.go`

- `TestGQLCustomDomainsFilters` — `verificationStatus`/`domainType` narrow the
  list; unknown enum returns a GraphQL error.
- `TestGQLCustomDomainsPagination` — cursor/limit walk returns all 5 items
  exactly once with no duplicates.
- `TestMCPListCustomDomainsFilters` — `verificationStatus`/`domainType` filter
  and `cursor` returned on each page.

### `docs/ADR018-render-parity.md`

- Custom domains row updated to document the GraphQL + MCP filter parity.
- Gap-backlog row added for w7/m40.
