# Render dashboard operations

Every GraphQL operation and every feature-flag gate in [Render's dashboard](https://dashboard.render.com/) bundle, with what bex does about each (w1/m645). The API pins see only Render's public contracts, and [render-changelog.md](render-changelog.md) sees only what Render announces. A feature Render ships in its dashboard alone, or keeps behind a per-workspace experiment flag, moves neither. The dashboard's JavaScript is public and readable without signing in, and every operation it can send is a literal in it, so this file gives each one an owner. The first pass found three features no other ledger had seen: the rollback comparison (`w1/m646`), the 2FA reminders (`w1/m647`) and avatars (`w1/m648`). It also found that Render's Postgres parameter overrides editor is a flagged experiment, which a brainstorm had nearly taken as a reason to reverse `w2/066`.

`scripts/render-dashboard-ops.sh` starts at the dashboard page's module script (`assets/index-*.js`) and follows every static import, dynamic import and preload reference, recursively. It lists each operation (`query`, `mutation` or `subscription` and its name) and each flag gate: a call, with a flag name, to a hook the `featureflags-*.js` chunk exports. The `render-dashboard-ops` job in [`render-schema-drift.yml`](../../.github/workflows/render-schema-drift.yml) runs it every Monday. It fails when an operation or flag has no row, when a row's operation or flag has left the bundle, or when a row breaks the rules below. It never writes this file: a person reads the new operation in the bundle and adds its row. `scripts/render-dashboard-ops.sh list` prints the inventory as JSON, with each operation's root fields and the chunk families that hold it. `scripts/render-dashboard-ops.test.sh` checks the script against a small synthetic bundle, and this file's rows on every push.

Chunk file names carry content hashes that change with each Render deploy (the entry was `index-BPMVVQol.js` on the morning of 2026-11-25 and `index-D5V4ZgbE.js` by evening), so rows name operations and flags, never chunk files.

Read 2026-11-26: 459 chunks, 703 operations (387 queries, 290 mutations, 26 subscriptions) and 8 flags.

## Dispositions

Each row has one disposition, and its pointer says why:

- `matched`: bex has the capability the operation serves. The pointer is the ADR018 row, ADR or artifact that shows it. Render's dashboard and bex's often name the same operation differently; the ADR018 row has bex's names.
- `partial`: bex has part of it. The pointer names the board item that owns the rest, or the ADR018 row that records the divergence.
- `gap`: bex lacks it. The pointer names the open or parked board item that owns it (`wN/mN` or `wN/NNN`); a gap with no owner is filed with `/pm add` before its row lands.
- `non-goal`: bex declines it. The pointer cites the [`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) line or the [ADR018](../ADR018-render-parity.md) `—` row.
- `n/a`: there is nothing for bex to match: Render's own staff tools, support vendors, legal paperwork or growth experiments. The pointer gives the reason.
- `flagged`: the operation serves a feature behind a flag gate. The pointer names the flag from the table below.

A disposition is about the capability an operation serves, as in the changelog ledger. Many operations are one feature's reads and writes, so they share one pointer.

## Flag gates

A flag gate shows a feature only to the workspaces or users whose `featureFlags` hold the flag (or `owner-all`, `all`). Most are experiments: the hook records "Experiment Exposed" with `experiment_group` `treatment` or `control`. What each flag gates was read from the code that calls it. On 2026-11-26 an ordinary Render Pro workspace showed no overrides card on its Postgres Info page, and its Workspace Settings Early Access list ("Opt in to new features before General Availability (GA)") was empty, so no workspace can turn a flag on itself.

A flag row is always `flagged`. Its pointer names the board item or ADR018 row to reopen or compare when the gate goes: a flag that leaves the bundle means Render made the feature GA or dropped it, and the job's failure quotes the pointer. "Called from" lists the chunk families that call the flag. When they change, the job prints a notice and does not fail, because Rollup moves call sites between chunks from one deploy to the next and most of them sit in the entry chunk.

| Flag | Called from | Disposition | Pointer |
| --- | --- | --- | --- |
| `experiment-post-deploy-upgrade` | `index` | flagged | A growth experiment: when a Hobby workspace's first deploy goes live, the celebration dialog offers a workspace upgrade. If it goes GA, compare bex's first-deploy screen (ADR018 § Identity, workspaces & account: Workspace lifecycle) |
| `experiment-zero-state` | `index` | flagged | A growth experiment on the new-service form. If it goes GA, compare bex's create pages (ADR018 § Services & lifecycle: Create service (web / private)) |
| `expose-auto-deploy-trigger` | `index`, `NewServiceSourceCode` | flagged | The Auto-Deploy trigger selector (On Commit, After CI Checks Pass, Off) in service and cron job settings and the create form; the control group gets a Yes/No toggle. bex shows the selector to everyone (ADR018 § Services & lifecycle: CI-gated dispatch, `w1/m181`), so there is nothing to reopen |
| `grandfather-owners-into-sharing` | `CronJob`, `index`, `ServiceSharing` | flagged | Render's deprecated per-resource sharing ("Legacy — This sharing method is deprecated"), kept for grandfathered owners. bex shares through workspace members (ADR018 § Identity, workspaces & account: Workspace members & roles), so there is nothing to reopen |
| `okta-app-available` | `Settings` | flagged | An Okta tab beside Generic SAML in an organization's SSO setup. SSO is a non-goal (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—)) |
| `postgres-parameter-overrides` | `index` | flagged | Reopen `w2/066` D2=A: the PostgreSQL Configuration Overrides card on a database's Info page, for Basic instances and higher (ADR018 § Managed Postgres: Postgres observability, `parameterOverrides` retired) |
| `share-postgres` | `index` | flagged | With `grandfather-owners-into-sharing`, a Sharing page on a database for grandfathered owners. Nothing to reopen (ADR018 § Identity, workspaces & account: Workspace members & roles) |
| `skip-check-suspendable-when-showing-payment-failed-banner` | `index` | flagged | Which workspaces see the payment-failed banner. If it goes GA, compare bex's banner (ADR018 § Billing & invoicing: Unpaid invoice, suspension and recovery) |

## Operations

Grouped by product area, then sorted by name.

### Services

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addDisk` | mutation | matched | ADR018 § Other datastores & storage: Persistent disks |
| `allEnvs` | query | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `allResourcesForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `createServer` | mutation | matched | ADR018 § Services & lifecycle: Create service (web / private); Background worker; Static site |
| `deleteDisk` | mutation | matched | ADR018 § Other datastores & storage: Persistent disks |
| `deleteServer` | mutation | matched | ADR018 § Gap backlog: Delete service (done on every surface, `w2/m4`, `w5/m14`) |
| `DiskSnapshotRestores` | query | matched | ADR018 § Other datastores & storage: Persistent disks |
| `diskSnapshots` | query | matched | ADR018 § Other datastores & storage: Persistent disks |
| `glob` | query | matched | ADR018 § Services & lifecycle: Build Filters (path-scoped auto-deploy); bex validates each pattern on save (`normalizeBuildFilter`) |
| `globalSearchStatuses` | query | matched | ADR018 § Services & lifecycle: Get service (status badge and live state) |
| `headersForService` | query | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites |
| `isDeployableImage` | query | gap | `w1/153` (check an image before creating the service) |
| `isServiceNameAvailable` | query | matched | ADR018 § Services & lifecycle: Create service (name availability); service display-name validation (`w8/034`) |
| `jobs` | query | non-goal | ADR018 § Services & lifecycle: One-off jobs (run a command) (—) |
| `minimalResourcesForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `minimalServer` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `minimalServerMetadata` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `minimalServerSideNav` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `ownerLimits` | query | matched | ADR018 § Identity, workspaces & account: Workspace resource-cap visibility |
| `performMaintenance` | mutation | non-goal | ADR018 § Platform events & integrations: Maintenance runs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `performServerMaintenance` | mutation | non-goal | ADR018 § Platform events & integrations: Maintenance runs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `purgeServerCache` | mutation | matched | ADR018 § Services & lifecycle: Edge caching (`serviceDetails.cache.profile`); Cache purge |
| `queryServerNameUrl` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `queryServerRepoCommit` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `queryServerRepoCommits` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `redirectRules` | query | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites |
| `regenerateDeployKey` | mutation | matched | ADR018 § Deploys: Secret deploy hook (`regenerateDeployHook`) |
| `rescheduleMaintenance` | mutation | non-goal | ADR018 § Platform events & integrations: Maintenance runs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `resourceTypeByServiceId` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `restartServer` | mutation | matched | ADR018 § Services & lifecycle: Restart |
| `restoreDiskSnapshot` | mutation | matched | ADR018 § Other datastores & storage: Persistent disks |
| `resumeService` | mutation | matched | ADR018 § Services & lifecycle: Suspend / Resume |
| `rootDirSuggestion` | query | matched | [runtime-inference.md](runtime-inference.md) (`w2/m87`): repo selection and Root Directory suggest the runtime and commands |
| `saveHeaders` | mutation | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites |
| `saveRedirectRules` | mutation | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites |
| `server` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serverCacheEvents` | query | matched | ADR018 § Services & lifecycle: Edge caching (`serviceDetails.cache.profile`); Cache purge |
| `serverCacheProfileQuery` | query | matched | ADR018 § Services & lifecycle: Edge caching (`serviceDetails.cache.profile`); Cache purge |
| `ServerDeployEndedEvents` | query | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `serverInstances` | query | matched | ADR018 § Services & lifecycle: List live service instances |
| `serverIpAllowList` | query | matched | ADR018 § Custom domains: Inbound IP allowlist (`ipAllowList`) for web services + static sites |
| `serverOwnerLimits` | query | matched | ADR018 § Identity, workspaces & account: Workspace resource-cap visibility |
| `serverPlans` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serviceDeployStartedEvent` | subscription | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `serviceEvents` | query | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `serviceEventsSubscription` | subscription | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `serviceIdsForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serviceInstancesUpdated` | subscription | matched | ADR018 § Services & lifecycle: List live service instances |
| `servicesForBillingUpdatePlan` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesForCardRemoval` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesForOwnerOverrides` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesForOwnerOverview` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesLiteForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `servicesNamesTypesForOwner` | query | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serviceSuspension` | subscription | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serviceTimelineEvents` | query | matched | ADR018 § Services & lifecycle: Service events / activity feed |
| `serviceUpdated` | subscription | matched | ADR018 § Services & lifecycle: Get service; List services |
| `serviceWithShellURL` | query | matched | ADR018 § Services & lifecycle: SSH into a running instance (the dashboard Shell) |
| `startShell` | mutation | matched | ADR018 § Services & lifecycle: SSH into a running instance (the dashboard Shell) |
| `statusQuery` | query | matched | ADR018 § Services & lifecycle: Get service (status badge and live state) |
| `statusStateQuery` | query | matched | ADR018 § Services & lifecycle: Get service (status badge and live state) |
| `statusSubscription` | subscription | matched | ADR018 § Services & lifecycle: Get service (status badge and live state) |
| `suspendService` | mutation | matched | ADR018 § Services & lifecycle: Suspend / Resume |
| `updateAutoscalingConfig` | mutation | matched | ADR018 § Services & lifecycle: Autoscaling config (min/max + CPU/mem target) |
| `updateBuildFilter` | mutation | matched | ADR018 § Services & lifecycle: Build Filters (path-scoped auto-deploy); bex validates each pattern on save (`normalizeBuildFilter`) |
| `updateDisk` | mutation | matched | ADR018 § Other datastores & storage: Persistent disks |
| `updateServer` | mutation | matched | ADR018 § Services & lifecycle: Get service; service display-name validation (`w8/034`) |
| `UpdateServerAuthConfig` | mutation | gap | `w1/148` (Render's per-service Authentication setting) |
| `updateServerAutoDeploy` | mutation | matched | ADR018 § Deployment sources & IaC: Git connections (Auto-Deploy toggle); ADR018 § Services & lifecycle: CI-gated dispatch (`w1/m181`) |
| `updateServerAutoDeployTrigger` | mutation | matched | ADR018 § Deployment sources & IaC: Git connections (Auto-Deploy toggle); ADR018 § Services & lifecycle: CI-gated dispatch (`w1/m181`) |
| `updateServerBaseDir` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `updateServerBranch` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `updateServerBuildCommand` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `updateServerCacheProfile` | mutation | matched | ADR018 § Services & lifecycle: Edge caching (`serviceDetails.cache.profile`); Cache purge |
| `updateServerDockerCommand` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `updateServerDockerfilePath` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |
| `updateServerHealthCheckPath` | mutation | matched | ADR018 § Services & lifecycle: Health checks (path → readiness gating) |
| `updateServerInstanceCount` | mutation | matched | ADR018 § Services & lifecycle: Manual scale (instance count) |
| `updateServerIpAllowList` | mutation | matched | ADR018 § Custom domains: Inbound IP allowlist (`ipAllowList`) for web services + static sites |
| `updateServerName` | mutation | matched | ADR018 § Services & lifecycle: Get service; service display-name validation (`w8/034`) |
| `updateServerPlan` | mutation | matched | ADR018 § Services & lifecycle: Change instance plan / type |
| `updateServerStartCommand` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory (start command) |
| `updateServerStaticPublishPath` | mutation | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory |

### Deploys and builds

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `build` | query | matched | ADR018 § Deployment sources & IaC: Build Sources and builds ([ADR097](../ADR097-build-sources.md)); ADR018 § Deploys: List / get deploy objects |
| `buildPlans` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |
| `buildPlansWithFreeMinutes` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |
| `buildTraces` | query | gap | `w1/155` (a deploy's step timeline: clone, cache, build, upload, pre-deploy, deploy) |
| `cancelDeploy` | mutation | matched | ADR018 § Deploys: Cancel deploy |
| `cronJobDeployTraces` | query | gap | `w1/155` (a deploy's step timeline: clone, cache, build, upload, pre-deploy, deploy) |
| `deploy` | query | matched | ADR018 § Deploys: List / get deploy objects |
| `deployCreated` | subscription | matched | ADR018 § Deploys: List / get deploy objects |
| `deployCreatedMinimal` | subscription | matched | ADR018 § Deploys: List / get deploy objects |
| `deploys` | query | matched | ADR018 § Deploys: List / get deploy objects |
| `deployServer` | mutation | matched | ADR018 § Deploys: Trigger a deploy |
| `deploysMinimal` | query | matched | ADR018 § Deploys: List / get deploy objects |
| `deployTraces` | query | gap | `w1/155` (a deploy's step timeline: clone, cache, build, upload, pre-deploy, deploy) |
| `deployUpdated` | subscription | matched | ADR018 § Deploys: List / get deploy objects |
| `deployUpdatedMinimal` | subscription | matched | ADR018 § Deploys: List / get deploy objects |
| `deployWithTrigger` | query | matched | ADR018 § Deploys: List / get deploy objects |
| `deployWithTriggerUpdated` | subscription | matched | ADR018 § Deploys: List / get deploy objects |
| `maxBuildSpendCents` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |
| `minimalPreviewServers` | query | non-goal | DO_NOT_DO: PR preview environments |
| `previewServerCreatedOrDeleted` | subscription | non-goal | DO_NOT_DO: PR preview environments |
| `previewServers` | query | non-goal | DO_NOT_DO: PR preview environments |
| `rollbackComparison` | query | partial | ADR018 § Deploys: Rollback (the Confirm Rollback page, w1/m646); a static site's comparison is `w1/156` |
| `rollbackToDeploy` | mutation | matched | ADR018 § Deploys: Rollback |
| `serverCapableOfPreviews` | query | non-goal | DO_NOT_DO: PR preview environments |
| `ServerHeaderDeploys` | query | matched | ADR018 § Deploys: List / get deploy objects |
| `teamWithDefaultBuildPlan` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |
| `updateDeployHandling` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace Overlapping Deploy Policy (Wait · Override) |
| `updateLogStreamPreviewSetting` | mutation | non-goal | DO_NOT_DO: PR preview environments |
| `updateOwnerMaxBuildSpendCents` | mutation | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |
| `updateUserNotifyOnPrUpdate` | mutation | non-goal | DO_NOT_DO: PR preview environments |
| `usageOwnerPipelineMinutes` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails (pipeline minutes); ADR018 § Identity, workspaces & account: pipeline spend limit (`pipelineSettings`, `setPipelineSpendLimit`) |

### Cron jobs

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `buildCronJob` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `buildForCronJobWithTrigger` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `buildsForCronJob` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `buildsForCronJobMinimal` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `cancelCronJobRun` | mutation | matched | ADR018 § Services & lifecycle: Cron job (run history); [cron-runs.md](cron-runs.md) |
| `createCronJob` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `cronJob` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `CronJobDeployDetails` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `CronJobDeploys` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `cronJobHeader` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `CronJobHeaderLiveDeploy` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `cronJobPlans` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `cronJobRuns` | query | matched | ADR018 § Services & lifecycle: Cron job (run history); [cron-runs.md](cron-runs.md) |
| `deleteCronJob` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `minimalCronJob` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `minimalCronJobSideNav` | query | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `runCronJob` | mutation | matched | ADR018 § Services & lifecycle: Cron job (run history); [cron-runs.md](cron-runs.md) |
| `updateCronJob` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobAutoDeploy` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobAutoDeployTrigger` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobBaseDir` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobBranch` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobBuildCommand` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobCommand` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobDockerCommand` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobDockerfilePath` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobName` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobPlan` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |
| `updateCronJobSchedule` | mutation | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-dashboard.md](cron-dashboard.md) |

### Build Sources

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addEnvGroupToArtifactSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactBuildEnvVarValues` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactDeploys` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactDetails` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSource` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceDeployableBuilds` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceEnvFileKeys` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceEnvFiles` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceEnvVarKeys` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceEnvVars` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceEventLink` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `ArtifactSourceName` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourcePreview` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourcePreviewLiveDeploy` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceRepoCommits` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourceServices` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourcesForOwner` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `artifactSourcesNamesForOwner` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `buildArtifactSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `createArtifactSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `deleteArtifactSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `newServiceArtifactSource` | query | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `removeEnvGroupFromArtifactSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceBaseDir` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceBranch` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceBuildCommand` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceBuildFilter` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceDockerfilePath` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceDockerRootDir` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceEnvVars` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceExternalImage` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceGitSource` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceName` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceRegistryCredential` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |
| `updateArtifactSourceRootDir` | mutation | matched | ADR018 § Services & lifecycle: Build Sources (shared build for several services); [ADR097](../ADR097-build-sources.md) |

### Environment variables, secret files and environment groups

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addEnvGroupToService` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `createEnvGroup` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `deleteEnvGroup` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroup` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupFileKeys` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupFiles` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupNameExists` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupsForOwner` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupsForOwnerSearch` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupsForService` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupVarKeys` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `envGroupVars` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `minimalEnvGroup` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `minimalEnvGroupsForOwner` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `removeEnvGroupFromService` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `resourceTypeByEnvGroupId` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `serviceEnvFileKeys` | query | matched | ADR018 § Environment & config: Secret files (mounted at `/etc/secrets`) |
| `serviceEnvFiles` | query | matched | ADR018 § Environment & config: Secret files (mounted at `/etc/secrets`) |
| `serviceEnvVarKeys` | query | matched | ADR018 § Environment & config: Env vars (list · get · replace · set · delete) |
| `serviceEnvVars` | query | matched | ADR018 § Environment & config: Env vars (list · get · replace · set · delete) |
| `servicesForEnvGroup` | query | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `servicesForOwnerToLinkSearch` | query | gap | `w1/154` (Render's env var editor links a service, database or Key Value; DO_NOT_DO's Link-database line rests on a premise Render no longer holds) |
| `updateEnvGroup` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `updateEnvGroupName` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `updateEnvGroupVars` | mutation | matched | ADR018 § Environment & config: Environment groups (+ link / unlink) |
| `updateServiceEnvVars` | mutation | matched | ADR018 § Environment & config: Env vars (list · get · replace · set · delete) |

### Custom domains and networking

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `arnPrincipal` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `certificate` | query | matched | ADR018 § Custom domains: Certificates from Let's Encrypt and Google Trust Services |
| `createCustomDomainsMutation` | mutation | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `createDedicatedStaticIP` | mutation | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `createPrivateLink` | mutation | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `customDomainCountsByService` | query | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `customDomainCreateEligibility` | query | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `customDomains` | query | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `customDomainsForOwner` | query | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `dedicatedStaticIP` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `dedicatedStaticIPCreateEligibility` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `dedicatedStaticIPsForEnvironment` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `dedicatedStaticIPsForOwner` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `deleteCustomDomainMutation` | mutation | matched | ADR018 § Custom domains: Custom domains (list · add · get · delete) |
| `deleteDedicatedStaticIP` | mutation | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `deletePrivateLink` | mutation | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `minimalDedicatedStaticIP` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `minimalDedicatedStaticIPsForOwner` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `minimalPrivateLink` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `minimalPrivateLinks` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `privateLink` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `privateLinks` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `privateLinkSelfServeMax` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `privateLinkStatus` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `refreshCustomDomainStatusMutation` | mutation | matched | ADR018 § Custom domains: Verify / DNS instructions |
| `regionFromEndpointServiceName` | query | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `serviceDedicatedStaticIPAssignmentsForOwner` | query | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `updateDedicatedStaticIP` | mutation | non-goal | ADR018 § Platform events & integrations: Dedicated outbound IPs (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `updatePrivateLink` | mutation | gap | Render's AWS private links have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |

### Managed Postgres

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `acknowledgeDatabaseUpgrade` | mutation | matched | ADR018 § Managed Postgres: Postgres major-version upgrade; [postgres-version-upgrade.md](postgres-version-upgrade.md) |
| `createDatabase` | mutation | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `createDatabaseBackup` | mutation | matched | ADR018 § Managed Postgres: Backups · PITR / recovery; [postgres-exports.md](postgres-exports.md) |
| `createDatabaseCredential` | mutation | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler |
| `createDatabaseReplica` | mutation | matched | ADR018 § Managed Postgres: HA · failover · read replicas |
| `createDatabaseV2` | mutation | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `database` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databaseBackupsQuery` | query | matched | ADR018 § Managed Postgres: Backups · PITR / recovery; [postgres-exports.md](postgres-exports.md) |
| `databaseCredentialList` | query | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler |
| `databaseEvents` | query | matched | ADR018 § Managed Postgres: Dashboard pages (sidebar · Info · Logs · Metrics · Plan · Apps · Recovery); [postgres-dashboard.md](postgres-dashboard.md) |
| `databaseExtensions` | query | partial | ADR018 § Managed Postgres: Postgres extensions (`CREATE EXTENSION`); the names that still fail are `w21/m585`, `w21/m586` |
| `databaseIdsForOwner` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databaseIpAllowList` | query | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler |
| `databaseMemoryMetrics` | query | matched | ADR018 § Metrics: Extended metrics (datastore CPU and memory, `w1/m180`) |
| `databaseParameterOverrides` | query | flagged | `postgres-parameter-overrides`: the PostgreSQL Configuration Overrides card; bex retired overrides (`w2/066` D2=A) |
| `databasePassword` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databasePlans` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info (plans); [pricing.md](pricing.md) |
| `databaseProcesses` | query | matched | ADR018 § Managed Postgres: Postgres observability (live queries · top-queries · sizes · table-scans) |
| `databaseRecoveryInfo` | query | matched | ADR018 § Managed Postgres: Backups · PITR / recovery; [postgres-recovery-window.md](postgres-recovery-window.md) |
| `databasesForOwner` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databasesForOwnerBilling` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databaseSizes` | query | matched | ADR018 § Managed Postgres: Postgres observability (live queries · top-queries · sizes · table-scans) |
| `databasesMajorVersionsForNewInstance` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databaseStatusQuery` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `databaseTopQueries` | query | matched | ADR018 § Managed Postgres: Postgres observability (live queries · top-queries · sizes · table-scans) |
| `deleteDatabase` | mutation | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `deleteDatabaseCredential` | mutation | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler |
| `minimalDatabase` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `minimalDatabaseSideNav` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `postgresConfigParameters` | query | flagged | `postgres-parameter-overrides`: the PostgreSQL Configuration Overrides card; bex retired overrides (`w2/066` D2=A) |
| `resourceTypeByDatabaseId` | query | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `restartDatabase` | mutation | matched | ADR018 § Managed Postgres: Lifecycle (suspend / resume / restart) |
| `restoreNewDatabase` | mutation | matched | ADR018 § Managed Postgres: Backups · PITR / recovery; [postgres-recovery-window.md](postgres-recovery-window.md) |
| `resumeDatabase` | mutation | matched | ADR018 § Managed Postgres: Lifecycle (suspend / resume / restart) |
| `suspendDatabase` | mutation | matched | ADR018 § Managed Postgres: Lifecycle (suspend / resume / restart) |
| `switchoverDatabaseClusterLeader` | mutation | matched | ADR018 § Managed Postgres: HA · failover · read replicas |
| `updateDatabaseDatadogAPIKey` | mutation | non-goal | ADR018 § Metrics: Metric streams (external) (—); ADR009 refuses Render's Datadog fields by name |
| `updateDatabaseDisplayName` | mutation | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info |
| `updateDatabaseIpAllowList` | mutation | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler |
| `updateDatabaseParameterOverrides` | mutation | flagged | `postgres-parameter-overrides`: the PostgreSQL Configuration Overrides card; bex retired overrides (`w2/066` D2=A) |
| `updateDatabasePlan` | mutation | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info (plans); [pricing.md](pricing.md) |
| `upgradeDatabaseVersion` | mutation | matched | ADR018 § Managed Postgres: Postgres major-version upgrade; [postgres-version-upgrade.md](postgres-version-upgrade.md) |

### Key Value

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `createRedis` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `deleteRedis` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `enableRedisInternalAuth` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Enable Internal Authentication, `w5/m268`) |
| `minimalRedis` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `minimalRedisSideNav` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redis` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redisConnection` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redisesForOwner` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redisIdsForOwner` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redisIpAllowList` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `redisPlans` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `resourceTypeByRedisId` | query | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `resumeRedis` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `suspendRedis` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `updateRedisIpAllowList` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `updateRedisMaxmemoryPolicy` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `updateRedisName` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `updateRedisPersistenceMode` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |
| `updateRedisPlan` | mutation | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis) |

### Projects and environments

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `changeEnvironmentForResources` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `createEnvironment` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `createProject` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `deleteEnvironment` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `deleteProject` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `environmentIpAllowList` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (environment inbound IP rules); [protected-environments.md](protected-environments.md) |
| `environmentResourceBlueprints` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `environmentResources` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalEnvironment` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalEnvironmentEnvGroups` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalEnvironmentResources` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalProject` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalProjectResources` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalProjects` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `minimalProjectWithEnvironments` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `project` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectAndEnvironment` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectResources` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectResourcesExistence` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projects` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectsForOwner` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectsForOwnerSearch` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectsMinimal` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `projectsWithEnvironmentsAndResources` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `updateEnvironment` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |
| `updateEnvironmentIpAllowList` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (environment inbound IP rules); [protected-environments.md](protected-environments.md) |
| `updateProject` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (grouping) |

### Blueprints

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `approveIACSync` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `blueprintContents` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `blueprintExport` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC (Generate Blueprint, GraphQL `generateBlueprint`) |
| `blueprintNameExists` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `blueprintsForOwnerSearch` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `deleteIACExecutionSource` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `executionSourceResources` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutionForSync` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutions` | subscription | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutionSource` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutionSources` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutionSubscription` | subscription | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `iacExecutionSubscriptionForSync` | subscription | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `minimalIacExecutionSources` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `minimalSource` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `planNewSource` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `planSync` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `resourcesForBlueprintExport` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC (Generate Blueprint, GraphQL `generateBlueprint`) |
| `resourceTypeByBlueprintId` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `setIACAutoSync` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `source` | query | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `updateIacSourceBranch` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `updateIacSourceNameMutation` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |
| `updateIacSourcePath` | mutation | matched | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC |

### Git and registry credentials

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `bitbucketConnectCredentialMutation` | mutation | non-goal | DO_NOT_DO: Bitbucket git-provider integration |
| `claimArtifactSourceBuildConfigUser` | mutation | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `claimBuildConfigUser` | mutation | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `claimExecutionSourceOwnershipMutation` | mutation | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `claimServiceOwnership` | mutation | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `communalServicesForUser` | query | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `createRegistryCredential` | mutation | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `cursorOriginConfirm` | mutation | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `cursorOriginConfirmUrl` | query | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `cursorOriginConnect` | mutation | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `cursorOriginDisconnect` | mutation | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `cursorOriginInstallations` | query | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `cursorOriginInstallUrl` | query | gap | `w1/149` (Cursor Origin as a Git provider: build it, or sign it off beside GitLab and Bitbucket) |
| `deleteRegistryCredential` | mutation | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `gitCredentialsForUser` | query | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `githubAppInstallationAccessForUser` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `githubAppInstallationStatusesForUser` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `githubConnectCredentialMutation` | mutation | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `gitlabConnectCredentialMutation` | mutation | non-goal | DO_NOT_DO: GitLab git-provider integration |
| `gitRepo` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `gitRepoBranch` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `gitRepoBranchSearch` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `gitRepoDirectory` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `registryCredential` | query | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `registryCredentials` | query | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `removeUserCredential` | mutation | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `servicesForDeployCredential` | query | non-goal | DO_NOT_DO #GIT-WORKSPACE-OWNED: Git access stays workspace-owned (ADR078); no per-member Git credentials |
| `servicesForRegistryCredential` | query | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `updateRegistryCredentialOnServices` | mutation | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |
| `userRecentRepos` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `userRepoList` | query | matched | ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app) |
| `validateRegistryCredentialUpdate` | query | matched | ADR018 § Deployment sources & IaC: Registry credentials (private images) |

### Logs and metrics

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `createOtelIntegration` | mutation | non-goal | ADR018 § Metrics: Metric streams (external) (—) |
| `createSavedSearch` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (Observability views: saved searches, `w3/m505`) |
| `deleteLogStreamSetting` | mutation | non-goal | ADR018 § Logs: Log streams (external drains) (—) |
| `deleteOtelIntegration` | mutation | non-goal | ADR018 § Metrics: Metric streams (external) (—) |
| `deleteSavedSearch` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (Observability views: saved searches, `w3/m505`) |
| `logAdded` | subscription | matched | ADR018 § Logs: Application logs (query + live tail); Request / HTTP logs + structured filters |
| `logs` | query | matched | ADR018 § Logs: Application logs (query + live tail); Request / HTTP logs + structured filters |
| `logStreamSettings` | query | non-goal | ADR018 § Logs: Log streams (external drains) (—) |
| `logValues` | query | matched | ADR018 § Logs: Application logs (query + live tail); Request / HTTP logs + structured filters |
| `metrics` | query | matched | ADR018 § Metrics: Core metrics (CPU · memory · instance-count · HTTP requests · latency …) |
| `metricsFilters` | query | matched | ADR018 § Metrics: Core metrics (CPU · memory · instance-count · HTTP requests · latency …) |
| `metricsSearch` | query | matched | ADR018 § Metrics: Core metrics (CPU · memory · instance-count · HTTP requests · latency …) |
| `monthToDateBandwidth` | query | matched | ADR018 § Metrics: Extended metrics (bandwidth); ADR018 § Billing & invoicing: included usage (outbound bandwidth) |
| `mostRecentLog` | query | matched | ADR018 § Logs: Application logs (query + live tail); Request / HTTP logs + structured filters |
| `otelIntegrationForOwner` | query | non-goal | ADR018 § Metrics: Metric streams (external) (—) |
| `ownerLogStreamEligability` | query | non-goal | ADR018 § Logs: Log streams (external drains) (—) |
| `savedSearch` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (Observability views: saved searches, `w3/m505`) |
| `savedSearches` | query | matched | ADR018 § Deployment sources & IaC: Projects & environments (Observability views: saved searches, `w3/m505`) |
| `updateLogStream` | mutation | non-goal | ADR018 § Logs: Log streams (external drains) (—) |
| `updateOtelIntegration` | mutation | non-goal | ADR018 § Metrics: Metric streams (external) (—) |
| `updateSavedSearch` | mutation | matched | ADR018 § Deployment sources & IaC: Projects & environments (Observability views: saved searches, `w3/m505`) |
| `usageOwnerBuildBandwidth` | query | matched | ADR018 § Metrics: Extended metrics (bandwidth); ADR018 § Billing & invoicing: included usage (outbound bandwidth) |

### Webhooks and notifications

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `createWebhook` | mutation | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `deleteWebhook` | mutation | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `isWebhookNameAvailable` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `minimalWebhook` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `minimalWebhooksForOwner` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `notificationOverride` | query | matched | ADR018 § Platform events & integrations: Notifications (deploy/failure alerts); [notify-on-fail.md](notify-on-fail.md) |
| `notificationSettings` | query | matched | ADR018 § Platform events & integrations: Notifications (deploy/failure alerts); [notify-on-fail.md](notify-on-fail.md) |
| `removeSlackAuth` | mutation | non-goal | DO_NOT_DO: Slack deploy-notification delivery |
| `rerunWebhook` | mutation | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `resourceTypeByWebhookId` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `slackAuth` | mutation | non-goal | DO_NOT_DO: Slack deploy-notification delivery |
| `updateNotificationOverride` | mutation | matched | ADR018 § Platform events & integrations: Notifications (deploy/failure alerts); [notify-on-fail.md](notify-on-fail.md) |
| `updateNotificationSettings` | mutation | matched | ADR018 § Platform events & integrations: Notifications (deploy/failure alerts); [notify-on-fail.md](notify-on-fail.md) |
| `updateWebhook` | mutation | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhookEventFilters` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhookEvents` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhookForOwner` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhooksForOwner` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhooksForOwnerSearch` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |
| `webhookUsageForOwner` | query | matched | ADR018 § Platform events & integrations: Outbound event webhooks; [webhooks-ui.md](webhooks-ui.md) |

### Sandboxes

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `createSandboxGroup` | mutation | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `CreateSandboxShellSession` | mutation | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `deleteSandboxGroup` | mutation | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `DeleteSandboxSnapshot` | mutation | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `minimalSandboxGroup` | query | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `minimalSandboxGroupsForOwner` | query | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `resourceTypeBySandboxGroupId` | query | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `resourceTypeBySandboxId` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `resourceTypeBySandboxSnapshotId` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `SandboxBuilds` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `SandboxBuildsForGroupUpdated` | subscription | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `SandboxBuildSources` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxDetails` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxesForGroup` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxesForGroupUpdated` | subscription | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxEvents` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxEventsSubscription` | subscription | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxGroup` | query | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `SandboxGroupRunningCount` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxGroupsForOwner` | query | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `sandboxGroupShellPermission` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxGroupUpdated` | subscription | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `SandboxSnapshot` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `SandboxSnapshots` | query | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `SandboxSnapshotsForGroupUpdated` | subscription | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `sandboxUpdated` | subscription | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `setDefaultSandboxGroup` | mutation | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |
| `terminateSandbox` | mutation | gap | `w1/152` (a sandbox group's sandboxes, snapshots, builds and shell in the dashboard) |
| `updateSandboxGroupName` | mutation | matched | ADR018 § Render operation omissions: Render sandbox groups and snapshots (✅); [ADR104](../ADR104-sandbox-render-parity.md) |

### Workflows

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `cancelFlowRun` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `cancelTaskRun` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `createWorkflowService` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `deployWorkflowService` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `getTaskRun` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `minimalWorkflowService` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `minimalWorkflowTask` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `minimalWorkflowTasks` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `newWorkflowServicesForOwner` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `projectGroupedWorkflowRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `projectWorkflowRunFilterOptions` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `projectWorkflowRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `projectWorkflowRunSummary` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `projectWorkflowRunTimeSeries` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `resourceTypeByFlowRunId` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `resourceTypeByWorkflowId` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `startFlow` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `startTask` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `taskPlans` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `taskResourceRedirect` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `taskRunFilters` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `taskRunResourceRedirect` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflow` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowAutoDeployTrigger` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowBaseDir` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowBranch` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowBuildCommand` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowDockerfilePath` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowEnvVars` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowRootDir` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowService` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `updateWorkflowStartCommand` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowDefaultVersion` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowDefinitions` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowDefinitionsForSearch` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowDelete` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowEnvFiles` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowEnvVarKeys` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowEnvVars` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowEvents` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowEventsSubscription` | subscription | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFileKeys` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlow` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowRun` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowRunDetailsUpdates` | subscription | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowRunStep` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowRunStepAttempt` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowRunSteps` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowFlowSummary` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowGroupedRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowGroupTaskRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowListRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowListTaskRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowListTaskRunsForViewer` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowService` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceDefinitions` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceDefinitionTypes` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceDescription` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceFilterTasks` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceName` | mutation | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceRepoCommit` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServiceRepoCommits` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServicesForOwner` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServicesForOwnerSearch` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowServicesNamesForOwner` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTask` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskDetailsUpdates` | subscription | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskRun` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskRunOwnership` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskRuns` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskRunStatus` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskRunUpdates` | subscription | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTasksSummary` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowTaskSummary` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowVersion` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowVersionCard` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowVersionsForNewWorkflow` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |
| `workflowVersionsForStatus` | query | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) |

### Account and sign-in

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `agenticTokenSignIn` | mutation | gap | `w1/150` (Render's Stripe integrations: the Stripe account connection and Stripe's agentic account link) |
| `ApiKeyWorkspace` | query | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `apiTokens` | query | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `avatarPresignedUpload` | query | gap | `w1/m648` (avatars) |
| `avatarsForWorkspaceMembers` | query | gap | `w1/m648` (avatars) |
| `bitbucketAuthMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `bitbucketSignUpMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `canChangeEmail` | query | matched | ADR018 § Identity, workspaces & account: Email recovery / verification |
| `confirmAgenticAccountLink` | mutation | gap | `w1/150` (Render's Stripe integrations: the Stripe account connection and Stripe's agentic account link) |
| `createPassword` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings (password set and change in `/settings`) |
| `createSSHPublicKey` | mutation | matched | ADR018 § Identity, workspaces & account: SSH public keys |
| `deleteAvatar` | mutation | gap | `w1/m648` (avatars) |
| `deleteSSHPublicKey` | mutation | matched | ADR018 § Identity, workspaces & account: SSH public keys |
| `deleteUserAccount` | mutation | matched | [ADR018 § Account deletion](../ADR018-render-parity.md#account-deletion-w2m84-adr086) ([ADR086](../ADR086-account-deletion.md)) |
| `disable2FA` | mutation | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `githubSignInMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `githubSignUpMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `gitlabAuthMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `googleSignInMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `googleSignUpMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Sign-in providers: Google, GitHub, GitLab, Bitbucket |
| `markTOSAccepted` | mutation | n/a | Render's own terms-of-service re-acceptance banner; bex's legal terms are its own |
| `memberAvatarsForOrganization` | query | gap | `w1/m648` (avatars) |
| `new2FABackupCodes` | mutation | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `newOTPRequest` | mutation | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `oauthConnections` | query | matched | [ADR025](../ADR025-connect-an-agent.md): Account Settings → Connected agents lists and revokes OAuth clients |
| `organizationsForUserWithAvatars` | query | gap | `w1/m648` (avatars) |
| `provisionAPIToken` | mutation | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `ProvisionScopedApiToken` | mutation | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `requestEmailReset` | mutation | matched | ADR018 § Identity, workspaces & account: Email recovery / verification |
| `resendEmailVerification` | mutation | matched | ADR018 § Identity, workspaces & account: Email recovery / verification |
| `resetEmail` | mutation | matched | ADR018 § Identity, workspaces & account: Email recovery / verification |
| `resetPassword` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings (password set and change in `/settings`) |
| `resetPasswordConfirm` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings (password set and change in `/settings`) |
| `revokeAPIToken` | mutation | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `revokeOAuthConnection` | mutation | matched | [ADR025](../ADR025-connect-an-agent.md): Account Settings → Connected agents lists and revokes OAuth clients |
| `ScopedApiTokenPermissions` | query | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `session` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `signIn` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `signOut` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `signUp` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `sshPublicKeys` | query | matched | ADR018 § Identity, workspaces & account: SSH public keys |
| `twoFactorBackupCodes` | query | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `updatePassword` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings (password set and change in `/settings`) |
| `UpdateScopedApiTokenScopes` | mutation | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `updateUserLogThemeSetting` | mutation | matched | ADR018 § Identity, workspaces & account: Account Appearance: Dashboard Theme, Log Explorer Theme, High Contrast Mode |
| `updateUserProfile` | mutation | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `updateUserThemeSetting` | mutation | matched | ADR018 § Identity, workspaces & account: Account Appearance: Dashboard Theme, Log Explorer Theme, High Contrast Mode |
| `user` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `validateResetToken` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings (password set and change in `/settings`) |
| `verifyEmail` | mutation | matched | ADR018 § Identity, workspaces & account: Email recovery / verification |
| `verifyOneTimePassword` | mutation | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `verifyOTPRequest` | mutation | matched | ADR018 § Identity, workspaces & account: MFA (TOTP / passkeys) |
| `whoami` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `WorkspaceApiTokenScopes` | query | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata); fine-grained keys (`w9/m496`) |
| `workspaceAvatarsForOrganization` | query | gap | `w1/m648` (avatars) |
| `workspacesForUser` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |
| `workspacesForUserMinimal` | query | matched | ADR018 § Identity, workspaces & account: Login · sessions · account settings |

### Workspaces, members and organizations

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `acceptOrgInvite` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `acceptTeamInvite` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `authenticatedWithSSOForOrg` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `cancelOrgInvite` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `cancelTeamInvite` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `createOrgDomain` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `createSSOConnection` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `createTeam` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `currentWorkspaceForNav` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `deleteOrgDomain` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `deleteSSOConnection` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `deleteTeam` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `enforcedLoginCheck` | query | matched | ADR018 § Identity, workspaces & account: Organization-enforced 2FA and Google login |
| `grantPermissionsOrInvite` | mutation | flagged | `grandfather-owners-into-sharing` (and `share-postgres` for databases): Render's deprecated per-resource sharing, shown only to grandfathered owners; bex shares through workspace members |
| `handlePersonalWorkspaces` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `inviteUser` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `inviteUsersToOrg` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `inviteUsersToTeam` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `joinHintLookup` | query | matched | [ADR115](../ADR115-organizations.md): organizations; [organizations.md](organizations.md) |
| `joinTeam` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `organizationAPITokens` | query | gap | `w1/151` (organization audit log API keys and each org member's login status) |
| `organizationAuthenticationStatus` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `organizationById` | query | matched | [ADR115](../ADR115-organizations.md): organizations; [organizations.md](organizations.md) |
| `organizationSCIMTokens` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `organizationsForUser` | query | matched | [ADR115](../ADR115-organizations.md): organizations; [organizations.md](organizations.md) |
| `orgInvite` | query | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `orgUserAuthStatus` | query | gap | `w1/151` (organization audit log API keys and each org member's login status) |
| `orgUserDomainCaptured` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `ownerForNav` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `ownerIpAllowList` | query | matched | ADR018 § Identity, workspaces & account: Workspace inbound IP rules |
| `ownerPermissions` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `ownerSettings` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle; [workspace-lifecycle.md](workspace-lifecycle.md) |
| `ownerTier` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `permissionsGrantedForPostgres` | query | flagged | `grandfather-owners-into-sharing` (and `share-postgres` for databases): Render's deprecated per-resource sharing, shown only to grandfathered owners; bex shares through workspace members |
| `permissionsGrantedForService` | query | flagged | `grandfather-owners-into-sharing` (and `share-postgres` for databases): Render's deprecated per-resource sharing, shown only to grandfathered owners; bex shares through workspace members |
| `personalWorkspacesForOrgUser` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `provisionOrganizationAuditLogToken` | mutation | gap | `w1/151` (organization audit log API keys and each org member's login status) |
| `provisionOrganizationSCIMToken` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `removeOrgMember` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `removeUserFromTeam` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `revokeAllPermissions` | mutation | flagged | `grandfather-owners-into-sharing` (and `share-postgres` for databases): Render's deprecated per-resource sharing, shown only to grandfathered owners; bex shares through workspace members |
| `revokeOrganizationAPIToken` | mutation | gap | `w1/151` (organization audit log API keys and each org member's login status) |
| `revokeOrganizationSCIMToken` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `samlLoginURL` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `samlSSORequired` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `sendReminder2FAEnforcementNotificationMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace-enforced 2FA (`w1/m647`, GraphQL `sendWorkspaceTwoFactorReminders`) |
| `ssoConnectionsForOrganization` | query | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `team` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `teamInvite` | query | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `TeamSessionSecurity` | query | matched | ADR018 § Identity, workspaces & account: Workspace-enforced 2FA; Workspace-enforced Google login |
| `teamsForUser` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `teamsForUserWithRequirements` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `teamWithOrganization` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `updateAllowedLoginMethods` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace-enforced 2FA; Workspace-enforced Google login |
| `updateIs2FARequiredForOrg` | mutation | matched | ADR018 § Identity, workspaces & account: Organization-enforced 2FA and Google login |
| `updateIs2FARequiredTeamMutation` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace-enforced 2FA; Workspace-enforced Google login |
| `updateOrgAllowedLoginMethods` | mutation | matched | ADR018 § Identity, workspaces & account: Organization-enforced 2FA and Google login |
| `updateOrganizationName` | mutation | matched | [ADR115](../ADR115-organizations.md): organizations; [organizations.md](organizations.md) |
| `updateOrgUserRole` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `updateOwnerIpAllowList` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace inbound IP rules |
| `updateSSOConnection` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `updateTeamMemberRole` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `updateTeamOrgPrivacy` | mutation | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `updateTeamProfile` | mutation | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `usersAndPendingForTeam` | query | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `userWithOrgInvites` | query | matched | [ADR115](../ADR115-organizations.md) D2–D3: org members, invites and workspace privacy; ADR018 § Identity, workspaces & account: Workspace members & roles |
| `userWithTeamInvites` | query | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove) |
| `verifyOrgDomain` | mutation | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces |
| `workspacesForOrganization` | query | matched | [ADR115](../ADR115-organizations.md): organizations; [organizations.md](organizations.md) |
| `workspacesForUserForBillingUpdate` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |
| `workspacesForUserWithPricing` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan) |

### Billing and usage

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addPromotion` | mutation | non-goal | DO_NOT_DO: ADR071 tenant billing credits (Stripe admin issues credits by hand) |
| `chargeAndRemoveCard` | mutation | matched | ADR018 § Billing & invoicing: Billing page: plan, payment method, accrued charges, included usage |
| `connectStripeAccountMutation` | mutation | gap | `w1/150` (Render's Stripe integrations: the Stripe account connection and Stripe's agentic account link) |
| `customerInfo` | query | matched | ADR018 § Billing & invoicing: Billing page: plan, payment method, accrued charges, included usage |
| `disconnectStripeAccountMutation` | mutation | gap | `w1/150` (Render's Stripe integrations: the Stripe account connection and Stripe's agentic account link) |
| `EnableHIPAA` | mutation | gap | Render's HIPAA-enabled workspaces have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `enablePricingFY2027` | mutation | n/a | Render's own FY2027 price-list migration opt-in; bex prices are ADR030's |
| `GetBAALink` | mutation | gap | Render's HIPAA-enabled workspaces have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `hasStripeConnection` | query | gap | `w1/150` (Render's Stripe integrations: the Stripe account connection and Stripe's agentic account link) |
| `HIPAAStartBAAFlow` | mutation | gap | Render's HIPAA-enabled workspaces have no bex disposition yet: `w7/071` (parked on the user's build-or-non-goal decision) |
| `includedUsageOwner` | query | matched | ADR018 § Billing & invoicing: Billing page: plan, payment method, accrued charges, included usage |
| `invoice` | query | matched | ADR018 § Billing & invoicing: Invoices for past months: list, PDF, Pay Now |
| `invoicesForOwner` | query | matched | ADR018 § Billing & invoicing: Invoices for past months: list, PDF, Pay Now |
| `orgDefaultPaymentMethodAvailability` | query | matched | [ADR115](../ADR115-organizations.md): one org plan and payment method; ADR018 § Billing & invoicing: Who sees and manages billing |
| `ownerBilling` | query | matched | ADR018 § Billing & invoicing: Billing page: plan, payment method, accrued charges, included usage |
| `ownerSuspensionInfo` | query | matched | ADR018 § Billing & invoicing: Unpaid invoice, suspension and recovery |
| `tierUpdateWarnings` | query | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (change plan); [workspace-plan-change.md](workspace-plan-change.md) |
| `unbilledInvoiceForOwner` | query | matched | ADR018 § Billing & invoicing: Invoices for past months: list, PDF, Pay Now |
| `updateBillingInfo` | mutation | matched | ADR018 § Billing & invoicing: Billing info on invoices: company name, address, VAT or tax ID |
| `updateCard` | mutation | matched | ADR018 § Billing & invoicing: Billing page: plan, payment method, accrued charges, included usage |
| `updateOrgDefaultBillingEmail` | mutation | matched | [ADR115](../ADR115-organizations.md): one org plan and payment method; ADR018 § Billing & invoicing: Who sees and manages billing |
| `updateOrgDefaultPaymentMethod` | mutation | matched | [ADR115](../ADR115-organizations.md): one org plan and payment method; ADR018 § Billing & invoicing: Who sees and manages billing |
| `usageOwner` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails; [ADR030](../ADR030-pricing.md) |
| `usageOwnerProjects` | query | matched | ADR018 § Billing & invoicing: Included usage and approaching-limit emails; [ADR030](../ADR030-pricing.md) |
| `useOrgDefaultPaymentMethod` | mutation | matched | [ADR115](../ADR115-organizations.md): one org plan and payment method; ADR018 § Billing & invoicing: Who sees and manages billing |

### Search and regions

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `activeRegions` | query | matched | ADR018 § Services & lifecycle: Create service (region picker); bex serves one region (`fsn1`) |
| `regionCountForOwner` | query | matched | ADR018 § Services & lifecycle: Create service (region picker); bex serves one region (`fsn1`) |
| `servicesForOwnerSearch` | query | matched | [search-shortcuts.md](search-shortcuts.md): the dashboard's search across services, databases, projects and more |

### Render-internal

| Operation | Kind | Disposition | Pointer |
| --- | --- | --- | --- |
| `addFeatureFlagToOwner` | mutation | n/a | Render's Early Access opt-in list; it was empty on 2026-11-26 (no section renders). A feature that appears there gets its own rows |
| `adminLoginAs` | mutation | n/a | Render staff tooling (sign in as a user, flag overlays), not a customer surface |
| `allFrontendFlags` | query | n/a | Render staff tooling (sign in as a user, flag overlays), not a customer surface |
| `cannyToken` | query | n/a | Render's own support and feedback vendors (Intercom, Canny, surveys) |
| `documentDownloadURL` | mutation | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork, not a product surface |
| `documents` | query | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork, not a product surface |
| `firstServiceCreationCompleted` | query | n/a | Render's first-service onboarding experiment: whether to auto-create a project; growth UI, no contract |
| `ownerFeatureFlags` | query | n/a | Render's feature-flag plumbing; the flags themselves are rows in the Flag table |
| `ownerForIntercom` | query | n/a | Render's own support and feedback vendors (Intercom, Canny, surveys) |
| `ownerSettingsJSON` | query | n/a | Render's dashboard UI state kept server-side (dismissed banners, collapsed panels); no contract beyond the page itself |
| `removeFeatureFlagFromOwner` | mutation | n/a | Render's Early Access opt-in list; it was empty on 2026-11-26 (no section renders). A feature that appears there gets its own rows |
| `saveOwnerSettingsJSON` | mutation | n/a | Render's dashboard UI state kept server-side (dismissed banners, collapsed panels); no contract beyond the page itself |
| `saveUserSettingsJSON` | mutation | n/a | Render's dashboard UI state kept server-side (dismissed banners, collapsed panels); no contract beyond the page itself |
| `sessionFlagOverlayFlags` | query | n/a | Render staff tooling (sign in as a user, flag overlays), not a customer surface |
| `signedNDAActive` | query | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork, not a product surface |
| `signNDA` | mutation | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork, not a product surface |
| `submitSurveyResponse` | mutation | n/a | Render's own support and feedback vendors (Intercom, Canny, surveys) |
| `unbilledChargesForOwnerForIntercom` | query | n/a | Render's own support and feedback vendors (Intercom, Canny, surveys) |
| `userFeatureFlags` | query | n/a | Render's feature-flag plumbing; the flags themselves are rows in the Flag table |
| `userSettingsJSON` | query | n/a | Render's dashboard UI state kept server-side (dismissed banners, collapsed panels); no contract beyond the page itself |
