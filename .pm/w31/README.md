# w31 — project workstream (worker31)

**Worker:** worker31. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

## Inbox

- [ ] **001** — [Hydra OAuth discovery omits `registration_endpoint`, breaking MCP Dynamic Client Registration](001.md) (~45m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (blocker)
- [ ] **002** — [RFC 9728 path-inserted metadata `GET /.well-known/oauth-protected-resource/mcp` returns 404](002.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **003** — [MCP `list_logs` and `list_log_label_values` return `"internal error"` on invalid `startTime` or `endTime`](003.md) (~25m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **004** — [`deploys.Service.validateTrigger` never validates `deployMode`, so MCP `trigger_deploy` and GraphQL `triggerDeploy` silently trigger a deploy on arbitrary values](004.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **005** — [MCP `trigger_build_source_build.clearCache` emits an untyped JSON Schema property (no `"type"`), bypassing SDK validation](005.md) (~20m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (minor)
- [ ] **006** — [MCP `get_metrics` / `get_datastore_metrics` (and GraphQL `metrics`) bypass `minResolutionSeconds = 30`, overflow `time.Duration`, and reject float `resolution`](006.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **007** — [HTTP latency `quantile=0` silently returns p95 on REST/MCP/GraphQL, and multi-quantile `quantiles` silently drops `0` and `1`](007.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **008** — [Blueprint plan (`update_blueprint` / `plan_blueprint` / `POST /v1/blueprints/{id}/syncs`) returns `no_changes` when `render.yaml` drops env vars, leaving them permanently locked as `managedBy: blueprint`](008.md) (~45m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **009** — [`deploys.FilterOf` never validates `status`, so MCP `list_deploys` and GraphQL `deploys` silently return empty results on unknown statuses](009.md) (~25m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **010** — [Scoped API keys (and narrow OAuth tokens) return `api_key_permission_denied` ("create a key with All") on unknown MCP tools and GraphQL root fields instead of `-32602 unknown tool` / `"Cannot query field"`](010.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **011** — [`events.FilterOf` never validates `eventTypes`/`type` and silently drops malformed `startTime`/`endTime`, so MCP `list_events` / `list_service_events` and GraphQL `serviceEvents` diverge from REST `400 bad_request`](011.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **012** — [MCP `list_logs` (and GraphQL `logs`) silently accept out-of-range `limit` (`0`, `-5`, `101`, `9999`), diverging from Render's pinned MCP contract (`1..100`) and REST `GET /v1/logs`](012.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **013** — [MCP `create_key_value` and GraphQL `createKeyValue` ignore non-empty `ipAllowList` / `ipAllowListEntries` when `public` is omitted, leaving `public: false` and `ipAllowList: []` while REST `POST /v1/key-value` enables external access](013.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **014** — [MCP `clone_env_group` declares an unreachable `ownerId` field (stripped and rejected by `mcp_workspace.go`) and cannot clone an environment group into another workspace](014.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **015** — [Cloud-agent MCP list/wait tools silently accept non-positive `limit` and `timeoutSeconds: 0`, diverging from REST `400` and GraphQL errors](015.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **016** — [Cron-job run listing disagrees three ways on non-positive `limit`: REST silently returns 1 item, MCP returns 10, and GraphQL returns 20](016.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **017** — [Build-source-linked services publish bogus `"branch": "main"` on MCP, REST, and GraphQL while write verbs refuse `"branch": "main"`](017.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **018** — [`CreateBuildSource` bypasses `image.ownerId` validation when `workspaceId` is omitted on MCP, and `UpdateBuildSource` never validates `image.ownerId` on MCP, REST, or GraphQL](018.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **019** — [GraphQL `service`, `server`, `services`, and `linkBuildSource` omit `linkedBuildView`, returning `runtime: ""` for build-source-linked services while MCP, REST, and GraphQL `createService` return the build source's runtime](019.md) (~35m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
