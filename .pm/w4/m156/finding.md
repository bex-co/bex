# Successful image deploys report a zero-second execution window

Why: a tenant cannot use deploy durations or log chronology to investigate a rollout when its start is recorded after the application has already started and become ready.

**Severity:** minor (reporting; service creation, restart, and HTTP serving succeeded). **Observed:** 2026-10-03 UTC, sweep 34, requested muse.env QA identity in bex workspace `tea-d98210cbbpdc73dcrkvg`. **Research HEAD:** `727ec4bd4`.

## Reproduction and independent evidence

Create a Free image web service named `qa-20261002-image-r34`, image `traefik/whoami:v1.10.1`, port 8080 and `WHOAMI_PORT_NUMBER=8080`. Wait for Live, then Manual Deploy → Restart service. Open the new deploy detail from a fresh navigation. Both deployments reported identical start and finish timestamps; the restarted deployment displayed **Duration: 0s**. Both public requests returned HTTP 200 with the expected whoami response.

- Service `srv-db075jrajv7s73eg1e30`, first deploy `dep-db075jrajv7s73eg1e3g`, restart `dep-db076b05od3c73dqi5ug`.
- First: created 03:13:19.488051Z, started and finished 03:13:38.985458Z.
- Restart: created 03:14:52.339929Z, started and finished 03:15:08.748639Z.
- Restart log order: 03:14:52 Deploy queued; **03:14:55 Starting up on port 8080**; **03:15:08 Deploying image**; 03:15:08 Your service is live.
- Independent read-only Kubernetes observation of this owned revision: pod created 03:14:52Z, app container started 03:14:55Z, Ready 03:15:02Z. Thus execution demonstrably preceded the advertised start. Pod timings prove the mismatch; they are not a proposal to substitute pod start for every deploy's start.
- Screenshot inspected: `.playwright-mcp/qa-r34-zero-duration.png`; complete safe API capture `.playwright-mcp/qa-r34-api-captures.json`; pod metadata `.playwright-mcp/qa-r34-pod-timing.json`; network `.playwright-mcp/qa-r34-network.txt`. Durable requests/responses are below, since these local files are ignored.
- Native build sweep 33 rendered a 1m29s duration and passed Save-only→manual-deploy behavior. That was a UI control, not a raw final timestamp capture; it does **not** establish that all native deploys are correct. Existing-start preservation is established from the SQL below and must be regression-tested directly.

## Root and target behavior

`lego/backend/internal/store/deploy_lifecycle.go:79-87` explicitly includes **live** in `DeployStatusStampsDispatch`. `store.go:2304-2323` consequently uses `COALESCE(started_at, clock_timestamp())` for a first observation at Live, and stamps `finished_at` in the same UPDATE. The two independent clock calls may return equal or near-equal values; exact equality is not required for the defect. This is a sampled-state transition, not evidence that work began at that instant. The row-lock/transition guard and existing-start COALESCE are correct and must remain.

`reconciler.go:759-767` supplies recorded build-start evidence only for `build_failed`; Live receives nil. `buildRunStart` at `:1356` already provides generation-checked build-window evidence. `buildStartedAt` at `:958-996` independently uses the stamp predicate for queued rows and uses CreatedAt for never-queued rows, so changing only the SQL predicate is insufficient for coherent build facts. `buildLifecycleFacts:896-907` correctly suppresses build facts when the deploy's own Image is nonempty; preserve that per-deploy guard (including repo-backed rollback/reuse).

**Specified fix:** for future successful terminal skips with no prior start, preserve a trustworthy start belonging to this deploy/release if one exists; otherwise retain SQL NULL and render unknown duration. Never invent a start from the Live observation, finish time, deploy creation time, another generation's build window, or an earlier rollout's pod. Preserve an already-recorded start byte-for-byte. Keep existing in-progress observation semantics and eleven wire statuses. Apply the same evidence rule to build facts, retaining the distinction between image/reuse rollouts and actual builds. Do not add a new operator rollout-timestamp contract merely to avoid an honest unknown, and do not rewrite historical rows without evidence.

## Consumers, schema, dependencies, and blast radius

