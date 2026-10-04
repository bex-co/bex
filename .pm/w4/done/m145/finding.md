# Save-only changes are missing from the saved-versus-running indicator

- **Severity:** major — saved configuration persists and deferred execution works, but the product reports no saved/runtime difference. No lost secret or unintended deployment was observed.
- **Observed:** 2026-10-02, continuous `qa-find-bugs` sweep 5 using `muse.env`; workspace `tea-d98210cbbpdc73dcrkvg` (bex, Scale/admin), disposable free service `qa-20261002-web-r5` / `srv-davmi3k5o9vs73dt7ot0`.
- **Expected:** Save only retains the new file without rolling, while service reads report `undeployedChanges: true` and the dashboard shows its existing “Saved changes aren't live yet” notices. The next successful standard deploy applies the file and clears the difference.
- **Actual:** two effective Save-only edits succeeded and survived reload, leaving the old marker running. GraphQL returned false, REST/MCP omitted the false field, and both notices were absent.
- **Estimate:** 3h 10m including shared-code audit and closing tasks.
- **Unverified:** service variables, CAS writes, first reference, deletion/mixed edits, linked groups, static/cron/worker/private paths, legacy records, failure paths, sleep/pod/operator replacement, and unauthorized callers. These are verification work, not additional observed failures.

## Reproduce

1. Create a free Go web service from `https://github.com/bex-co/bex`, main, root `examples/hello-go`, build `go build -o app .`, start `export MESSAGE="$(cat /etc/secrets/qa-r5-message)"; exec ./app`, port 3000, auto-deploy off. Add file `qa-r5-message` containing harmless marker `qa-20261002-r5-secret-v1`.
2. Wait for initial deploy `dep-davmi3k5o9vs73dt7otg` to be Live (08:22:07Z). External `curl --silent --show-error --max-time 20 --write-out '\nHTTP %{http_code}\n' https://qa-20261002-web-r5.onbex.co/` returns v1 and HTTP 200.
3. Open `https://dashboard.bex.co/services/srv-davmi3k5o9vs73dt7ot0/env` → Edit → View contents → replace v1 with v2 → Done → Environment save options → Save only.
4. Reload, wait for the file row, and Reveal. Saved content is v2; external HTTP still returns v1; no deploy was added. Neither header nor Environment page shows a pending notice. Fresh reads at 08:23:00 and 08:23:32Z agree.
5. After the passing lifecycle controls below, wait for the deploy-hook release to be Live and serve v2. Repeat Save only with v3. Fresh read at 08:36:17Z again reports false, saved v3 and HTTP v2; the settled Reveal at 08:36:39Z has no pending notice.

The fixture is deleted; recreate a uniquely named QA service and substitute its IDs.

## Durable probes

GraphQL requests are authenticated POSTs to `https://api.bex.co/graphql`, JSON content type, browser `credentials: "include"`. Bodies are complete; cookie headers are excluded. Markers are dummy values, not credentials.

### First Save-only mutation

Captured 2026-10-02T08:22:48.256Z; HTTP 200.

Request:

```json
[
  {
    "operationName": "PatchServiceEnvironment",
    "variables": {
      "serviceId": "srv-davmi3k5o9vs73dt7ot0",
      "envVars": [],
      "secretFiles": [
        {
          "name": "qa-r5-message",
          "content": "qa-20261002-r5-secret-v2"
        }
      ],
      "saveMode": "save_only"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation PatchServiceEnvironment($serviceId: String!, $envVars: [EnvironmentEnvVarPatchInput!], $secretFiles: [EnvironmentSecretFilePatchInput!], $saveMode: String!) {\n  patchServiceEnvironment(\n    serviceId: $serviceId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n  ) {\n    envVarKeys\n    secretFileNames\n    rolledOut\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "patchServiceEnvironment": {
        "__typename": "EnvironmentPatchResult",
        "envVarKeys": [],
        "rolledOut": false,
        "secretFileNames": ["qa-r5-message"]
      }
    }
  }
]
```

### Fresh read after first save

Captured 2026-10-02T08:23:00.755Z; HTTP 200.

Request:

```json
{
  "query": "query { service(id:\"srv-davmi3k5o9vs73dt7ot0\") { id phase undeployedChanges secretFile(name:\"qa-r5-message\") { name content } } deploys(serviceId:\"srv-davmi3k5o9vs73dt7ot0\",limit:10) { id status trigger } }"
}
```

