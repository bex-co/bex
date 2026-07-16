# Environment-group list filters

Captured 2026-07-15 from Render's official [List environment groups reference](https://api-docs.render.com/reference/list-env-groups) and public OpenAPI.

## Contract and bex decision

Render documents three array filters (`name`, `ownerId`, `environmentId`), four ISO-8601 timestamp boundaries, and the existing `cursor`/`limit` page controls. bex implements every documented filter because env-group metadata durably carries owner, Environment membership, `createdAt`, and `updatedAt`; no filter needs an unsupported 400.

| Parameter | Render type | bex behavior |
| --- | --- | --- |
| `name` | string array | Exact-name OR alternatives |
| `ownerId` | string array | Workspace OR alternatives; every named workspace passes the existing membership check |
| `environmentId` | string array | Exact Environment-membership OR alternatives |
| `createdBefore` / `createdAfter` | RFC3339 date-time | Strict before/after boundaries |
| `updatedBefore` / `updatedAfter` | RFC3339 date-time | Strict before/after boundaries |
| `cursor` / `limit` | string / integer 1–100 | Existing stable-id cursor paging, default 20 |

Array query parameters accept both comma-separated and repeated-key forms, trim whitespace, and deduplicate values. Alternatives within one parameter are ORed; different parameters compose with AND. Invalid timestamp input returns Render's `{id,message}` bad-request envelope and names the parameter. Filtering happens on non-secret metadata before group contents are loaded and before cursor pagination, so a selective query can still return a full page.

## Surface decision

The filters are REST-only. GraphQL `envGroups` and MCP `list_env_groups` are bex-native dialects and retain their existing single-workspace plus optional paging arguments, matching the Environments-list precedent. Their descriptions do not claim REST filter support.

## Regression evidence

`internal/envgroups/rest_test.go` covers repeated/comma array forms, each timestamp boundary, cross-parameter composition, named invalid-timestamp 400s, and filter-before-pagination. Shared query parsing lives in `internal/core/pagination.go` and remains covered by both env-group and Environment list tests.
