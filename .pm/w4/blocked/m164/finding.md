# Sweep 71: a free web service can sleep before its last request's idle window expires

- **Severity: major.** Real premature scale-to-zero and avoidable activator 503/cold-start behavior, reproduced on two independent Free web services. Not a paid-plan or authorization bug.
- **Where:** `https://dashboard.bex.co`, QA workspace `tea-d98210cbbpdc73dcrkvg`, 2026-10-04 UTC (2026-10-03 local). Fixture A `srv-db0q5p9rgajs73fr34jg`, `qa-20261003-wake-r71`; fixture B `srv-db0qb29rgajs73fr34rg`, `qa-20261003-wake-r71b`. Both created through dashboard, Free Go, repo `https://github.com/bex-co/bex`, branch main, root `examples/hello-go`, build `go build -o app .`, start `./app`, port 3000, auto-deploy Off.
- **Expected:** the positive idle timeout runs from the last inbound activity, including the first request to a new metric series. A quiet service eventually sleeps; subsequent inbound traffic wakes it.
- **Actual:** A initially Running at **00:53:19Z**, dashboard-selected/persisted TTL **300**. GET **00:53:46.38055Z** and HEAD **00:53:49.18023Z** returned 200. Hibernated condition at **00:58:19Z**, last-active still **00:53:19Z**: only ~270 seconds after HEAD. On an independently created B, TTL **60**, Running/last-active **01:04:35Z**, its only GET **01:05:13.843Z** returned 200. Hibernated condition **01:05:35Z**, unchanged last-active: only **21.2 seconds** after GET. Fresh dashboard showed Service Sleeping; REST/GraphQL/MCP agreed with the actual Hibernated phase. Backend event projection followed at 01:05:43Z, eight seconds after the condition; use the condition for sleep timing.
- **Reproduce:** follow the milestone DoD. Observe service state through authenticated APIs or App metadata, not additional public requests that would change the idle clock. Obtain request logs to detect incidental scanner traffic. A's bounded request-log response contains only the two probes; B's contains only the one GET.

## Root cause and consumer

