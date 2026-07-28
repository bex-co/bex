# Render `owners` (workspaces) REST + MCP — pinned from the OpenAPI spec

Captured 2026-07-09 from Render's primary sources (the generated OpenAPI behind `api-docs.render.com`, served as Markdown at `…/reference/*.md`, plus the official MCP server source). This is the contract bex-api's `w6/m2` owners read API + MCP workspace tools mirror — the same "one Core, Render-consistent adapters" rule the service/postgres/logs verbs already follow. The REST surface is **read-only** (research finding 9: Render exposes no POST/PATCH/DELETE on `/v1/owners`); workspace lifecycle mutations live in the dashboard GraphQL (`w6/m1`).

> **Vocabulary.** Render uses three words for one entity: **workspace** (UI), **team** (dashboard GraphQL: `owner.team`, id prefix `tea-`), and **owner** (the REST resource parent — a `user` or a `team`). The `Workspaces` tag note in the spec is verbatim: _"This category was previously called `Owners`, as reflected by endpoint paths."_ bex's `workspace` model type _is_ that entity.

## Sources

- **REST OpenAPI** — `https://api-docs.render.com/reference/list-owners.md`, `…/retrieve-owner.md`, `…/retrieve-owner-members.md`, `…/pagination.md` (each embeds the full OpenAPI 3.0.2 fragment for its endpoint; server `https://api.render.com/v1`, auth `BearerAuth`).
- **Endpoint index** — `https://api-docs.render.com/llms.txt`.
- **MCP** — `github.com/render-oss/render-mcp-server` `pkg/owner/tools.go` + `pkg/session/{session,inmemory}.go` (selection persistence); docs `https://render.com/docs/mcp-server`.

## `owner` object (the resource shape)

Verbatim from `retrieve-owner.md` / `list-owners.md` → `components.schemas.owner`:

```json
{
  "owner": {
    "type": "object",
    "required": ["id", "name", "email", "type"],
    "properties": {
      "id": { "type": "string" },
      "name": { "type": "string" },
      "email": { "type": "string" },
      "ipAllowList": {
        "type": "array",
        "items": { "$ref": "#/components/schemas/cidrBlockAndDescription" }
      },
      "twoFactorAuthEnabled": {
        "type": "boolean",
        "description": "Whether two-factor authentication is enabled for the owner. Only present if `type` is `user`."
      },
      "type": { "type": "string", "enum": ["user", "team"] }
    }
  },
  "cidrBlockAndDescription": {
    "type": "object",
    "required": ["cidrBlock", "description"],
    "properties": {
      "cidrBlock": { "type": "string" },
      "description": { "type": "string" }
    }
  }
}
```

So: `id`, `name`, `email`, `type` are required; `ipAllowList` and `twoFactorAuthEnabled` are optional (`twoFactorAuthEnabled` only when `type == "user"`).

## `GET /v1/owners` — List workspaces

_"List the workspaces that your API key has access to, optionally filtered by name or owner email address."_

Query parameters:

| param | in | type | notes |
| --- | --- | --- | --- |
| `name` | query | array of string | "Only return workspaces with one of the provided names. Only exact matches are returned." |
| `email` | query | array of string | "Only return workspaces owned by one of the provided email addresses." |
| `cursor` | query | string | pagination position (`pagination.md`) |
| `limit` | query | integer | default 20, min 1, max 100 |

Response `200` — a JSON **array** of `ownerWithCursor` (NOT a `{owners:[…]}` envelope; the cursor is a **sibling** of the resource object, not a member — confirmed verbatim in `pagination.md`):

```json
[
  {
    "owner": { "id": "tea-…", "name": "…", "email": "…", "type": "team" },
    "cursor": "cfQ74cE2sDI="
  },
  {
    "owner": { "id": "tea-…", "name": "…", "email": "…", "type": "team" },
    "cursor": "mpFjFKeYgnw="
  }
]
```

```json
{
  "ownerWithCursor": {
    "type": "object",
    "properties": {
      "owner": { "$ref": "#/components/schemas/owner" },
      "cursor": { "$ref": "#/components/schemas/cursor" }
    }
  },
  "cursor": { "type": "string" }
}
```

Pagination contract (`pagination.md`): omit `cursor` on the first request; each returned item pairs with its cursor; to fetch the next page, set `cursor` to the _last_ item's cursor from the prior response. Repeat until a shorter page is returned.

## `GET /v1/owners/{ownerId}` — Retrieve workspace

Path param `ownerId` (string, required) — _"The ID of the user or team."_ Verbatim spec note (the load-bearing sentence for the `own-` prefix):

> _"Workspace IDs start with `tea-`. If you provide a user ID (starts with `own-`), this endpoint returns the user's default workspace."_

Response `200` → a single `owner` object (no envelope). Errors: `401`, `404`, `406`, `410`, `429`, `500`, `503`.

## `GET /v1/owners/{ownerId}/members` — List workspace members

_"Retrieves the list of users belonging to the workspace with the provided ID."_ Path param `ownerId` — _"The ID of the team."_

Response `200` → a plain JSON **array** of `teamMember` (no cursor — members are not cursor-paginated, unlike `/v1/owners` and `/v1/services`):

