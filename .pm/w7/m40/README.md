# w7 · m40 — Custom-domains list parity: GraphQL + MCP filters & pagination

**Worker:** worker7 **Goal:** The `cursor`/`limit` pagination and `verificationStatus`/`domainType` filters w7/m38 shipped on `GET /v1/services/{id}/custom-domains` exist with identical semantics on the GraphQL `customDomains` field and the MCP `list_custom_domains` tool — closing a one-day-old violation of the three-adapter rule. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | GraphQL `customDomains`: `cursor`/`limit` + `verificationStatus`/`domainType` args via `core.StablePage`   | 45m | —          |
| t002 | MCP `list_custom_domains`: same args + paged output, descriptions matched to REST vocabulary               | 45m | t001       |
| t003 | Render parity                                                                                                | 20m | t002       |
| t004 | Simplify                                                                                                     | 15m | t003       |
| t005 | Test coverage                                                                                                | 30m | t003       |
| t006 | Closeout                                                                                                     | 15m | t005       |

## Definition of done

The same filtered, paged custom-domain list is retrievable via REST, GraphQL, and MCP with identical filter semantics (`pending|verified`, `apex|subdomain`) and identical invalid-enum error behavior; GraphQL/MCP filter + pagination-walk tests mirroring the REST tests at `lego/backend/internal/apps/domains_test.go:1061-1206` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 19, 2026-07-15 — shipped-diff mine over `436fd9c2..bb9cc60b`, confirmed by two independent miners: REST half at `lego/backend/internal/apps/rest.go:1002-1044` (commit 7b3a1d40, w7/m38); GraphQL `customDomains` takes only `gqlutil.IDArg()` (`graphql.go:755-761`); MCP `list_custom_domains` takes only `serviceArgs` (`mcp.go:968-981`). The dashboard needs no change — Render's own UI is a flat settings list (miner-verified), so bex's fetch-all matches.
- **Goal linkage:** Render parity pillar + `internal/api/CLAUDE.md`'s three-adapter guarantee ("a change to one surface fans out to the other two"). Third round of the list-consistency pattern: w8/m13 closed it for datastores, w2/m43 owns services/environments — neither owns custom domains.
- **Expected outcome:** an MCP agent or GraphQL client can filter by verification status / domain type and page large domain lists exactly like a REST/CLI client.
- **Why now:** the drift is one day old — cheapest to close before clients wire against the asymmetry; sits in w7 as the direct continuation of w7/m38.
- **Render parity:** included — t001/t002 are GraphQL/MCP surface changes mirroring an existing Render REST behavior.
