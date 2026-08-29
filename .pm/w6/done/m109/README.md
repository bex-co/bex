# w6 · m109 — Postgres/KeyValue REST+MCP omit Render-required `ipAllowList`/`readReplicas` when empty

**Worker:** worker6 **Goal:** `GET`/list/create/update responses for Postgres (REST `/v1/postgres`, MCP `get_postgres`/`list_postgres_instances`/etc.) and Key Value (REST `/v1/key-value`, MCP `get_key_value`/etc.) always include `ipAllowList` (and, for Postgres, `readReplicas`) as `[]` when there are no entries — matching Render's own pinned OpenAPI schema, which declares both `required` — instead of omitting the key entirely, which is what happens today for the overwhelmingly common case (no CIDR restriction, no read replicas). GraphQL already gets this right; REST and MCP must match it. **Status:** done — closed 2026-08-29: live `api.bex.co` re-probe passed on fresh fixtures (REST + MCP empty-case key presence, non-empty round-trip control); see done/t007.md for the recorded evidence

## Background (found live, 2026-08-26, 20th `/qa-find-bugs` hunt)

This run dug into whether `w6/m106`'s finding (service `ipAllowList` at the wrong REST nesting level) had siblings in other resource types. It doesn't — Postgres/KeyValue/Redis's own Render schemas place `ipAllowList` at the root of the resource object (no `*Details` sub-object the way `service` has), and bex's REST responses already nest it correctly there. But the same investigation surfaced a different, real wire-shape bug on the same field.

**Ground truth, not guessed.** Render's pinned OpenAPI spec (`lego/backend/internal/api/openapi/render-public-api-1.json`, sha256-pinned in `lego/backend/internal/api/render_openapi.go:39`) lists `ipAllowList` in `required` for all three of `postgres`, `keyValue`, and `redis`, and additionally lists `readReplicas` in `required` for `postgres`:

```
postgres.required  = [..., "ipAllowList", ..., "readReplicas", ...]
keyValue.required  = [..., "ipAllowList", ...]
redis.required     = [..., "ipAllowList", ...]
```

**Live probe 1 — Postgres, a real production database with no CIDR allowlist and no read replicas (`dpg-d9nqg95cavls73fp8m20`, pre-existing resource, read-only probe):**

```
REST:    GET https://api.bex.co/v1/postgres/dpg-d9nqg95cavls73fp8m20  (200)
=> body has NO "ipAllowList" key and NO "readReplicas" key at all (confirmed via `'ipAllowList' in body` / `'readReplicas' in body` both false; `createdAt`/`updatedAt`/`owner`/`region` — all present on this real resource — confirmed true, isolating the omission to the two empty-array fields, not a general marshaling failure).

GraphQL: POST https://api.bex.co/graphql
         { database(id:"dpg-d9nqg95cavls73fp8m20") { id ipAllowListEntries { cidrBlock description } readReplicas { name } } }
=> {"data":{"database":{"id":"dpg-...","ipAllowListEntries":[],"readReplicas":[]}}}  — correct, present, empty.

MCP:     tools/call get_postgres { postgresId: "dpg-d9nqg95cavls73fp8m20" }
=> structuredContent has neither "ipAllowList" nor "readReplicas" — same omission as REST.
```

**Live probe 2 — Key Value, a real production instance with no CIDR allowlist (`red-d9p49kdrtmes73c34ovg`, pre-existing resource, read-only probe):**

```
REST:    GET https://api.bex.co/v1/key-value/red-d9p49kdrtmes73c34ovg  (200)
=> body has NO "ipAllowList" key.

GraphQL: { keyValue(id:"red-d9p49kdrtmes73c34ovg") { id ipAllowListEntries { cidrBlock description } } }
=> {"data":{"keyValue":{"id":"red-...","ipAllowListEntries":[]}}}  — correct.

MCP:     tools/call get_key_value { keyValueId: "red-d9p49kdrtmes73c34ovg" }
=> structuredContent has no "ipAllowList" key — same omission as REST.
```

All probes run from inside the authenticated browser session (`page.evaluate` + `fetch(..., {credentials:'include'})`), not a bare script, per this hunt's own rule about `api.bex.co`'s Cloudflare bot protection.

