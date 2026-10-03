# w4 · m155 — Keep serving static sites Running after a failed publish

**Worker:** worker4 **Goal:** report the serving static release separately from a failed replacement publish **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bound the publish-failure change and audit shared consumers | 25m | — |
| t002 | Preserve prior static release status and durable failed-deploy verdict | 50m | t001 |
| t003 | Verify REST, GraphQL, MCP and dashboard Render parity | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout with live replay and cleanup | 20m | t004, t005 |

## Definition of done

Repeat the owned Free static-site journey below (new qa-prefixed name; public bex-co/bex repo, main, root examples/static-site, publish directory ".", no build inputs, auto-deploy Off):

- After its first successful publish, change the publish directory to a nonexistent directory. Once the replacement publish fails, the old URL still returns HTTP 200 with the same body; REST service, GraphQL service and MCP get_service report **Running** with the old revision. The dashboard header says **Service Running**, while Latest deploy still says **Failed**.
- The replacement deploy reaches **update_failed** with the specific missing-directory failureReason. The successful previous deploy remains live. Do not turn the failure into a successful deploy, build_failed, a timeout, or a canceled row.
- Restore "." and wait for a successful publish: the service reports Running, the replacement deploy becomes live, the prior successful deploy becomes deactivated, and the old failed row retains its diagnosis. Reload Settings and repeat the invalid-directory change; the same split between serving service and failed deploy holds.
- Suspend while the latest publish is failed: the dashboard says Suspended and the public URL stops serving site content. Resume restores the prior content without activating the failed revision. This run observed 404 while suspended; preserving traffic blocking is the assertion, not freezing 404 as the desired response (ADR029 specifies the suspended responder when available).
- Delete the fixture through the dashboard, verify API/public URL no longer serve it, wait for its exact App UID and owned Kubernetes artifacts to disappear, and revoke only the QA session.

First-ever publish failure, built-image extraction failures, concurrent/superseded releases and real origin outages were **not** probed in this sweep; t001/t005 require explicit tests before closeout, not a claim that they passed live.

## Source + Goal linkage