Complete response:

```json
{
  "data": {
    "deploys": [
      {
        "id": "dep-davmi3k5o9vs73dt7otg",
        "status": "live",
        "trigger": "create"
      }
    ],
    "service": {
      "id": "srv-davmi3k5o9vs73dt7ot0",
      "phase": "Running",
      "secretFile": {
        "content": "qa-20261002-r5-secret-v2",
        "name": "qa-r5-message"
      },
      "undeployedChanges": false
    }
  }
}
```

External response remained:

```text
qa-20261002-r5-secret-v1
HTTP 200
```

### REST and MCP cross-check

Captured 2026-10-02T08:24:15.098Z. REST uses `https://api.bex.co`; MCP is POST `https://api.bex.co/mcp` with JSON content type and `Accept: application/json, text/event-stream`. The workspace contact email is the sole response-value redaction. Request/status/complete REST response:

```json
{
  "request": {
    "method": "GET",
    "path": "/v1/services/srv-davmi3k5o9vs73dt7ot0"
  },
  "status": 200,
  "response": {
    "id": "srv-davmi3k5o9vs73dt7ot0",
    "name": "qa-20261002-web-r5",
    "immutableName": "qa-20261002-web-r5",
    "slug": "qa-20261002-web-r5",
    "displayName": "",
    "type": "web_service",
    "suspended": "not_suspended",
    "dashboardUrl": "https://dashboard.bex.co/web/srv-davmi3k5o9vs73dt7ot0",
    "createdAt": "2026-10-02T08:19:27Z",
    "updatedAt": "2026-10-02T08:22:02Z",
    "owner": {
      "id": "tea-d98210cbbpdc73dcrkvg",
      "name": "bex",
      "email": "[redacted workspace contact]",
      "type": "team"
    },
    "serviceDetails": {
      "env": "go",
      "envSpecificDetails": {
        "buildCommand": "go build -o app .",
        "preDeployCommand": "",
        "startCommand": "export MESSAGE=\"$(cat /etc/secrets/qa-r5-message)\"; exec ./app"
      },
      "internalAddress": "qa-20261002-web-r5:3000",
      "maintenanceMode": {
        "enabled": false,
        "uri": ""
      },
      "maxShutdownDelaySeconds": 30,
      "numInstances": 1,
      "plan": "free",
      "port": 3000,
      "region": "fsn1",
      "renderSubdomainPolicy": "enabled",
      "runtime": "go",
      "url": "https://qa-20261002-web-r5.onbex.co"
    },
    "suspenders": [],
    "ownerId": "tea-d98210cbbpdc73dcrkvg",
    "phase": "Running",
    "replicas": 1,
    "revision": "rev-1",
    "urls": ["https://qa-20261002-web-r5.onbex.co"],
    "idleTTLSeconds": 0,
    "rootDir": "examples/hello-go",
    "repo": "https://github.com/bex-co/bex",
    "branch": "main",
    "autoDeploy": "no",
    "autoDeployTrigger": "off",
    "pushDeliveryMethod": "github_app",
    "notifyOnFail": "default",
    "notificationsToSend": "default"
  }
}
```

Request/status/complete MCP SSE response, with original newlines preserved in the JSON string:

