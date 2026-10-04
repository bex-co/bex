# QA sweep 8 — a Blueprint's first fromGroup deploy is canceled while its service runs

**Severity:** major. **Observed:** 2026-10-02 09:33–09:49 UTC, production dashboard/API, workspace `bex` (`tea-d98210cbbpdc73dcrkvg`), Scale workspace / free service plans. Credentials came from muse.env through the QA login harness; no credentials appear here. **Research HEAD:** `1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e`.

## Reproduction and target

1. Sign in, submit capture A once to `POST /v1/blueprints/deploy`. It creates one environment group with the harmless value `MESSAGE=qa-r8-group`, a native Go web service with `fromGroup`, and an otherwise equivalent literal-value control. Both services use `autoDeployTrigger: off`.
2. Open the grouped service's Deploys page and follow its returned `latestDeployId`. Wait through the queued/building state. Do not cancel, manually deploy, edit settings or re-apply.
3. The literal control reaches Running/rev-1 with one Live deploy and HTTP 200 `qa-r8-literal`. The grouped service runs a second build, reaches Running/rev-2 and HTTP 200 `qa-r8-group`, but its only deploy is Canceled, “Superseded by a newer release.”
4. Reload the grouped service page. Query GraphQL, REST and MCP as in capture D: all retain that lone canceled row. This persists after successful serving and is not a stale Events page.
5. Submit capture C once to create a second service referencing the now-existing group, without modifying the first service or group. It repeats the same two-build / Running rev-2 / only-deploy-Canceled result. Fresh-page and three-API evidence is capture E.
6. Confirm public responses with the external curl commands below, then delete the three services and group and revoke the QA session.

**Target:** the returned first deploy tracks the complete initial Blueprint configuration and becomes Live when that configuration serves. Initial attachment must not publish an untracked replacement release or make a successful first deployment look canceled. The correct service phase/public response are the control; do not normalize them to a false failure or label a genuinely superseded generation Live. Before readiness, the same initial deploy should truthfully remain queued/building/deploying. Genuine failures, user cancellation and later separate deploys retain their real terminal results.

## Live timeline

| Fixture | First deploy | Runtime result | History result |
| --- | --- | --- | --- |
| `qa-20261002-groupweb-r8` / `srv-davnkn6de41s73cantsg` | `dep-davnkn6de41s73cantt0`, created 09:33:16.874170Z | Running rev-2; 200 `qa-r8-group` | One row, canceled 09:37:26.059917Z |
| `qa-20261002-literal-r8` / `srv-davnko6de41s73cantu0` | `dep-davnko6de41s73cantug`, created 09:33:20.885178Z | Running rev-1; 200 `qa-r8-literal` | One row, live 09:36:37.530236Z |
| `qa-20261002-groupweb2-r8` / `srv-davnncmde41s73canu00` | `dep-davnncmde41s73canu0g`, created 09:38:58.528854Z | Running rev-2; 200 `qa-r8-group` | One row, canceled 09:41:37.518712Z |

Read-only `hetzner-prod` observations: all three Apps lived in namespace `tea-d98210cbbpdc73dcrkvg`; Jobs lived in `bex-build`.

- First grouped App: metadata generation 2, release annotation `1`, active release generation 1 and pending generation 2 while its first build ran. The grouped Secret refs were already in spec; `restartedAt` was absent. Native artifact fingerprint changed from `3201f526…` to `f14fe55d…`.
- First grouped build Jobs: gen-1 09:35:45–09:37:13Z; gen-2 09:37:14–09:38:31Z, both succeeded.
- Repeat grouped App: the same generation-2 / annotation-1 / active-1 / pending-2 state. Its gen-1 Job ran 09:39:57–09:41:23Z; gen-2 ran 09:41:25–09:42:43Z, both succeeded. Ready became True at 09:43:00Z; observedGeneration/releaseGeneration/configSnapshotGeneration were 2.
- Literal control: metadata/release generation 1, one gen-1 build (09:34:07–09:35:32Z), Ready 09:36:36Z and one Live deploy. Its literal is in the initial Blueprint spec, so it has no post-create group-link patch.

Screenshots, verified on disk and viewed:

- `.playwright-mcp/qa-r8-running-group-only-canceled-deploy.png` — fresh first-service page, 09:39:56Z.
- `.playwright-mcp/qa-r8-repeat-running-group-only-canceled-deploy.png` — fresh repeat page, 09:45:43Z.
- `.playwright-mcp/qa-r8-complete-captures.json` and `qa-r8-evidence-progress.json` — local request/progress/cluster observations. They also retain harness attempts; only successful contract probes support this finding.

The screenshots are local supplements. Complete replayable request/response captures below are the durable evidence. The repeat page had zero console warnings/errors before cleanup; its REST/MCP reads returned 200. Expected post-delete 404s are cleanup, not additional bugs. An investigator query using unsupported GraphQL ID/Time scalar names was corrected to String; its validation errors are not a product finding.

## Root cause and feasible fix

