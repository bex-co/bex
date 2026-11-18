# w9 · m449 — The audit log uses Render's event name for every workspace action Render's audit log names

**Worker:** worker9 **Goal:** every workspace action that Render's audit-log reference names is recorded under Render's event name, and a guard test keeps that list complete **Status:** done (2026-11-18: every DoD action observed on production; see § Production replay)

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
| [t011](done/t011.md) | A build source delete records one DeleteBuildSourceEvent and no DeleteServerEvent — **DONE** | 30m | t008, t009 |
| [t010](done/t010.md) | Closeout — **DONE** | 15m | t008, t009, t011 |

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

- 2026-11-16: t001–t009 shipped (code, migration 0285, docs, dashboard). Every row the DoD names now carries Render's event in unit and real-Postgres tests; t010 checks it live once the production pin carries the commit. Unconfirmed matches (a branch change, a build source unlink, env group variable writes) are filed as [w9/100](../../blocked/100.md).
- 2026-11-18 (`/loopx w9`, `/pm unblock w9/m449`): **the release gate cleared.** The production pin `359aee37d` (`chore(deploy): pin platform images to 14eaa71b98e5`, deploy.yml run 44213541822, success) carries `093b0696c` (`git merge-base --is-ancestor 093b0696c 14eaa71b98e5`). t010's replay ran and every bullet but the member rows holds (below). It found one defect, now fixed as t011. The milestone stays parked on two remaining gates.

## Production replay (2026-11-18, t010)

As the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale plan, which includes Pro's audit logs), with fixtures named `m449-11180803-*`. Each action was read back on REST `GET /v1/owners/tea-daif693dqjvc73e7as3g/audit-logs?startTime=2026-11-18T08:03:24Z`, on GraphQL `auditLogs(ownerId, startTime)` `action`, and in the dashboard's Audit Log → Export as CSV for 2026-11-18. GraphQL agreed with REST on all 24 rows by id, and the CSV `event` column carried the same 24 names.

| DoD action | Request | Row (all `success`, actor the QA user) |
| --- | --- | --- |
| Invite a member | `POST /v1/workspaces/{id}/members` (`m449-11180803@example.com`, developer) | `InviteToTeamEvent`, `metadata.invite` `inv-dbulnlo7ud6s739sdn7g`, `roleTo` developer |
| Create and delete a cron job | `POST /v1/services` (`cron_job`, busybox image), `DELETE` | `CreateCronJobEvent`, `DeleteCronJobEvent` (`srv-dbulnn07ud6s739sdn9g`). The web service create beside it stayed `CreateServerEvent` |
| Add and delete a custom domain | `POST`/`DELETE …/custom-domains` | `CreateCustomDomainEvent`, `DeleteCustomDomainEvent` (`metadata.domain` `m449-11180803.example.com`) |
| Toggle auto-deploy | `PATCH` `autoDeploy` `no`, then `yes` | `UpdateAutoDeployEvent` ×2 |
| Change the build filter | `PATCH` `buildFilter` | `UpdateAutoDeployTriggerEvent` |
| Link and unlink an env group on a build source | `POST`/`DELETE /v1/build-sources/{bds}/env-groups/{evg}` | `LinkBuildSourceEnvGroupEvent`, `UnlinkBuildSourceEnvGroupEvent` (`envGroupId`, `envGroupName`, `service` `bds-dbulo7g7ud6s739sdnng`) |
| Link a service to a build source | `PATCH /v1/services/{srv}` `buildSourceId` | `UpdateServiceBuildSourceEvent` |
| Toggle an environment's protection and isolation | REST `PATCH /v1/environments/{evm}`, one field at a time, on then off | `ChangeEnvironmentProtectionEvent` ×2, `UpdateEnvironmentIsolatedEvent` ×2. Rows are per field: no IP list row, since the list did not change |
| The same, on the dashboard's mutation | GraphQL `setEnvironmentACL` changing both fields, then the same save again | The first save wrote one row each for protection, isolation and the IP list: `setEnvironmentACL` is full-replace (ADR032), so omitting `ipAllowListEntries` replaced the default allow-all pair. The repeat save wrote nothing |
| Env var patch adding one key and changing another | REST `PUT /v1/services/{srv}/env-vars` with `M449_K1` new and `M449_K2` changed | exactly one `CreateEnvVarsEvent` (`envVarKeys` `M449_K1`) and one `UpdateEnvVarsEvent` (`envVarKeys` `M449_K2`); no value in any row, REST, GraphQL or CSV |
| A save that changes nothing | the same `PUT` again (08:04:27Z), and GraphQL `setEnvVars` with the same values (08:04:38Z) | no row at all |