```json
{
  "request": {
    "jsonrpc": "2.0",
    "id": 5,
    "method": "tools/call",
    "params": {
      "name": "get_service",
      "arguments": {
        "serviceId": "srv-davmi3k5o9vs73dt7ot0"
      }
    }
  },
  "status": 200,
  "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":5,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"autoDeploy\\\":\\\"no\\\",\\\"autoDeployTrigger\\\":\\\"off\\\",\\\"branch\\\":\\\"main\\\",\\\"createdAt\\\":\\\"2026-10-02T08:19:27Z\\\",\\\"dashboardUrl\\\":\\\"https://dashboard.bex.co/web/srv-davmi3k5o9vs73dt7ot0\\\",\\\"displayName\\\":\\\"\\\",\\\"id\\\":\\\"srv-davmi3k5o9vs73dt7ot0\\\",\\\"idleTTLSeconds\\\":0,\\\"immutableName\\\":\\\"qa-20261002-web-r5\\\",\\\"name\\\":\\\"qa-20261002-web-r5\\\",\\\"notificationsToSend\\\":\\\"default\\\",\\\"notifyOnFail\\\":\\\"default\\\",\\\"ownerId\\\":\\\"tea-d98210cbbpdc73dcrkvg\\\",\\\"phase\\\":\\\"Running\\\",\\\"pushDeliveryMethod\\\":\\\"github_app\\\",\\\"replicas\\\":1,\\\"repo\\\":\\\"https://github.com/bex-co/bex\\\",\\\"revision\\\":\\\"rev-1\\\",\\\"rootDir\\\":\\\"examples/hello-go\\\",\\\"serviceDetails\\\":{\\\"env\\\":\\\"go\\\",\\\"envSpecificDetails\\\":{\\\"buildCommand\\\":\\\"go build -o app .\\\",\\\"preDeployCommand\\\":\\\"\\\",\\\"startCommand\\\":\\\"export MESSAGE=\\\\\\\"$(cat /etc/secrets/qa-r5-message)\\\\\\\"; exec ./app\\\"},\\\"internalAddress\\\":\\\"qa-20261002-web-r5:3000\\\",\\\"maintenanceMode\\\":{\\\"enabled\\\":false,\\\"uri\\\":\\\"\\\"},\\\"maxShutdownDelaySeconds\\\":30,\\\"numInstances\\\":1,\\\"plan\\\":\\\"free\\\",\\\"port\\\":3000,\\\"region\\\":\\\"fsn1\\\",\\\"renderSubdomainPolicy\\\":\\\"enabled\\\",\\\"runtime\\\":\\\"go\\\",\\\"url\\\":\\\"https://qa-20261002-web-r5.onbex.co\\\"},\\\"slug\\\":\\\"qa-20261002-web-r5\\\",\\\"suspended\\\":\\\"not_suspended\\\",\\\"suspenders\\\":[],\\\"type\\\":\\\"web_service\\\",\\\"updatedAt\\\":\\\"2026-10-02T08:22:02Z\\\",\\\"urls\\\":[\\\"https://qa-20261002-web-r5.onbex.co\\\"]}\"}],\"structuredContent\":{\"autoDeploy\":\"no\",\"autoDeployTrigger\":\"off\",\"branch\":\"main\",\"createdAt\":\"2026-10-02T08:19:27Z\",\"dashboardUrl\":\"https://dashboard.bex.co/web/srv-davmi3k5o9vs73dt7ot0\",\"displayName\":\"\",\"id\":\"srv-davmi3k5o9vs73dt7ot0\",\"idleTTLSeconds\":0,\"immutableName\":\"qa-20261002-web-r5\",\"name\":\"qa-20261002-web-r5\",\"notificationsToSend\":\"default\",\"notifyOnFail\":\"default\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"phase\":\"Running\",\"pushDeliveryMethod\":\"github_app\",\"replicas\":1,\"repo\":\"https://github.com/bex-co/bex\",\"revision\":\"rev-1\",\"rootDir\":\"examples/hello-go\",\"serviceDetails\":{\"env\":\"go\",\"envSpecificDetails\":{\"buildCommand\":\"go build -o app .\",\"preDeployCommand\":\"\",\"startCommand\":\"export MESSAGE=\\\"$(cat /etc/secrets/qa-r5-message)\\\"; exec ./app\"},\"internalAddress\":\"qa-20261002-web-r5:3000\",\"maintenanceMode\":{\"enabled\":false,\"uri\":\"\"},\"maxShutdownDelaySeconds\":30,\"numInstances\":1,\"plan\":\"free\",\"port\":3000,\"region\":\"fsn1\",\"renderSubdomainPolicy\":\"enabled\",\"runtime\":\"go\",\"url\":\"https://qa-20261002-web-r5.onbex.co\"},\"slug\":\"qa-20261002-web-r5\",\"suspended\":\"not_suspended\",\"suspenders\":[],\"type\":\"web_service\",\"updatedAt\":\"2026-10-02T08:22:02Z\",\"urls\":[\"https://qa-20261002-web-r5.onbex.co\"]}}}\n\n"
}
```

Missing REST/MCP `undeployedChanges` is the existing `omitempty` behavior for false, not another serializer bug.

### Successful standard deploy control and second effective save

Captured 2026-10-02T08:35:38.912Z; HTTP 200.

Request:

```json
{
  "query": "query { service(id:\"srv-davmi3k5o9vs73dt7ot0\") { id phase undeployedChanges secretFile(name:\"qa-r5-message\") { name content } } deploy(serviceId:\"srv-davmi3k5o9vs73dt7ot0\",deployId:\"dep-davmoqede41s73cans00\") { id status trigger finishedAt } }"
}
```