1. **Creation is dispatched before the initial group composition finishes.** `lego/backend/internal/apps/deploy.go:955` calls applyBlueprintCreate; `:963–970` then calls LinkEnvGroup for each requested group. The response appended at `:981` is the original create view. Missing Apps flow through `:2733` → createFromStack (`:2800`) → createNewApp (`apps/service.go:2109`) → materializeNewApp (`:2264`).
2. **That creation already owns a deploy.** `apps/service.go:2182–2187` provisions the App identity through Store.CreateApp. `store/store.go:872–878,926–956` fixes the first deploy generation at 1 and inserts it with trigger create. `apps/service.go:2333–2341` stamps release generation 1 before writing the CR, as w6/m46 required. This stamp is present in both live reproductions.
3. **The later link changes release inputs but intentionally does not open a deploy when gated.** `envgroups/service.go:1471–1489` authorizes/finds the group and delegates to linkFetched. `:1111–1114` adds EnvFromSecrets/FilesFromSecrets. For repo-backed auto-deploy-off Apps, `:1169,1182–1186` chooses a direct MergeFrom patch; it skips the Rollout.Patch path and restart stamp at `:1145,1188`. This existing policy is intentional for existing linked services (w2/m94), but the initial Blueprint creation has no previously serving complete release.
4. **The patch really increments generation.** Pinned controller-runtime v0.23.3 `pkg/client/patch.go:101–134` marshals old/new objects and computes the merge patch. Pinned Kubernetes v0.35.0 `apiextensions-apiserver/pkg/registry/customresource/strategy.go:145–155,158–185` initializes CR generation 1 and increments it when the non-metadata content changes. These actual dependency paths were opened in the local module cache. The live App confirms generation 2 after group attachment.
5. **The operator sees a second desired release.** `operator/internal/controller/release_identity.go:130–138` includes group env refs in native artifact identity; `:164–166` also includes group refs in release identity. `:251–267` pins an active build and queues the newer generation; `:273–279` adopts its changed fingerprint after the build settles. `:369–375` only accepts an annotation newer than the active release; an already-consumed annotation 1 does not mask a genuine spec change at metadata generation 2. This explains the two Jobs and the unchanged annotation.
6. **The row is canceled according to the state it sees.** `backend/internal/store/reconciler.go:1259–1260` closes an open deploy when the reported release generation has passed its generation. `:1220–1229` cannot find a deploy for generation 2, so the reason falls back to “Superseded by a newer release.” Suppressing this cancellation globally would hide real supersedes and leave the history gap unresolved.
7. **Consumers have no alternative Live row to display.** `deploys/graphql.go:96–115`, `deploys/rest.go:213–224` and `deploys/mcp.go:104–143` share the deploy list service. `dashboard/src/features/deploys/hooks/use-latest-deploy.ts:29–49` reads the newest row and returns its status; `service-detail-header.tsx:121–138` renders it. The fix must deliver a real, correctly associated Live deploy, which clears these existing predicates without changing their status mapping.

**Implementation direction:** prepare the new service's authorized group refs and membership as part of initial creation before the operator can dispatch a partial desired spec. Introduce an internal preparation/finalization seam or a coherent explicit initialization barrier; preserve compensation on failure. The layer can express the refs: `lego/types/v1alpha1/app_types.go:563–580` has EnvFromSecrets and FilesFromSecrets. Public CreateRequest does not accept arbitrary Secret names and must not gain an unsafe shortcut. Preserve the workspace/sensitive/environment and CAS checks described below.

**Why existing tests missed it:** `apps/blueprint_e2e_test.go:126–132` wires a fake Kubernetes client and group/secrets stores but no apps IntentStore or operator. `:157–166` proves the ref was attached, not that the returned deploy tracks the eventual release. `blueprint_env_test.go:97`'s fake LinkEnvGroup only records the call. The new regression must exercise actual deploy identity and controlled operator observation/generation behavior.

**Capture anomalies checked:** initial create responses legitimately have empty phase/revision because creation is asynchronous; their latestDeployId is nevertheless the durable first row. Auto-deploy is false in both manifest and API; two real build Jobs still exist because group refs change identity without requiring restartedAt. Both jobs succeeded, so build failure is not the cause. First-service Events lacks a build_ended row while the repeat includes one; this finding does not infer an independent event-emitter defect from that difference. Both contain a canceled deploy_ended for the initial row and no replacement deploy identity.

## Shared scope, aliases and neighbouring behavior

Counted production call sites, excluding declarations/tests:

| Shared code | Count and callers | Fix boundary |
| --- | --- | --- |
| EnvGroupApplier.LinkEnvGroup | 1: applyStackServices, deploy.go:969 | Initial Blueprint composition; preserve other group API semantics |
| applyStackServices | 1: deployParsedStack, deploy.go:767 | Shared executor |
| deployParsedStack | 3: deploy.go:637, blueprint.go:1008,1341 | Direct stack deploy; connected create; sync executor |
| applyBlueprintCreate | 2: deploy.go:955,1001 | First service pass; deferred host-reference pass |
| materializeNewApp | 2: service.go:2000,2156 | Ordinary create and Blueprint create |
| rollLinked | 3: envgroups/service.go:682,1111,1254 | Group contents, link and unlink; broad bypass would change existing policy |

Direct deploy aliases are REST `POST /v1/blueprints/deploy` and MCP `deploy` (two DeployStack adapter calls). There is no direct GraphQL DeployStack mutation. Connected create/sync expose REST `POST /v1/blueprints` and `POST /v1/blueprints/{id}/sync`, GraphQL `createBlueprint`/`syncBlueprint`, MCP `create_blueprint`/`sync_blueprint`, and dashboard `/blueprints/new`/`/blueprints/$blueprintId`. The auto-sync worker calls runSync at `blueprint_auto_sync_worker.go:99`, reaching the same sync executor. `render.yaml` and legacy `bex.yml` are filename aliases under ADR049. Deploy by-id consumers accompany each list API; they were not separately live-probed in this sweep.

