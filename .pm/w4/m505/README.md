# w4 · m505 — A build-source link change and a build that cannot be fetched record Render's `artifact_source_changed` and `artifact_fetch_failed`

**Worker:** worker4 **Goal:** a service's build-source link changes record `artifact_source_changed {fromBuildSourceId, toBuildSourceId}` instead of bex's `source_changed`, and a linked service's deploy whose build cannot be pulled records `artifact_fetch_failed {buildId, message}`, on REST, GraphQL, MCP, webhooks and the dashboard. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record `artifact_source_changed` for every build-source link change | 60m | — |
| t002 | Record `artifact_fetch_failed` when a linked service cannot fetch its build | 50m | — |
| t003 | Serve both types on REST, GraphQL, MCP and webhooks | 40m | t001, t002 |
| t004 | Show both types in the dashboard's Events feed and webhook picker | 35m | t003 |
| t005 | Correct the docs that file artifact events as unproduced | 25m | t003 |
| t006 | Verify link changes and a fetch failure on dev-4 | 40m | t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

1. On dev-4, linking a web service to a build source, moving it to a second, and unlinking it each record exactly one `artifact_source_changed` with the right `fromBuildSourceId`/`toBuildSourceId` (absent side omitted) on REST list and retrieve, GraphQL and MCP, and none of them records `source_changed`; a branch change still records `source_changed`.
2. A deploy of a build-source build whose image cannot be pulled records one `artifact_fetch_failed` whose `buildId` equals the deploy's `buildId` and whose `message` is the pull failure, and no `image_pull_failed`.
3. A webhook subscribed to both receives each; both pass `TestRenderEventDetailsMatchTheirMember` with no missing fields.
4. `docs/render-artifacts/webhooks-api.md`, `docs/render-artifacts/service-events.md` and ADR018 describe both as produced, with the `image_pull_failed` choice recorded as unverified.
5. `cd lego/backend && go test ./...` and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w4` (the service events and outbound webhooks aspect), 2026-11-18, item 2; filed by `/pm them to w4`, 2026-11-19.
- **Render reference:** the pinned OpenAPI `retrieve-event` details members name build sources: Artifact Source Changed (`fromBuildSourceId`, `toBuildSourceId`) and Artifact Fetch Failed (`buildId`, `message`, "The build published by the build source that could not be fetched."). render.com/changelog, 2026-11-10: "Reuse a build across multiple services".
- **bex today:** Build Sources shipped in w2/m254 (2026-11-08). A link change writes the audit row `apps.LinkBuildSource` (`lego/backend/internal/core/audit.go:471-474`), but the service feed reports it as bex's own `source_changed` (`lego/backend/internal/events/service.go:455`), outside Render's enum, and a failed pull of a build reports `image_pull_failed`. `docs/render-artifacts/webhooks-api.md` lists both artifact types under "No truthful producer", and ADR018's round-3 drift note (2026-09-07) calls artifact sources "a Render feature bex does not target", both written before Build Sources existed.
- **Goal linkage:** ADR008 Render parity first; ADR018's Build Sources row (✅ on its operations, missing its events) and the Outbound event webhooks row.
- **Expected outcome:** a build-source link change reads as Render's event on every surface, and a broken build fetch is named as one.
- **Why now:** Build Sources is ten days old; adding its events now costs one producer per fact, before receivers learn bex's `source_changed`.
- **Recommendation, not a user decision:** `artifact_fetch_failed` replaces `image_pull_failed` for a linked service (unverified against Render, `w10/blocked/002`).
- **Render parity:** included: the change touches REST, GraphQL, MCP, webhooks and the dashboard.
