# w9 · m449 — The audit log uses Render's event name for every workspace action Render's audit log names

**Worker:** worker9 **Goal:** every workspace action that Render's audit-log reference names is recorded under Render's event name, and a guard test keeps that list complete **Status:** blocked (t001–t009 done; t010 waits on the production pin carrying the w9/m449 commit)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Member events carry Render's names — **DONE** | 45m | — |
| [t002](done/t002.md) | A cron job's create and delete record CreateCronJobEvent and DeleteCronJobEvent — **DONE** | 45m | — |
| [t003](done/t003.md) | Custom-domain, auto-deploy and build-source actions carry Render's names — **DONE** | 1h | — |
| [t004](done/t004.md) | An env var write records CreateEnvVarsEvent, UpdateEnvVarsEvent and DeleteEnvVarsEvent for what it changed — **DONE** | 1h | — |
| [t005](done/t005.md) | An environment's protection and isolation changes record Render's events — **DONE** | 45m | — |
| [t006](done/t006.md) | Coverage inventory, guard test and dashboard labels — **DONE** | 45m | t001, t002, t003, t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007 |
| [t010](t010.md) | Closeout | 15m | t008, t009 |

## Definition of done

- On production, as the QA user in a Pro workspace, each of these records Render's event name on REST `GET /v1/owners/{ownerId}/audit-logs`, with the same name in GraphQL `event` and the CSV export: invite, accept, change role, remove a member and a member leaving; create and delete a cron job; add and delete a custom domain; toggle auto-deploy; change the build filter; link and unlink an env group on a build source; link a service to a build source; toggle an environment's protection and its network isolation.
- One env var patch that adds one key and changes another records exactly one `CreateEnvVarsEvent` and one `UpdateEnvVarsEvent`, whose metadata names keys and never values; a save that changes nothing records no `success` row.
- `docs/render-artifacts/audit-logs-api.md` lists every workspace event on render.com/docs/audit-logs with bex's status, and a Go test fails when a listed event has neither a mapping nor a non-goal or owner entry.
- ADR018's Audit logs row no longer says env var writes keep `secrets.*` names.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w9` (2026-11-16), filed by `/pmx them all to w9`. Evidence:
  - [render.com/docs/audit-logs](https://render.com/docs/audit-logs) (read 2026-11-16) lists these workspace events: `InviteToTeamEvent`, `RemoveUserFromTeamEvent`, `AcceptTeamInviteEvent`, `ChangeTeamMemberRoleEvent`, `CreateCronJobEvent`, `DeleteCronJobEvent`, `UpdateAutoDeployEvent`, `UpdateAutoDeployTriggerEvent`, `CreateCustomDomainEvent`, `DeleteCustomDomainEvent`, `CreateEnvVarsEvent`, `UpdateEnvVarsEvent`, `ChangeEnvironmentProtectionEvent`, `UpdateEnvironmentIsolatedEvent`.
  - The pinned enum (`lego/backend/internal/api/openapi/render-public-api-1.json`, 87 values) carries all of them except the four auto-deploy/custom-domain names and `UpdateEnvironmentIsolatedEvent`, plus `LinkBuildSourceEnvGroupEvent`, `UnlinkBuildSourceEnvGroupEvent` and `UpdateServiceBuildSourceEvent`, whose verbs bex has (ADR097).
  - `renderEvents` (`lego/backend/internal/audit/service.go`) maps none of them: the verbs pass through as bex names (`members.Invite`, `apps.AddDomain`, `apps.SetAutoDeploy`, `environments.SetACL`, `secrets.PatchEnvironment`), and a cron job's create and delete record `CreateServerEvent`/`DeleteServerEvent`.
  - [ADR018](../../../../docs/ADR018-render-parity.md)'s Audit logs row keeps env var writes on `secrets.*` names ("not a 1:1 mapping") with no user sign-off.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first): every customer-facing behavior matches render.com, and a divergence stands only when ADR018 records it with the user's sign-off.
- **Expected outcome:** an admin's audit export reads like Render's, so SIEM rules and scripts written for Render's audit log match bex's rows, and the set of Render events bex records is checked by a test rather than remembered.
- **Why now:** events have been added one at a time (w4/m321, w5/183, and w21/105 shipped `DownloadDatabaseBackupEvent` on 2026-11-16), and each pass left others unmapped; one inventory plus a guard ends that. The env var divergence is unsigned, which the AGENTS.md parity rule does not let stand.
- **Decision state:** t004 and t005 split one write into Render's events. That is the `/pm-brainstorm` **recommendation**, the parity-sufficient choice the repository rules make the default (ADR008; parity work is never parked on an optional decision), not a user decision; keeping bex's names would need the user's sign-off recorded in ADR018. Render's metadata keys for these events are uncaptured (w10/blocked/002, no `RENDER_API_KEY`), so bex's existing metadata keys stay.
- **Render parity task:** included. REST, GraphQL, the CSV export and the dashboard labels change. MCP has no audit-log tool on Render or bex.

## Progress

- 2026-11-16: t001–t009 shipped (code, migration 0285, docs, dashboard). Every row the DoD names now carries Render's event in unit and real-Postgres tests; t010 checks it live once the production pin carries the commit. Unconfirmed matches (a branch change, a build source unlink, env group variable writes) are filed as [w9/100](../../100.md).