Complete response:

```json
{
  "data": {
    "deploy": {
      "finishedAt": "2026-10-02T08:35:27.415874Z",
      "id": "dep-davmoqede41s73cans00",
      "status": "live",
      "trigger": "deploy_hook"
    },
    "service": {
      "id": "srv-davmi3k5o9vs73dt7ot0",
      "phase": "Running",
      "secretFile": {
        "content": "qa-20261002-r5-secret-v2",
        "name": "qa-r5-message"
      },
      "undeployedChanges": false
    }
  }
}
```

Captured 2026-10-02T08:36:03.224Z; HTTP 200.

Request:

```json
[
  {
    "operationName": "PatchServiceEnvironment",
    "variables": {
      "serviceId": "srv-davmi3k5o9vs73dt7ot0",
      "envVars": [],
      "secretFiles": [
        {
          "name": "qa-r5-message",
          "content": "qa-20261002-r5-secret-v3"
        }
      ],
      "saveMode": "save_only"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation PatchServiceEnvironment($serviceId: String!, $envVars: [EnvironmentEnvVarPatchInput!], $secretFiles: [EnvironmentSecretFilePatchInput!], $saveMode: String!) {\n  patchServiceEnvironment(\n    serviceId: $serviceId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n  ) {\n    envVarKeys\n    secretFileNames\n    rolledOut\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "patchServiceEnvironment": {
        "__typename": "EnvironmentPatchResult",
        "envVarKeys": [],
        "rolledOut": false,
        "secretFileNames": ["qa-r5-message"]
      }
    }
  }
]
```

Captured 2026-10-02T08:36:17.039Z; HTTP 200.

Request:

```json
{
  "query": "query { service(id:\"srv-davmi3k5o9vs73dt7ot0\") { id phase undeployedChanges secretFile(name:\"qa-r5-message\") { name content } } deploys(serviceId:\"srv-davmi3k5o9vs73dt7ot0\",limit:10) { id status trigger } }"
}
```

Complete response:

```json
{
  "data": {
    "deploys": [
      {
        "id": "dep-davmoqede41s73cans00",
        "status": "live",
        "trigger": "deploy_hook"
      },
      {
        "id": "dep-davmn7k5o9vs73dt7ph0",
        "status": "deactivated",
        "trigger": "api"
      },
      {
        "id": "dep-davmmfude41s73canrq0",
        "status": "deactivated",
        "trigger": "rollback"
      },
      {
        "id": "dep-davml8k5o9vs73dt7p80",
        "status": "deactivated",
        "trigger": "api"
      },
      {
        "id": "dep-davmkfmde41s73canrgg",
        "status": "canceled",
        "trigger": "api"
      },
      {
        "id": "dep-davmi3k5o9vs73dt7otg",
        "status": "deactivated",
        "trigger": "create"
      }
    ],
    "service": {
      "id": "srv-davmi3k5o9vs73dt7ot0",
      "phase": "Running",
      "secretFile": {
        "content": "qa-20261002-r5-secret-v3",
        "name": "qa-r5-message"
      },
      "undeployedChanges": false
    }
  }
}
```

External curl at 08:36:17Z:

```text
qa-20261002-r5-secret-v2
HTTP 200
```

Read-only Kubernetes inspection at 08:37:10Z: App generation 6, configSnapshotGeneration 6, activeRevision rev-6, no active releaseConfig, no pending annotations, and the conventional filesFromSecrets reference. The mutable `…-files` Secret contained v3; serving snapshot `…-files-r6` contained v2. Only known dummy markers were compared. This independently establishes the difference after the deploy settled.

## Passing controls

| Action | Terminal observation |
| --- | --- |
| Initial create | v1 mounted at /etc/secrets; initial deploy Live |
| Save only v2 | saved v2, HTTP v1, no new deploy; indicator fails |
| Manual build then cancel | `dep-davmkfmde41s73canrgg` canceled 08:25:31Z; HTTP v1, saved v2, flag true and header notice visible |
| Standard manual deploy | `dep-davml8k5o9vs73dt7p80` Live 08:27:56Z; HTTP v2, flag false |
| Rollback to initial release | `dep-davmmfude41s73canrq0` Live 08:29:07Z; HTTP v1, saved v2, flag true |
| Restart after rollback | `dep-davmn7k5o9vs73dt7ph0` Live 08:30:37Z; retained v1 image/config, saved v2, flag true |
| Deploy hook | `dep-davmoqede41s73cans00` Live 08:35:27Z; trigger deploy_hook, HTTP v2, flag false |
| Save only v3 | saved v3, HTTP v2, no new deploy; indicator fails again |