Resource disposition: web · static · cron · worker · private share the App service path; Postgres and Key Value use separate appliers. Only native Go web / auto-deploy-off was reproduced live. Native and buildpack group inputs share current-main artifact handling; Docker/prebuilt and auto-deploy-on take distinct branches and require their own tests. Paid service creation was not attempted.

Authorization/failure disposition: `deploy.go:1598–1605` requires fresh sensitive permission before apply; LinkEnvGroup resolves only the acting workspace; `envgroups/service.go:1090–1108` rechecks workspace, fresh sensitive permission before idempotency, and environment compatibility. `:1117–1133` commits link metadata with CAS and handles deletion/fencing compensation. Preserve forbidden/unauthenticated refusals, scoped unknown-name failure, store-unavailable refusal, environment mismatch, conflict and write-failure compensation. Do not turn a failed preparation into a partial successful create or an existence oracle. No negative-authorization production probes were performed.

Adjacent initial forms are not presumed identical: `secrets/service.go:627,664–681` seeds sync:false/generateValue only when unset; `:932–939` calls materializeEnv → rollApp → Rollout.Patch (`:99–100`), which creates tracked config-change intent. Deferred fromService host refs run the second apply pass at `deploy.go:1001`. t002 verifies these separate paths and preserves seed-once semantics.

## Dedupe, deployment lag and prior guarantees

Scanned 37 open/blocked milestone READMEs across the board, open/completed notes for fromGroup/initial-group/first-deploy/cancel terms, DO_NOT_DO, the latest 40 product commits and symbol history. No anti-goal applies.

- w6/m46, commit `65b7cfbaa`: **regression of the observable first-deploy guarantee**, not a reverted annotation fix. The stamp still exists. w2/m94's gate (`0471fc791`) exposes the post-create case without a replacement row.
- w8/blocked/m46 concerns two near-simultaneous user deploy triggers and row/image races. These probes issue exactly one create per service and no subsequent trigger. Its shared terminal/concurrency invariants must survive, but it does not cover initial group composition.
- w4/m146 concerns repeated apply of explicit empty env; this is first apply with a nonempty group. w4/m147 concerns stale Activity reads; both fresh pages and all APIs reproduce the persisted canceled result here.
- w4/blocked/m110 → w5/m105 concerns a Settings edit failing to rebuild. Here two genuine builds succeeded. w4/done/089's neutral cancel reason is appropriate for the untracked newer generation. w1/done/m164's never-served cancellation/legacy fallback is preserved, not reopened.

Production bex-api/operator used `ghcr.io/bex-co/bex-operator@sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c`, pinned by `2790aaffe` to `e97ca42273a9`. Compared those source paths with research HEAD: initial create/link order, first-deploy stamp, gated patch and supersede decision are unchanged. Differences are unrelated region/environment-ID/proxy-warning/schema work and buildpack expansion of env-ref identity; native handling already exists in the deployed version. A fetch confirmed HEAD equals origin/main. This is not a fix awaiting deployment.

### Entire w6/m46 DoD audit

| Original criterion | This sweep and required preservation |
| --- | --- |
| UI private create never publicly serves | Not live-tested: private plans are paid. Preserve its private-route tests in t002/t005. |
| Forced expose/custom-host private App never gets an Ingress; red-before-fix envtest | Not rerun during filing. Keep this operator guard; initial assembly must not bypass it. |
| REST/GraphQL/MCP/Blueprint private create/read has no URL | Not live-tested; retain every adapter regression under t002/t005. |
| Fresh first deploy, no other trigger, becomes Live or a real failure, never Canceled | **Failed twice**, with new and existing group; literal control passes. This is the reopened guarantee. |
| Header converges after any deploy-closing event without reload | Header reflects the API's terminal canceled row, and fresh loads agree. No independent header-polling failure shown; preserve existing server-transition polling tests. |

### Other touched guarantees, item by item

| Prior DoD | Disposition |
| --- | --- |
| w2/m94 #1: existing linked group value edit, auto-deploy off, no row/old runtime until later deploy | Intentional existing behavior. Not exercised here; protect with regression. Initial composition is a separate case. |
| w2/m94 #2: auto-deploy on opens tracked config_change; pending-service IDs/copy accurate | Not exercised; verify in t002/t003. |
| w2/m94 #3: own file wins, unlink/remove leaves no stale file | Not exercised; preserve precedence and cleanup tests. |
| w2/m94 #4: last-linked group wins; env/file collision markers name the winner | Not exercised; preserve group order and UI/API regressions. |
| w2/m94 #5: over-quota shrink allowed, growth refused | Not exercised; group preparation must retain quota enforcement. |
| w2/m94 #6: ADR013/ADR018 describe gate and three precedence rules | Existing documented policy remains the baseline; record only the new initial-create behavior. |
| w1/m35: five env forms validate/deploy, later sync respects sync:false, ledger lists all five | envVarGroups/fromGroup materialize and serve here; first-deploy attribution fails. sync:false, fromService.envVarKey, generateValue, later-sync retention and ledger accuracy were not re-probed; t002 verifies the separate code paths. |
| w6/m51 #1: API Start Command edit creates a row | Not exercised; keep tracked Settings path. |
| w6/m51 #2: UI Start Command save appears in Deploys/Events | Not exercised; preserve, with Events freshness separately tracked by m147. |
| w6/m51 #3: broken command fails and header stabilizes | Not exercised; do not weaken failure projection. |
| w6/m51 #4: recovery save reaches Live without another deploy click | Not exercised; preserve recovery path. |
| w6/m51 #5: every build-relevant Settings verb covered or explicitly excluded | No new Settings finding; retain that inventory and exceptions. |
| w6/m51 #6: all 16 App patch sites tracked or verified exclusions | w2/m94 intentionally added a gate for existing group operations. Initial Blueprint composition needs its own coherent identity; do not remove the documented existing-service exclusion globally. |