- Production `TransitionDeploy` call sites: **3** — reconciler timeout closure (`reconciler.go:651`), ordinary observation (`:767`), and `PGStore.CloseDeploy` (`store.go:2361`). CloseDeploy has **1** production invocation (`deploys/service.go:942`, cancellation). Test memStore mirrors the behavior in `store/fake_test.go:832`. `DeployStatusStampsDispatch` has **2** production callers, the PG store and `buildStartedAt`. The change is global to successful terminal skips, not an image-only UI workaround.
- Database `migrations/0005_deploys.up.sql:18` permits NULL; DeployView has `*time.Time` (`deploys/service.go:125,166`). REST `renderDeploy.StartedAt` has `omitempty` (`rest.go:72`); its `formatTimePtr:107` returns empty string for nil. GraphQL `graphql.go:50` uses nullable String but currently returns **empty string**, not null, through that formatter. Keep this established missing-value encoding unless separately justified. MCP get/list share `toRenderDeploy` (`mcp.go:104-159`). No adapter may synthesize a replacement timestamp.
- Actual pinned framework read: pgx **v5.10.0** `tx.go:391` BeginFunc calls the transaction callback then commits/rolls back; it does not choose timestamp values. graphql-go **v0.8.1** `executor.go:869` serializes through the scalar and `scalars.go:307` preserves empty string. The faulty clock choice is application SQL, not serializer rounding. REST evidence retains subsecond precision.
- Deploy detail/header and list are the **2 production callers** of `formatDeployDuration` (`deploy-header.tsx:79`, `deploys-list-page.tsx:85`). Its `deploy-presentation.ts:38-57,94-98` returns null on empty/missing timestamps; header/list already render the absent duration. `deploy-timeline.ts:41` gates the start step on truthiness. Verify these consumers, do not replace 0 with a guessed duration in React.
- Logs `internal/logs/progress.go:273-295` only emit Building/Deploying-image/Rolling-out banners when StartedAt is nonzero. Missing evidence must omit the fabricated start banner while preserving queued/end and actual app logs. This directly clears the observed inverted chronology.
- Events reads (`store/events.go:272,320,577`, `events/service.go:874`, REST/GraphQL Details adapters) carry the same nullable start. Product analytics trigger `migrations/0114_product_analytics.up.sql:60-66` derives duration_ms only when start exists; future unknown starts should remain unknown there too. No historical analytics backfill is part of this fix. Webhook/metrics build markers use the build facts; verify their chronology without fabricating unseen start events.
- Read aliases: REST GET `/v1/services/{id}/deploys` and `/{deployId}`; GraphQL `deploys(serviceId:)` and `deploy(serviceId:,deployId:)`; MCP `list_deploys` and `get_deploy`; dashboard history, detail, and Events. Create/manual/restart/rollback/cancel responses also project the same DeployView/renderDeploy, so do not introduce an adapter-only correction.
- Resource census: web, static, cron, worker, and private use App deploy history and this shared reconciler. Postgres and Key Value use their own Database lifecycles, not this App deploy-start path. Live reproduction here covers **image web only**; other App families and build/reuse transitions require focused tests, not claims of live verification.
- Adjacent states: queued has no execution proof; known in-progress keeps its start; Live without a start uses owned evidence or unknown; subsequent deactivation preserves that result; canceled/superseded and all three failure classes retain existing evidence/unknown semantics and reason taxonomy. Authorization, foreign/missing IDs, rate limits, and timeout error classification are unchanged; no new resource-existence disclosure. Before fresh query settlement, existing loading/cached behavior remains; do not optimistically fabricate a start.

## Dedupe and precedent disposition

Searched all `.pm` open, blocked and done content for zero duration, terminal skip, identical timestamps, and start/live stamping; scanned open milestone titles across workstreams, reread DO_NOT_DO, reviewed 40 product commits and targeted history (`git log -S DeployStatusStampsDispatch`). No current filing or newer implementation covers this successful-skip gap. No anti-goal applies.

Closest predecessor **w6/done/m123** deliberately repaired terminal **failure** skips and explicitly retained Live stamping (`bcd05c6cf`; `TestDeployStatusStampingSplit` still requires it). This is an uncovered sibling, not evidence the failed-build fix regressed. Full predecessor DoD disposition: failure reason, actual failed-build duration, builder error logs, failed-build banner order, tenant/infra classification were **not rerun** and remain required controls; successful-deploy preservation applies to already evidenced starts, which must remain untouched, while the newly demonstrated no-start success case is the additional gap. No claims that those five unrelated guarantees failed. `w6/done/035` explains why CreatedAt is not a safe fallback after queueing. `w6/done/m128` cancellation evidence and `w4/blocked/m110` image-versus-build narration stay intact. `w7/done/051` happens to show a rollback with 0s but filed trigger capitalization only; it is not ownership of this defect. `w1/done/m149` concerns pre-deploy failure messages, not successful execution windows.