- **Source:** continuous user-requested $qa-find-bugs, sweep 23, 2026-10-02 America/Los_Angeles (2026-10-03 UTC), muse.env authentication, filing explicitly requested in w4. Severity **major**: the service's status falsely reports an outage on UI/REST/GraphQL/MCP while the published page works.
- **Goal linkage:** ADR008 reliable Render-alternative hosting; docs/ADR029-static-sites.md atomic immutable-prefix publication; docs/ADR004-app-deployment.md and docs/ADR006-bex-api.md truthful service/deploy state.
- **Expected outcome:** a failed replacement publish leaves the previously served release Running and preserves the failed deploy's exact reason independently.
- **Why now:** w6/m124 already established this separation for build failures; static publication still takes an unconditional failure branch. It misleads tenants into outage response despite successful serving.
- **Render parity:** included. [Render's deploy documentation](https://render.com/docs/deploys), checked 2026-10-03, says failed deploy commands leave the most recent successful deploy running. This supports release retention; the specific bex phase vocabulary is governed by the existing bex contract. No paid Render fixture was created.
- **Scope:** filing only; no product code changed in this QA run.

## Reproduction and evidence

Owned fixture: **qa-20261002-publish-r23**, **srv-db05l1ede41s73cao1fg**, workspace **tea-d98210cbbpdc73dcrkvg**, Free static_site. App **tea-d98210cbbpdc73dcrkvg-qa-20261002-publish-r23**, UID **02d0e7b3-5bd1-4e91-8cb5-49288f080936**. URL https://qa-20261002-publish-r23.onbex.co. Source commit e9021ce5f.

All times UTC on 2026-10-03:

| Time | Action / observed result |
| --- | --- |
| 01:29:42 | Created from dashboard, public repo, root examples/static-site, publish ".", no build command/runtime |
| 01:30:08 | First deploy dep-db05l1ede41s73cao1g0 live, rev-1, public HTTP 200 |
| 01:30:50 | Settings → Edit publish directory → qa-r23-missing-output → Save changes |
| 01:31:26 | dep-db05lik5o9vs73dt7ulg update_failed; specific clone missing-directory reason |
| 01:31:50 | Fresh page and GraphQL: Service Failed, revision rev-1, prior deploy live; external HTTP 200 |
| 01:32:30 | Restore publish "." |
| 01:32:56 | dep-db05mbk5o9vs73dt7umg live, service Running, rev-3; older live row deactivated |
| 01:33:23 | Reload Settings; change publish directory to qa-r23-missing-again |
| 01:33:55 | dep-db05mos5o9vs73dt7uo0 update_failed |
| 01:34:12–01:39:30 | GraphQL/REST/MCP phase Failed, revision rev-3; App Ready=False/PublishFailed; external HTTP 200, unchanged bytes |
| 01:39:44 | Suspend accepted; reloaded UI Suspended, public HTTP 404 (24 bytes) |
| 01:40:07 | Resume accepted; public HTTP 200 (1548 bytes), revision still rev-3 |
| 01:41:34 | Dashboard Delete Service succeeded |
| 01:41:58 | REST GET returned 404 not_found; public URL subsequently 404 |

The failed publish did **not** destroy the old site. The failure message correctly identifies the deliberate invalid input. Only the service availability status is wrong.

External GET body was 1548 bytes, SHA256 **1eb4041e1174aaf5fc0d19dc504fccca37749fd03e73974ed230433614d7e8bb**, including "Hello from bex". It matched after both failed publishes. The public prefix remained rev-3 during the second failure; this is the intended immutable-prefix retention, not evidence that the failed release became live.

Local supplementary artifacts (existence checked): .playwright-mcp/qa-r23-static-failed-serving.png (inspected; first failure rev-1), qa-r23-api-captures.json, qa-r23-failed-cr.json, qa-r23-second-failure.html, qa-r23-suspended.html, qa-r23-resumed.html, qa-r23-inventory-before-delete.json, qa-r23-inventory-final.json, qa-r23-network-final.txt. All are under .playwright-mcp; they are ignored and **not** the durable handoff. Exact requests and complete responses are embedded below.

Console after the journey contained only the intentional deleted-service GET 404. Normal mutation/probe requests returned 200; invalid publish failed asynchronously as expected.

## Root cause and implementation boundary

Research baseline **e9021ce5f**; origin/main rechecked with a clean pull 2026-10-03 01:40 UTC.

1. **Producer:** lego/operator/internal/controller/app_controller.go:3386–3387 routes terminal publish.PhaseFailed through r.fail(..., "PublishFailed", ...). r.fail:4975–5003 preserves a served release only for IsBuildFailureReason; PublishFailed is excluded and falls through to PhaseFailed plus Ready=False. This is independent of whether the prior immutable prefix serves.
2. **Serving remains correct:** reconcileStaticSite:3428–3434 returns before updating the active revision/prefix when publishing fails. lego/operator/internal/staticserver/resolver.go:78–90 excludes suspended sites and requires ActiveRevision; it serves the active prefix without gating on phase. staticServePrefix:140 honors Status.StaticPrefix. Do not change the server to stop serving merely to make the wrong phase truthful.
3. **Durable deploy verdict:** lego/backend/internal/store/reconciler.go:1166 reads recordedRolloutFailure before its phase switch; :843–850 requires False, a nonempty message and an exact nonzero release-generation match. deployCloseFailureReason:825 consumes the same verdict. Changing phase alone loses the existing Ready/PhaseFailed fallback (:1409 onward) and can strand or misclassify the failed row.
4. **Proposed narrow fix:** at the terminal static-publish failure boundary, retain the precise PublishFailed diagnosis as a durable release-scoped failure (ConditionRollout is the existing update-failure mechanism), and settle the previously served release's phase/Ready independently. Extend the contract comment in lego/types/v1alpha1/app_types.go:165–178 if Rollout now includes publication. Do not classify publication as a build failure. Use releaseHasServed (release_identity.go:297, ActiveRevision), **not Status.Image**: this live fixture never built an image.
5. **Consumer caveat:** supersededDeployStatus:1242 runs before recordedRolloutFailure and gives an earlier Build verdict priority before canceling superseded rows. Audit publication verdict ordering under generation advance explicitly; current-generation support is proven by reading the consumer, but overlap behavior needs a regression test. Avoid widening unrelated rollout semantics without a separate justified change.
6. **Controls:** settleFailureOverPriorRelease:5043 defaults no-Deployment types to Running; static sites have no Deployment. Preserve spec.Suspended and actual routing disposition instead of blindly invoking a helper that interprets no Deployment as serving. The observed suspension blocked traffic, so a fix must not regress that control. Do not claim a real origin outage is healthy merely because ActiveRevision is nonempty.
7. **Projection:** apps/service.go:1002,1043 reads App phase into AppView; apps/render.go:360 exposes it on REST/MCP; GraphQL Service.phase reads the same view. dashboard/src/features/services/lib/status.ts:205–227 maps Failed; deriveStatus:235 makes suspension win. No UI-only remapping based on old revision is sufficient.
8. **Persistence/dependency verified:** actual pinned k8s.io/apimachinery v0.35.0 pkg/api/meta/conditions.go:31 updates condition reason/message/observedGeneration even when status stays False; lastTransitionTime changes only with status. Thus the capture's old Ready transition time is expected and not proof of an old error. status_update.go:30 compares complete JSON status before PUT. updateStatusRetrying:4893 reapplies complete status on conflict while retaining UndeployedChanges; persist verdict+Ready together and test conflict/generation handling.

### Counted blast radius and aliases

Exhaustive production Go grep in lego/operator/internal/controller found **52 r.fail call sites**, **5 r.failStep calls** (one wrapper delegates to r.fail), and **3 settleFailureOverPriorRelease calls**. publishStaticRevision has **1 production caller**, reconcileStaticSite. Its failure exits are credential validation (3337), clone-secret relocation (3372), Ensure error (3378), terminal observation (3387). **Only the terminal missing-directory path was live-probed.** Scope the first implementation to terminal publication failure over a previously served static release; separately disposition infrastructure/setup errors rather than globally treating all 52 failures as healthy.

Resource-family matrix: static affected; web/private/worker/cron share App fail helpers and must retain existing semantics, but do not call publishStaticRevision; Postgres and Key Value have separate controllers/status models and are outside this writer. No live sibling failure matrix was run here.

Dashboard ServiceStatusBadge has **3 production JSX consumer sites**: service-detail-header.tsx:120, projects/resource-table.tsx:315, environments/manage-resources-dialog.tsx:165 (test mocks excluded). Only the detail header was live-verified for this symptom. Static canonical /static/:id and generic /services/:id aliases share service data; list/project and environment placements need parity checks.

API aliases: REST GET /v1/services/:id and list services, GraphQL service/services, MCP get_service/list_services project the same service view; deploy list/detail must retain their separate verdict. Only single-service reads on REST/GraphQL/MCP and GraphQL deploys were probed. Do not present unexercised aliases as observed.

Adjacent classes: first publish with no ActiveRevision must remain Failed; ordinary in-flight publish may stay Deploying until settlement (observed in captures); invalid path stays an update_failed user configuration error; setup/credential/routing errors and real serving/origin failures must not be masked. Authentication/authorization/not-found behavior remains unchanged; deleted-service 404 was verified. Cancellation and older-generation verdicts must not contaminate later healthy deploys.

## Dedupe and prior guarantees

Searched open and done .pm files for PublishFailed, publish failure, static/prior-release status; scanned **65 open README files** across workstreams; re-read DO_NOT_DO.md, the latest 40 dashboard/lego commits and git log -S PublishFailed. No open item covers this terminal static publish status gap, and no fix is waiting on main. No anti-goal applies.

- **w6/done/m124:** precedent for separating service phase from latest **build** failure, not a claim that its web-build implementation regressed. Entire DoD disposition: (1) web build-failure Running guarantee not re-probed; static publication analogue fails twice; (2) same header contradiction observed for static; (3) build_failed unprobed, static update_failed correctly retained today; (4) first-ever failure unprobed, required test; (5) genuine unhealthy instance/origin unprobed, must not be masked; (6) healthy control verified here via initial publish and recovery Running/live.
- **w4/done/m103 / w8/blocked/m44:** rollout termination/diagnosis siblings; current ConditionRollout mechanism is useful precedent, but neither files static publication. Do not duplicate their rollout work.
- **w1/done/m160:** served identity must be ActiveRevision, not merely a resolved image; retain that rule.
- **w1/done/m163:** entire DoD disposition: cron suspend/schedule requirement not re-probed in sweep 23; static blocking/resume passes for a terminal publish failure (in-flight/failed-build combinations not re-run); healthy cron and scalable sibling controls not re-run. Current 404 is an existing failed-publish routing residual relative to ADR029's 503 responder, not evidence of traffic leaking during suspension; no separate outage claim is filed.
- **w6/done/038:** specific publish reason is correct today; no duplicate message-quality finding.
- **w6/done/m100:** release vs metadata generation accounting remains required. This fixture reached metadata generation 6 after suspend/resume while failed publication still belonged to release generation 4; tests must preserve that attribution.

## Durable API evidence

The following records preserve the exact GraphQL request payloads (including Apollo batching), complete response bodies, explicit read probes, and the safe App status projection. Authentication material is excluded. Replay against a newly created owned fixture; the original was deleted.

```json
{
  "captures": [
    {
      "at": "2026-10-03T01:29:42.743Z",
      "request": "[{\"operationName\":\"CreateService\",\"variables\":{\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"name\":\"qa-20261002-publish-r23\",\"type\":\"static_site\",\"rootDir\":\"examples/static-site\",\"autoDeploy\":false,\"publishPath\":\".\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation CreateService($name: String!, $ownerId: String, $environmentId: String, $type: String, $repo: String, $image: String, $registryCredentialId: String, $branch: String, $rootDir: String, $runtime: String, $buildCommand: String, $startCommand: String, $dockerfilePath: String, $buildFilter: BuildFilterInput, $plan: String, $autoDeploy: Boolean, $schedule: String, $command: String, $publishPath: String, $port: Int, $envVars: [EnvVarInput], $secretFiles: [SecretFileInput]) {\\n  createService(\\n    name: $name\\n    ownerId: $ownerId\\n    environmentId: $environmentId\\n    type: $type\\n    repo: $repo\\n    image: $image\\n    registryCredentialId: $registryCredentialId\\n    branch: $branch\\n    rootDir: $rootDir\\n    runtime: $runtime\\n    buildCommand: $buildCommand\\n    startCommand: $startCommand\\n    dockerfilePath: $dockerfilePath\\n    buildFilter: $buildFilter\\n    plan: $plan\\n    autoDeploy: $autoDeploy\\n    schedule: $schedule\\n    command: $command\\n    publishPath: $publishPath\\n    port: $port\\n    envVars: $envVars\\n    secretFiles: $secretFiles\\n  ) {\\n    id\\n    name\\n    type\\n    phase\\n    projectId\\n    environmentId\\n    registryCredentialId\\n    latestDeployId\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db05l1ede41s73cao1fg\",\"latestDeployId\":\"dep-db05l1ede41s73cao1g0\",\"name\":\"qa-20261002-publish-r23\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"static_site\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:30:50.776Z",
      "request": "[{\"operationName\":\"SetPublishPath\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\"qa-r23-missing-output\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation SetPublishPath($id: String!, $publishPath: String!) {\\n  setPublishPath(id: $id, publishPath: $publishPath) {\\n    id\\n    publishPath\\n    revision\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"setPublishPath\":{\"__typename\":\"Service\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\"qa-r23-missing-output\",\"revision\":\"rev-1\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:32:30.535Z",
      "request": "[{\"operationName\":\"SetPublishPath\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\".\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation SetPublishPath($id: String!, $publishPath: String!) {\\n  setPublishPath(id: $id, publishPath: $publishPath) {\\n    id\\n    publishPath\\n    revision\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"setPublishPath\":{\"__typename\":\"Service\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\".\",\"revision\":\"rev-1\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:33:23.848Z",
      "request": "[{\"operationName\":\"SetPublishPath\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\"qa-r23-missing-again\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation SetPublishPath($id: String!, $publishPath: String!) {\\n  setPublishPath(id: $id, publishPath: $publishPath) {\\n    id\\n    publishPath\\n    revision\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"setPublishPath\":{\"__typename\":\"Service\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"publishPath\":\"qa-r23-missing-again\",\"revision\":\"rev-3\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:39:44.050Z",
      "request": "[{\"operationName\":\"SuspendService\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation SuspendService($id: String!, $confirm: String) {\\n  suspendService(id: $id, confirm: $confirm) {\\n    id\\n    suspended\\n    phase\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"suspendService\":{\"__typename\":\"Service\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Failed\",\"suspended\":\"suspended\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:40:07.133Z",
      "request": "[{\"operationName\":\"ResumeService\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation ResumeService($id: String!) {\\n  resumeService(id: $id) {\\n    id\\n    suspended\\n    phase\\n    __typename\\n  }\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"resumeService\":{\"__typename\":\"Service\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Failed\",\"suspended\":\"not_suspended\"}}}]\n"
    },
    {
      "at": "2026-10-03T01:41:34.788Z",
      "request": "[{\"operationName\":\"DeleteService\",\"variables\":{\"id\":\"srv-db05l1ede41s73cao1fg\"},\"extensions\":{\"clientLibrary\":{\"name\":\"@apollo/client\",\"version\":\"4.1.3\"}},\"query\":\"mutation DeleteService($id: String!, $confirm: String) {\\n  deleteService(id: $id, confirm: $confirm)\\n}\"}]",
      "status": 200,
      "body": "[{\"data\":{\"deleteService\":true}}]\n"
    }
  ],
  "reads": [
    {
      "at": "2026-10-03T01:31:13.464Z",
      "query": "query($id:String!){service(id:$id){id phase revision publishPath} deploys(serviceId:$id){id status trigger failureReason createdAt startedAt finishedAt}}",
      "variables": {
        "id": "srv-db05l1ede41s73cao1fg"
      },
      "status": 200,
      "body": "{\"data\":{\"deploys\":[{\"createdAt\":\"2026-10-03T01:30:50.658484Z\",\"failureReason\":\"\",\"finishedAt\":\"\",\"id\":\"dep-db05lik5o9vs73dt7ulg\",\"startedAt\":\"2026-10-03T01:30:56.006064Z\",\"status\":\"update_in_progress\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:29:41.964041Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:30:08.28127Z\",\"id\":\"dep-db05l1ede41s73cao1g0\",\"startedAt\":\"2026-10-03T01:29:56.024785Z\",\"status\":\"live\",\"trigger\":\"create\"}],\"service\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Deploying\",\"publishPath\":\"qa-r23-missing-output\",\"revision\":\"rev-1\"}}}\n"
    },
    {
      "at": "2026-10-03T01:31:50.284Z",
      "query": "query($id:String!){service(id:$id){id phase revision publishPath} deploys(serviceId:$id){id status trigger failureReason createdAt startedAt finishedAt}}",
      "variables": {
        "id": "srv-db05l1ede41s73cao1fg"
      },
      "status": 200,
      "body": "{\"data\":{\"deploys\":[{\"createdAt\":\"2026-10-03T01:30:50.658484Z\",\"failureReason\":\"clone: the publish directory \\\"examples/static-site/qa-r23-missing-output\\\" does not exist in the repository at \\\"main\\\" (exit 2)\",\"finishedAt\":\"2026-10-03T01:31:26.010264Z\",\"id\":\"dep-db05lik5o9vs73dt7ulg\",\"startedAt\":\"2026-10-03T01:30:56.006064Z\",\"status\":\"update_failed\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:29:41.964041Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:30:08.28127Z\",\"id\":\"dep-db05l1ede41s73cao1g0\",\"startedAt\":\"2026-10-03T01:29:56.024785Z\",\"status\":\"live\",\"trigger\":\"create\"}],\"service\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Failed\",\"publishPath\":\"qa-r23-missing-output\",\"revision\":\"rev-1\"}}}\n"
    },
    {
      "at": "2026-10-03T01:33:02.190Z",
      "query": "query($id:String!){service(id:$id){id phase revision publishPath} deploys(serviceId:$id){id status trigger failureReason createdAt startedAt finishedAt}}",
      "variables": {
        "id": "srv-db05l1ede41s73cao1fg"
      },
      "status": 200,
      "body": "{\"data\":{\"deploys\":[{\"createdAt\":\"2026-10-03T01:32:30.406748Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:32:56.015375Z\",\"id\":\"dep-db05mbk5o9vs73dt7umg\",\"startedAt\":\"2026-10-03T01:32:37.493219Z\",\"status\":\"live\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:30:50.658484Z\",\"failureReason\":\"clone: the publish directory \\\"examples/static-site/qa-r23-missing-output\\\" does not exist in the repository at \\\"main\\\" (exit 2)\",\"finishedAt\":\"2026-10-03T01:31:26.010264Z\",\"id\":\"dep-db05lik5o9vs73dt7ulg\",\"startedAt\":\"2026-10-03T01:30:56.006064Z\",\"status\":\"update_failed\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:29:41.964041Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:30:08.28127Z\",\"id\":\"dep-db05l1ede41s73cao1g0\",\"startedAt\":\"2026-10-03T01:29:56.024785Z\",\"status\":\"deactivated\",\"trigger\":\"create\"}],\"service\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Running\",\"publishPath\":\".\",\"revision\":\"rev-3\"}}}\n"
    },
    {
      "at": "2026-10-03T01:33:47.968Z",
      "query": "query($id:String!){service(id:$id){id phase revision publishPath} deploys(serviceId:$id){id status trigger failureReason createdAt startedAt finishedAt}}",
      "variables": {
        "id": "srv-db05l1ede41s73cao1fg"
      },
      "status": 200,
      "body": "{\"data\":{\"deploys\":[{\"createdAt\":\"2026-10-03T01:33:23.71754Z\",\"failureReason\":\"\",\"finishedAt\":\"\",\"id\":\"dep-db05mos5o9vs73dt7uo0\",\"startedAt\":\"2026-10-03T01:33:25.946515Z\",\"status\":\"update_in_progress\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:32:30.406748Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:32:56.015375Z\",\"id\":\"dep-db05mbk5o9vs73dt7umg\",\"startedAt\":\"2026-10-03T01:32:37.493219Z\",\"status\":\"live\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:30:50.658484Z\",\"failureReason\":\"clone: the publish directory \\\"examples/static-site/qa-r23-missing-output\\\" does not exist in the repository at \\\"main\\\" (exit 2)\",\"finishedAt\":\"2026-10-03T01:31:26.010264Z\",\"id\":\"dep-db05lik5o9vs73dt7ulg\",\"startedAt\":\"2026-10-03T01:30:56.006064Z\",\"status\":\"update_failed\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:29:41.964041Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:30:08.28127Z\",\"id\":\"dep-db05l1ede41s73cao1g0\",\"startedAt\":\"2026-10-03T01:29:56.024785Z\",\"status\":\"deactivated\",\"trigger\":\"create\"}],\"service\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Deploying\",\"publishPath\":\"qa-r23-missing-again\",\"revision\":\"rev-3\"}}}\n"
    },
    {
      "at": "2026-10-03T01:34:12.529Z",
      "query": "query($id:String!){service(id:$id){id phase revision publishPath} deploys(serviceId:$id){id status trigger failureReason createdAt startedAt finishedAt}}",
      "variables": {
        "id": "srv-db05l1ede41s73cao1fg"
      },
      "status": 200,
      "body": "{\"data\":{\"deploys\":[{\"createdAt\":\"2026-10-03T01:33:23.71754Z\",\"failureReason\":\"clone: the publish directory \\\"examples/static-site/qa-r23-missing-again\\\" does not exist in the repository at \\\"main\\\" (exit 2)\",\"finishedAt\":\"2026-10-03T01:33:55.982771Z\",\"id\":\"dep-db05mos5o9vs73dt7uo0\",\"startedAt\":\"2026-10-03T01:33:25.946515Z\",\"status\":\"update_failed\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:32:30.406748Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:32:56.015375Z\",\"id\":\"dep-db05mbk5o9vs73dt7umg\",\"startedAt\":\"2026-10-03T01:32:37.493219Z\",\"status\":\"live\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:30:50.658484Z\",\"failureReason\":\"clone: the publish directory \\\"examples/static-site/qa-r23-missing-output\\\" does not exist in the repository at \\\"main\\\" (exit 2)\",\"finishedAt\":\"2026-10-03T01:31:26.010264Z\",\"id\":\"dep-db05lik5o9vs73dt7ulg\",\"startedAt\":\"2026-10-03T01:30:56.006064Z\",\"status\":\"update_failed\",\"trigger\":\"config_change\"},{\"createdAt\":\"2026-10-03T01:29:41.964041Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-03T01:30:08.28127Z\",\"id\":\"dep-db05l1ede41s73cao1g0\",\"startedAt\":\"2026-10-03T01:29:56.024785Z\",\"status\":\"deactivated\",\"trigger\":\"create\"}],\"service\":{\"id\":\"srv-db05l1ede41s73cao1fg\",\"phase\":\"Failed\",\"publishPath\":\"qa-r23-missing-again\",\"revision\":\"rev-3\"}}}\n"
    },
    {
      "at": "2026-10-03T01:34:30.856Z",
      "method": "GET",
      "url": "https://api.bex.co/v1/services/srv-db05l1ede41s73cao1fg",
      "status": 200,
      "body": "{\"id\":\"srv-db05l1ede41s73cao1fg\",\"name\":\"qa-20261002-publish-r23\",\"immutableName\":\"qa-20261002-publish-r23\",\"slug\":\"qa-20261002-publish-r23\",\"displayName\":\"\",\"type\":\"static_site\",\"suspended\":\"not_suspended\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db05l1ede41s73cao1fg\",\"createdAt\":\"2026-10-03T01:29:42Z\",\"updatedAt\":\"2026-10-03T01:34:10Z\",\"owner\":{\"id\":\"tea-d98210cbbpdc73dcrkvg\",\"name\":\"bex\",\"email\":\"puncsky@gmail.com\",\"type\":\"team\"},\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\"qa-r23-missing-again\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261002-publish-r23.onbex.co\"},\"suspenders\":[],\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Failed\",\"replicas\":1,\"revision\":\"rev-3\",\"urls\":[\"https://qa-20261002-publish-r23.onbex.co\"],\"idleTTLSeconds\":0,\"rootDir\":\"examples/static-site\",\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"pushDeliveryMethod\":\"github_app\",\"notifyOnFail\":\"default\",\"notificationsToSend\":\"default\"}\n"
    },
    {
      "at": "2026-10-03T01:39:29.076Z",
      "request": {
        "jsonrpc": "2.0",
        "id": 23,
        "method": "tools/call",
        "params": {
          "name": "get_service",
          "arguments": {
            "serviceId": "srv-db05l1ede41s73cao1fg"
          }
        }
      },
      "response": {
        "status": 200,
        "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":23,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-03T01:29:42Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/static/srv-db05l1ede41s73cao1fg\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-db05l1ede41s73cao1fg\\\",\\\"idleTTLSeconds\\\":0,\\\"immutableName\\\":\\\"qa-20261002-publish-r23\\\",\\\"name\\\":\\\"qa-20261002-publish-r23\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Failed\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-3\\\",\\\"rootDir\\\":\\\"examples/static-site\\\",\\\"serviceDetails\\\":{\\\"buildCommand\\\":\\\"\\\",\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"publishPath\\\":\\\"qa-r23-missing-again\\\",\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"url\\\":\\\"https://qa-20261002-publish-r23.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261002-publish-r23\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"static_site\\\",\\\"updatedAt\\\":\\\"2026-10-03T01:39:18Z\\\",\\\"urls\\\":[\\\"https://qa-20261002-publish-r23.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-03T01:29:42Z\",\"dashboardUrl\":\"https://dashboard.bex.co/static/srv-db05l1ede41s73cao1fg\",\"displayName\":\"\",\"id\":\"srv-db05l1ede41s73cao1fg\",\"idleTTLSeconds\":0,\"immutableName\":\"qa-20261002-publish-r23\",\"name\":\"qa-20261002-publish-r23\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Failed\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-3\",\"rootDir\":\"examples/static-site\",\"serviceDetails\":{\"buildCommand\":\"\",\"numInstances\":1,\"plan\":\"free\",\"publishPath\":\"qa-r23-missing-again\",\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"url\":\"https://qa-20261002-publish-r23.onbex.co\"},\"slug\":\"qa-20261002-publish-r23\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"static_site\",\"updatedAt\":\"2026-10-03T01:39:18Z\",\"urls\":[\"https://qa-20261002-publish-r23.onbex.co\"]}}}\n\n"
      }
    },
    {
      "at": "2026-10-03T01:41:58.706Z",
      "request": {
        "method": "GET",
        "url": "https://api.bex.co/v1/services/srv-db05l1ede41s73cao1fg"
      },
      "response": {
        "status": 404,
        "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      }
    }
  ]
}
```

App status projection (kubectl get app, metadata + status only):

```json
{
  "at": "2026-10-03T01:39:30.804762+00:00",
  "metadata": {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261002-publish-r23",
    "namespace": "tea-d98210cbbpdc73dcrkvg",
    "uid": "02d0e7b3-5bd1-4e91-8cb5-49288f080936",
    "generation": 4
  },
  "status": {
    "activeRevision": "rev-3",
    "artifactFingerprint": "artifact-v1:62b27578c0dae242499c3635daacb84c3444c6c5b932742a373c221b2e7caa99",
    "conditions": [
      {
        "lastTransitionTime": "2026-10-03T01:33:23Z",
        "message": "clone: the publish directory \"examples/static-site/qa-r23-missing-again\" does not exist in the repository at \"main\" (exit 2)",
        "observedGeneration": 4,
        "reason": "PublishFailed",
        "status": "False",
        "type": "Ready"
      },
      {
        "lastTransitionTime": "2026-10-03T01:29:58Z",
        "message": "serving at qa-20261002-publish-r23.onbex.co",
        "observedGeneration": 3,
        "reason": "Routed",
        "status": "True",
        "type": "PublicRouting"
      }
    ],
    "observedGeneration": 3,
    "phase": "Failed",
    "releaseArtifactFingerprint": "artifact-v1:62b27578c0dae242499c3635daacb84c3444c6c5b932742a373c221b2e7caa99",
    "releaseFingerprint": "release-v1:3126fc81f74de9211b5e4f8296de8585270c28acd3683139ae057d3302d1caf9",
    "releaseGeneration": 4,
    "staticPrefix": "tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261002-publish-r23/rev-3/",
    "url": "https://qa-20261002-publish-r23.onbex.co",
    "urls": ["https://qa-20261002-publish-r23.onbex.co"]
  }
}
```

## Cleanup

Dashboard deletion accepted at 01:41:34 UTC; API/public URL returned 404. At **01:42:52 UTC**, exact-fixture cluster inventory returned **items: []**, **secretsMetadataOnly: []** (App, publication Jobs, pods, aliases, Ingress/certificate and credentials gone). Static deletion needed its asynchronous purge/finalization; no manual cluster deletion was necessary. QA logout returned **ok logged-out**; session files removed and browser cookies cleared.