- **Members, observed later the same run.** At first these rows looked unreachable: the QA user is the only identity a worker can sign in as, and the production kubeconfig fetch failed on a stale `~/.ssh/known_hosts` entry. w3/m472 (`db8200471`, landed during this run) made the fetch pin the stable control-plane host key, and it then succeeded. Through the Kratos admin API (`kratos-admin.auth`), the worker created a throwaway identity, `m449-qa2-11180822@example.com`, with its email marked verified. The QA user invited it as developer, and its first sign-in redeemed the invite (production redeems verified-email invites at sign-in). The QA user changed its role to viewer and removed it. It was invited again, accepted again within the 30 s tenant cache, and left with `DELETE /v1/workspaces/{id}/members/me`. REST and GraphQL `action` agreed on every row: `InviteToTeamEvent` (`inv-dbum0nceoepc73dlkt8g`, `roleTo` developer), `AcceptTeamInviteEvent` (actor the invitee), `ChangeTeamMemberRoleEvent` (`roleFrom` developer, `roleTo` viewer), `RemoveUserFromTeamEvent` (actor the QA user), `InviteToTeamEvent` (`inv-dbum0u4eoepc73dlkta0`), `AcceptTeamInviteEvent`, and `RemoveUserFromTeamEvent` (actor the invitee, the leave). The export's own query (`startTime`/`endTime` for 2026-11-18, `limit=1000`, which the dashboard's `auditLogsToCSV` copies into its `event` column) returned the same seven names. The dashboard export itself was not rerun for these rows, because the Playwright browser crashed. The identity then deleted its own account (`deleteAccount`), which removed its personal workspace and its Kratos identity (admin `GET` 404). Its rows now name it `Deleted User ID#usr-dbum0qseoepc73dlkt9g`, with `targetName` `deleted:own-…` (ADR086).
- **Found and fixed (t011).** Deleting the build source wrote `DeleteBuildSourceEvent` and also a `DeleteServerEvent` naming `bds-dbulo7g7ud6s739sdnng` as a service. A delete refused with `BUILD_SOURCE_LINKED` would also have written a success row. The fix is in `DeleteBuildSource` (`6fb748320`), with a regression test. Production shows it once a pin carries that commit; the DoD does not list this action.
- **Cleanup.** The invite was revoked (`members.RevokeInvite`). The web service, build source, env group, environment and project were deleted, each `GET` answers 404, and their delete rows read `DeleteServerEvent`, `DeleteBuildSourceEvent`, `DeleteEnvGroupEvent`, `DeleteEnvironmentEvent` and `DeleteProjectEvent`. The downloaded CSV was removed. `bex-canary` again holds only `hello-go` and the earlier `qa-20261025-b5p1-grp`.

## Decision record (2026-11-18, `/pm unblock w9/m449`)

- **Affected:** t010 (Closeout), t011 (new, done).
- **Source:** verified evidence (the pin check and the replay above). No user decision was needed or taken.
- **Chosen:** the release gate is cleared. The DoD stays as written: the member rows must be observed on production, not waived. The build-source defect the replay found is fixed inside this milestone (t011), since t002 introduced it.
- **Remaining gates:**
  1. A second sign-in identity, from the user. Either (a) the login of a second verified bex account that is not a member of `bex-canary`, added to `.env` as `QA2_EMAIL`/`QA2_PASSWORD` with the names in `.env.example`, or (b) the Gmail connector connected for `puncsky@gmail.com`, so a worker can register `puncsky+m449@gmail.com` and read its verification code and invite token. Re-pinning the control-plane host keys (`BEX_SSH_KNOWN_HOSTS` from the node console, `.env.example` § SSH) is a third route through the Kratos admin API.
  2. The production pin carrying t011's commit. It clears by itself: check with `git merge-base --is-ancestor <t011 commit> <pinned sha>`.
- **Next steps for the worker:** sign in as the second identity. As the QA user, invite it to `bex-canary` and read `InviteToTeamEvent`. Accept as the second identity, by token or at sign-in with the verified email, and read `AcceptTeamInviteEvent`. Change its role and read `ChangeTeamMemberRoleEvent`. Remove it and read `RemoveUserFromTeamEvent` (actor: the QA user). Invite and accept again, leave as the second identity (`DELETE /v1/workspaces/{id}/members/me`) and read `RemoveUserFromTeamEvent` (actor: the second identity). Check REST, GraphQL `action` and the CSV for each row. Then delete a fresh build source and read exactly one `DeleteBuildSourceEvent` with its name and no `DeleteServerEvent`. Then close out.

## Closure (2026-11-18)

Gate 1 cleared without the user, through the route the decision record listed third: production cluster access (w3/m472) and a verified throwaway identity created through the Kratos admin API. Gate 2 was this worker's own recheck of t011 and is not a DoD item. All four DoD bullets hold:

1. Every listed action records Render's name on production REST, GraphQL and the export (§ Production replay).
2. The env var patch records exactly one `CreateEnvVarsEvent` and one `UpdateEnvVarsEvent`, keys only, and a no-op save records nothing.
3. `docs/render-artifacts/audit-logs-api.md` § Workspace event inventory and `TestRenderWorkspaceEventsAreRecorded` exist and pass.
4. ADR018's Audit logs row no longer says env var writes keep `secrets.*` names. That sentence had survived t006/t007 and was rewritten at closeout to describe the shipped mappings.
