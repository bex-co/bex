# Activity freezes at its initial time window while the service header keeps updating

Why: users watching a deploy see a completed live release labelled In Progress with Cancel, and later service changes remain absent until navigation.

- **Severity:** major — misleading lifecycle state on a core hosting page; the backend operation and serving workload remain correct.
- **Observed:** 2026-10-02, 09:12–09:22 UTC, authenticated QA workspace bex (tea-d98210cbbpdc73dcrkvg).
- **Fixture:** qa-20261002-web-r7, srv-davnajmde41s73cantfg; initial deploy dep-davnajmde41s73cantg0; free Go service from https://github.com/bex-co/bex, main, examples/hello-go, build go build -o app ., start ./app, port 3000, autoDeploy false. No secrets or paid resources.
- **Page:** https://dashboard.bex.co/services/srv-davnajmde41s73cantfg/events.
- **Research HEAD:** ec1fe500fc9f84fe650b465f5479412bca13a9e2. Dashboard image: ghcr.io/bex-co/bex-dashboard@sha256:ab909cdb65a869179db611630cd5f9f28c07765d7f117459b34abad620af7308. API/operator: ghcr.io/bex-co/bex-operator@sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c.

## Reproduction and controls

1. Create that fixture through New Web Service; open Events before the queued build starts. The initial query ends at 09:12:07.334Z and returns only deploy_started.
2. Leave that visible page open. The API records build_started at 09:12:55Z, build_ended at 09:14:07Z and deploy_ended at 09:14:25Z. The header updates to Running / Live / rev-1. At 09:16:58Z Activity still contains one row, Deploy started / In Progress / Cancel. No ServiceEvents request occurred between the listener's installation at 09:12:38Z and this capture.
3. Replay the exact initial window: it still returns one event, correctly. Advancing just endTime gets the correct range-cap error because the starting window was already 720 hours. Move both bounds together to keep 720 hours: the new events are returned.
4. Reload Events at 09:17:23Z: it now shows all four lifecycle rows, Live and no Cancel on the started row. This excludes missing facts, vocabulary filtering and permission failures as the cause.
5. Without another navigation, issue the captured SetIdleTimeout mutation at 09:17:39Z. API idle_timeout_changed, service_hibernated and service_woken facts follow. At 09:19:02Z the Activity count is still four. The configuration and first sleep events have been missing for more than 60 seconds. The wake event is also absent at this sample, but has existed for only 25 seconds, so it alone is not the timing failure proof.
6. The one ServiceEvents request recorded in the second watch is the explicitly injected valid-window probe at 09:17:40.731Z; it is not an automatic UI refresh. Direct probes used fetch and did not update Apollo's cache.
7. On Metrics, Last 30 minutes + Show event timeline produces a second ServiceEvents query 15 seconds later with advanced bounds and eight events instead of seven. A final fresh Events page shows all eight rows, including the second hibernation.

The public service served HTTP 200 / OK before sleep. Read-only Kubernetes confirmed spec.replicas=0 and zero ready/available replicas after hibernation. Its first JSON wake request returned 503 plus Retry-After: 5; the follow-up 15.6 seconds later returned 200 / OK. Sleep/wake itself passed. Routine public-traffic wakes are designed behavior, consistent with DO_NOT_DO; no scanner-wake finding is proposed.

## Root cause and target

| Source | Mechanism |
| --- | --- |
| dashboard/src/routes/services.$serviceId.events.tsx:97 | historyWindow is initialized once with useState and has no setter. Its start/end never advance during the page lifetime. |
| Same route:105 | Passes the frozen bounds, limit 20 and historical paging floor to useServiceEvents. |
| dashboard/src/features/events/hooks/use-service-events.ts:84 | Memoized variables change only with the supplied bounds/service/limit. useQuery uses cache-and-network and no polling. |
| Same hook:207 | resetAndRefetch resets paging then calls refetch with those same old variables. |
| dashboard/src/routes/services.$serviceId.events.tsx:113 | finishedDeployIds is derived only from the feed's deploy_ended rows. |
| Same route:239 and :265 | A deploy_started without that terminal ID gets update_in_progress; isCancelableDeployStatus makes Cancel appear. Fetching the missing completion clears both predicates. |
| Same route:203 and :301 | Error retry and DeployActions' onChanged use the fixed-range refetch. Adding a mutation refetch alone cannot repair the range. |
| lego/backend/internal/events/service.go:693, :751 | Parses and authorizes the explicit range, enforces its cap, and forwards it to the store. |
| lego/backend/internal/store/events.go:448 and :463 | SQL deliberately excludes at > Until; the UI upper bound maps directly to that predicate. |

Read the installed and locked Apollo Client **4.1.3**, matching the live request metadata: ObservableQuery.js:844–860 cancels polling when pollInterval is absent; :366–395 refetch preserves variables unless replacements are supplied; QueryManager.js's cache-and-network branch performs a network read for an invocation, not a recurring timer. The ordinary header updates cannot append ServiceEvent identities into this fixed query result. Both the missing network reads and the unchanged-window replay support the mechanism.

**Target:** refresh the Events page's current head on Bex's visible resource cadence (30 seconds baseline; reuse existing polling policy), with current bounded variables that can include newly available facts. After deploy_ended arrives, render the ended row's true status and withdraw the started row's stale progress/action. Include action-triggered refresh in the same advancing-window behavior.

**Scope the live behavior to Events.** Preserve useServiceEvents' explicit fixed-range contract for Metrics. Keep loaded historical rows/cursors, deduplicate by stable event ID, and order new arrivals consistently. Moving both bounds must not silently discard older pages or reset the user's filters; a pollInterval alone is insufficient, and advancing only endTime makes the initial 720-hour window invalid. The current requestKey resets pageRef on any bound change (:97–108), so this needs deliberate paging integration, not a timer pasted onto the route. Delayed facts carry their occurrence time: do not assume an exclusive last-poll watermark is enough without overlap/reconciliation.

During initial load, retain the existing Events skeleton geometry. During later refreshes keep the last good rows visible and reconcile the new result; report actual read failures rather than an empty success. Hidden documents should follow the existing skip-poll policy.

## Shared callers, aliases and family

Exhaustive production search finds **three useServiceEvents call sites**:

1. Events route :105 — broken live bound/refresh intent.
2. services.$serviceId.metrics.tsx:86 — marker feed, selected range and autoPaginate.
3. features/events/components/event-timeline.tsx:35 — timeline, same explicit range and autoPaginate.

