# Deprecated surface removal — 2026-07-27

This record is the breaking-change boundary for w1/m55. The cleanup follows Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), [official MCP server](https://github.com/render-oss/render-mcp-server), and [Blueprint YAML reference](https://render.com/docs/blueprint-spec). Render's MCP workspace change is visible in the upstream [request-scoped workspace commit](https://github.com/render-oss/render-mcp-server/commit/48a35785c99c); upstream temporarily retains session-selection tools for old clients, while bex removes them now after its own transition window.

## Caller migration

| Removed contract | Canonical replacement |
| --- | --- |
| MCP `select_workspace`, `get_selected_workspace`, and implicit transport-session selection | Call `list_workspaces`, confirm an id, then pass that `workspaceId` on every related resource-tool call. Omission uses the deterministic default workspace. |
| MCP `ownerId` arguments | MCP `workspaceId`. REST and GraphQL continue to use their Render-native `ownerId` spellings. |
| MCP `list_key_value_instances` | `list_key_value` |
| Public `/v1/apps` and subresources | `/v1/services` and the same subresources. The internal control-plane listener on port 8091 intentionally retains its private `/v1/apps` API. |
| Public `/v1/databases` | `/v1/postgres` |
| Public `/v1/registry-credentials` | `/v1/registrycredentials` |
| Public `/v1/webhooks/endpoints` and `.../deliveries` | `/v1/webhooks` and `/v1/webhooks/{id}/events` |
| REST webhook `eventTypes` and GraphQL webhook `eventFilter` | REST `eventFilter`. GraphQL and MCP retain their own `eventTypes` field. |
| Postgres `.../exports` | `.../export` |
| `GET /v1/postgres/{id}/recovery-info` | `POST /v1/postgres/{id}/recovery-info` |
| Blueprint root `apps` | `services` |
| Blueprint `tier`, `replicas`, `imagePath`, `publishPath` | `plan`, `numInstances`, `image: {url: ...}`, `staticPublishPath` |
| Blueprint bare-string `image` | `image: {url: ...}` |
| Blueprint `port` | Remove it and listen on the platform-provided `PORT`. |
| Dashboard `localStorage["bex.selectedWorkspaceId"]` | The existing `bex.selectedWorkspaceId` cookie, read during SSR and written when switching. No authentication material is browser-stored by this change. |
| Go `v1alpha1.GroupVersion` | `v1alpha1.SchemeGroupVersion` |
| `CF_ZONE_ID` in Cloudflare scripts | `CLOUDFLARE_ZONE_ID` |

Removed public REST routes fail closed with 404, retired REST payload fields fail strict validation, and retired Blueprint fields return a named bad-request error. The database migration `0050_drop_mcp_workspace_selections` removes the obsolete MCP selection table without changing the applied `0044` migration.

## Compatibility deliberately retained

These spellings are part of a current Render contract and must not be removed by a later generic “legacy” sweep:

- Blueprint `autoDeploy` remains accepted because Render documents it as deprecated but supported; `autoDeployTrigger` wins when both are present.
- Blueprint Key Value `type: redis` remains accepted because Render still documents it as a deprecated alias for `keyvalue`.
- The response-only service `env` field remains supported where Render still emits it; it is not the removed Blueprint/runtime alias set.
- REST and GraphQL `ownerId` remain their surface-native workspace field. Only MCP moved to `workspaceId`.
- The self-hosted `BEX_REGISTRY_PULL_SECRET` code path remains available when per-App registry credentials are disabled. Production no longer sets it because `BEX_REGISTRY_NS` enables per-App credentials.
- `BEX_REGISTRY_BUILD_PULL_SECRET` remains a distinct build-namespace credential for static-site extraction.
- MCP protocol transport sessions remain; only bex-owned workspace state was removed from them.

## Verification anchors

- `internal/api` tests assert the retired public REST paths return 404, webhook `eventTypes` is rejected, every workspace-scoped MCP schema has `workspaceId`, removed tools are absent, cross-workspace resource mismatches fail, and two replicas need no shared selection state.
- `internal/apps` tests assert canonical Blueprint input succeeds and each retired Blueprint spelling is rejected by name.
- Dashboard workspace-provider tests assert first-load fallback, cookie-backed restoration, deleted-workspace fallback, switching, hard reload, and zero localStorage access.
- The official Render CLI continues to use only the canonical REST route families recorded in `docs/cli-compatibility-checklist.md`.