1. `lego/operator/internal/controller/activity.go:110–123` builds three `sum(increase(series[1m])) > 0` activity predicates under `max_over_time(timestamp(...)[lookback:15s])`. A new positive series has no zero baseline. A's GET and HEAD began on different Traefik pods at 1 and stayed flat. B's sole GET had samples **01:05:14.727=1** and **01:05:29.727=1**. No increase was observed, despite the first sample proving served traffic. The exact production-shaped three-branch query returned an empty vector even with the maximum allowed lookback (315/75 seconds), so reducing that lookback to the actual elapsed window cannot recover it.
2. Deployed Prometheus reported **2.54.1**, revision `e6cfa720fbe6280153fab13090a483dbd40bece3`. Its actual [extrapolatedRate / funcIncrease implementation](https://raw.githubusercontent.com/prometheus/prometheus/v2.54.1/promql/functions.go), lines 66–115 and 239–241, needs two samples and computes a reset-adjusted difference; a flat 1→1 contributes zero. This is expected library behavior, not a Prometheus defect.
3. `activity.go:86–100` bounds the lookup and rejects timestamps not newer than the stamp. `recentlyActive:143–163` only stamps a newer result. `app_controller.go:2560` then combines the expired stamp with `!recentlyActive` to scale down. `runningRequeue:2697` seeds the initial Running stamp; `autoSleepWindow:2097`, `autoSleepEligible:2113`, `shouldAutoHibernate:2140` govern eligibility and expiry. These consumers explain the unchanged initial stamp and exact initial-window sleep.
4. **The attempted steady control was not a clean pass.** At TTL60, A woke on 01:08:13.513 (503), returned 200 at 01:08:34.051 and 01:08:54.576, then returned another wake 503 at 01:09:15.187. Raw metrics show a newly recreated GET series rising 1→2 at **01:09:08.738**. At **01:09:13**, the HTTP increase instant expression was **1.3920666666666668**, but the outer 15-second subquery returned **[]**; at **01:09:15**, it returned activity **01:09:15**. Thus even an already-ingested increase can be omitted until the next aligned step. [Prometheus 2.54.1 engine.go](https://raw.githubusercontent.com/prometheus/prometheus/v2.54.1/promql/engine.go), lines 1776–1806, aligns subquery steps independently; the last step at :13 is :00. This is a second traced boundary in the same helper. The historical queries reproduce the mechanism; this control had one live run, unlike the two-fixture first-sample reproduction.
5. Later 200s at 01:09:35.729, 01:09:56.267 and 01:10:16.802 did advance A's stamp to **01:10:30Z**. It eventually Hibernated at **01:11:30Z**, ~73.2 seconds after the last GET. This proves some established traffic is detected and flat counters do not currently keep the service awake forever. It does not justify saying all steady traffic passed. Initial browser wake at 00:59:08 returned the normal retry-after-5 loading HTML, then automatic navigation showed `OK` by 00:59:28.396 (20.881s observation including tool delay, not an exact startup latency).

## Fix target and blast radius

- Keep the last-request quiet-window contract from ADR003:80. Correct the shared signal calculation, not phase rendering or the activator's honest temporary 503. `activity.go:111` needs bounded first-positive evidence alongside reset-safe increases, and the result needs activity at the current evaluation instant alongside its historical steps. An observed-positive-series difference against `offset 15s` detected B's first sample historically; it is a candidate to validate, not a ready-made production patch.
- Handle each original series identity before aggregation, including method/status/pod labels. Test missing scrapes, zero baselines, stale/reappearing series, resets and new pods. A constant positive counter must not remain active forever. Preserve the bounded lookback and query cost. State the conservative bounded treatment of requests newer than the latest successful scrape; a sampled metric cannot provide immediate knowledge of unsampled traffic.
- Exhaustive `rg` census of operator Go: **one** production `activityQuery` call (`activity.go:87`), **one** production constructor call (`cmd/manager/main.go:68`, wired at 364), **one** production `recentlyActive` call (`app_controller.go:2560`), **three** signal branches (HTTP requests and both WebSocket byte directions). Tests additionally call the constructor/query. Global correction inside the current eligibility gate, not a new service-type policy.
- Resource family: Free public web (`web_service` and legacy empty App type) is eligible; non-Free web, private, worker, cron, static, Postgres and Key Value are outside this gate. Git-native/Docker/CNB/prebuilt-image/Blueprint creation all eventually share the App decision; only native Go was live-tested. Same-helper first-positive/step issues for WebSocket counters are source-inferred; no WebSocket-only live failure is claimed.
- Adjacent states: unreadable/timeout Prometheus remains fail-awake with the existing circuit breaker (`activity.go:151–153`); optimistic-lock stamp conflicts remain fail-awake (`159–173`); manual suspension stays distinct from automatic hibernation. Preserve activator wake stamping, default900/custom positive TTL, missing-plugin-series handling, and no-reader behavior. Do not introduce unauthenticated metrics endpoints or change REST/GraphQL/MCP auth/error taxonomy.
- UI and APIs already report actual Hibernated honestly. Keep their last settled phase until the operator changes it; do not disguise the failure with optimistic Running. REST `/v1/services/:id`, GraphQL `service(id:)`, MCP `get_service(serviceId:)` all observed the same state. `lego/backend/internal/apps/graphql.go:330` projects phase; API views and MCP share the service representation. Verify exact paths while implementing if lines drift.
- Existing `activity_test.go:236–278` returns predetermined Prometheus timestamps and inspects query text. It does not evaluate these first-sample/grid semantics. Add real production-query evaluation with an explicitly pinned engine/promtool, then assert the controller decision. CI's GitOps workflow currently installs promtool **2.55.1**, distinct from the observed live **2.54.1**; do not claim those versions match. Preserve `activity_scrape_test.go:56`'s counter scrape guard.

## Dedupe and prior definition-of-done audit

Tree-wide open and done text search for activityQuery/recentlyActive/initial counters/first samples, every open milestone README title, `.pm/DO_NOT_DO.md`, the latest 40 product commits and targeted activity-query history were reviewed. Main **45c7abdf1830c7d505846deac6b6c799c30432e6**, re-pulled before filing, still has this query. `9359e5a1f` added m151 activity detection; `430f1ee27` added the missing WebSocket direction. No open item owns these two query gaps; this is not merely deploy lag. Search lookalikes w5/m81/m93, w6/m102, w7/054 and w4/139 concern unrelated telemetry/timing/counters or metrics-rate sampling, not idle decisions. No anti-goal applies: the historical rejected w6/m120 self-wake framing is explicitly excluded; here actual served requests are followed by premature sleep.

Full prior guarantees:

- **w1/done/m151 DoD 1, steady GETs every15s for20min/default15min:** not repeated at that duration this run. The shorter TTL60/every20s control encountered a second wake503 and exposes the current-step gap; restore the full prior control during implementation, do not claim complete regression coverage from this shortened run.
- **m151 DoD 2, idle sleep timed from last request:** fails on both independent sparse-first-request fixtures. Later established-counter activity did move the stamp and then expired, a partial positive control.
- **m151 DoD 3, wake still works:** observed browser automatic wake to OK plus subsequent curl wakes; preserve it.
- **w1/done/m161 DoD 1, client-only WebSocket:** not live-run here; preserve and reverify isolated from unrelated HTTP traffic.
- **m161 DoD 2, server-only WebSocket plus truly idle sleep:** WebSocket side unverified this run; eventual idle sleep observed after A's established traffic. Preserve both.
- **m161 DoD 3, real exported client counter and query both directions:** source query has both branches; the plugin's live export was not probed because these fixtures use HTTP only. Keep the scrape allowlist test and verify the export in the WebSocket control.

Render [free-service docs](https://render.com/docs/free) define idle as no inbound HTTP or WebSocket traffic for15minutes. This run did not create a Render fixture. Custom60/300-second TTL is an existing Bex extension; dropping first traffic is not an intentional divergence.

## Cleanup and local evidence

Both dashboard deletes returned true, both authenticated REST reads then returned404. An all-namespace metadata inventory of1382 Apps/Deployments/ReplicaSets/Pods/Services/Ingresses/Secrets/ConfigMaps/Jobs/PVCs found no fixture name/ID/UID remnants. Read-only Prometheus port-forward stopped. `qa-login.sh --logout` returned `ok logged-out`; this session's cookie files and browser cookies removed. No paid or foreign resources modified.

Verified local supplemental files: `.playwright-mcp/qa-sleep-r71b-early.png` (fresh UI Sleeping state, not timing proof), `qa-r71-api-captures.json`, `qa-r71-prom-first-samples.json`, `qa-r71b-prom-evidence.json`, `qa-r71-control-prom.json`, `qa-r71-control-boundary.json`, `qa-r71-full-production-query.json`, `qa-r71-steady-control.json`, `qa-r71-cleanup-inventory.json`, `qa-r71-console.txt`, `qa-r71-network.txt` (all under `.playwright-mcp/`). The two console errors are intentional post-delete404s. These files are gitignored; durable requests and complete corresponding responses follow.

## Durable probes

API requests below use the QA browser's existing authenticated session; no credentials are embedded. GraphQL captures POST to `https://api.bex.co/graphql`, REST captures GET to `https://api.bex.co` plus path, MCP POSTs to `https://api.bex.co/mcp` with Accept `application/json, text/event-stream`. JSON-string bodies preserve complete responses, including normal MCP SSE framing. Public probes are GET `/` on the named fixture host unless noted HEAD. Prometheus queries are GET `/api/v1/query` with the recorded query and time parameters, through an authenticated read-only operator port-forward; the localhost URL is an observation aid, not a tenant API.

### Fixture creation and persisted timeouts

```json
[
  {
    "request": [
      {
        "operationName": "CreateService",
        "variables": {
          "repo": "https://github.com/bex-co/bex",
          "branch": "main",
          "name": "qa-20261003-wake-r71",
          "type": "web_service",
          "rootDir": "examples/hello-go",
          "runtime": "go",
          "buildCommand": "go build -o app .",
          "startCommand": "./app",
          "plan": "free",
          "autoDeploy": false,
          "port": 3000,
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
    "body": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db0q5p9rgajs73fr34jg\",\"latestDeployId\":\"dep-db0q5p9rgajs73fr34k0\",\"name\":\"qa-20261003-wake-r71\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"web_service\"}}}]\n"
  },
  {
    "request": [
      {
        "operationName": "SetIdleTimeout",
        "variables": {
          "id": "srv-db0q5p9rgajs73fr34jg",
          "idleTTLSeconds": 300
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation SetIdleTimeout($id: String!, $idleTTLSeconds: Int!) {\n  setIdleTimeout(id: $id, idleTTLSeconds: $idleTTLSeconds) {\n    id\n    idleTTLSeconds\n    phase\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"setIdleTimeout\":{\"__typename\":\"Service\",\"id\":\"srv-db0q5p9rgajs73fr34jg\",\"idleTTLSeconds\":300,\"phase\":\"Building\"}}}]\n"
  },
  {
    "idleTimeoutAfterReload": "5 min",
    "lastPublicRequest": "2026-10-04T00:53:49Z"
  },
  {
    "request": [
      {
        "operationName": "CreateService",
        "variables": {
          "repo": "https://github.com/bex-co/bex",
          "branch": "main",
          "name": "qa-20261003-wake-r71b",
          "type": "web_service",
          "rootDir": "examples/hello-go",
          "runtime": "go",
          "buildCommand": "go build -o app .",
          "startCommand": "./app",
          "plan": "free",
          "autoDeploy": false,
          "port": 3000,
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
    "body": "[{\"data\":{\"createService\":{\"__typename\":\"Service\",\"environmentId\":null,\"id\":\"srv-db0qb29rgajs73fr34rg\",\"latestDeployId\":\"dep-db0qb29rgajs73fr34s0\",\"name\":\"qa-20261003-wake-r71b\",\"phase\":\"\",\"projectId\":null,\"registryCredentialId\":null,\"type\":\"web_service\"}}}]\n"
  },
  {
    "at": "2026-10-04T01:02:22.797Z",
    "request": {
      "query": "mutation { setIdleTimeout(id:\"srv-db0qb29rgajs73fr34rg\",idleTTLSeconds:60) { id phase idleTTLSeconds } }"
    },
    "status": 200,
    "body": "{\"data\":{\"setIdleTimeout\":{\"id\":\"srv-db0qb29rgajs73fr34rg\",\"idleTTLSeconds\":60,\"phase\":\"Building\"}}}\n"
  }
]
```

### Public first request and phase

```json
[
  {
    "stage": "B first and only request",
    "at": "2026-10-04T01:05:13.843Z",
    "status": 200,
    "body": "OK"
  },
  {
    "at": "2026-10-04T01:05:42.983Z",
    "request": {
      "query": "query { service(id:\"srv-db0qb29rgajs73fr34rg\") { id phase idleTTLSeconds suspended } }"
    },
    "status": 200,
    "body": "{\"data\":{\"service\":{\"id\":\"srv-db0qb29rgajs73fr34rg\",\"idleTTLSeconds\":60,\"phase\":\"Hibernated\",\"suspended\":\"not_suspended\"}}}\n"
  }
]
```

### REST service request logs and MCP

```json
[
  {
    "path": "/v1/services/srv-db0qb29rgajs73fr34rg",
    "status": 200,
    "body": "{\"id\":\"srv-db0qb29rgajs73fr34rg\",\"name\":\"qa-20261003-wake-r71b\",\"immutableName\":\"qa-20261003-wake-r71b\",\"slug\":\"qa-20261003-wake-r71b\",\"displayName\":\"\",\"type\":\"web_service\",\"suspended\":\"not_suspended\",\"dashboardUrl\":\"https://dashboard.bex.co/web/srv-db0qb29rgajs73fr34rg\",\"createdAt\":\"2026-10-04T01:02:01Z\",\"updatedAt\":\"2026-10-04T01:05:45Z\",\"owner\":{\"id\":\"tea-d98210cbbpdc73dcrkvg\",\"name\":\"bex\",\"email\":\"puncsky@gmail.com\",\"type\":\"team\"},\"serviceDetails\":{\"env\":\"go\",\"envSpecificDetails\":{\"buildCommand\":\"go build -o app .\",\"preDeployCommand\":\"\",\"startCommand\":\"./app\"},\"internalAddress\":\"qa-20261003-wake-r71b:3000\",\"maintenanceMode\":{\"enabled\":false,\"uri\":\"\"},\"maxShutdownDelaySeconds\":30,\"numInstances\":1,\"plan\":\"free\",\"port\":3000,\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"runtime\":\"go\",\"url\":\"https://qa-20261003-wake-r71b.onbex.co\"},\"suspenders\":[],\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Hibernated\",\"replicas\":1,\"revision\":\"rev-1\",\"urls\":[\"https://qa-20261003-wake-r71b.onbex.co\"],\"idleTTLSeconds\":60,\"rootDir\":\"examples/hello-go\",\"repo\":\"https://github.com/bex-co/bex\",\"branch\":\"main\",\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"pushDeliveryMethod\":\"github_app\",\"notifyOnFail\":\"default\",\"notificationsToSend\":\"default\"}\n"
  },
  {
    "path": "/v1/logs?resource=srv-db0qb29rgajs73fr34rg&type=request&startTime=2026-10-04T01%3A04%3A00Z&endTime=2026-10-04T01%3A06%3A00Z&limit=20",
    "status": 200,
    "body": "{\"hasMore\":false,\"nextStartTime\":\"2026-10-04T01:04:00Z\",\"nextEndTime\":\"2026-10-04T01:05:13.787733857Z\",\"logs\":[{\"id\":\"srv-db0qb29rgajs73fr34rg-2026-10-04T01:05:13.787733858Z-1ce242f4\",\"message\":\"{\\\"ClientAddr\\\":\\\"162.224.81.143:50848\\\",\\\"ClientHost\\\":\\\"162.224.81.143\\\",\\\"ClientPort\\\":\\\"50848\\\",\\\"ClientUsername\\\":\\\"-\\\",\\\"DownstreamContentSize\\\":2,\\\"DownstreamStatus\\\":200,\\\"Duration\\\":4294573,\\\"KubernetesIngressName\\\":\\\"tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b\\\",\\\"KubernetesIngressNamespace\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"KubernetesServiceName\\\":\\\"tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b\\\",\\\"KubernetesServicePort\\\":\\\"3000\\\",\\\"OriginContentSize\\\":2,\\\"OriginDuration\\\":3929368,\\\"OriginStatus\\\":200,\\\"Overhead\\\":365205,\\\"RequestAddr\\\":\\\"qa-20261003-wake-r71b.onbex.co\\\",\\\"RequestContentSize\\\":0,\\\"RequestCount\\\":7225072,\\\"RequestHost\\\":\\\"qa-20261003-wake-r71b.onbex.co\\\",\\\"RequestMethod\\\":\\\"GET\\\",\\\"RequestPath\\\":\\\"/\\\",\\\"RequestPort\\\":\\\"-\\\",\\\"RequestProtocol\\\":\\\"HTTP/1.1\\\",\\\"RequestScheme\\\":\\\"https\\\",\\\"RetryAttempts\\\":0,\\\"RouterName\\\":\\\"websecure-tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-qa-20261003-wake-r71b-onbex-co@kubernetes\\\",\\\"ServiceAddr\\\":\\\"10.244.90.192:3000\\\",\\\"ServiceName\\\":\\\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\\\",\\\"ServiceURL\\\":\\\"http://10.244.90.192:3000\\\",\\\"StartLocal\\\":\\\"2026-10-04T01:05:13.783161926Z\\\",\\\"StartUTC\\\":\\\"2026-10-04T01:05:13.783161926Z\\\",\\\"TLSCipher\\\":\\\"TLS_AES_128_GCM_SHA256\\\",\\\"TLSVersion\\\":\\\"1.3\\\",\\\"entryPointName\\\":\\\"websecure\\\",\\\"level\\\":\\\"info\\\",\\\"msg\\\":\\\"\\\",\\\"time\\\":\\\"2026-10-04T01:05:13Z\\\"}\",\"timestamp\":\"2026-10-04T01:05:13.787733858Z\",\"labels\":[{\"name\":\"type\",\"value\":\"request\"},{\"name\":\"resource\",\"value\":\"srv-db0qb29rgajs73fr34rg\"},{\"name\":\"method\",\"value\":\"GET\"},{\"name\":\"statusCode\",\"value\":\"200\"}]}]}\n"
  },
  {
    "request": {
      "jsonrpc": "2.0",
      "id": 71,
      "method": "tools/call",
      "params": {
        "name": "get_service",
        "arguments": {
          "serviceId": "srv-db0qb29rgajs73fr34rg"
        }
      }
    },
    "status": 200,
    "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":71,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-04T01:02:01Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/web/srv-db0qb29rgajs73fr34rg\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-db0qb29rgajs73fr34rg\\\",\\\"idleTTLSeconds\\\":60,\\\"immutableName\\\":\\\"qa-20261003-wake-r71b\\\",\\\"name\\\":\\\"qa-20261003-wake-r71b\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Hibernated\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-1\\\",\\\"rootDir\\\":\\\"examples/hello-go\\\",\\\"serviceDetails\\\":{\\\"env\\\":\\\"go\\\",\\\"envSpecificDetails\\\":{\\\"buildCommand\\\":\\\"go build -o app .\\\",\\\"preDeployCommand\\\":\\\"\\\",\\\"startCommand\\\":\\\"./app\\\"},\\\"internalAddress\\\":\\\"qa-20261003-wake-r71b:3000\\\",\\\"maintenanceMode\\\":{\\\"enabled\\\":false,\\\"uri\\\":\\\"\\\"},\\\"maxShutdownDelaySeconds\\\":30,\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"port\\\":3000,\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"runtime\\\":\\\"go\\\",\\\"url\\\":\\\"https://qa-20261003-wake-r71b.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261003-wake-r71b\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"web_service\\\",\\\"updatedAt\\\":\\\"2026-10-04T01:05:45Z\\\",\\\"urls\\\":[\\\"https://qa-20261003-wake-r71b.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-04T01:02:01Z\",\"dashboardUrl\":\"https://dashboard.bex.co/web/srv-db0qb29rgajs73fr34rg\",\"displayName\":\"\",\"id\":\"srv-db0qb29rgajs73fr34rg\",\"idleTTLSeconds\":60,\"immutableName\":\"qa-20261003-wake-r71b\",\"name\":\"qa-20261003-wake-r71b\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Hibernated\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-1\",\"rootDir\":\"examples/hello-go\",\"serviceDetails\":{\"env\":\"go\",\"envSpecificDetails\":{\"buildCommand\":\"go build -o app .\",\"preDeployCommand\":\"\",\"startCommand\":\"./app\"},\"internalAddress\":\"qa-20261003-wake-r71b:3000\",\"maintenanceMode\":{\"enabled\":false,\"uri\":\"\"},\"maxShutdownDelaySeconds\":30,\"numInstances\":1,\"plan\":\"free\",\"port\":3000,\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"runtime\":\"go\",\"url\":\"https://qa-20261003-wake-r71b.onbex.co\"},\"slug\":\"qa-20261003-wake-r71b\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"web_service\",\"updatedAt\":\"2026-10-04T01:05:45Z\",\"urls\":[\"https://qa-20261003-wake-r71b.onbex.co\"]}}}\n\n"
  }
]
```

### Deletion responses

```json
[
  {
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db0qb29rgajs73fr34rg"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"deleteService\":true}}]\n"
  },
  {
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-db0q5p9rgajs73fr34jg"
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "mutation DeleteService($id: String!, $confirm: String) {\n  deleteService(id: $id, confirm: $confirm)\n}"
      }
    ],
    "status": 200,
    "body": "[{\"data\":{\"deleteService\":true}}]\n"
  }
]
```

### qa-r71b-before-request.json

```json
{
  "name": "tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b",
  "uid": "b104b599-af08-4256-902a-bcb23257e5cd",
  "lastActive": "2026-10-04T01:04:35Z",
  "phase": "Running",
  "conditions": [
    {
      "lastTransitionTime": "2026-10-04T01:04:35Z",
      "message": "1/1 replicas ready",
      "observedGeneration": 2,
      "reason": "Deployed",
      "status": "True",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2026-10-04T01:04:24Z",
      "message": "serving at qa-20261003-wake-r71b.onbex.co",
      "observedGeneration": 2,
      "reason": "Routed",
      "status": "True",
      "type": "PublicRouting"
    },
    {
      "lastTransitionTime": "2026-10-04T01:04:24Z",
      "message": "serving revision has not been observed",
      "observedGeneration": 2,
      "reason": "ServingUnobserved",
      "status": "Unknown",
      "type": "Serving"
    }
  ]
}
```

### qa-r71b-after-request.json

```json
{
  "observedAt": "2026-10-04T01:05:42.732590+00:00",
  "name": "tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b",
  "lastActive": "2026-10-04T01:04:35Z",
  "phase": "Hibernated",
  "conditions": [
    {
      "lastTransitionTime": "2026-10-04T01:05:35Z",
      "message": "idle \u226560s on free tier; wakes on next request",
      "observedGeneration": 2,
      "reason": "AutoHibernated",
      "status": "False",
      "type": "Ready"
    },
    {
      "lastTransitionTime": "2026-10-04T01:04:24Z",
      "message": "serving at qa-20261003-wake-r71b.onbex.co",
      "observedGeneration": 2,
      "reason": "Routed",
      "status": "True",
      "type": "PublicRouting"
    },
    {
      "lastTransitionTime": "2026-10-04T01:04:24Z",
      "message": "serving revision has not been observed",
      "observedGeneration": 2,
      "reason": "ServingUnobserved",
      "status": "Unknown",
      "type": "Serving"
    }
  ]
}
```

### qa-r71b-prom-evidence.json

```json
[
  {
    "query": "traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\"}[2m]",
    "time": "2026-10-04T01:05:35Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "matrix",
        "result": [
          {
            "metric": {
              "__name__": "traefik_service_requests_total",
              "code": "200",
              "instance": "10.244.36.122:9100",
              "job": "traefik",
              "method": "GET",
              "pod": "traefik-776c44f94f-l9bfd",
              "protocol": "http",
              "service": "tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes"
            },
            "values": [
              [1791075914.727, "1"],
              [1791075929.727, "1"]
            ]
          }
        ]
      }
    }
  },
  {
    "query": "max_over_time(timestamp(sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\"}[1m])) > 0)[60s:15s])",
    "time": "2026-10-04T01:05:35Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": []
      }
    }
  },
  {
    "query": "max_over_time(timestamp(sum((traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\"} > 0) unless traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\"} offset 15s) > 0)[60s:15s])",
    "time": "2026-10-04T01:05:35Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791075935, "1791075915"]
          }
        ]
      }
    }
  }
]
```

### qa-r71-full-production-query.json

```json
[
  {
    "query": "max_over_time(timestamp((sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m])) > 0) or (sum(increase(bex_websocket_egress_bytes_total{app_id=\"srv-db0q5p9rgajs73fr34jg\"}[1m])) > 0) or (sum(increase(bex_websocket_ingress_bytes_total{app_id=\"srv-db0q5p9rgajs73fr34jg\"}[1m])) > 0))[315s:15s])",
    "time": "2026-10-04T00:58:19Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": []
      }
    }
  },
  {
    "query": "max_over_time(timestamp((sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71b-3000@kubernetes\"}[1m])) > 0) or (sum(increase(bex_websocket_egress_bytes_total{app_id=\"srv-db0qb29rgajs73fr34rg\"}[1m])) > 0) or (sum(increase(bex_websocket_ingress_bytes_total{app_id=\"srv-db0qb29rgajs73fr34rg\"}[1m])) > 0))[75s:15s])",
    "time": "2026-10-04T01:05:35Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": []
      }
    }
  },
  {
    "query": "max_over_time(timestamp((sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m])) > 0) or (sum(increase(bex_websocket_egress_bytes_total{app_id=\"srv-db0q5p9rgajs73fr34jg\"}[1m])) > 0) or (sum(increase(bex_websocket_ingress_bytes_total{app_id=\"srv-db0q5p9rgajs73fr34jg\"}[1m])) > 0))[75s:15s])",
    "time": "2026-10-04T01:09:13Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": []
      }
    }
  }
]
```

### qa-r71-control-prom.json

```json
{
  "request": "http://127.0.0.1:19097/api/v1/query?query=traefik_service_requests_total%7Bservice%3D%22tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000%40kubernetes%22%7D%5B4m%5D&time=2026-10-04T01%3A10%3A45Z",
  "response": {
    "status": "success",
    "data": {
      "resultType": "matrix",
      "result": [
        {
          "metric": {
            "__name__": "traefik_service_requests_total",
            "code": "200",
            "instance": "10.244.35.149:9100",
            "job": "traefik",
            "method": "GET",
            "pod": "traefik-776c44f94f-pjhp2",
            "protocol": "http",
            "service": "tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes"
          },
          "values": [
            [1791076118.738, "1"],
            [1791076133.738, "1"],
            [1791076148.738, "2"],
            [1791076163.738, "2"],
            [1791076223.738, "1"],
            [1791076238.738, "1"]
          ]
        },
        {
          "metric": {
            "__name__": "traefik_service_requests_total",
            "code": "200",
            "instance": "10.244.36.122:9100",
            "job": "traefik",
            "method": "GET",
            "pod": "traefik-776c44f94f-l9bfd",
            "protocol": "http",
            "service": "tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes"
          },
          "values": [
            [1791076184.727, "1"],
            [1791076199.727, "2"],
            [1791076214.727, "2"],
            [1791076229.727, "2"],
            [1791076244.727, "2"]
          ]
        }
      ]
    }
  }
}
```

### qa-r71-control-boundary.json

```json
[
  {
    "query": "max_over_time(timestamp(sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m])) > 0)[60s:15s])",
    "time": "2026-10-04T01:09:13Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": []
      }
    }
  },
  {
    "query": "sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m]))",
    "time": "2026-10-04T01:09:13Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791076153, "1.3920666666666668"]
          }
        ]
      }
    }
  },
  {
    "query": "max_over_time(timestamp(sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m])) > 0)[60s:15s])",
    "time": "2026-10-04T01:09:15Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791076155, "1791076155"]
          }
        ]
      }
    }
  },
  {
    "query": "sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m]))",
    "time": "2026-10-04T01:09:15Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791076155, "1.4587333333333334"]
          }
        ]
      }
    }
  },
  {
    "query": "max_over_time(timestamp(sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m])) > 0)[60s:15s])",
    "time": "2026-10-04T01:10:15Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791076215, "1791076215"]
          }
        ]
      }
    }
  },
  {
    "query": "sum(increase(traefik_service_requests_total{service=\"tea-d98210cbbpdc73dcrkvg-tea-d98210cbbpdc73dcrkvg-qa-20261003-wake-r71-3000@kubernetes\"}[1m]))",
    "time": "2026-10-04T01:10:15Z",
    "response": {
      "status": "success",
      "data": {
        "resultType": "vector",
        "result": [
          {
            "metric": {},
            "value": [1791076215, "1.2591"]
          }
        ]
      }
    }
  }
]
```

### qa-r71-steady-control.json

```json
[
  {
    "at": "2026-10-04T01:08:13.513165+00:00",
    "status": "503",
    "body": "{\"error\":\"service hibernated\",\"retryAfter\":5}",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:08:34.050765+00:00",
    "status": "200",
    "body": "OK",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:08:54.575521+00:00",
    "status": "200",
    "body": "OK",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:09:15.186933+00:00",
    "status": "503",
    "body": "{\"error\":\"service hibernated\",\"retryAfter\":5}",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:09:35.728819+00:00",
    "status": "200",
    "body": "OK",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:09:56.267121+00:00",
    "status": "200",
    "body": "OK",
    "exit": 0
  },
  {
    "at": "2026-10-04T01:10:16.802452+00:00",
    "status": "200",
    "body": "OK",
    "exit": 0
  }
]
```

### qa-r71-prom-version.json

```json
{
  "status": "success",
  "data": {
    "version": "2.54.1",
    "revision": "e6cfa720fbe6280153fab13090a483dbd40bece3",
    "branch": "HEAD",
    "buildUser": "root@812ffd741951",
    "buildDate": "20240827-10:56:41",
    "goVersion": "go1.22.6"
  }
}
```
