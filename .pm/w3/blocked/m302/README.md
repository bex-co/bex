# w3 · m302 — Service events carry Render's per-type details: keys, types and ids, as on Render

**Worker:** worker3 **Goal:** every Render-defined event type's `details` matches its member in Render's pinned schema on REST and MCP **Status:** blocked — t001–t008 done; t009 Closeout waits on the production deploy and the live check

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Per-type schema check: each event's details must match the member its type names — **DONE** | 1h | — |
| t002 | Lifecycle-step events use Render's status keys and ids — **DONE** | 1h | t001 |
| t003 | Rename the keys Render spells differently — **DONE** | 1.5h | t001 |
| t004 | Fill details bex stores but sends empty — **DONE** | 1h | t001 |
| t005 | Retire the allowlist entry and update the ledger — **DONE** | 45m | t002, t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
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

## Scope change (2026-11-10, loopx w3)

- **t004:** the disk, maintenance-mode and auto-deploy-reason details are not recorded on the audit rows, contrary to the brainstorm's assumption, so they stay omitted (never invented) and the write-side work is `w3/061`. The DoD's "no longer empty" bullet therefore holds for `branch_deleted` and `job_run_ended`, and for the other three types it is recorded as owed in ADR018's events row and pinned by `TestRenderEventDetailsMatchTheirMember`.

## Blocked (2026-11-10, loopx w3)

- **Gate:** the live DoD bullet needs bex-api built from this milestone's commit (`feat(events): … (w3/m302)`).
- **Release condition:** a `chore(deploy): pin platform images to <sha>` on main whose sha contains the commit (`git merge-base --is-ancestor`). Then, with the QA account and read-only, find a `build_ended` and an `instance_count_changed` event in `bex-canary` (`GET /v1/services/{id}/events?type=…`; scale a throwaway starter service 1 → 2 if none exists, then delete it with a 404 proof), read each through `GET /v1/events/{id}` and MCP `get_service_event`, and confirm `buildStatus` (no string `status`) and `fromInstances`/`toInstances`. Then run t009.
- No user decision needed.

## Post-release probe (QA loop round 139, 2026-11-11, partial)

The pin `f609163ada8e` (`6ee0197c7`) contains `f38f75686` (`git merge-base --is-ancestor` true). One half of the release condition was checked read-only on a QA-owned fixture: Free Git web service `qa-20261111-r139-bad` (`srv-dbqdpfgfss2c73f203bg`), whose first build failed on a missing Dockerfile. It was deleted afterwards (DELETE 204, then GET 404).

- **`build_ended` passes on REST.**
  - `GET /v1/services/srv-dbqdpfgfss2c73f203bg/events?type=build_ended` returned `evt-71up5uh9hp574cn5u7pp`.
  - `GET /v1/events/evt-71up5uh9hp574cn5u7pp` answered 200 with the same body: `details: {"deployId":"dep-dbqdpfgfss2c73f203c0","buildStatus":"failed","reason":{}}`.
  - It carries `buildStatus` and no string `status`. `buildId` is omitted, as t002 records.
- **Not run this round:**
  - MCP `get_service_event`, which this browser session can't call;
  - the `instance_count_changed` half, which needs a paid starter service scaled 1 → 2.
  - The milestone stays blocked on those two.