List, detail/API and Events agreed on cancel, rollback, restart and hook terminal outcomes. Events at 08:37:23Z named the matching IDs and outcomes. Hook credentials were not recorded. These controls show the flag reaches the UI and historical selection is working.

## Root cause and target fix

Citations refer to current main `59433b8ee7e6684b67e4053bcbd707832418c5f2`.

1. `lego/backend/internal/secrets/batch.go:274` projects changed maps in finalizeEnvironmentPatch. Save only calls stagePendingProjectionReferences at 298; that helper at 487 restores old references and stages a pending name only when the conventional reference is new. Existing-file updates leave the App equal, so line 300 returns after updating the mutable Secret. Its comment equating unchanged App with unchanged maps does not describe this case.
2. `lego/operator/internal/controller/release_config_snapshot.go:528–531` sets UndeployedChanges from cancellation or active historical selection. Ordinary releases have neither. `release_config_selection.go:147` calls selectedConfigurationDiffers (157), which compares Secret values, only along the historical-selection path.
3. Neither a one-time status write nor checking pending-name annotations fixes this: a normal reconcile overwrites the bool and this repro has no pending annotation. Scheduling matters too. `app_controller.go:129–141` extends the lifecycle predicate only for registry rotation and historical scaling; `event_predicates.go:30–44` admits generation/deletion/finalizers. The builder at `app_controller.go:5684` watches App, Deployment, Pod, Ingress, NetworkPolicy and CronJob, with no Secret watch. Actual pinned controller-runtime v0.23.3 `pkg/predicate/predicate.go:211–224` returns true only when generation differs. Existing-reference Save only does not patch App at all; first-reference metadata is a separate adjacent case.
4. API projection is faithful: `apps/service.go:1042` copies status; `apps/graphql.go:333` returns it through the typed bool helper (`gqlutil/gqlutil.go:35–63`); `apps/render.go:120,361` carries false with omitempty. The optional bool at `lego/types/v1alpha1/app_types.go:1483–1488` can express true. Its cancel/historical wording and older API comments need reconciling with the broader saved/runtime contract.
5. `dashboard/src/features/services/lib/status.ts:48–49` preserves true; `service-detail-header.tsx:301` and `routes/services.$serviceId.env.tsx:50` each render a notice only for true. The frontend never receives true for this save.

Track effective saved/runtime divergence durably for ordinary service-local Save-only writes, including existing Secret references. Reconcile it without opening a deploy, changing the serving template/snapshot, or applying deferred values. Reuse comparison/selection machinery where semantics fit; audit source additions/removals before assuming its current comparison is complete. Preserve cancellation/rollback/Restart controls and lazy migration. A no-op or reverting saved values to running values must not invent a difference. Clear the flag when a successful standard deploy actually matches saved state. Keep raw secrets out of public status, metadata and logs.

During save/refetch, show pending feedback until authoritative divergence is known; do not imply “applied” from cached false. After fresh navigation both existing notices follow the same bool.

## Shared callers, aliases and neighbouring behavior

Counts below are exhaustive production-source rg results, excluding tests/generated code, on 2026-10-02:

- **3 service write adapters:** REST PATCH `/v1/services/{id}/environment` (`secrets/rest.go:77`), GraphQL patchServiceEnvironment (`graphql.go:221`), MCP patch_service_environment (`mcp.go:113`).
- **2 finalizer callers:** sparse batch at 268 and CAS at 194; **1 stage-helper caller** at 298. Observed file edits use sparse, not the narrower CAS env-variable operation.
- **2 serving-status callers:** applyServingDeployment at snapshot.go:567 and applyServingCronJob at 588. **1 selected-comparison caller:** ensureSelectedReleaseConfig at selection.go:147.
- **2 UI notice predicates:** header and ServiceEnvPage. Header has **1 production caller**, ServiceDetailLayout:135; that layout has **2 route callers**, services and static. ServiceEnvPage likewise serves canonical services and the concrete static Environment route.
- **1 ServiceEnvironmentEditor caller:** service Environment page line 57. Generic EnvironmentEditor has **2 callers**, that wrapper and `env-groups/components/env-group-editors.tsx:30`. The group backend has its own PatchEnvironment and three adapters; shared UI does not prove the same backend defect.
- Read aliases: GraphQL service(id)/server(id) (`apps/graphql.go:1100–1101`), REST service GET, MCP get_service. Dashboard /web, /worker, /pserv, /cron redirect to /services; static has concrete shared routes plus its catch-all alias (`common/lib/render-alias.ts:12–20,58–70`). Preserve subpath/query/hash behavior.
- **Family disposition:** web is confirmed; private/worker share Deployment code but were not created because starting plans are paid; cron shares status code but this symptom was not probed; static shares UI/API and has a separate build/publication path to verify. Postgres (/databases, /d) and Key Value (/keyvalue, /r) have separate handlers and are not findings here.

