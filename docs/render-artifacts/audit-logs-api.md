# Render audit-logs API — captured response schema

Captured 2026-07-15 (w4/m26) from Render's current [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), corroborated by the unmodified official [`render-oss/cli`](https://github.com/render-oss/cli) at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68` (generated client types in `pkg/client/types_gen.go` match the spec; the CLI ships **no** user-facing audit-log command). When bex's audit surface was built (w4/m10, 2026-06 era) this schema was not resolvable from public docs — api-docs.render.com's reference page then documented only the request parameters. The spec has since been extended; this capture closes ADR018's last evidence-blocked ◐.

A live capture against a real Render account was not performed in this session (no `RENDER_API_KEY` credential surfaced in the environment; the w5/m32 real-account workflow can add one later) — the OpenAPI schema below is normative (required fields + closed enums) and two independent artifacts agree on it.

## Endpoints

- `GET /v1/owners/{ownerId}/audit-logs` — operation `list-owner-audit-logs` (bex's surface; workspace-scoped)
- `GET /v1/organizations/{orgId}/audit-logs` — operation `list-organization-audit-logs`: the org's log, its Owners' (w7/m464, [ADR115](../ADR115-organizations.md) D7), with the same parameters and rows

Query params: `startTime`/`endTime` (RFC3339), `direction` (shared `$ref` with `/logs`: enum `forward|backward`, default `backward` = most recent first), `cursor`, and an **audit-specific** `limit` (`auditLogLimitParam`: default 20, **maximum 1000** — not the general pagination cap of 100).

## Response: array of `auditLogWithCursor`

```json
[{ "cursor": "…", "auditLog": { … } }]
```

Both members required. `auditLog` (all six fields **required**):

| field | type | notes |
| --- | --- | --- |
| `id` | string | example `aud-123456789` |
| `timestamp` | string (date-time) | ISO 8601 |
| `event` | string, closed enum | PascalCase `…Event` names, e.g. `CreateServerEvent`, `SuspendServiceEvent`, `UpdateIPAllowListEvent` — 75 values in the capture, including org/team/SSO/SCIM events bex has no counterpart for |
| `status` | string, enum `success\|error` | binary |
| `actor` | object | `{type (required, enum user\|rest_api\|system), email?, id?}` — example id `usr-123456789` |
| `metadata` | object, `additionalProperties: string` | free string map; spec example `{"service": "srv-123456789", "field": "env_vars"}` |

## bex alignment (w4/m26)

`internal/audit/rest.go` now emits exactly this shape: `event` (1:1 verb→event mapping for the unambiguous lifecycle verbs, bex `<feature>.<Verb>` passthrough otherwise — a documented extension of the enum), `status` `success|error` with a denial mapped to `error` **and** preserved as `metadata.outcome: "denied"` (bex records authorization denials, which Render's binary enum cannot distinguish from runtime errors), `actor` `{type, id}` (`session`→`user`, `oauth2`→`rest_api`, system/unattributed→`system`; `email` omitted — the audit row deliberately stores only the subject), and always-present string-map `metadata` carrying the audited target keyed by kind (`service`/`database`/`keyvalue`/`envGroup`) plus the maintenance-mode `to` flag. **Payment method (w2/067, 2026-11-03):** a completed setup Checkout (the dashboard's Add/Update payment method) writes `billing.UpdatePaymentMethod` → `UpdateWorkspacePaymentMethodEvent` from the Stripe webhook, attributed to the member who opened the session (`bex_actor`/`bex_actor_method` session metadata), one row per session however often Stripe redelivers; no card details are stored. A default-card change made in the Stripe Billing Portal (w2/070, 2026-11-04) arrives as `customer.updated` and writes the same verb once per event, attributed to the member who last opened the portal (`bex_portal_actor` Customer metadata, within an hour) or else `system`; the Checkout's own change is not recorded twice (`bex_bound_payment_method`). **Reads and one row per action (w4/m321, 2026-11-11):** `ViewEnvVarValuesEvent` (any read returning env var values, `metadata.service` or `metadata.envGroup`) and `ViewConnectionInfoEvent` (Postgres/Key Value connection info, `metadata.database`/`metadata.keyvalue`) are recorded once per successful request (and `DownloadDatabaseBackupEvent`, below); reads gated by a write relation that change nothing (billing status, the audit log itself, invites, key lists, agent-session reads) record denials only; the create/delete/move/webhook/disk/Blueprint/build-source verbs map to Render's names, and a delete or move records one row. A create records its row once the resource exists, naming it (`metadata.service`/`project`/`environment`/`envGroup`/`webhook`/`blueprint` + `targetName`), and `metadata.relation` appears only on a denied row (w4/m321 t011/t012). See ADR018's audit row for the full list and the env var write decision. `TestRenderEventNamesAreInThePinnedEnum` checks every mapped name against the pinned enum. **Object storage (w21/087):** the pinned enum has no object or bucket event, so the routes of [ADR112](../ADR112-object-storage.md) report bex's own `objectstorage.<Action>` value (`objectstorage.PutObject`, `objectstorage.CreateBucket`, …) through the passthrough above. **Object and bucket targets, and the download row (w21/094):** each of those rows names its bucket, and an object row its key, as `metadata.bucket` (a bucket row carries only that) and `metadata.key`; Render's `/v1/objects` routes carry no bucket, so they name the workspace's system bucket `render-oregon` or `render-frankfurt`. The key is bounded to 256 bytes, written as valid UTF-8 with control characters escaped, and cut at a signed query, and no query string, header or presigned URL is ever stored. A bucket create's allowed row is written once the bucket exists. `objectstorage.GetObject` is the one read of the object store whose success is recorded, once per request (a streamed get once the object opened, a presigned-URL mint once it was minted); a HEAD, a list and a get that found nothing record none, and a refused one is the denied row as before. Render's enum has no object event, though it records downloads of customer data of its own (`DownloadDatabaseBackupEvent`, `DocumentDownloadEvent`), so this is a bex extension recorded in [ADR112](../ADR112-object-storage.md) D6. **Postgres export download (w21/105, 2026-11-16):** render.com/docs/audit-logs (read 2026-11-16) lists `DownloadDatabaseBackupEvent`, "A logical backup of a Render Postgres database was downloaded", and says these events are logged only for primary instances; it gives no metadata keys and does not say whether the click or the link request triggers it. bex records it from the one request that sees a download, the list that mints the 15-minute presigned URL: REST `GET /v1/postgres/{id}/export`, GraphQL `databaseExports` (only for a query that selects `url`) and MCP `list_postgres_exports` write one row, verb `postgres.DownloadDatabaseBackup`, once the URLs were minted, however many the list returned. It reports `DownloadDatabaseBackupEvent` with `metadata.database` (the database's id) and `targetName` (its name); the URL, its signature, the object key and the export id are never stored. A list that returned no URL (no export, one running, failed or expired) records none, and a refused one is the denial (`postgres.ListExports`, status `error`, `metadata.outcome: denied`, `metadata.relation: can_view_sensitive`). The metadata keys are bex's own, as for the other views: no capture of Render's rows for this event exists. It is the fourth verb of `core.auditedViews`; [ADR006](../ADR006-bex-api.md) § Audit log has the reasoning, including why the dashboard's status poll no longer selects `url`. **OAuth access (w4/m454, 2026-11-16):** `GrantOAuthAccessEvent` and `RevokeOAuthAccessEvent` are mapped. render.com/docs/audit-logs (read 2026-11-16) does not describe either; Render's MCP server has signed agents in with OAuth since 2026-07-22, which is the access these events name. bex records the grant (verb `oauth.GrantAccess`) when a member approves an OAuth client on the dashboard consent page and the revocation (`oauth.RevokeAccess`) when they revoke it under Connected agents, once in each workspace they belong to, with the member as a `user` actor and `metadata.client_id`, `metadata.client_name` and, on a grant, space-joined `metadata.scopes`. The operator's `revoke-oauth-grant` records the revocation too, as a `system` actor in the ops workspace with `metadata.client_id` and the member in `metadata.subject`; its earlier rows, verb `oauth.RevokeGrant`, read as the same event. The trigger, the placement and the metadata keys are bex's own until a capture of Render's rows shows otherwise; [ADR012](../ADR012-auth.md) § 7 has the mechanism. **Background and adapter paths (w21/093):** `agentsessions.FireAgentAutomation` (an automation's scheduled run), `agentsessions.CheckCreateAccess` and `agentsessions.CheckSteerAccess` (the pull request intake's access probes), `apps.AutoSyncBlueprint`, `envgroups.RevealEnvGroup` and `envgroups.GroupPlacements` are likewise bex's own values; the pinned enum has no event for them. The store's limit cap was raised to Render's 1000. `resource` (the OpenFGA object authorized against) remains a bex extra field. GraphQL `auditLogs` keeps bex's own flat dialect (`actor` string, `action`, `status: denied`) for the dashboard — Render has no public GraphQL to diverge from.

## Workspace event inventory (w9/m449)

Every **workspace** event [render.com/docs/audit-logs](https://render.com/docs/audit-logs) lists (read 2026-11-16) and what bex records for it. `renderWorkspaceEvents` in `lego/backend/internal/audit/render_inventory_test.go` holds the same list, and `TestRenderWorkspaceEventsAreRecorded` fails when a listed event has neither a mapped verb nor a non-goal or owner, or when a verb mapped onto one of them is missing from it. Organization events have their own inventory below. Events marked _docs only_ are not yet in the pinned OpenAPI enum; bex emits them as the same documented extension as its own verbs (`docsOnlyEvents`).

| Render event | bex verb(s) | Status |
| --- | --- | --- |
| `InviteToTeamEvent` | `members.Invite` | recorded |
| `RemoveUserFromTeamEvent` | `members.Remove`, `members.Leave` | recorded (a member leaving is the actor removing themselves) |
| `AcceptTeamInviteEvent` | `members.AcceptInvite` | recorded |
| `ChangeTeamMemberRoleEvent` | `members.ChangeRole` | recorded |
| `ChangeTeamAllowedLoginMethodsEvent` | `workspaces.SetRequireGoogleLogin` | recorded (w7/m465: the Google-login requirement, `metadata.to`) |
| `ChangeTeam2FAEnforcementEvent` | `workspaces.SetRequireTwoFactor` | recorded |
| `LoginEvent`, `LogoutEvent` | `auth.Login`, `auth.Logout` | recorded (w9/m452): once in each of the member's workspaces, a sign-in at the first sight of a new Kratos session, a sign-out from the dashboard's `recordSignOut` and the CLI's logout; `metadata.source_ip` and `user_agent`; an expired session records no sign-out |
| `CreateServerEvent` | `apps.Create` | recorded |
| `SuspendServiceEvent`, `ResumeServiceEvent` | `apps.Suspend`, `apps.Resume` | recorded |
| `MaintenanceModeEnabledEvent`, `MaintenanceModeURIUpdatedEvent` | `apps.SetMaintenanceMode`, `apps.SetMaintenanceModeURI` | recorded |
| `UpdateServiceNameEvent` | `apps.SetDisplayName` | recorded |
| `DeleteServerEvent` | `apps.Delete` | recorded |
| `StartShellEvent`, `EndShellEvent` | `shell.StartShell`, `shell.EndShell`; a refused shell's `apps.ResolveSSHSession` or `apps.CreateShellSession` denial | recorded (w9/m451): native SSH and the dashboard Shell, `metadata.service`, `instance`, `source_ip` and the web shell's `user_agent`; a refusal is `StartShellEvent` with `status: error`; agent-session SSH records neither |
| `ApplyBlueprintEvent` | `apps.ApproveBlueprint`, `apps.SyncBlueprint` | recorded (covers the services a Blueprint creates) |
| `CreateCronJobEvent`, `DeleteCronJobEvent` | `apps.CreateCronJob`, `apps.DeleteCronJob` | recorded |
| `UpdateAutoDeployEvent` | `apps.SetAutoDeploy` | recorded, docs only |
| `UpdateAutoDeployTriggerEvent` | `apps.SetAutoDeployTrigger`, `apps.SetBuildFilter` | recorded, docs only |
| `CreateCustomDomainEvent`, `DeleteCustomDomainEvent` | `apps.AddDomain`, `apps.DeleteDomain` | recorded, docs only |
| `UpdateIPAllowListEvent` | `apps.SetIPAllowList`, `postgres.SetIPAllowList`, `keyvalue.SetIPAllowList`, `workspaces.SetIPAllowList`, `environments.SetIPAllowList` | recorded |
| `CreatePostgresEvent`, `DeletePostgresEvent` | `postgres.CreatePostgres`, `postgres.DeletePostgres` | recorded |
| `SuspendPostgresEvent`, `ResumePostgresEvent` | `postgres.Suspend`, `postgres.Resume` | recorded |
| `DownloadDatabaseBackupEvent` | `postgres.DownloadDatabaseBackup` | recorded |
| `ViewConnectionInfoEvent` | `datastores.ViewConnectionInfo` | recorded |
| `CreateRedisEvent`, `DeleteRedisEvent` | `keyvalue.CreateKeyValue`, `keyvalue.DeleteKeyValue` | recorded |
| `CreateServerDiskEvent`, `DeleteServerDiskEvent` | `apps.AddDisk`, `apps.DeleteDisk` | recorded |
| `RestoreDiskSnapshotEvent` | `apps.RestoreDiskSnapshot` | recorded |
| `CreateEnvVarsEvent`, `UpdateEnvVarsEvent` | `secrets.CreateEnvVars`, `secrets.UpdateEnvVars` | recorded, one row per kind of change a save made, `metadata.envVarKeys` naming the keys |
| `DeleteEnvVarsEvent` | `secrets.DeleteEnvVars`, `secrets.DeleteEnvVar` (rows before w9/m449, and refusals) | recorded |
| `ViewEnvVarValuesEvent` | `secrets.ViewEnvVarValues` | recorded |
| `DeleteEnvGroupEvent` | `envgroups.DeleteEnvGroup` | recorded |
| `CreateWebhookEvent`, `UpdateWebhookEvent`, `DeleteWebhookEvent` | `webhooks.Create`, `webhooks.Update`, `webhooks.Delete` | recorded |
| `CreateOtelIntegrationEvent`, `UpdateOtelIntegrationEvent`, `DeleteOtelIntegrationEvent` | — | non-goal: external metrics streams (`.pm/DO_NOT_DO.md`, external log/metric drains) |
| `CreateProjectEvent`, `DeleteProjectEvent` | `projects.Create`, `projects.CreateWithEnvironments`, `projects.Delete`, `projects.DeleteIfEmpty` | recorded |
| `CreateEnvironmentEvent`, `DeleteEnvironmentEvent` | `environments.Create`, `environments.CreateWithACL`, `environments.Delete`, `environments.DeleteIfEmpty` | recorded |
| `MoveEnvironmentResourceEvent` | `projects.MoveService`, `environments.MoveService` | recorded |
| `ChangeEnvironmentProtectionEvent` | `environments.SetProtection` | recorded |
| `UpdateEnvironmentIsolatedEvent` | `environments.SetNetworkIsolation` | recorded, docs only |
| `DocumentDownloadEvent`, `SignNDAEvent` | — | non-goal: bex has no compliance-document portal |

Five enum events Render's page does not list are recorded too: `LinkBuildSourceEnvGroupEvent` and `UnlinkBuildSourceEnvGroupEvent` (`envgroups.LinkBuildSource`, `envgroups.UnlinkBuildSource`, an env group linked to or unlinked from a build source), `UpdateServiceBuildSourceEvent` (`apps.LinkBuildSource`), and `GrantOAuthAccessEvent` and `RevokeOAuthAccessEvent` (`oauth.GrantAccess`, `oauth.RevokeAccess`, a member approving or revoking an OAuth client, w4/m454 above). Where the service activity feed or webhooks read the action's own row (an env var save, an auto-deploy trigger change, a build source link), that row is kept but stored `feed_only`, which the audit log skips ([ADR006](../ADR006-bex-api.md) § Audit log).

## Organization event inventory (w7/m464)

Every **organization** event [render.com/docs/audit-logs](https://render.com/docs/audit-logs) lists (read 2026-11-17) and what bex records for it in the org's own log ([ADR115](../ADR115-organizations.md) D7). `renderOrganizationEvents` in `lego/backend/internal/audit/render_inventory_test.go` holds the same list, and `TestRenderOrganizationEventsAreRecorded` fails when it drifts. An org row's resource is the org object, so it is keyed by the org id and never appears in a workspace log.

| Render event | bex verb(s) | Status |
| --- | --- | --- |
| `InviteToOrgEvent` | `members.InviteToOrganization` | recorded |
| `AcceptOrgInviteEvent` | `members.AcceptOrganizationInvite` | recorded when the invite is redeemed at the invitee's sign-in |
| `AddOrgMemberEvent` | `members.AddToOrganization` | recorded: a workspace invite that made someone a Member, or a support transfer |
| `RemoveOrgMemberEvent` | `members.RemoveFromOrganization` | recorded |
| `JoinTeamEvent` | `members.JoinWorkspace`, `members.JoinWorkspaceAsAdmin` | recorded |
| `ChangeOrgRoleEvent` | `members.ChangeOrganizationRole` | recorded |
| `ChangeOrgAllowedLoginMethodsEvent` | `workspaces.SetOrganizationRequireGoogleLogin` | recorded (w7/m466: the org's Google-login requirement, `metadata.to`) |
| `ChangeOrg2FAEnforcementEvent` | `workspaces.SetOrganizationRequireTwoFactor` | recorded (w7/m466: the org's 2FA requirement, `metadata.to`) |
| `CreateWorkspaceEvent` | `workspaces.CreateOrganizationWorkspace` | recorded |
| `DeleteWorkspaceEvent` | `workspaces.Delete` | recorded (an org workspace's delete, in the org's log) |
| `ChangeWorkspacePrivacyEvent` | `workspaces.SetWorkspacePrivacy` | recorded |
| `CreateOrgDomainEvent`, `VerifyOrgDomainEvent`, `DeleteOrgDomainEvent`, `CreateSSOConnectionEvent`, `UpdateSSOConnectionEvent`, `DeleteSSOConnectionEvent`, `ProvisionOrganizationSCIMToken`, `RevokeOrganizationSCIMToken` | — | non-goal: SSO/SAML and SCIM (`.pm/DO_NOT_DO.md`) |

The pinned enum's `ChangeOrgNameEvent` (no docs entry) is recorded too, by `workspaces.RenameOrganization`. `UpdateOrgPaymentMethodEvent` has no bex action: an org has no payment method of its own, each workspace keeps its own.