## Render comparison

Render documents root environment groups and service fromGroup references in its [Blueprint specification](https://render.com/docs/blueprint-spec#environment-variables). Its [environment-variable guide](https://render.com/docs/configure-environment-variables) describes a deploy on group linking and automatic deployment of group changes for services with auto-deploy enabled. Bex's existing group gate/precedence choices are recorded in w2/m94. This finding uses Bex's first-deploy guarantee to define the target; it does not claim an authenticated Render reproduction of this precise timing/history case.

## Cleanup and evidence boundary

Dashboard deletion returned deleteService=true for each of the three service IDs. The group's REST DELETE returned 204; all four subsequent GETs returned 404. A fresh Overview contained none of the service names. Read-only cluster scans at 09:49 UTC found no matching Apps, Deployments, ReplicaSets, Pods, Services, Ingresses, Jobs, CronJobs, build Jobs or Secret names. The first scan caught normal finalization and a later scan verified zero residue. The QA logout returned `ok logged-out`; its jar/state were removed and browser cookies cleared.

No paid services, external domains, existing resources or product code were changed. Other runtime/types, connected sync/auto-sync and negative authorization are explicit remaining verification in the milestone. Initial group-link probes, literal control, cross-surface reads and deletion were exercised.

External runtime checks (each printed HTTP 200 and the expected body):

```sh
curl -fsS --max-time 20 -w '\nHTTP %{http_code}\n' https://qa-20261002-groupweb-r8.onbex.co/
curl -fsS --max-time 20 -w '\nHTTP %{http_code}\n' https://qa-20261002-groupweb2-r8.onbex.co/
curl -fsS --max-time 20 -w '\nHTTP %{http_code}\n' https://qa-20261002-literal-r8.onbex.co/
```

## Complete replayable captures

Send these requests inside the signed-in dashboard page with credentials included. POST bodies are JSON, Content-Type application/json; MCP also accepts application/json, text/event-stream. Cookies and auth headers are intentionally absent. Resources below have been deleted; replace the names with fresh QA names and carry their returned IDs into the reads. Do not re-apply the create manifest as a readiness poll.

### A. First creation — new group plus literal control

```json
{
  "at": "2026-10-02T09:33:21.650Z",
  "label": "first-apply-group-and-literal-control",
  "request": {
    "url": "https://api.bex.co/v1/blueprints/deploy",
    "method": "POST",
    "body": {
      "ownerId": "tea-d98210cbbpdc73dcrkvg",
      "bexYaml": "envVarGroups:\n  - name: qa-20261002-group-r8\n    envVars:\n      - key: MESSAGE\n        value: qa-r8-group\nservices:\n  - name: qa-20261002-groupweb-r8\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/hello-go\n    buildCommand: go build -o app .\n    startCommand: ./app\n    plan: free\n    autoDeployTrigger: off\n    envVars:\n      - fromGroup: qa-20261002-group-r8\n  - name: qa-20261002-literal-r8\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/hello-go\n    buildCommand: go build -o app .\n    startCommand: ./app\n    plan: free\n    autoDeployTrigger: off\n    envVars:\n      - key: MESSAGE\n        value: qa-r8-literal\n"
    }
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davnkn6de41s73cantsg",
        "name": "qa-20261002-groupweb-r8",
        "slug": "qa-20261002-groupweb-r8",
        "displayName": "",
        "type": "web_service",
        "phase": "",
        "undeployedChanges": false,
        "url": "https://qa-20261002-groupweb-r8.onbex.co",
        "urls": null,
        "image": "",
        "runtime": "go",
        "buildCommand": "go build -o app .",
        "startCommand": "./app",
        "builder": "native",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "",
        "internalAddress": "qa-20261002-groupweb-r8:3000",
        "port": 3000,
        "createdAt": "2026-10-02T09:33:17Z",
        "updatedAt": "2026-10-02T09:33:17.534149858Z",
        "dashboardUrl": "https://dashboard.bex.co/web/srv-davnkn6de41s73cantsg",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/hello-go",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "maxShutdownDelaySeconds": 30,
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        },
        "latestDeployId": "dep-davnkn6de41s73cantt0"
      },
      {
        "id": "srv-davnko6de41s73cantu0",
        "name": "qa-20261002-literal-r8",
        "slug": "qa-20261002-literal-r8",
        "displayName": "",
        "type": "web_service",
        "phase": "",
        "undeployedChanges": false,
        "url": "https://qa-20261002-literal-r8.onbex.co",
        "urls": null,
        "image": "",
        "runtime": "go",
        "buildCommand": "go build -o app .",
        "startCommand": "./app",
        "builder": "native",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "",
        "internalAddress": "qa-20261002-literal-r8:3000",
        "port": 3000,
        "createdAt": "2026-10-02T09:33:21Z",
        "updatedAt": "2026-10-02T09:33:21.516472561Z",
        "dashboardUrl": "https://dashboard.bex.co/web/srv-davnko6de41s73cantu0",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/hello-go",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "maxShutdownDelaySeconds": 30,
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        },
        "latestDeployId": "dep-davnko6de41s73cantug"
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": [
      {
        "id": "evg-davnkl6de41s73cants0",
        "name": "qa-20261002-group-r8"
      }
    ]
  }
}
```

### B. Literal control is Live while the first grouped deploy is canceled

```json
{
  "at": "2026-10-02T09:37:35.692Z",
  "label": "literal-control-serves-one-live-deploy",
  "public": {
    "url": "https://qa-20261002-literal-r8.onbex.co/",
    "status": 200,
    "body": "qa-r8-literal"
  },
  "request": {
    "url": "https://api.bex.co/graphql",
    "method": "POST",
    "body": {
      "query": "query {group:service(id:\"srv-davnkn6de41s73cantsg\"){id phase revision autoDeploy} literal:service(id:\"srv-davnko6de41s73cantu0\"){id phase revision autoDeploy} groupDeploys:deploys(serviceId:\"srv-davnkn6de41s73cantsg\",limit:20){id status trigger createdAt startedAt finishedAt cancelReason failureReason} literalDeploys:deploys(serviceId:\"srv-davnko6de41s73cantu0\",limit:20){id status trigger createdAt startedAt finishedAt cancelReason failureReason}}"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "group": {
        "autoDeploy": false,
        "id": "srv-davnkn6de41s73cantsg",
        "phase": "Building",
        "revision": ""
      },
      "groupDeploys": [
        {
          "cancelReason": "Superseded by a newer release",
          "createdAt": "2026-10-02T09:33:16.87417Z",
          "failureReason": "",
          "finishedAt": "2026-10-02T09:37:26.059917Z",
          "id": "dep-davnkn6de41s73cantt0",
          "startedAt": "2026-10-02T09:35:56.138015Z",
          "status": "canceled",
          "trigger": "create"
        }
      ],
      "literal": {
        "autoDeploy": false,
        "id": "srv-davnko6de41s73cantu0",
        "phase": "Running",
        "revision": "rev-1"
      },
      "literalDeploys": [
        {
          "cancelReason": "",
          "createdAt": "2026-10-02T09:33:20.885178Z",
          "failureReason": "",
          "finishedAt": "2026-10-02T09:36:37.530236Z",
          "id": "dep-davnko6de41s73cantug",
          "startedAt": "2026-10-02T09:34:26.025691Z",
          "status": "live",
          "trigger": "create"
        }
      ]
    }
  },
  "ui": "Projects\nqa-20261002-groupweb-r8\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-groupweb-r8\nService\nBuilding\nLatest deploy\nCanceled\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnkn6de41s73cantsg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-groupweb-r8.onbex.co\nSlug\nqa-20261002-groupweb-r8\n·\nInstances\n1\n·\nRevision\n—\n·\nCreated\n4m\nDeploys\n\n1 deploy\n\nAll statuses\nDeploy\tTrigger\tDuration\t\nActions\n\n\nCanceled\ndep-davnkn6de41s73cantt0\n\nSuperseded by a newer release\n\n1d2c03b docs(pm): file stale service activity from live QA\n\nCanceled just now\n\tFirst Deploy\t1m 29s\t"
}
```

### C. Second creation — another service with the existing group

```json
{
  "at": "2026-10-02T09:39:01.762Z",
  "label": "fresh-create-existing-group-control",
  "request": {
    "url": "https://api.bex.co/v1/blueprints/deploy",
    "method": "POST",
    "body": {
      "ownerId": "tea-d98210cbbpdc73dcrkvg",
      "bexYaml": "services:\n  - name: qa-20261002-groupweb2-r8\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    branch: main\n    rootDir: examples/hello-go\n    buildCommand: go build -o app .\n    startCommand: ./app\n    plan: free\n    autoDeployTrigger: off\n    envVars:\n      - fromGroup: qa-20261002-group-r8\n"
    }
  },
  "status": 200,
  "response": {
    "services": [
      {
        "id": "srv-davnncmde41s73canu00",
        "name": "qa-20261002-groupweb2-r8",
        "slug": "qa-20261002-groupweb2-r8",
        "displayName": "",
        "type": "web_service",
        "phase": "",
        "undeployedChanges": false,
        "url": "https://qa-20261002-groupweb2-r8.onbex.co",
        "urls": null,
        "image": "",
        "runtime": "go",
        "buildCommand": "go build -o app .",
        "startCommand": "./app",
        "builder": "native",
        "replicas": 1,
        "suspended": false,
        "plan": "free",
        "revision": "",
        "internalAddress": "qa-20261002-groupweb2-r8:3000",
        "port": 3000,
        "createdAt": "2026-10-02T09:38:59Z",
        "updatedAt": "2026-10-02T09:38:59.080952919Z",
        "dashboardUrl": "https://dashboard.bex.co/web/srv-davnncmde41s73canu00",
        "region": "fsn1",
        "idleTTLSeconds": 0,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "rootDir": "examples/hello-go",
        "repo": "https://github.com/bex-co/bex",
        "branch": "main",
        "autoDeploy": false,
        "notifyOnFail": "default",
        "notificationsToSend": "default",
        "renderSubdomainPolicy": "enabled",
        "maxShutdownDelaySeconds": 30,
        "maintenanceMode": {
          "enabled": false,
          "uri": ""
        },
        "latestDeployId": "dep-davnncmde41s73canu0g"
      }
    ],
    "databases": null,
    "keyValues": null,
    "envGroups": null
  }
}
```

### D. First grouped service — fresh UI, public response and three deploy APIs

```json
{
  "at": "2026-10-02T09:39:56.666Z",
  "label": "running-group-web-has-only-canceled-deploy-across-surfaces",
  "runtime": {
    "url": "https://qa-20261002-groupweb-r8.onbex.co/",
    "status": 200,
    "body": "qa-r8-group"
  },
  "ui": "Projects\nqa-20261002-groupweb-r8\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-groupweb-r8\nService\nRunning\nLatest deploy\nCanceled\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnkn6de41s73cantsg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-groupweb-r8.onbex.co\nSlug\nqa-20261002-groupweb-r8\n·\nInstances\n1\n·\nRevision\nrev-2\n·\nCreated\n6m\nDeploys\n\n1 deploy\n\nAll statuses\nDeploy\tTrigger\tDuration\t\nActions\n\n\nCanceled\ndep-davnkn6de41s73cantt0\n\nSuperseded by a newer release\n\n1d2c03b docs(pm): file stale service activity from live QA\n\nCanceled 2 minutes ago\n\tFirst Deploy\t1m 29s\t",
  "probes": [
    {
      "request": {
        "surface": "GraphQL",
        "url": "https://api.bex.co/graphql",
        "method": "POST",
        "body": {
          "query": "query($id:String!,$end:String!){service(id:$id){id phase revision autoDeploy} deploys(serviceId:$id,limit:20){id status trigger createdAt startedAt finishedAt cancelReason failureReason} serviceEvents(serviceId:$id,startTime:\"2026-10-02T09:32:00Z\",endTime:$end,limit:20){id type timestamp details{deployId deployStatus status cancelReason}}}",
          "variables": {
            "id": "srv-davnkn6de41s73cantsg",
            "end": "2026-10-02T09:39:56.430Z"
          }
        }
      },
      "status": 200,
      "response": {
        "data": {
          "deploys": [
            {
              "cancelReason": "Superseded by a newer release",
              "createdAt": "2026-10-02T09:33:16.87417Z",
              "failureReason": "",
              "finishedAt": "2026-10-02T09:37:26.059917Z",
              "id": "dep-davnkn6de41s73cantt0",
              "startedAt": "2026-10-02T09:35:56.138015Z",
              "status": "canceled",
              "trigger": "create"
            }
          ],
          "service": {
            "autoDeploy": false,
            "id": "srv-davnkn6de41s73cantsg",
            "phase": "Running",
            "revision": "rev-2"
          },
          "serviceEvents": [
            {
              "details": {
                "cancelReason": "Superseded by a newer release",
                "deployId": "dep-davnkn6de41s73cantt0",
                "deployStatus": "canceled",
                "status": ""
              },
              "id": "evt-8mt104g6dessd71uoloq",
              "timestamp": "2026-10-02T09:37:26Z",
              "type": "deploy_ended"
            },
            {
              "details": {
                "cancelReason": "Superseded by a newer release",
                "deployId": "dep-davnkn6de41s73cantt0",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-sv54k1l9b8jcmk2mmdmt",
              "timestamp": "2026-10-02T09:35:56Z",
              "type": "build_started"
            },
            {
              "details": {
                "cancelReason": "",
                "deployId": "",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-02ambqv8peki4n1nrdig",
              "timestamp": "2026-10-02T09:33:19Z",
              "type": "env_group_linked"
            },
            {
              "details": {
                "cancelReason": "",
                "deployId": "dep-davnkn6de41s73cantt0",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-68oja3bhqjjn9sg8jqcr",
              "timestamp": "2026-10-02T09:33:16Z",
              "type": "deploy_started"
            }
          ]
        }
      }
    },
    {
      "request": {
        "surface": "REST deploy list",
        "url": "https://api.bex.co/v1/services/srv-davnkn6de41s73cantsg/deploys?limit=20",
        "method": "GET"
      },
      "status": 200,
      "response": [
        {
          "deploy": {
            "id": "dep-davnkn6de41s73cantt0",
            "serviceId": "srv-davnkn6de41s73cantsg",
            "status": "canceled",
            "trigger": "create",
            "commit": {
              "id": "1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e",
              "message": "docs(pm): file stale service activity from live QA"
            },
            "createdAt": "2026-10-02T09:33:16.87417Z",
            "updatedAt": "2026-10-02T09:37:26.059931Z",
            "startedAt": "2026-10-02T09:35:56.138015Z",
            "finishedAt": "2026-10-02T09:37:26.059917Z",
            "cancelReason": "Superseded by a newer release"
          },
          "cursor": "dep-davnkn6de41s73cantt0"
        }
      ]
    },
    {
      "request": {
        "surface": "MCP list_deploys",
        "url": "https://api.bex.co/mcp",
        "method": "POST",
        "body": {
          "jsonrpc": "2.0",
          "id": 80,
          "method": "tools/call",
          "params": {
            "name": "list_deploys",
            "arguments": {
              "serviceId": "srv-davnkn6de41s73cantsg",
              "limit": 20
            }
          }
        }
      },
      "status": 200,
      "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":80,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"cursor\\\":\\\"dep-davnkn6de41s73cantt0\\\",\\\"deploys\\\":[{\\\"cancelReason\\\":\\\"Superseded by a newer release\\\",\\\"commit\\\":{\\\"id\\\":\\\"1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e\\\",\\\"message\\\":\\\"docs(pm): file stale service activity from live QA\\\"},\\\"createdAt\\\":\\\"2026-10-02T09:33:16.87417Z\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:37:26.059917Z\\\",\\\"id\\\":\\\"dep-davnkn6de41s73cantt0\\\",\\\"serviceId\\\":\\\"srv-davnkn6de41s73cantsg\\\",\\\"startedAt\\\":\\\"2026-10-02T09:35:56.138015Z\\\",\\\"status\\\":\\\"canceled\\\",\\\"trigger\\\":\\\"create\\\",\\\"updatedAt\\\":\\\"2026-10-02T09:37:26.059931Z\\\"}]}\"}],\"structuredContent\":{\"cursor\":\"dep-davnkn6de41s73cantt0\",\"deploys\":[{\"cancelReason\":\"Superseded by a newer release\",\"commit\":{\"id\":\"1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e\",\"message\":\"docs(pm): file stale service activity from live QA\"},\"createdAt\":\"2026-10-02T09:33:16.87417Z\",\"finishedAt\":\"2026-10-02T09:37:26.059917Z\",\"id\":\"dep-davnkn6de41s73cantt0\",\"serviceId\":\"srv-davnkn6de41s73cantsg\",\"startedAt\":\"2026-10-02T09:35:56.138015Z\",\"status\":\"canceled\",\"trigger\":\"create\",\"updatedAt\":\"2026-10-02T09:37:26.059931Z\"}]}}}\n\n"
    }
  ]
}
```

### E. Repeat — fresh UI, public response and three deploy APIs

```json
{
  "label": "repeat-running-group-web-only-canceled-deploy-cross-surfaces",
  "at": "2026-10-02T09:45:43.345Z",
  "url": "https://dashboard.bex.co/services/srv-davnncmde41s73canu00/deploys",
  "ui": "B\nbex\nToggle Sidebar\nDashboard\n\nqa-20261002-groupweb2-r8\n\nDeploys\nSettings\nMonitor\nEvents\nLogs\nMetrics\nManage\nEnvironment\nDisk\nShell\nScaling\nPlan\nProjects\nqa-20261002-groupweb2-r8\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-groupweb2-r8\nService\nRunning\nLatest deploy\nCanceled\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnncmde41s73canu00\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-groupweb2-r8.onbex.co\nSlug\nqa-20261002-groupweb2-r8\n·\nInstances\n1\n·\nRevision\nrev-2\n·\nCreated\n5m\nDeploys\n\n1 deploy\n\nAll statuses\nDeploy\tTrigger\tDuration\t\nActions\n\n\nCanceled\ndep-davnncmde41s73canu0g\n\nSuperseded by a newer release\n\n1d2c03b docs(pm): file stale service activity from live QA\n\nCanceled 4 minutes ago\n\tFirst Deploy\t1m 28s\t",
  "apis": [
    {
      "request": {
        "url": "https://api.bex.co/graphql",
        "method": "POST",
        "body": {
          "query": "query Q($id:String!,$end:String!){service(id:$id){id phase revision autoDeploy} deploys(serviceId:$id,limit:20){id status trigger createdAt startedAt finishedAt cancelReason failureReason} serviceEvents(serviceId:$id,startTime:\"2026-10-02T09:32:00Z\",endTime:$end,limit:20){id type timestamp details{deployId deployStatus status cancelReason}}}",
          "variables": {
            "id": "srv-davnncmde41s73canu00",
            "end": "2026-10-02T09:45:41.010Z"
          }
        }
      },
      "status": 200,
      "response": "{\"data\":{\"deploys\":[{\"cancelReason\":\"Superseded by a newer release\",\"createdAt\":\"2026-10-02T09:38:58.528854Z\",\"failureReason\":\"\",\"finishedAt\":\"2026-10-02T09:41:37.518712Z\",\"id\":\"dep-davnncmde41s73canu0g\",\"startedAt\":\"2026-10-02T09:40:08.585681Z\",\"status\":\"canceled\",\"trigger\":\"create\"}],\"service\":{\"autoDeploy\":false,\"id\":\"srv-davnncmde41s73canu00\",\"phase\":\"Running\",\"revision\":\"rev-2\"},\"serviceEvents\":[{\"details\":{\"cancelReason\":\"Superseded by a newer release\",\"deployId\":\"dep-davnncmde41s73canu0g\",\"deployStatus\":\"canceled\",\"status\":\"\"},\"id\":\"evt-vhltc27ngp1eqg1t0c8l\",\"timestamp\":\"2026-10-02T09:41:37Z\",\"type\":\"deploy_ended\"},{\"details\":{\"cancelReason\":\"Superseded by a newer release\",\"deployId\":\"dep-davnncmde41s73canu0g\",\"deployStatus\":\"\",\"status\":\"succeeded\"},\"id\":\"evt-rjj82eir7lin8d914otq\",\"timestamp\":\"2026-10-02T09:41:26Z\",\"type\":\"build_ended\"},{\"details\":{\"cancelReason\":\"Superseded by a newer release\",\"deployId\":\"dep-davnncmde41s73canu0g\",\"deployStatus\":\"\",\"status\":\"\"},\"id\":\"evt-hf22srigjlfdbvhvf7g6\",\"timestamp\":\"2026-10-02T09:40:08Z\",\"type\":\"build_started\"},{\"details\":{\"cancelReason\":\"\",\"deployId\":\"\",\"deployStatus\":\"\",\"status\":\"\"},\"id\":\"evt-83927vu62rnicvb7ouep\",\"timestamp\":\"2026-10-02T09:39:01Z\",\"type\":\"env_group_linked\"},{\"details\":{\"cancelReason\":\"\",\"deployId\":\"dep-davnncmde41s73canu0g\",\"deployStatus\":\"\",\"status\":\"\"},\"id\":\"evt-3g0dvlgb4nrdocotrfjc\",\"timestamp\":\"2026-10-02T09:38:58Z\",\"type\":\"deploy_started\"}]}}\n"
    },
    {
      "request": {
        "url": "https://api.bex.co/v1/services/srv-davnncmde41s73canu00/deploys?limit=20",
        "method": "GET"
      },
      "status": 200,
      "response": "[{\"deploy\":{\"id\":\"dep-davnncmde41s73canu0g\",\"serviceId\":\"srv-davnncmde41s73canu00\",\"status\":\"canceled\",\"trigger\":\"create\",\"commit\":{\"id\":\"1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e\",\"message\":\"docs(pm): file stale service activity from live QA\"},\"createdAt\":\"2026-10-02T09:38:58.528854Z\",\"updatedAt\":\"2026-10-02T09:41:37.518713Z\",\"startedAt\":\"2026-10-02T09:40:08.585681Z\",\"finishedAt\":\"2026-10-02T09:41:37.518712Z\",\"cancelReason\":\"Superseded by a newer release\"},\"cursor\":\"dep-davnncmde41s73canu0g\"}]\n"
    },
    {
      "request": {
        "url": "https://api.bex.co/mcp",
        "method": "POST",
        "body": {
          "jsonrpc": "2.0",
          "id": 81,
          "method": "tools/call",
          "params": {
            "name": "list_deploys",
            "arguments": {
              "serviceId": "srv-davnncmde41s73canu00",
              "limit": 20
            }
          }
        }
      },
      "status": 200,
      "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":81,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"cursor\\\":\\\"dep-davnncmde41s73canu0g\\\",\\\"deploys\\\":[{\\\"cancelReason\\\":\\\"Superseded by a newer release\\\",\\\"commit\\\":{\\\"id\\\":\\\"1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e\\\",\\\"message\\\":\\\"docs(pm): file stale service activity from live QA\\\"},\\\"createdAt\\\":\\\"2026-10-02T09:38:58.528854Z\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:41:37.518712Z\\\",\\\"id\\\":\\\"dep-davnncmde41s73canu0g\\\",\\\"serviceId\\\":\\\"srv-davnncmde41s73canu00\\\",\\\"startedAt\\\":\\\"2026-10-02T09:40:08.585681Z\\\",\\\"status\\\":\\\"canceled\\\",\\\"trigger\\\":\\\"create\\\",\\\"updatedAt\\\":\\\"2026-10-02T09:41:37.518713Z\\\"}]}\"}],\"structuredContent\":{\"cursor\":\"dep-davnncmde41s73canu0g\",\"deploys\":[{\"cancelReason\":\"Superseded by a newer release\",\"commit\":{\"id\":\"1d2c03b5b1f261b9f60ca2144f97af5cfb3d815e\",\"message\":\"docs(pm): file stale service activity from live QA\"},\"createdAt\":\"2026-10-02T09:38:58.528854Z\",\"finishedAt\":\"2026-10-02T09:41:37.518712Z\",\"id\":\"dep-davnncmde41s73canu0g\",\"serviceId\":\"srv-davnncmde41s73canu00\",\"startedAt\":\"2026-10-02T09:40:08.585681Z\",\"status\":\"canceled\",\"trigger\":\"create\",\"updatedAt\":\"2026-10-02T09:41:37.518713Z\"}]}}}\n\n"
    }
  ],
  "public": {
    "url": "https://qa-20261002-groupweb2-r8.onbex.co/",
    "status": 200,
    "body": "qa-r8-group"
  }
}
```

### F. Cleanup mutations and read verification

```json
[
  {
    "at": "2026-10-02T09:48:18.005Z",
    "label": "dashboard-cleanup-three-r8-webs",
    "results": [
      {
        "id": "srv-davnncmde41s73canu00",
        "name": "qa-20261002-groupweb2-r8",
        "status": 200,
        "response": "[{\"data\":{\"deleteService\":true}}]\n"
      },
      {
        "id": "srv-davnkn6de41s73cantsg",
        "name": "qa-20261002-groupweb-r8",
        "status": 200,
        "response": "[{\"data\":{\"deleteService\":true}}]\n"
      },
      {
        "id": "srv-davnko6de41s73cantu0",
        "name": "qa-20261002-literal-r8",
        "status": 200,
        "response": "[{\"data\":{\"deleteService\":true}}]\n"
      }
    ]
  },
  {
    "at": "2026-10-02T09:48:37.607Z",
    "label": "r8-api-cleanup-verification",
    "results": [
      {
        "request": {
          "method": "GET",
          "url": "https://api.bex.co/v1/services/srv-davnncmde41s73canu00"
        },
        "status": 404,
        "response": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      },
      {
        "request": {
          "method": "GET",
          "url": "https://api.bex.co/v1/services/srv-davnkn6de41s73cantsg"
        },
        "status": 404,
        "response": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      },
      {
        "request": {
          "method": "GET",
          "url": "https://api.bex.co/v1/services/srv-davnko6de41s73cantu0"
        },
        "status": 404,
        "response": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      },
      {
        "request": {
          "method": "DELETE",
          "url": "https://api.bex.co/v1/env-groups/evg-davnkl6de41s73cants0"
        },
        "status": 204,
        "response": ""
      },
      {
        "request": {
          "method": "GET",
          "url": "https://api.bex.co/v1/env-groups/evg-davnkl6de41s73cants0"
        },
        "status": 404,
        "response": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
      }
    ]
  }
]
```
