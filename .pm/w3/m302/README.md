# w3 · m302 — Service events carry Render's per-type details: keys, types and ids, as on Render

**Worker:** worker3 **Goal:** every Render-defined event type's `details` matches its member in Render's pinned schema on REST and MCP **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Per-type schema check: each event's details must match the member its type names | 1h | — |
| t002 | Lifecycle-step events use Render's status keys and ids | 1h | t001 |
| t003 | Rename the keys Render spells differently | 1.5h | t001 |
| t004 | Fill details bex stores but sends empty | 1h | t001 |
| t005 | Retire the allowlist entry and update the ledger | 45m | t002, t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- The conformance suites check each event's `details` against the member its `type` names, and `rendertest.Allowlist` has no `details does not match the documented union` entry for `list-events` or `retrieve-event` (the bex-named `event/type` entry stays); `TestAllowlistOnlyShrinks` pins the lower size
- `build_ended`/`pre_deploy_ended` carry `buildStatus`/`preDeployStatus` and no string `status`; `instance_count_changed`, `server_failed`, `image_pull_failed`, `commit_ignored` and `autoscaling_config_changed` use Render's keys; `branch_deleted`, `disk_*`, `maintenance_mode_enabled` and `auto_deploy_disabled` are no longer empty
- Live after deploy: a read-only `GET /v1/events/{id}` in the canary workspace returns Render's keys for a `build_ended` and an `instance_count_changed` event, and MCP `get_service_event` returns the same
- GraphQL and the dashboard Events tab are unchanged

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-09/10, item 1 (filed by `/pmx them all to w3`); verified against the pinned `/events/{eventId}` details members and `lego/backend/internal/events/rest.go:92-123,258-265`; the w3/m272 allowlist entry "Service event types and details"
- **Goal linkage:** pillar 1, Render-compatible REST (ADR006); Render parity first (ADR008)
- **Expected outcome:** typed Render SDK decoders (e.g. `AsBuildEndedDetails()`) parse bex events, and webhook consumers reading `retrieve-event` stop losing fields
- **Why now:** the `status` type conflict breaks typed decoders today and the renamed keys lose data silently; every new event type grows the dialect, and external consumers will start depending on it now that signup is open
- **Recommendation in place of an open decision:** the proposal recommended replacing bex's keys on REST/MCP without dual-writing the old ones (Render parity first). Keeping the old keys alongside would be a divergence needing the user's sign-off in ADR018; none was given
- **Render parity:** included — REST and MCP response shapes change