## Render and scope

[Render deploy documentation](https://render.com/docs/deploys), checked 2026-10-03, documents deploy history and the build/pre-deploy/start sequence. Repository comparison `docs/render-artifacts/deploy-detail-page.md` records the four timestamps and shared duration presentation; ADR004 and ADR018 govern the local deploy contract. Render's public documentation does not define exact timing under a missed observation, and no authenticated Render duration experiment was run. Do not claim a vendor precision guarantee. This fix makes Bex's own evidenced facts honest while preserving API compatibility.

## Cleanup and limits

Deleted this owned service through the dashboard (`deleteService` returned true), GET service and public URL both 404, owned cluster inventory empty, and session revoked with the login helper. The post-logout 401/login-flow 403 are expected revocation fallout. The service admission/restart success is a narrow control for w4/m142, not completion of its full DoD. No product code was changed by this QA filing.

## Complete captured API probes

Authenticated browser requests; authentication headers/cookies intentionally excluded. Exact JSON bodies and full response bodies follow. GraphQL endpoint is `POST https://api.bex.co/graphql`, Content-Type application/json; direct REST is GET with no body; MCP POST uses Content-Type application/json and Accept application/json,text/event-stream.

### Capture 1

```json
{
  "at": "2026-10-03T03:13:20.043Z",
  "request": [
    {
      "operationName": "CreateService",
      "variables": {
        "image": "traefik/whoami:v1.10.1",
        "name": "qa-20261002-image-r34",
        "type": "web_service",
        "registryCredentialId": "",
        "runtime": "image",
        "plan": "free",
        "port": 8080,
        "envVars": [
          {
            "key": "WHOAMI_PORT_NUMBER",
            "value": "8080"
          }
        ],
        "ownerId": "tea-d98210cbbpdc73dcrkvg"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation CreateService($name: String!, $ownerId: String, $environmentId: String, $type: String, $repo: String, $image: String, $registryCredentialId: String, $branch: String, $rootDir: String, $runtime: String, $buildCommand: String, $startCommand: String, $dockerfilePath: String, $buildFilter: BuildFilterInput, $plan: String, $autoDeploy: Boolean, $schedule: String, $command: String, $publishPath: String, $port: Int, $envVars: [EnvVarInput], $secretFiles: [SecretFileInput]) {\n  createService(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    type: $type\n    repo: $repo\n    image: $image\n    registryCredentialId: $registryCredentialId\n    branch: $branch\n    rootDir: $rootDir\n    runtime: $runtime\n    buildCommand: $buildCommand\n    startCommand: $startCommand\n    dockerfilePath: $dockerfilePath\n    buildFilter: $buildFilter\n    plan: $plan\n    autoDeploy: $autoDeploy\n    schedule: $schedule\n    command: $command\n    publishPath: $publishPath\n    port: $port\n    envVars: $envVars\n    secretFiles: $secretFiles\n  ) {\n    id\n    name\n    type\n    phase\n    projectId\n    environmentId\n    registryCredentialId\n    latestDeployId\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "createService": {
          "__typename": "Service",
          "environmentId": null,
          "id": "srv-db075jrajv7s73eg1e30",
          "latestDeployId": "dep-db075jrajv7s73eg1e3g",
          "name": "qa-20261002-image-r34",
          "phase": "",
          "projectId": null,
          "registryCredentialId": "",
          "type": "web_service"
        }
      }
    }
  ]
}
```

### Capture 2

```json
{
  "at": "2026-10-03T03:14:52.498Z",
  "request": [
    {
      "operationName": "RestartServer",
      "variables": {
        "serviceId": "srv-db075jrajv7s73eg1e30"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RestartServer($serviceId: String!) {\n  restartServer(serviceId: $serviceId) {\n    id\n    status\n    createdAt\n    trigger\n    rollbackOf\n    image\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "restartServer": {
          "__typename": "Deploy",
          "createdAt": "2026-10-03T03:14:52.339929Z",
          "id": "dep-db076b05od3c73dqi5ug",
          "image": "traefik/whoami:v1.10.1",
          "rollbackOf": "",
          "status": "created",
          "trigger": "api"
        }
      }
    }
  ]
}
```

### Capture 3

```json
{
  "request": {
    "query": "query($s:String!,$a:String!,$b:String!){first:deploy(serviceId:$s,deployId:$a){id status trigger createdAt startedAt finishedAt} restart:deploy(serviceId:$s,deployId:$b){id status trigger createdAt startedAt finishedAt}}",
    "variables": {
      "s": "srv-db075jrajv7s73eg1e30",
      "a": "dep-db075jrajv7s73eg1e3g",
      "b": "dep-db076b05od3c73dqi5ug"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "first": {
        "createdAt": "2026-10-03T03:13:19.488051Z",
        "finishedAt": "2026-10-03T03:13:38.985458Z",
        "id": "dep-db075jrajv7s73eg1e3g",
        "startedAt": "2026-10-03T03:13:38.985458Z",
        "status": "deactivated",
        "trigger": "create"
      },
      "restart": {
        "createdAt": "2026-10-03T03:14:52.339929Z",
        "finishedAt": "2026-10-03T03:15:08.748639Z",
        "id": "dep-db076b05od3c73dqi5ug",
        "startedAt": "2026-10-03T03:15:08.748639Z",
        "status": "live",
        "trigger": "api"
      }
    }
  }
}
```

### Capture 4

```json
{
  "url": "https://api.bex.co/v1/services/srv-db075jrajv7s73eg1e30/deploys/dep-db076b05od3c73dqi5ug",
  "status": 200,
  "response": "{\"id\":\"dep-db076b05od3c73dqi5ug\",\"serviceId\":\"srv-db075jrajv7s73eg1e30\",\"status\":\"live\",\"trigger\":\"api\",\"image\":{\"ref\":\"traefik/whoami:v1.10.1\"},\"createdAt\":\"2026-10-03T03:14:52.339929Z\",\"updatedAt\":\"2026-10-03T03:15:08.74864Z\",\"startedAt\":\"2026-10-03T03:15:08.748639Z\",\"finishedAt\":\"2026-10-03T03:15:08.748639Z\"}\n"
}
```

### Capture 5

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 34,
    "method": "tools/call",
    "params": {
      "name": "get_deploy",
      "arguments": {
        "serviceId": "srv-db075jrajv7s73eg1e30",
        "deployId": "dep-db076b05od3c73dqi5ug"
      }
    }
  },
  "status": 200,
  "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":34,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"createdAt\\\":\\\"2026-10-03T03:14:52.339929Z\\\",\\\"finishedAt\\\":\\\"2026-10-03T03:15:08.748639Z\\\",\\\"id\\\":\\\"dep-db076b05od3c73dqi5ug\\\",\\\"image\\\":{\\\"ref\\\":\\\"traefik/whoami:v1.10.1\\\"},\\\"serviceId\\\":\\\"srv-db075jrajv7s73eg1e30\\\",\\\"startedAt\\\":\\\"2026-10-03T03:15:08.748639Z\\\",\\\"status\\\":\\\"live\\\",\\\"trigger\\\":\\\"api\\\",\\\"updatedAt\\\":\\\"2026-10-03T03:15:08.74864Z\\\"}\"}],\"structuredContent\":{\"createdAt\":\"2026-10-03T03:14:52.339929Z\",\"finishedAt\":\"2026-10-03T03:15:08.748639Z\",\"id\":\"dep-db076b05od3c73dqi5ug\",\"image\":{\"ref\":\"traefik/whoami:v1.10.1\"},\"serviceId\":\"srv-db075jrajv7s73eg1e30\",\"startedAt\":\"2026-10-03T03:15:08.748639Z\",\"status\":\"live\",\"trigger\":\"api\",\"updatedAt\":\"2026-10-03T03:15:08.74864Z\"}}}\n\n"
}
```

### Independent owned pod timing

```json
[
  {
    "name": "tea-d98210cbbpdc73dcrkvg-qa-20261002-image-r34-7f9485675c-9sxhl",
    "uid": "d278f337-062b-4aca-9621-79c4208a4fe2",
    "created": "2026-10-03T03:14:52Z",
    "conditions": [
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-03T03:14:55Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "PodReadyToStartContainers"
      },
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-03T03:14:52Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "Initialized"
      },
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-03T03:15:02Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "Ready"
      },
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-03T03:15:02Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "ContainersReady"
      },
      {
        "lastProbeTime": null,
        "lastTransitionTime": "2026-10-03T03:14:52Z",
        "observedGeneration": 1,
        "status": "True",
        "type": "PodScheduled"
      }
    ],
    "containers": [
      {
        "name": "app",
        "state": {
          "running": {
            "startedAt": "2026-10-03T03:14:55Z"
          }
        }
      }
    ]
  }
]
```