**Root cause.** Both resources' response structs declare the required array fields with Go's `omitempty`, which drops the JSON key entirely when the slice is nil/empty (Go's `encoding/json` zero-value rule) — the exact opposite of what `omitempty` should do to a field Render's schema marks `required`:

- `lego/backend/internal/postgres/service.go:107` — `ReadReplicas []ReadReplicaView \`json:"readReplicas,omitempty"\``on`PostgresView`.
- `lego/backend/internal/postgres/service.go:124` — `IPAllowList []core.IPAllowListEntry \`json:"ipAllowList,omitempty"\``on`PostgresView`.
- `lego/backend/internal/keyvalue/service.go:95` — `IPAllowList []core.IPAllowListEntry \`json:"ipAllowList,omitempty"\``on`KeyValueView`.
- `lego/backend/internal/keyvalue/rest.go:66` — `IPAllowList []core.IPAllowListEntry \`json:"ipAllowList,omitempty"\``on the REST-only`renderKeyValue`wrapper, fed straight from`KeyValueView.IPAllowList` (`rest.go:89`) so it inherits the same nil instead of re-deriving anything.

`PostgresView` is embedded directly into REST's response wrapper (`renderPostgres`, `postgres/rest.go:38-41`) — so every Postgres REST route (create, get, list, PATCH) is covered by fixing this one struct — and is also returned directly by 8 MCP tool handlers (`postgres/mcp.go`: `get_postgres` L106, `list_postgres_instances` L95 via `listPostgresResult{Postgres []PostgresView}`, `create_postgres` L117, `suspend_postgres` L215, the two verbs at L222/L229, `recover_postgres` L262, `update_postgres` L348 — exact grep: `grep -n "CallToolResult, PostgresView" postgres/mcp.go` → 7 hits + the list wrapper). `KeyValueView` is returned directly by 5 MCP handlers (`keyvalue/mcp.go`: `get_key_value` L115, `list_key_values` L104 via `listKeyValueResult{KeyValues []KeyValueView}`, `create_key_value` L126, `suspend_key_value` L150, `update_key_value` L158); `renderKeyValue` is built separately for every KeyValue REST route via `toRenderKeyValue` (`keyvalue/rest.go:73-94`).

**GraphQL is unaffected — confirmed by trace, not assumed.** `postgres/graphql.go`/`keyvalue/graphql.go`'s `ipAllowList`/`ipAllowListEntries`/`readReplicas` fields are typed GraphQL list fields resolved straight off the same underlying Go slices; graphql-go serializes a nil Go slice bound to a `GraphQLList` type as `[]`, not as an omitted key or `null` — there is no `encoding/json` struct-tag layer in that path at all, so `omitempty` never applies to GraphQL's output. This is why GraphQL alone gets it right today.

**The correct fix pattern already ships in this exact codebase, for this exact field, on a sibling resource.** `lego/backend/internal/environments/rest.go:72` declares `IPAllowList []core.IPAllowListEntry \`json:"ipAllowList"\``— **no**`omitempty`— and`toRenderEnvironment` (`rest.go:80-84`) explicitly nil-coalesces before assigning: `allowList := e.IPAllowList; if allowList == nil { allowList = []core.IPAllowListEntry{} }`. The same two-line pattern (drop `omitempty`, nil-coalesce at construction) is the fix for all three structs here.

**Exhaustive check of every other `required` field on these two schemas — not estimated, checked field-by-field against the actual struct tags:**

| required field | struct tag | live status |
| --- | --- | --- |
| `ipAllowList` (postgres/keyValue/redis) | `omitempty` | **BUG** — confirmed omitted live (both resource types) |
| `readReplicas` (postgres) | `omitempty` | **BUG** — confirmed omitted live |
| `createdAt`, `updatedAt`, `dashboardUrl`, `region` | `omitempty` | same mechanism, but scalar/string and always non-empty on any real provisioned resource — live-verified present (`true`) on the same probed resource that was missing `readReplicas`/`ipAllowList`, so this does not manifest in practice; not part of this fix |
| `owner` (postgres, via REST's `renderPostgres.Owner *renderOwner \`json:"owner,omitempty"\`\`) | `omitempty` (pointer) | same mechanism, but every real Database has a resolvable owner — live-verified present; not part of this fix |
| `connectionPool`, `diskAutoscalingEnabled`, `highAvailabilityEnabled`, `options` | no `omitempty` | correct today, unaffected |
| `role`, `suspenders` (postgres) | _(no Go field exists at all)_ | **not this bug** — a distinct, deeper gap (the field is never implemented, not merely omitted when empty); not investigated this run, flagged as Unverified below, explicitly out of scope for this milestone |

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fix `PostgresView.IPAllowList`/`ReadReplicas`: drop `omitempty`, nil-coalesce to `[]T{}` at response construction, matching `environments/rest.go`'s pattern — **DONE** | 30m | — |
| t002 | Fix `KeyValueView.IPAllowList` and REST's separate `renderKeyValue.IPAllowList`: same drop-omitempty + nil-coalesce treatment in both structs — **DONE** | 30m | — |
| t003 | Regression tests: REST GET/list/create/PATCH and MCP `get_postgres`/`get_key_value` on a fixture with zero entries assert the key is present as `[]`, not absent; non-empty case still round-trips correctly (no regression) — **DONE** | 45m | t001, t002 |
| t004 | Render parity — confirm GraphQL is unaffected (still `[]`) and REST/MCP now agree with it; correct any doc claim about this shape — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 20m | t004 |
| t008 | Connection-info responses omit Render-required `externalConnectionString` when the datastore is not public — **DONE** | 30m | — |
| t007 | Closeout — **DONE** | 10m | t005, t006, t008 |

## Definition of done

- `GET /v1/postgres/{id}` for a real Postgres instance with no CIDR allowlist and no read replicas returns `"ipAllowList":[]` and `"readReplicas":[]` in the body — not an absent key. Re-run this hunt's exact live probe (`fetch('https://api.bex.co/v1/postgres/<id>', {credentials:'include'})` from an authenticated dashboard tab, `'ipAllowList' in body` and `'readReplicas' in body` both `true`) against a real instance and confirm.
- `GET /v1/key-value/{id}` for a real instance with no CIDR allowlist returns `"ipAllowList":[]`, not an absent key — same live-probe shape against `api.bex.co`.
- `tools/call get_postgres` and `tools/call get_key_value` (MCP) return the identical `[]` shape for the same fixtures — not an absent key.
- GraphQL's `ipAllowListEntries`/`readReplicas` fields are unchanged (`[]` today, `[]` after) — the fix only adds presence to REST/MCP, it does not change any surface's values.
- A Postgres with real read replicas, and a Postgres/KeyValue with a real saved CIDR entry, still round-trip the non-empty array correctly through REST, GraphQL, and MCP alike (regression control case) — `cd lego/backend && go test ./internal/postgres/... ./internal/keyvalue/...` green, including the new empty-case assertions from t003.
- `go test ./...` (backend) and `make lint` (all four modules) stay green.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 20th run, 2026-08-26. Evidence is the live REST/GraphQL/MCP request-response triples pasted verbatim above, captured against real pre-existing production resources (`dpg-d9nqg95cavls73fp8m20`, `red-d9p49kdrtmes73c34ovg`) via read-only `GET`/query/tool calls — no resource was created, modified, or deleted to find this bug. Cross-checked against Render's own pinned OpenAPI spec's `required` arrays (`lego/backend/internal/api/openapi/render-public-api-1.json`), not assumed from behavior alone.
- **Goal linkage:** [ADR006](../../../docs/ADR006-bex-api.md) (bex-api Render-compatibility) and [ADR018](../../../docs/ADR018-render-parity.md) (parity ledger) — a strict or generated Render-API client (the `render-oss/cli` train this project already pins compatibility to, a Terraform provider, or any OpenAPI-conformance test) can legitimately treat a missing `required` property as a parse failure or a validation error; today's REST/MCP responses fail that check on the single most common state (no allowlist configured, no read replicas) for two of bex's three datastore resource types.
- **Expected outcome:** any Render-compatible REST or MCP client sees the same "no restriction, no replicas" signal GraphQL and the dashboard already show correctly, with zero change to enforcement, to GraphQL, or to the dashboard UI (which is GraphQL-only for these fields — confirmed via `grep -rn "v1/postgres\|v1/key-value" dashboard/src`, no direct REST call sites found).
- **Why now:** cheap, precisely diagnosed to file:line, with a proven two-line fix pattern already shipped once in this exact codebase for this exact field on a sibling resource (`environments/rest.go`) — low risk, mechanical, and closes a real (if narrow) Render-parity gap on the two datastore resource types this hunt had not yet checked for this bug class.
- **Render parity task included:** yes (t004) — this milestone exists specifically to close a Render-parity gap on a `required`-field contract; t004 re-confirms GraphQL stays correct and REST/MCP now match it.