The Metrics route supplies useLiveRange (:80). Its relative timer advances both bounds; absolute custom ranges do not arm a timer (use-live-range.ts:19–39). The live 30-minute control confirms the relative case; custom ranges are source-checked only and need regression coverage.

ServiceEventsPage has **two production mount sites**: the generic services route :55 and static.$serviceId.events.tsx:13. Web, cron, worker and private services share /services; static has /static, with the generic loader redirecting static services to that base. The /web, /worker, /pserv and /cron catch-all aliases call redirectRenderAlias and preserve the events suffix. Only web was reproduced in this sweep; static/cron/worker/private are inferred affected callers to verify, not separately measured defects. Postgres and Key Value use their own datastore activity pages and are outside these three hook callers.

API reads involved: REST GET /v1/services/{id}/events, GraphQL serviceEvents, MCP list_service_events and list_events. Their time-filtered facts were correct in the four live probes below; no API/authentication change is needed. The deployed list_events output uses the older envelope; main's a9407d21d already changes that Render MCP wire shape, so this is known deploy lag, not a new finding.

**Adjacent classes:** bounded historical windows stay historical; empty successful pages stay empty; unauthorized, missing-resource, unavailable-source and query errors retain the existing backend classification. Do not widen authorization, remove range caps or mask failed refreshes. Stop timers on unmount/service change; a response from the previous service must not append to the new service. Event filters, unknown-type fail-open behavior, terminal failure/cancellation variants and loaded-page deduplication need guards.

## Dedupe and prior guarantees

Searched open, blocked and done notes for service events, Activity freshness, useServiceEvents, historyWindow, polling and fixed windows; scanned all 36 open/blocked milestone READMEs. Reviewed recent 40 dashboard/lego commits and targeted history: 1a70196b8 introduced the current historical window; no newer fix advances it. Read DO_NOT_DO and QA precedent w9/m89 and w9/m92; neither covers this hosting defect.

- **w4/done/073:** repaired useDeploys, with a previous unverified question about Events. This route reads useServiceEvents instead. The old fix's list polling remains in place; this is the uncovered second consumer, not reopening the repaired list.
- **w4/done/m138:** Blueprint Sync History and plan summaries, different query/hooks.
- **w4/done/m129:** immutable event ownership and no-op audit writes. Source association is not the failure; typed IDs and distinct new facts are correct.
- **w6/done/m45/t003:** mutation-driven header refresh. The header is correct here, and the operator/config facts arrive without a local deploy mutation.
- **w5/done/m43:** Metrics markers; the live control passed. Other search matches were webhooks, logs/SSE or agent sessions, not this page.

This is an uncovered live freshness gap. No evidence shows the old producer, catalog or historical-window implementations regressed. For completeness, their entire DoD scope is dispositioned below so the fix preserves its inheritance:

| Earlier DoD | This sweep / required preservation |
| --- | --- |
| w3/m19: quiet services retain history beyond the API's default hour | Fixture is only minutes old; long history unprobed. Keep explicit bounded history and test older windows in t002. |
| w3/m19: cursor older pages without duplicate/gap; Render-shaped filtering | Not live-paged or filter-toggled here. These are required sibling regressions because head refresh can disturb pageRef and exclusions. |
| w3/m19: Metrics selected range | Relative 30-minute range passed; custom absolute selection remains an explicit test task. |
| w3/m19: REST/GQL/MCP/UI/markers/webhooks agree across all 19 sourceable transitions | Seven event rows agree across API list surfaces at one cutoff; final UI has eight after a later sleep. Webhooks and the full 19-transition sequence were not run. No producer change proposed. |
| w3/m19: at most once per real edge; retries/resyncs no duplicates; secret/env values cannot enter facts; unsupported labels absent | This fixture's repeated sleep rows represent different real edges. Full retry/19-type/redaction/non-goal matrix was not rerun; keep existing producer and schema guards. |
| w7/m66 #1: build lifecycle pair, including no pair on image-only deploys | Repo-backed build pair and terminal status present in APIs and after reload. Image-only negative unprobed. |
| w7/m66 #2–4: pre-deploy pair, one-off-job completion, branch-deleted decision | No such operations in this fixture; do not infer defects or change their sources. |
| w7/m66 #5: new types visible/filterable/localized/marked and adapter-compatible | Build pair visible after reload; API event identities agree and Metrics refresh works. Full filter/locale/type matrix unprobed; preserve tests. |
| w7/m66 #6–7: omission docs corrected, named-or-excused/surface/event guards, structurally redacted facts | Source vocabulary is unchanged; keep existing documentation and regression suite. Those backend checks were not rerun by this filing. |
| w6/m122 #1–2: verified-domain event and disk types visible/selectable | Those types were not generated; preserve catalog/filter behavior. |
| w6/m122 #3–4: unknown types visible and backend drift fails build | Not mutated/probed here; preserve existing fail-open and vocabulary guards. |
| w6/m122 #5–7: count covers returned feed, Metrics unaffected, select-all admits unknowns | Count correctly matches each fetched result (1, then 4, then 8). Metrics relative refresh passed. Unknown select-all unprobed; required shared-caller regression. |

## Render comparison

