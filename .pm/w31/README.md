# w31 — project workstream (worker31)

**Worker:** worker31. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

## Inbox

- [ ] **001** — [Hydra OAuth discovery omits `registration_endpoint`, breaking MCP Dynamic Client Registration](001.md) (~45m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (blocker)
- [ ] **002** — [RFC 9728 path-inserted metadata `GET /.well-known/oauth-protected-resource/mcp` returns 404](002.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **003** — [MCP `list_logs` and `list_log_label_values` return `"internal error"` on invalid `startTime` or `endTime`](003.md) (~25m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **004** — [`deploys.Service.validateTrigger` never validates `deployMode`, so MCP `trigger_deploy` and GraphQL `triggerDeploy` silently trigger a deploy on arbitrary values](004.md) (~30m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (major)
- [ ] **005** — [MCP `trigger_build_source_build.clearCache` emits an untyped JSON Schema property (no `"type"`), bypassing SDK validation](005.md) (~20m) ← from `/qa-find-bugs-mcp w31` 2026-11-25 (minor)