```json
{
  "teamMember": {
    "type": "object",
    "required": ["userId", "name", "email", "status", "role", "mfaEnabled"],
    "properties": {
      "userId": { "type": "string" },
      "name": { "type": "string" },
      "email": { "type": "string" },
      "status": { "type": "string", "enum": ["active", "inactive"] },
      "role": { "$ref": "#/components/schemas/teamMemberRole" },
      "mfaEnabled": { "type": "boolean" }
    }
  },
  "teamMembers": {
    "type": "array",
    "items": { "$ref": "#/components/schemas/teamMember" }
  },
  "teamMemberRole": {
    "type": "string",
    "description": "The member's workspace role. Values are always returned in uppercase.",
    "enum": [
      "ADMIN",
      "DEVELOPER",
      "WORKSPACE_CONTRIBUTOR",
      "WORKSPACE_BILLING",
      "WORKSPACE_VIEWER"
    ],
    "example": "DEVELOPER"
  }
}
```

The role enum is **UPPERCASE** (verified live against the dashboard in [`team-members.graphql`](team-members.graphql): `"ADMIN"`). bex maps its lowercase OpenFGA relations (`admin`/`developer`/`contributor`/`viewer`/`billing`, [`deploy/gitops/authz/model.fga`](../../deploy/gitops/authz/model.fga)) to this uppercase wire enum at the adapter boundary.

## `error` object

```json
{
  "error": {
    "type": "object",
    "properties": {
      "id": { "type": "string" },
      "message": { "type": "string" }
    }
  }
}
```

Used by every error response (`401Unauthorized`, `404NotFound`, `406NotAcceptable`, `410Gone`, `429RateLimit`, `500InternalServerError`, `503ServiceUnavailable`). **Shipped behavior:** `w9/m38` unified `core.WriteErr`/`WriteErrStatus` (`lego/backend/internal/core/http.go:37-90`) to emit Render's `{error, id, message}` envelope on every non-2xx path while keeping the `error` key for bex-only callers; live-verified via the official CLI's bad-key 401 (`docs/cli-compatibility-checklist.md` line 74). The earlier `{"error": "…"}` divergence is gone.

## `own-` vs `usr-` — resolved

**The user-ID prefix is `own-`, not `usr-`.** Primary source: the `retrieve-owner.md` spec note quoted above ("a user ID (starts with `own-`)"). The `usr-…` mention in [`team-members.graphql`](team-members.graphql) line 8 is a stale comment from an older dashboard capture (2026-07-06) and is corrected here. bex's workspace ids are `tea-` (already, via `internal/id`); user/identity ids are `own-` at the API boundary.

## MCP workspace tools — current contract

Render's [request-scoped workspace change](https://github.com/render-oss/render-mcp-server/commit/48a35785c99c) adds optional `workspaceId` to every resource tool so a confirmed target survives reconnects and transport-session changes. The upstream repository temporarily retains session selection for existing clients, but marks it deprecated.

bex completed that transition in w1/m55:

- `list_workspaces` is the only workspace discovery tool and returns `{workspaces: […]}` without selecting one.
- Every workspace-resource tool exposes optional `workspaceId`. The API middleware strips it before feature-specific schema decoding, binds it to the request context, validates membership, and verifies that any resource id belongs to that workspace.
- `select_workspace` and `get_selected_workspace` are absent. There is no in-memory or Postgres selection backend; migration `0050_drop_mcp_workspace_selections` removes the old table.
- Omitted `workspaceId` resolves the caller's deterministic default workspace. Unknown, inaccessible, or resource-mismatched ids fail closed.
- MCP transport sessions continue to exist for the protocol, but carry no bex-owned workspace state. Two HTTP replicas therefore need no sticky routing or shared selection store.

The older session-selection capture is historical evidence only; it is superseded by [the removal record](deprecated-surface-removal-2026-07-27.md).

## Parity check (t006)

Implemented against the current contract (`lego/backend/internal/workspaces/{render,rest,mcp}.go`): field names, the bare-array list/members envelopes, the `own-` retrieve quirk, the uppercase `teamMemberRole` enum, and MCP `list_workspaces` discovery match. No POST/PATCH/DELETE exists under `/v1/owners` (`rest.go` registers `GET` only). **Cursor pagination** (`cursor`/`limit`, default 20, max 100) is honored on `GET /v1/owners` via the shared `core.Page`/`core.PageParams` helper (also applied to `GET /v1/services` for a consistent list surface). The `ownerId` filter on `/v1/postgres` is **no longer** a no-op — Database CRs carry `core.LabelTenant` (`w6/m4`).

**Member `userId` is an opaque `own-` id (`w6/m7`), not the raw Kratos subject.** bex now mints a stable `own-<xid>` per identity (`id.Owner`, persisted subject⇄own-id in the `owner_ids` table) and reports it as the members `userId`. Two adjacent Render behaviors are **deliberate non-goals**, source-documented so they don't get silently re-attempted:

- **Owner `type` is always `"team"`.** bex mints only `tea-` team workspaces — it has no personal-account/user-owner entity — so retrieving by an `own-` id returns the caller's default **team**, whose `type` is `team`. `type: "user"` would only appear if bex modeled personal workspaces.
- **`own-` retrieve resolves the caller only.** `GET /v1/owners/{own-id}` returns the **caller's** default workspace; resolving a _peer's_ `own-` id is not a real Render workflow (a key only reaches its own workspaces) and has no consumer — see [`w6/001.md`](../../.pm/w6/001.md).

(The `internal/members` dashboard surface — w4/m12's GraphQL/MCP team management — deliberately keeps a `subject`-named field and takes `subject` as mutation input; it is a bex-native contract, not the Render `userId` surface, so it is intentionally left subject-keyed.)