Fix shared service-local reporting, with any supported-type boundary explicitly decided in t002. Trace group staging separately. Preserve forbidden/unauthenticated/not-found/timeout/store-failure distinctions and CAS compensation. Never use errors as proof that saved state matches runtime, or expose secret/resource existence through the marker.

## Contract and dedupe

[ADR004](../../../../docs/ADR004-app-deployment.md) §Saved settings and selected runtime configuration describes the saved/runtime difference; `api/server.graphql:31–34` lists cancellation, rollback and Restart as examples. [Render environment docs](https://render.com/docs/configure-environment-variables) describe deferred application on Save only. This particular bool/banner is Bex correctness, not a documented Render wire-field requirement. [Render rollback docs](https://render.com/docs/rollbacks) preserve saved settings, as the passing controls do.

Searched open/blocked/done notes for undeployed, Save-only and saved/pending/banner terms, scanned open milestone READMEs across workstreams, and read DO_NOT_DO. No open item covers this ordinary Save-only gap. Current-main path history and `git log -S UndeployedChanges` / `-S stagePendingProjectionReferences` show no pending fix for this path.

- w5/done/m44: Save-only persistence/no-rollout implementation; those controls pass.
- w1/done/m152: cancellation snapshots/indicator; its after-cancel contract passes. Preserve its lazy migration and no-rollout guarantees.
- w5/done/m107: historical selection controls pass here. The ordinary pre-deploy save path was not its acceptance recipe; this is an uncovered adjacent path, not a claim that the whole milestone regressed.
- w4/blocked/m130: CAS write/compensation failures; this sparse UI write succeeds and sends no expectedEnvRevision.
- w4/blocked/m110 → w5/blocked/m105: native command/build behavior, not this status gap.
- w6/done/m51: Save only intentionally creates no deploy row; preserve that.

| Full w5/m107 DoD clause | This sweep's coverage |
| --- | --- |
| A → B → rollback A; saved B; next deploy B | Passed for secret file; saved Settings/env-var fields and original dev-5 recipe not repeated |
| Reconciliation, pod replacement, Restart, operator restart, shared groups | Explicit Restart passed; process/operator replacement and groups unverified |
| Full target/current matrix, cancel, current plan/disks/domains | Cancel and file selection passed; rest of matrix unverified |
| All adapters/UI, auto-deploy policy, missing/same-image records and indicators | Ordinary false observed across reads; historical true observed via GraphQL/UI; API auto-deploy, missing/evicted/same-image cases unverified |
| Suites, local evidence and cleanup | No product code/suite rerun; production cleanup passes; prior test claims not re-certified |

## Evidence and cleanup

Local artifacts: `.playwright-mcp/qa-env-r5-save-only-pending.png`, `qa-env-r5-save-only-repeat.png`, `qa-r5-evidence-final.json`, `qa-r5-runtime-final.json`, `qa-r5-cleanup.json`, and `qa-muse-r5-ledger.json` (all under .playwright-mcp). Existence was checked before filing; screenshots are local/ignored and the durable probes are above.

The settled Environment page had zero warning/error console entries before cleanup. Captured mutations/probes were HTTP 200. One in-flight Deploys read around navigation had no response body; later equivalent reads succeeded. Malformed exploratory queries and locator timeouts are harness mistakes, not findings.

Typed dashboard deletion completed. At 08:38:18Z GET returned HTTP 404 with `{"error":"not found","id":"not_found","message":"not found"}`; Overview no longer listed the service. At 08:38:43Z its App, Deployment, Pods, Jobs, CronJobs, Service, Ingress, Secrets and PVCs were absent across the cluster. The session was revoked through qa-login.sh --logout and its local session files removed. No other resource/session was changed.