Read on 2026-10-02: [Render List events](https://api-docs.render.com/reference/list-events) documents an explicit start/end window, a one-hour default and cursor pages; [Deploying on Render](https://render.com/docs/deploys) describes deploy history/current live deploy and skipped-deploy entries. Neither source specifies an authenticated dashboard refresh interval. No Render account was used here. The proposed cadence follows Bex's common/lib/polling.ts policy; backend historical-query semantics remain compatible.

## Artifacts and limits

Verified local screenshots: .playwright-mcp/qa-r7-stale-events-live-header.png and .playwright-mcp/qa-r7-events-stale-after-sleep-wake.png. Raw local record: .playwright-mcp/qa-r7-evidence-progress.json. These are gitignored; the replayable evidence below is the durable handoff.

No static/cron/worker/private/Postgres/Key Value live repro, hidden-tab test, older-page fixture, failing-refresh injection, custom Metrics range or authenticated Render cadence test was run. Do not promote those inferences into live acceptance evidence.

Fixture deletion succeeded through Delete Service at 09:22:43Z, REST read returned 404, and the exact App/Deployment/Service/Ingress name had no matches. The QA session was revoked successfully and local session files removed.

## Durable request/response captures

Every JSON block below contains the exact captured request and complete corresponding response, without credentials. GraphQL is POST https://api.bex.co/graphql with the logged-in cookie; credentials are intentionally omitted. Recreate a disposable fixture and substitute its service ID and a current bounded window when replaying.

### Initial page query and result

```json
{
  "at": "2026-10-02T09:12:07.687Z",
  "request": [
    {
      "operationName": "ServiceEvents",
      "variables": {
        "serviceId": "srv-davnajmde41s73cantfg",
        "startTime": "2026-09-02T09:12:07.334Z",
        "endTime": "2026-10-02T09:12:07.334Z",
        "limit": 20
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "serviceEvents": [
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": {
                "__typename": "DeployTrigger",
                "clearCache": false,
                "deployHook": false,
                "deployedByRender": false,
                "envUpdated": false,
                "firstBuild": true,
                "manual": false,
                "rollback": false
              },
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            },
            "id": "evt-p2986dqmjiapvibau5ok",
            "timestamp": "2026-10-02T09:11:43Z",
            "type": "deploy_started"
          }
        ]
      }
    }
  ]
}
```

### API knows the deploy is live while the page remains stale

```json
{
  "at": "2026-10-02T09:14:29.713Z",
  "label": "events-arrival-watch-2",
  "url": "https://dashboard.bex.co/services/srv-davnajmde41s73cantfg/events",
  "visibility": "visible",
  "eventPollRequests": [],
  "text": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nRunning\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n2m\nActivity\n1\nRecent deploys and service changes.\nFilter events\n\nDeploy started\n\nIn Progress\nFirst Deploy\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n2m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\nCancel\n\nYou've reached the beginning of this service's history.",
  "request": {
    "query": "query($start:String!,$end:String!){service(id:\"srv-davnajmde41s73cantfg\"){id phase revision} deploys(serviceId:\"srv-davnajmde41s73cantfg\",limit:5){id status startedAt finishedAt} serviceEvents(serviceId:\"srv-davnajmde41s73cantfg\",startTime:$start,endTime:$end,limit:20){id type timestamp details{deployId deployStatus status}}}",
    "variables": {
      "start": "2026-10-02T09:11:00Z",
      "end": "2026-10-02T09:14:29.713Z"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "finishedAt": "2026-10-02T09:14:25.995456Z",
          "id": "dep-davnajmde41s73cantg0",
          "startedAt": "2026-10-02T09:12:55.997893Z",
          "status": "live"
        }
      ],
      "service": {
        "id": "srv-davnajmde41s73cantfg",
        "phase": "Running",
        "revision": "rev-1"
      },
      "serviceEvents": [
        {
          "details": {
            "deployId": "dep-davnajmde41s73cantg0",
            "deployStatus": "succeeded",
            "status": ""
          },
          "id": "evt-s3qgt8g74f3vjairkpsm",
          "timestamp": "2026-10-02T09:14:25Z",
          "type": "deploy_ended"
        },
        {
          "details": {
            "deployId": "dep-davnajmde41s73cantg0",
            "deployStatus": "",
            "status": "succeeded"
          },
          "id": "evt-15950bkbualndban9r7m",
          "timestamp": "2026-10-02T09:14:07Z",
          "type": "build_ended"
        },
        {
          "details": {
            "deployId": "dep-davnajmde41s73cantg0",
            "deployStatus": "",
            "status": ""
          },
          "id": "evt-03bvi6kp06iejopm5r1n",
          "timestamp": "2026-10-02T09:12:55Z",
          "type": "build_started"
        },
        {
          "details": {
            "deployId": "dep-davnajmde41s73cantg0",
            "deployStatus": "",
            "status": ""
          },
          "id": "evt-p2986dqmjiapvibau5ok",
          "timestamp": "2026-10-02T09:11:43Z",
          "type": "deploy_started"
        }
      ]
    }
  }
}
```

### Still stale more than two minutes after terminal completion

```json
{
  "url": "https://dashboard.bex.co/services/srv-davnajmde41s73cantfg/events",
  "at": "2026-10-02T09:16:58.042Z",
  "visible": "visible",
  "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nRunning\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n2m\nActivity\n1\nRecent deploys and service changes.\nFilter events\n\nDeploy started\n\nIn Progress\nFirst Deploy\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n2m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\nCancel\n\nYou've reached the beginning of this service's history.",
  "polls": []
}
```

### Replaying the original bounds and the rejected end-only extension

The second result is an intentionally invalid diagnostic, not a product bug. It proves the fix must preserve the 720-hour bound.

```json
{
  "at": "2026-10-02T09:17:19.439Z",
  "label": "fixed-end-versus-current-end",
  "fixed": {
    "request": [
      {
        "operationName": "ServiceEvents",
        "variables": {
          "serviceId": "srv-davnajmde41s73cantfg",
          "startTime": "2026-09-02T09:12:07.334Z",
          "endTime": "2026-10-02T09:12:07.334Z",
          "limit": 20
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "serviceEvents": [
            {
              "__typename": "ServiceEvent",
              "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk",
              "details": {
                "__typename": "ServiceEventDetails",
                "actor": "",
                "branchFrom": "",
                "branchTo": "",
                "cancelReason": "",
                "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
                "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
                "commitUrl": "",
                "deployId": "dep-davnajmde41s73cantg0",
                "deployStatus": "",
                "environmentFrom": null,
                "environmentTo": null,
                "failureReason": "",
                "finishedAt": "2026-10-02T09:14:25Z",
                "fromCount": null,
                "fullDeployStatus": "",
                "image": "",
                "instanceId": "",
                "preDeployStatus": "",
                "projectFrom": null,
                "projectTo": null,
                "reasonCode": "",
                "stallReason": "",
                "startedAt": "2026-10-02T09:12:55Z",
                "status": "",
                "toCount": null,
                "trigger": {
                  "__typename": "DeployTrigger",
                  "clearCache": false,
                  "deployHook": false,
                  "deployedByRender": false,
                  "envUpdated": false,
                  "firstBuild": true,
                  "manual": false,
                  "rollback": false
                },
                "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
              },
              "id": "evt-p2986dqmjiapvibau5ok",
              "timestamp": "2026-10-02T09:11:43Z",
              "type": "deploy_started"
            }
          ]
        }
      }
    ]
  },
  "moving": {
    "request": [
      {
        "operationName": "ServiceEvents",
        "variables": {
          "serviceId": "srv-davnajmde41s73cantfg",
          "startTime": "2026-09-02T09:12:07.334Z",
          "endTime": "2026-10-02T09:17:19.157Z",
          "limit": 20
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
      }
    ],
    "status": 200,
    "response": [
      {
        "data": {
          "serviceEvents": null
        },
        "errors": [
          {
            "message": "bad request: query range exceeds 720 hours",
            "locations": [
              {
                "line": 2,
                "column": 3
              }
            ],
            "path": ["serviceEvents"]
          }
        ]
      }
    ]
  }
}
```

### Valid moving-window control

```json
{
  "label": "current-valid-720-hour-window",
  "at": "2026-10-02T09:17:40.958Z",
  "request": [
    {
      "operationName": "ServiceEvents",
      "variables": {
        "serviceId": "srv-davnajmde41s73cantfg",
        "startTime": "2026-09-02T09:17:40.731Z",
        "endTime": "2026-10-02T09:17:40.731Z",
        "limit": 20
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "serviceEvents": [
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-3k4247m3j3bfrru7gurt",
            "timestamp": "2026-10-02T09:17:39Z",
            "type": "idle_timeout_changed"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "succeeded",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "2026-10-02T09:14:25Z",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "2026-10-02T09:12:55Z",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            },
            "id": "evt-s3qgt8g74f3vjairkpsm",
            "timestamp": "2026-10-02T09:14:25Z",
            "type": "deploy_ended"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "succeeded",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-15950bkbualndban9r7m",
            "timestamp": "2026-10-02T09:14:07Z",
            "type": "build_ended"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-03bvi6kp06iejopm5r1n",
            "timestamp": "2026-10-02T09:12:55Z",
            "type": "build_started"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "2026-10-02T09:14:25Z",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "2026-10-02T09:12:55Z",
              "status": "",
              "toCount": null,
              "trigger": {
                "__typename": "DeployTrigger",
                "clearCache": false,
                "deployHook": false,
                "deployedByRender": false,
                "envUpdated": false,
                "firstBuild": true,
                "manual": false,
                "rollback": false
              },
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            },
            "id": "evt-p2986dqmjiapvibau5ok",
            "timestamp": "2026-10-02T09:11:43Z",
            "type": "deploy_started"
          }
        ]
      }
    }
  ]
}
```

### External settings change on the fresh page

```json
{
  "label": "fresh-page-external-change-and-sleep-start",
  "publicResult": {
    "at": "2026-10-02T09:17:39.146Z",
    "status": 200,
    "contentType": "text/plain; charset=utf-8",
    "body": "OK"
  },
  "mutation": {
    "at": "2026-10-02T09:17:39.434Z",
    "request": {
      "query": "mutation SetIdleTimeout($id:String!,$idleTTLSeconds:Int!){setIdleTimeout(id:$id,idleTTLSeconds:$idleTTLSeconds){id idleTTLSeconds phase}}",
      "variables": {
        "id": "srv-davnajmde41s73cantfg",
        "idleTTLSeconds": 60
      }
    },
    "status": 200,
    "response": {
      "data": {
        "setIdleTimeout": {
          "id": "srv-davnajmde41s73cantfg",
          "idleTTLSeconds": 60,
          "phase": "Running"
        }
      }
    }
  },
  "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nRunning\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n5m\nActivity\n4\nRecent deploys and service changes.\nFilter events\n\nDeploy ended\n\nLive\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n2m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nBuild ended\n\nSucceeded\nDeploy dep-davnajmde41s73cantg0\n3m\n\nBuild started\n\nDeploy dep-davnajmde41s73cantg0\n4m\n\nDeploy started\n\nFirst Deploy\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n5m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nYou've reached the beginning of this service's history."
}
```

### Current facts through all four API entrypoints, with unchanged UI

MCP captures preserve the complete SSE response body. The single instrumented ServiceEvents request in snapshot was the explicit diagnostic above.

```json
{
  "label": "cross-surface-current-events-with-stale-ui",
  "snapshot": {
    "at": "2026-10-02T09:19:02.979Z",
    "visible": "visible",
    "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nRunning\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n6m\nActivity\n4\nRecent deploys and service changes.\nFilter events\n\nDeploy ended\n\nLive\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n4m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nBuild ended\n\nSucceeded\nDeploy dep-davnajmde41s73cantg0\n4m\n\nBuild started\n\nDeploy dep-davnajmde41s73cantg0\n5m\n\nDeploy started\n\nFirst Deploy\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n6m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nYou've reached the beginning of this service's history.",
    "observedServiceEventsRequests": [
      {
        "at": "2026-10-02T09:17:40.750Z",
        "request": [
          {
            "operationName": "ServiceEvents",
            "variables": {
              "serviceId": "srv-davnajmde41s73cantfg",
              "startTime": "2026-09-02T09:17:40.731Z",
              "endTime": "2026-10-02T09:17:40.731Z",
              "limit": 20
            },
            "extensions": {
              "clientLibrary": {
                "name": "@apollo/client",
                "version": "4.1.3"
              }
            },
            "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
          }
        ]
      }
    ]
  },
  "probes": [
    {
      "request": {
        "surface": "REST",
        "url": "https://api.bex.co/v1/services/srv-davnajmde41s73cantfg/events?startTime=2026-10-02T09%3A11%3A00Z&endTime=2026-10-02T09%3A19%3A02.986Z&limit=20",
        "method": "GET"
      },
      "status": 200,
      "response": [
        {
          "event": {
            "id": "evt-0rs8rash40sdcj6dk49g",
            "timestamp": "2026-10-02T09:18:37Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "service_woken",
            "details": {}
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU"
        },
        {
          "event": {
            "id": "evt-egff4clqidqns75q7kqd",
            "timestamp": "2026-10-02T09:17:55Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "service_hibernated",
            "details": {}
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw"
        },
        {
          "event": {
            "id": "evt-3k4247m3j3bfrru7gurt",
            "timestamp": "2026-10-02T09:17:39Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "idle_timeout_changed",
            "details": {}
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo"
        },
        {
          "event": {
            "id": "evt-s3qgt8g74f3vjairkpsm",
            "timestamp": "2026-10-02T09:14:25Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "deploy_ended",
            "details": {
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "succeeded",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "startedAt": "2026-10-02T09:12:55Z",
              "finishedAt": "2026-10-02T09:14:25Z",
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            }
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA"
        },
        {
          "event": {
            "id": "evt-15950bkbualndban9r7m",
            "timestamp": "2026-10-02T09:14:07Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "build_ended",
            "details": {
              "deployId": "dep-davnajmde41s73cantg0",
              "status": "succeeded"
            }
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA"
        },
        {
          "event": {
            "id": "evt-03bvi6kp06iejopm5r1n",
            "timestamp": "2026-10-02T09:12:55Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "build_started",
            "details": {
              "deployId": "dep-davnajmde41s73cantg0"
            }
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk"
        },
        {
          "event": {
            "id": "evt-p2986dqmjiapvibau5ok",
            "timestamp": "2026-10-02T09:11:43Z",
            "serviceId": "srv-davnajmde41s73cantfg",
            "type": "deploy_started",
            "details": {
              "deployId": "dep-davnajmde41s73cantg0",
              "trigger": {
                "firstBuild": true,
                "envUpdated": false,
                "manual": false,
                "deployedByRender": false,
                "clearCache": false,
                "rollback": false,
                "deployHook": false
              },
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "startedAt": "2026-10-02T09:12:55Z",
              "finishedAt": "2026-10-02T09:14:25Z",
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            }
          },
          "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk"
        }
      ]
    },
    {
      "request": {
        "surface": "GraphQL",
        "url": "https://api.bex.co/graphql",
        "method": "POST",
        "body": {
          "query": "query($id:String!,$start:String!,$end:String!){serviceEvents(serviceId:$id,startTime:$start,endTime:$end,limit:20){id type timestamp cursor details{deployId deployStatus status}}}",
          "variables": {
            "id": "srv-davnajmde41s73cantfg",
            "start": "2026-10-02T09:11:00Z",
            "end": "2026-10-02T09:19:02.986Z"
          }
        }
      },
      "status": 200,
      "response": {
        "data": {
          "serviceEvents": [
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU",
              "details": {
                "deployId": "",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-0rs8rash40sdcj6dk49g",
              "timestamp": "2026-10-02T09:18:37Z",
              "type": "service_woken"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw",
              "details": {
                "deployId": "",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-egff4clqidqns75q7kqd",
              "timestamp": "2026-10-02T09:17:55Z",
              "type": "service_hibernated"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo",
              "details": {
                "deployId": "",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-3k4247m3j3bfrru7gurt",
              "timestamp": "2026-10-02T09:17:39Z",
              "type": "idle_timeout_changed"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA",
              "details": {
                "deployId": "dep-davnajmde41s73cantg0",
                "deployStatus": "succeeded",
                "status": ""
              },
              "id": "evt-s3qgt8g74f3vjairkpsm",
              "timestamp": "2026-10-02T09:14:25Z",
              "type": "deploy_ended"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA",
              "details": {
                "deployId": "dep-davnajmde41s73cantg0",
                "deployStatus": "",
                "status": "succeeded"
              },
              "id": "evt-15950bkbualndban9r7m",
              "timestamp": "2026-10-02T09:14:07Z",
              "type": "build_ended"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk",
              "details": {
                "deployId": "dep-davnajmde41s73cantg0",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-03bvi6kp06iejopm5r1n",
              "timestamp": "2026-10-02T09:12:55Z",
              "type": "build_started"
            },
            {
              "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk",
              "details": {
                "deployId": "dep-davnajmde41s73cantg0",
                "deployStatus": "",
                "status": ""
              },
              "id": "evt-p2986dqmjiapvibau5ok",
              "timestamp": "2026-10-02T09:11:43Z",
              "type": "deploy_started"
            }
          ]
        }
      }
    },
    {
      "request": {
        "surface": "MCP list_service_events",
        "url": "https://api.bex.co/mcp",
        "method": "POST",
        "body": {
          "jsonrpc": "2.0",
          "id": 70,
          "method": "tools/call",
          "params": {
            "name": "list_service_events",
            "arguments": {
              "serviceId": "srv-davnajmde41s73cantfg",
              "startTime": "2026-10-02T09:11:00Z",
              "endTime": "2026-10-02T09:19:02.986Z",
              "limit": 20
            }
          }
        }
      },
      "status": 200,
      "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":70,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"events\\\":[{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-0rs8rash40sdcj6dk49g\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:18:37Z\\\",\\\"type\\\":\\\"service_woken\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-egff4clqidqns75q7kqd\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:17:55Z\\\",\\\"type\\\":\\\"service_hibernated\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-3k4247m3j3bfrru7gurt\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:17:39Z\\\",\\\"type\\\":\\\"idle_timeout_changed\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA\\\",\\\"event\\\":{\\\"details\\\":{\\\"commitId\\\":\\\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\\\",\\\"commitMessage\\\":\\\"docs(pm): file repeated empty-env Blueprint deployments from QA\\\",\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"deployStatus\\\":\\\"succeeded\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"startedAt\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"triggeredByUser\\\":\\\"c73bb20d-40ff-42df-8597-c9a428697d12\\\"},\\\"id\\\":\\\"evt-s3qgt8g74f3vjairkpsm\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"type\\\":\\\"deploy_ended\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA\\\",\\\"event\\\":{\\\"details\\\":{\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"status\\\":\\\"succeeded\\\"},\\\"id\\\":\\\"evt-15950bkbualndban9r7m\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:14:07Z\\\",\\\"type\\\":\\\"build_ended\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk\\\",\\\"event\\\":{\\\"details\\\":{\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\"},\\\"id\\\":\\\"evt-03bvi6kp06iejopm5r1n\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"type\\\":\\\"build_started\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk\\\",\\\"event\\\":{\\\"details\\\":{\\\"commitId\\\":\\\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\\\",\\\"commitMessage\\\":\\\"docs(pm): file repeated empty-env Blueprint deployments from QA\\\",\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"startedAt\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"trigger\\\":{\\\"clearCache\\\":false,\\\"deployHook\\\":false,\\\"deployedByRender\\\":false,\\\"envUpdated\\\":false,\\\"firstBuild\\\":true,\\\"manual\\\":false,\\\"rollback\\\":false},\\\"triggeredByUser\\\":\\\"c73bb20d-40ff-42df-8597-c9a428697d12\\\"},\\\"id\\\":\\\"evt-p2986dqmjiapvibau5ok\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:11:43Z\\\",\\\"type\\\":\\\"deploy_started\\\"}}]}\"}],\"structuredContent\":{\"events\":[{\"cursor\":\"MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU\",\"event\":{\"details\":{},\"id\":\"evt-0rs8rash40sdcj6dk49g\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:18:37Z\",\"type\":\"service_woken\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw\",\"event\":{\"details\":{},\"id\":\"evt-egff4clqidqns75q7kqd\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:17:55Z\",\"type\":\"service_hibernated\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo\",\"event\":{\"details\":{},\"id\":\"evt-3k4247m3j3bfrru7gurt\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:17:39Z\",\"type\":\"idle_timeout_changed\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA\",\"event\":{\"details\":{\"commitId\":\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\",\"commitMessage\":\"docs(pm): file repeated empty-env Blueprint deployments from QA\",\"deployId\":\"dep-davnajmde41s73cantg0\",\"deployStatus\":\"succeeded\",\"finishedAt\":\"2026-10-02T09:14:25Z\",\"startedAt\":\"2026-10-02T09:12:55Z\",\"triggeredByUser\":\"c73bb20d-40ff-42df-8597-c9a428697d12\"},\"id\":\"evt-s3qgt8g74f3vjairkpsm\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:14:25Z\",\"type\":\"deploy_ended\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA\",\"event\":{\"details\":{\"deployId\":\"dep-davnajmde41s73cantg0\",\"status\":\"succeeded\"},\"id\":\"evt-15950bkbualndban9r7m\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:14:07Z\",\"type\":\"build_ended\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk\",\"event\":{\"details\":{\"deployId\":\"dep-davnajmde41s73cantg0\"},\"id\":\"evt-03bvi6kp06iejopm5r1n\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:12:55Z\",\"type\":\"build_started\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk\",\"event\":{\"details\":{\"commitId\":\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\",\"commitMessage\":\"docs(pm): file repeated empty-env Blueprint deployments from QA\",\"deployId\":\"dep-davnajmde41s73cantg0\",\"finishedAt\":\"2026-10-02T09:14:25Z\",\"startedAt\":\"2026-10-02T09:12:55Z\",\"trigger\":{\"clearCache\":false,\"deployHook\":false,\"deployedByRender\":false,\"envUpdated\":false,\"firstBuild\":true,\"manual\":false,\"rollback\":false},\"triggeredByUser\":\"c73bb20d-40ff-42df-8597-c9a428697d12\"},\"id\":\"evt-p2986dqmjiapvibau5ok\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:11:43Z\",\"type\":\"deploy_started\"}}]}}}\n\n"
    },
    {
      "request": {
        "surface": "MCP list_events",
        "url": "https://api.bex.co/mcp",
        "method": "POST",
        "body": {
          "jsonrpc": "2.0",
          "id": 71,
          "method": "tools/call",
          "params": {
            "name": "list_events",
            "arguments": {
              "serviceId": "srv-davnajmde41s73cantfg",
              "startTime": "2026-10-02T09:11:00Z",
              "endTime": "2026-10-02T09:19:02.986Z",
              "limit": 20
            }
          }
        }
      },
      "status": 200,
      "response": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":71,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"events\\\":[{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-0rs8rash40sdcj6dk49g\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:18:37Z\\\",\\\"type\\\":\\\"service_woken\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-egff4clqidqns75q7kqd\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:17:55Z\\\",\\\"type\\\":\\\"service_hibernated\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo\\\",\\\"event\\\":{\\\"details\\\":{},\\\"id\\\":\\\"evt-3k4247m3j3bfrru7gurt\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:17:39Z\\\",\\\"type\\\":\\\"idle_timeout_changed\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA\\\",\\\"event\\\":{\\\"details\\\":{\\\"commitId\\\":\\\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\\\",\\\"commitMessage\\\":\\\"docs(pm): file repeated empty-env Blueprint deployments from QA\\\",\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"deployStatus\\\":\\\"succeeded\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"startedAt\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"triggeredByUser\\\":\\\"c73bb20d-40ff-42df-8597-c9a428697d12\\\"},\\\"id\\\":\\\"evt-s3qgt8g74f3vjairkpsm\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"type\\\":\\\"deploy_ended\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA\\\",\\\"event\\\":{\\\"details\\\":{\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"status\\\":\\\"succeeded\\\"},\\\"id\\\":\\\"evt-15950bkbualndban9r7m\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:14:07Z\\\",\\\"type\\\":\\\"build_ended\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk\\\",\\\"event\\\":{\\\"details\\\":{\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\"},\\\"id\\\":\\\"evt-03bvi6kp06iejopm5r1n\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"type\\\":\\\"build_started\\\"}},{\\\"cursor\\\":\\\"MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk\\\",\\\"event\\\":{\\\"details\\\":{\\\"commitId\\\":\\\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\\\",\\\"commitMessage\\\":\\\"docs(pm): file repeated empty-env Blueprint deployments from QA\\\",\\\"deployId\\\":\\\"dep-davnajmde41s73cantg0\\\",\\\"finishedAt\\\":\\\"2026-10-02T09:14:25Z\\\",\\\"startedAt\\\":\\\"2026-10-02T09:12:55Z\\\",\\\"trigger\\\":{\\\"clearCache\\\":false,\\\"deployHook\\\":false,\\\"deployedByRender\\\":false,\\\"envUpdated\\\":false,\\\"firstBuild\\\":true,\\\"manual\\\":false,\\\"rollback\\\":false},\\\"triggeredByUser\\\":\\\"c73bb20d-40ff-42df-8597-c9a428697d12\\\"},\\\"id\\\":\\\"evt-p2986dqmjiapvibau5ok\\\",\\\"serviceId\\\":\\\"srv-davnajmde41s73cantfg\\\",\\\"timestamp\\\":\\\"2026-10-02T09:11:43Z\\\",\\\"type\\\":\\\"deploy_started\\\"}}]}\"}],\"structuredContent\":{\"events\":[{\"cursor\":\"MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU\",\"event\":{\"details\":{},\"id\":\"evt-0rs8rash40sdcj6dk49g\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:18:37Z\",\"type\":\"service_woken\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw\",\"event\":{\"details\":{},\"id\":\"evt-egff4clqidqns75q7kqd\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:17:55Z\",\"type\":\"service_hibernated\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo\",\"event\":{\"details\":{},\"id\":\"evt-3k4247m3j3bfrru7gurt\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:17:39Z\",\"type\":\"idle_timeout_changed\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA\",\"event\":{\"details\":{\"commitId\":\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\",\"commitMessage\":\"docs(pm): file repeated empty-env Blueprint deployments from QA\",\"deployId\":\"dep-davnajmde41s73cantg0\",\"deployStatus\":\"succeeded\",\"finishedAt\":\"2026-10-02T09:14:25Z\",\"startedAt\":\"2026-10-02T09:12:55Z\",\"triggeredByUser\":\"c73bb20d-40ff-42df-8597-c9a428697d12\"},\"id\":\"evt-s3qgt8g74f3vjairkpsm\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:14:25Z\",\"type\":\"deploy_ended\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA\",\"event\":{\"details\":{\"deployId\":\"dep-davnajmde41s73cantg0\",\"status\":\"succeeded\"},\"id\":\"evt-15950bkbualndban9r7m\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:14:07Z\",\"type\":\"build_ended\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk\",\"event\":{\"details\":{\"deployId\":\"dep-davnajmde41s73cantg0\"},\"id\":\"evt-03bvi6kp06iejopm5r1n\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:12:55Z\",\"type\":\"build_started\"}},{\"cursor\":\"MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk\",\"event\":{\"details\":{\"commitId\":\"ec1fe500fc9f84fe650b465f5479412bca13a9e2\",\"commitMessage\":\"docs(pm): file repeated empty-env Blueprint deployments from QA\",\"deployId\":\"dep-davnajmde41s73cantg0\",\"finishedAt\":\"2026-10-02T09:14:25Z\",\"startedAt\":\"2026-10-02T09:12:55Z\",\"trigger\":{\"clearCache\":false,\"deployHook\":false,\"deployedByRender\":false,\"envUpdated\":false,\"firstBuild\":true,\"manual\":false,\"rollback\":false},\"triggeredByUser\":\"c73bb20d-40ff-42df-8597-c9a428697d12\"},\"id\":\"evt-p2986dqmjiapvibau5ok\",\"serviceId\":\"srv-davnajmde41s73cantfg\",\"timestamp\":\"2026-10-02T09:11:43Z\",\"type\":\"deploy_started\"}}]}}}\n\n"
    }
  ]
}
```

### Metrics control: advanced bounds fetch the newly available event

```json
{
  "label": "metrics-live-range-result",
  "at": "2026-10-02T09:21:07.581Z",
  "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nSleeping\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n7m\n\nSleeping to save resources — wakes on the next request.\n\nAll events\nLast 30 minutes\nEvent timeline\nservice hibernated\n1m\nservice woken\n2m\nservice hibernated\n3m\nidle timeout changed\n3m\ndeploy ended\n6m\nbuild ended\n7m\nbuild started\n8m\ndeploy started\n9m\nApplication Metrics\nInstances\nsrv-davnajmde41s73cantfg-k2sce\nsrv-davnajmde41s73cantfg-soroj\nRaw\nMin\nMax\nAvg\nPercentage\nTotal\nMemory\nLimit\n0.3%\n4\n0%\n0%\n0%\nsrv-davnajmde41s73cantfg-k2scelruvtajq5q1ja4n\nsrv-davnajmde41s73cantfg-soroj7nabhi8a1o8vrg6\nCPU\nLimit\n0.1%\n4\n0%\n0%\n0%\nsrv-davnajmde41s73cantfg-k2scelruvtajq5q1ja4n\nsrv-davnajmde41s73cantfg-soroj7nabhi8a1o8vrg6\nTotal Instances\nManage scaling\n1\n4\n0\n1\n2\nNetwork Metrics\nAggregated across all instances\nStatus Code\nAll\nHost\nAll\nTotal Requests\n3 requests\nGroup by\nAll requests\n4\n0\n0.5\n1\nResponse Times\nPercentile\np90\nNo data in range\nOutbound Bandwidth\nPartial data\n2\n3\n0 B\n0 B\n0 B\n\n0 B used this month *",
  "requests": [
    {
      "at": "2026-10-02T09:20:51.507Z",
      "operations": [
        {
          "operationName": "ServiceEvents",
          "variables": {
            "serviceId": "srv-davnajmde41s73cantfg",
            "startTime": "2026-10-02T08:49:06.401Z",
            "endTime": "2026-10-02T09:19:06.401Z",
            "limit": 100
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
        }
      ]
    },
    {
      "at": "2026-10-02T09:21:06.504Z",
      "operations": [
        {
          "operationName": "ServiceEvents",
          "variables": {
            "serviceId": "srv-davnajmde41s73cantfg",
            "startTime": "2026-10-02T08:51:06.484Z",
            "endTime": "2026-10-02T09:21:06.484Z",
            "limit": 100
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
        }
      ]
    }
  ],
  "response": {
    "request": {
      "operationName": "ServiceEvents",
      "variables": {
        "serviceId": "srv-davnajmde41s73cantfg",
        "startTime": "2026-10-02T08:51:06.484Z",
        "endTime": "2026-10-02T09:21:06.484Z",
        "limit": 100
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "query ServiceEvents($serviceId: String!, $startTime: String!, $endTime: String!, $cursor: String, $limit: Int) {\n  serviceEvents(\n    serviceId: $serviceId\n    startTime: $startTime\n    endTime: $endTime\n    cursor: $cursor\n    limit: $limit\n  ) {\n    id\n    type\n    timestamp\n    cursor\n    details {\n      deployId\n      deployStatus\n      preDeployStatus\n      fullDeployStatus\n      failureReason\n      cancelReason\n      stallReason\n      status\n      actor\n      triggeredByUser\n      image\n      commitId\n      commitMessage\n      startedAt\n      finishedAt\n      reasonCode\n      instanceId\n      fromCount\n      toCount\n      branchFrom\n      branchTo\n      commitUrl\n      projectFrom\n      projectTo\n      environmentFrom\n      environmentTo\n      trigger {\n        firstBuild\n        envUpdated\n        manual\n        deployedByRender\n        clearCache\n        rollback\n        deployHook\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "serviceEvents": [
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxOToyNS45NzY2MzhafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjc2NTk3NjYzODg4OA",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-kvj94c1jsdr01it40n6c",
            "timestamp": "2026-10-02T09:19:25Z",
            "type": "service_hibernated"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxODozNy40OTk5NzNafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOndva2VuOjE3OTA5MzI3MTc0OTk5NzMxMzU",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-0rs8rash40sdcj6dk49g",
            "timestamp": "2026-10-02T09:18:37Z",
            "type": "service_woken"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNzo1NS45MzU1MzdafGZhY3Q6b2JzZXJ2ZWQ6c3J2LWRhdm5ham1kZTQxczczY2FudGZnOmhpYmVybmF0ZWQ6MTc5MDkzMjY3NTkzNTUzNzM4Mw",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-egff4clqidqns75q7kqd",
            "timestamp": "2026-10-02T09:17:55Z",
            "type": "service_hibernated"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNzozOS4zNTQ3NTVafGF1ZC1kYXZuZGN1ZGU0MXM3M2NhbnRqMDo",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-3k4247m3j3bfrru7gurt",
            "timestamp": "2026-10-02T09:17:39Z",
            "type": "idle_timeout_changed"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNDoyNS45OTU0NTZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDplbmRlZA",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "succeeded",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "2026-10-02T09:14:25Z",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "2026-10-02T09:12:55Z",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            },
            "id": "evt-s3qgt8g74f3vjairkpsm",
            "timestamp": "2026-10-02T09:14:25Z",
            "type": "deploy_ended"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxNDowNy42OTcwMzVafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9lbmRlZA",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "succeeded",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-15950bkbualndban9r7m",
            "timestamp": "2026-10-02T09:14:07Z",
            "type": "build_ended"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxMjo1NS45OTIxNDNafGZhY3Q6ZGVwbG95OmRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpidWlsZF9zdGFydGVk",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "",
              "commitMessage": "",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "",
              "status": "",
              "toCount": null,
              "trigger": null,
              "triggeredByUser": ""
            },
            "id": "evt-03bvi6kp06iejopm5r1n",
            "timestamp": "2026-10-02T09:12:55Z",
            "type": "build_started"
          },
          {
            "__typename": "ServiceEvent",
            "cursor": "MjAyNi0xMC0wMlQwOToxMTo0My4wMDAyODZafGRlcC1kYXZuYWptZGU0MXM3M2NhbnRnMDpzdGFydGVk",
            "details": {
              "__typename": "ServiceEventDetails",
              "actor": "",
              "branchFrom": "",
              "branchTo": "",
              "cancelReason": "",
              "commitId": "ec1fe500fc9f84fe650b465f5479412bca13a9e2",
              "commitMessage": "docs(pm): file repeated empty-env Blueprint deployments from QA",
              "commitUrl": "",
              "deployId": "dep-davnajmde41s73cantg0",
              "deployStatus": "",
              "environmentFrom": null,
              "environmentTo": null,
              "failureReason": "",
              "finishedAt": "2026-10-02T09:14:25Z",
              "fromCount": null,
              "fullDeployStatus": "",
              "image": "",
              "instanceId": "",
              "preDeployStatus": "",
              "projectFrom": null,
              "projectTo": null,
              "reasonCode": "",
              "stallReason": "",
              "startedAt": "2026-10-02T09:12:55Z",
              "status": "",
              "toCount": null,
              "trigger": {
                "__typename": "DeployTrigger",
                "clearCache": false,
                "deployHook": false,
                "deployedByRender": false,
                "envUpdated": false,
                "firstBuild": true,
                "manual": false,
                "rollback": false
              },
              "triggeredByUser": "c73bb20d-40ff-42df-8597-c9a428697d12"
            },
            "id": "evt-p2986dqmjiapvibau5ok",
            "timestamp": "2026-10-02T09:11:43Z",
            "type": "deploy_started"
          }
        ]
      }
    }
  }
}
```

### Sleep/wake and cleanup controls

```json
{
  "sleepWake": [
    {
      "at": "2026-10-02T09:18:15.095Z",
      "label": "hibernated-service-first-wake-request",
      "elapsedMs": 266,
      "url": "https://qa-20261002-web-r7.onbex.co/",
      "status": 503,
      "headers": {
        "content-type": "application/json",
        "retry-after": "5"
      },
      "body": "{\"error\":\"service hibernated\",\"retryAfter\":5}"
    },
    {
      "at": "2026-10-02T09:18:30.702Z",
      "label": "wake-follow-up",
      "elapsedMs": 197,
      "status": 200,
      "headers": {
        "content-type": "text/plain; charset=utf-8"
      },
      "body": "OK",
      "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nRunning\nLatest deploy\nLive\nFree\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n6m\nActivity\n4\nRecent deploys and service changes.\nFilter events\n\nDeploy ended\n\nLive\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n4m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nBuild ended\n\nSucceeded\nDeploy dep-davnajmde41s73cantg0\n4m\n\nBuild started\n\nDeploy dep-davnajmde41s73cantg0\n5m\n\nDeploy started\n\nFirst Deploy\nby puncsky@gmail.com\nDeploy dep-davnajmde41s73cantg0\n6m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nYou've reached the beginning of this service's history."
    }
  ],
  "freshEvents": {
    "at": "2026-10-02T09:22:03.147Z",
    "label": "fresh-page-shows-later-lifecycle",
    "ui": "Projects\nqa-20261002-web-r7\nSearch\n⌘ K\nNew\nP\nWEB SERVICE\nqa-20261002-web-r7\nService\nSleeping\nRuntime\nGo\nConnect\nManual Deploy\nService ID:\nsrv-davnajmde41s73cantfg\ngithub.com · bex-co / bex\nmain\nhttps://qa-20261002-web-r7.onbex.co\nSlug\nqa-20261002-web-r7\n·\nInstances\n1\n·\nRevision\nrev-1\n·\nCreated\n10m\n\nSleeping to save resources — wakes on the next request.\n\nActivity\n8\nRecent deploys and service changes.\nFilter events\n\nService went to sleep\n\n2m\n\nService woke up\n\n3m\n\nService went to sleep\n\n4m\n\nIdle timeout updated\n\n4m\n\nDeploy ended\n\nLive\nby c73bb20d-40ff-42df-8597-c9a428697d12\nDeploy dep-davnajmde41s73cantg0\n7m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nBuild ended\n\nSucceeded\nDeploy dep-davnajmde41s73cantg0\n7m\n\nBuild started\n\nDeploy dep-davnajmde41s73cantg0\n9m\n\nDeploy started\n\nFirst Deploy\nby c73bb20d-40ff-42df-8597-c9a428697d12\nDeploy dep-davnajmde41s73cantg0\n10m\n\ndocs(pm): file repeated empty-env Blueprint deployments from QA\n\nec1fe500\n90s\n\nYou've reached the beginning of this service's history."
  },
  "cleanup": {
    "at": "2026-10-02T09:22:43.050Z",
    "label": "owned-web-ui-cleanup",
    "request": [
      {
        "operationName": "DeleteService",
        "variables": {
          "id": "srv-davnajmde41s73cantfg"
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
    "response": [
      {
        "data": {
          "deleteService": true
        }
      }
    ],
    "check": {
      "status": 404,
      "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
    },
    "url": "https://dashboard.bex.co/"
  }
}
```
