# Environment isolation saves immediately but its runtime policy waits for an unrelated reconcile

Why: a user can enable an environment boundary and continue making new private connections across it for minutes, or disable it and remain blocked, while the dashboard and APIs report the desired flag correctly.

- **Severity:** major. This is a delayed enforcement boundary, not a cosmetic switch problem. No foreign service, workspace, private endpoint or data was targeted.
- **Source:** infinite `$qa-find-bugs w4`, cycle 15, 2026-10-08 UTC; production `https://dashboard.bex.co`, QA workspace `tea-d98210cbbpdc73dcrkvg`, credentials supplied privately from `muse.env`. Main researched and deduped at `f34b4bed171a84fe1b3e770c1f5be4f76d6aef89`. Production platform image was `ghcr.io/bex-co/bex-operator@sha256:9f03e0fca0e6941d70dba0cf02d53cc4dd4501d674e27c1e64cef7da74d2260e`; no assertion that this digest is HEAD.
- **Fixtures:** project `qa-20261008-c15-net-573a` (`prj-db3muhdajrns73fufks0`), environments A `evm-db3muqt2gmjc73dioj60` and B `evm-db3muulajrns73fufkt0`, Free image web services A `srv-db3n1052gmjc73dioj7g` and B `srv-db3n1g5ajrns73fufkv0`. Both run `docker.io/library/busybox:1.37.0`, port 3000. A serves the synthetic body `qa-c15-server-573a `; B serves `qa-c15-client-573a` and a CGI that starts a fresh `wget -T 3` to A's own ClusterIP hostname on every request. These are new TCP connections, not connections kept open before enabling isolation.
- **Repro:** create both services in A and wait for Live/Ready and both public 200s. B's `/cgi-bin/probe` returns A's marker. On the project page select A → More actions → All settings → enable **Block cross-environment connections** → Save, retaining the default `0.0.0.0/0` and `::/0` entries and unprotected status. Move B to environment B with the resource checkbox and **Move selected resources to environment** control. Reload A's settings: the switch remains checked. Probe B again: new private connections still return A's marker. Do not edit the command, IP rules, plan or release during this observation.
- **First enable:** `SetEnvironmentACL` completed at `10:31:07.655Z`. The App's isolation label was present, its generation stayed 1, and the Deployment template had no isolation label. An authoritative `kubectl get networkpolicy <own-App-name> --ignore-not-found` completed with exit 0 and empty output at `10:33:19Z`. The fresh page and subsequent public CGI probes still crossed the boundary before the manual deploy at `10:35:03Z`.
- **Positive control:** A's manual image deploy `dep-db3n3lur2oss73e1v2dg` advanced generation to 2, created the correctly scoped policy and propagated the label. After its new pod became ready, B's private probe returned `BLOCKED` while both public root URLs continued returning the fixture markers and HTTP 200. The control was evaluated after rollout, not while old and new pods coexisted.
- **Reverse:** UI saved isolation off at `10:36:43.422Z`, preserving the allowlist. A's isolation label disappeared and generation remained 2. Its Ready pod and policy retained isolation; fresh desktop and 390×844 settings showed the unchecked switch while the CGI still returned `BLOCKED`. The policy was authoritatively re-read at `10:37:45Z`; one preceding kubectl TLS handshake timeout was discarded. A second manual deploy `dep-db3n5etfh1ac73drhqv0`, started `10:38:51Z`, removed the policy and restored the server marker.
- **Fresh repeat:** after that deploy was Live, isolation was saved on again at `10:40:10.853Z`. At `10:40:52Z`, A generation 3 carried the isolation label, its template lacked it, the policy read succeeded with no object, and new cross-environment CGI connections returned the marker. Full REST and GraphQL reads agreed on A=true/B=false and each service's correct environment. A fresh settings reload at `10:41:26Z` again showed the checked switch. This is not a stale Apollo flag or wrong environment id.
- **Same-environment control:** B was moved back into A. After A's next manual deploy and B's manual deploy `dep-db3n7hdfh1ac73drhr5g` reached Live, the `10:44:20Z` capture contained two Ready Deployments whose pod templates bore A's isolation label and two correctly owned policies. Private traffic returned A's marker; both public roots returned their markers. This verifies actual enforced same-environment access, independently of the initial unenforced case.
- **Counter-evidence / eventual repair:** a final UI move of B to B completed `10:44:32.222Z`; at `10:45:48Z` private traffic was correctly `BLOCKED`. B's generation and selected release remained 2; its template had cleared the label and its policy was gone. Its `last-active` changed from `10:30:34Z` to `10:45:15Z`, coinciding with the initial fifteen-minute idle timer. A likewise updated its old activity stamp around its timer. Thus this is **not** “never converges” or “always needs a deploy”: timer/child/spec/startup reconciles can repair the missed update. The final move is a passing eventual-convergence observation, not a second filed bug. Timer coincidence must not be mistaken for event admission.

## Root cause and target behavior

`lego/backend/internal/environments/service.go:1100` persists the ACL then `:1117` calls `applyAppEnvironmentLabels`; `:1415–1435` changes only `metadata.labels[core.LabelNetworkIsolation]`. `patchApps` at `:1381–1403` uses an authorized optimistic merge patch. The same-default allowlist is a no-op at `:1477–1485`, so it supplies no spec generation change to mask the missed label event.

The App watch at `lego/operator/internal/controller/app_controller.go:6057` uses `generationOrDeletionPredicate`. Its `Update` at `:143–150` admits generation/deletion/finalizers plus credential rotation and explicit historical scaling; isolation label add/remove/value changes satisfy none. The shared base at `event_predicates.go:34–43` is not a label-change predicate. Controller-runtime **v0.23.3**, pinned in `lego/operator/go.mod:23`, was opened locally: `pkg/builder/controller.go:322–324` installs the `For` predicates into `source.TypedKind`; `pkg/predicate/predicate.go:211–224` compares generations in `TypedGenerationChangedPredicate.Update`. Create/delete/generic default true (`:79–107`), explaining why initial creation/startup can pass while a later update fails. No Kubernetes rule that labels bump App generation is assumed: both directions were read live with unchanged generation.

The consumer is already expressible: `deployment_projection.go:189–190` reads the label into the pod template; `environment_network_policy.go:79–128` creates or removes the owned ingress+egress policy; `app_controller.go:2488` invokes it on the ordinary Kubernetes path. `runningRequeue` at `:2932–2955` takes the Free auto-sleep branch before the 30-second child-health default; its next deadline follows `idleRequeueAfter`, with default TTL fifteen minutes (`:2300`). That branch explains why a private-service or paid-service periodic control would conceal this Free web delay. Traffic checks above prove the correctly installed policies actually block, and that public ingress remains allowed.

**Target:** admit changes of the App's isolation label (including add, remove and A→B value) and converge the owned policy plus current/future workload labels promptly without depending on idle expiry, a manual deploy, restart or unrelated spec edit. Preserve the selected executable, environment/secret snapshot, port, resource plan, replicas, release identity and staged Save-only configuration. Do not mint a user deploy just to apply a network boundary. If an event enters a held/prior-release path, inspect `convergeServingRuntime` / `convergeServingRoute` (`app_controller.go:5887+`): those intentionally leave the pod template alone, so broadening the watch alone is not proof that policy converges there. Current pod labels and rollout overlap must enforce the boundary, not only the next template's labels. Implementation may use a narrowly scoped operational convergence path; do not apply pending release configuration to get a label onto the serving pod.

Prefer an **App-specific** predicate extension. Keep status-only, unrelated annotation/label and saved-configuration notification updates filtered. `saved_configuration_test.go:568` explicitly asserts that a Save-only notification cannot dispatch the runtime controller. Preserve deletion/finalizers, registry-rotation and historical scaling edges and optimistic ownership gates. No dashboard/schema field change is required to repair this cause. Before runtime settles, the saved desired flag can be visible; the fix must initiate ordinary bounded asynchronous enforcement rather than treating a persisted boolean as proof of a installed policy.

## Blast radius, aliases and unverified scope

- Exhaustive production call search found **five** calls to `applyAppEnvironmentLabels`, all in `environments/service.go`: `:773`, `:861`, `:943`, `:946`, `:1117` (project-member clear seam, environment-member clear, SetServices leaving/joining, ACL fan-out). These are audit edges, not five separately observed failures. Unchanged IP layers are the demonstrated trigger; IP changes can bump generation and mask it.
- The **shared base predicate has three production controller consumers**: App (`app_controller.go:6057`), Database (`database_controller.go:1606`) and KeyValue (`keyvalue_controller.go:1332`). The App-specific wrapper has one production registration. Keep the base's Database/KeyValue behavior unchanged unless their own traced requirement demands a separate change; no blanket ResourceVersion/any-metadata predicate.
- `appPodLabels` has **two production call sites**: `deployment_projection.go:401` and `image_network.go:102`. Trace both before changing label projection. The five hosting App families are web, private, worker, cron and static; Postgres and KeyValue have separate controllers. Live traffic here exercised **web only**. Private/worker pickers correctly offered paid plans with Starter selected and no Free; no paid fixture was created. Cron job/static origin/datastore peer behavior, suspended/hibernated/pending/failed/prior-release paths, A→B isolated-id replacement, environment/project teardown and another workspace were not traffic-probed and must remain explicit verification work.
- Relevant entrypoints share the environment core: REST `PATCH /v1/environments/{id}` and `/acl`; GraphQL `updateEnvironment` and `setEnvironmentACL`; MCP `update_environment` (legacy separate set-ACL tool retired). Membership aliases are REST `PUT /v1/environments/{id}/service-links`, GraphQL `setEnvironmentServices`, MCP `update_environment(serviceIds:)`; project member clearing is reached through project service replacement/deletion. Environment inputs/reads use public `evm-` ids while the isolation label uses the canonical stored `env-` id through the existing id conversion. That conversion is correct and unchanged.
- Only UI/GraphQL writes plus REST/GraphQL reads were live-probed; MCP and REST writes were traced, not exercised. Authn/authz, unknown/foreign ids, unavailable stores, optimistic conflicts and timeouts retain existing adapter/core outcomes. This fix adds no existence oracle, broader workspace discovery, paid access, node/cloud-metadata exemption or NetworkPolicy ownership adoption.

## Precedent and dedupe

This restores the network clause of **w6/done/m19**'s completed enforcement guarantee. Entire original DoD walked:

| Original clause | This sweep | Required handoff |
| --- | --- | --- |
| Protected status blocks unguarded delete/suspend/direct override on REST/GraphQL/MCP | Not exercised; both environments stayed unprotected | Re-run existing authorization/confirmation tests; coordinate known explicit-image/rollback guard work in w4/blocked/m176 rather than re-file it |
| Enabled isolation produces policy denying outside traffic | Fresh Free web toggle fails; correctly reconciled cross-environment denial and same-environment/public controls pass | Real-manager event regression for label transitions, then live toggle replay without forcing reconcile |
| Environment IP rules enforced on Postgres and KeyValue | No datastores created in this cycle; unchanged default IP entries are part of the trigger | Re-run existing two-layer allowlist tests for both kinds; carry unverified live enforcement explicitly |
| Tests assert enforcement, not merely storage | Existing `isolation_test.go:193–200` explicitly invokes `r.Reconcile` after label updates (`:245`, `:280`) | Add watch-driven manager coverage with a Free idle deadline well beyond the test; no direct Reconcile/manual deploy allowed as the transition trigger |

Searched open, blocked and done board paths for isolation, network-isolation, predicates, metadata-only/label-only and delayed reconciliation; scanned open milestone indexes across workstreams and DO_NOT_DO. No open owner covers this missed isolation event. w4/done/m28 and m32 own inbound IP rules/lifecycle; w6/done/m46 owns private-service public exposure; w1/done/m153 owns poll/unmount UI and reported a passing private-service isolation walk; w4/done/m145 owns Save-only status notifications whose filter must survive; none cover prompt Free isolation-label admission. Prior hunts w9/done/m89 and m92 are unrelated. Latest forty dashboard/lego commits and targeted `-S labelNetworkIsolation` / `-S generationOrDeletionPredicate` histories retain the rejecting watch at current main, so this is not a landed-fix/deploy-lag filing. The isolation feature shipped in `d3ea37a18` and lifecycle filter changed in `cfcebfc97` later the same day; these dates are history, not a claim that one commit alone accounts for every later path.

Render's [Projects and Environments](https://render.com/docs/projects#blocking-cross-environment-traffic), checked 2026-10-08, documents private boundary blocking with same-environment communication and public URLs retained, and says toggling does not trigger a deploy. It does not promise termination of pre-existing connections. Each CGI request here started a fresh connection, and the authenticated Render UI was not exercised. Restore bex ADR032/ADR043's existing promise without changing the intentional bex protected-confirmation or preserve-and-detach divergences.

## Evidence and cleanup

All local files below were verified on disk. Desktop on/off and mobile off screenshots were visually inspected. They support the saved switch, not packet enforcement. Runtime/API probes below are the durable evidence; no cookie, credential, deploy-hook or secret values are included.

Under `.playwright-mcp/`: `qa-c15-isolation-on-desktop.png`, `qa-c15-isolation-off-desktop.png`, `qa-c15-isolation-off-mobile.png`, `qa-c15-acl-enable.json`, `qa-c15-acl-disable.json`, `qa-c15-acl-enable-repeat.json`, `qa-c15-move-crossenv.json`, `qa-c15-repeat-isolation-read.json`, `qa-c15-repeat-rest-read.json`, `qa-c15-repeat-live-probe.json`, `qa-c15-sameenv-enforced-control.json`, `qa-c15-crossenv-stale-labels.json`, `qa-c15-final-move-read.json`, `qa-c15-workspace-baseline-policies.json`, `qa-c15-cleanup-cluster.json` and `qa-loop-c15-ledger.json`. The last crossenv filename was a hypothesis label; its actual body is **BLOCKED** and it is a passing timer-repair control. Pre-logout console capture had zero warnings/errors. Cumulative network history contains earlier sessions' logout 401s; they are not this journey's failures. Two early Manual Deploy locator races, a dialog-vs-alertdialog mistake, a wrong `/v1/redis` probe (correct `/v1/key-value` returned200), a guessed filename and the kubectl TLS timeout were discarded harness failures.

Cleanup completed: both services deleted through their typed-confirmation UI; both environments and the project returned204 to exact owned deletes, then all five API reads returned404. Fresh Overview had no project; both public hosts returned404. Individual namespace name inventories for sixteen kinds (App, Deployment, ReplicaSet, Pod, Service, Ingress, NetworkPolicy, Job, CronJob, Certificate, CertificateRequest, Secret, ConfigMap, EndpointSlice, PVC, ControllerRevision) all succeeded and contained no owned names/ids. Foreign Postgres/group/KeyValue read controls remained200. Only this session `a31cae58-1e8c-468a-b1d0-867d3146f586` was revoked; browser and its own jar whoami returned401, and own/default jars were removed only after session/byte/hash ownership checks.

## Exact API probes

Each block includes the exact request and complete matching response, with the Apollo batch slot normalized by operation. Authentication headers are intentionally absent. Recreate owned fixtures and substitute their returned ids for a new replay.

### qa-c15-acl-enable.json

```json
{
  "at": "2026-10-08T10:31:07.655Z",
  "httpStatus": 200,
  "request": {
    "operationName": "SetEnvironmentACL",
    "variables": {
      "id": "evm-db3muqt2gmjc73dioj60",
      "protectedStatus": "unprotected",
      "networkIsolationEnabled": true,
      "ipAllowListEntries": [
        {
          "cidrBlock": "0.0.0.0/0",
          "description": "Allow all (default)"
        },
        {
          "cidrBlock": "::/0",
          "description": "Allow all (default)"
        }
      ]
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetEnvironmentACL($id: String!, $protectedStatus: String!, $networkIsolationEnabled: Boolean!, $ipAllowListEntries: [IPAllowListEntryInput!]) {\n  setEnvironmentACL(\n    id: $id\n    protectedStatus: $protectedStatus\n    networkIsolationEnabled: $networkIsolationEnabled\n    ipAllowListEntries: $ipAllowListEntries\n  ) {\n    ...EnvironmentFields\n    __typename\n  }\n}\n\nfragment EnvironmentFields on Environment {\n  id\n  projectId\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  envGroupIds\n  protectedStatus\n  networkIsolationEnabled\n  ipAllowList\n  ipAllowListEntries {\n    cidrBlock\n    description\n    __typename\n  }\n  ipAllowListProxiedDomains\n  __typename\n}"
  },
  "response": {
    "data": {
      "setEnvironmentACL": {
        "__typename": "Environment",
        "createdAt": "2026-10-08T10:24:43Z",
        "databaseIds": [],
        "envGroupIds": [],
        "id": "evm-db3muqt2gmjc73dioj60",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "ipAllowListProxiedDomains": [],
        "keyValueIds": [],
        "name": "qa-20261008-c15-aenv-573a",
        "networkIsolationEnabled": true,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1052gmjc73dioj7g",
          "srv-db3n1g5ajrns73fufkv0"
        ]
      }
    }
  }
}
```

### qa-c15-move-crossenv.json

```json
{
  "at": "2026-10-08T10:32:13.514Z",
  "httpStatus": 200,
  "request": {
    "operationName": "SetEnvironmentServices",
    "variables": {
      "id": "evm-db3muulajrns73fufkt0",
      "serviceIds": [
        "srv-db3n1g5ajrns73fufkv0"
      ]
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetEnvironmentServices($id: String!, $serviceIds: [String!]!) {\n  setEnvironmentServices(id: $id, serviceIds: $serviceIds) {\n    ...EnvironmentFields\n    __typename\n  }\n}\n\nfragment EnvironmentFields on Environment {\n  id\n  projectId\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  envGroupIds\n  protectedStatus\n  networkIsolationEnabled\n  ipAllowList\n  ipAllowListEntries {\n    cidrBlock\n    description\n    __typename\n  }\n  ipAllowListProxiedDomains\n  __typename\n}"
  },
  "response": {
    "data": {
      "setEnvironmentServices": {
        "__typename": "Environment",
        "createdAt": "2026-10-08T10:24:58Z",
        "databaseIds": [],
        "envGroupIds": [],
        "id": "evm-db3muulajrns73fufkt0",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "ipAllowListProxiedDomains": [],
        "keyValueIds": [],
        "name": "qa-20261008-c15-benv-573a",
        "networkIsolationEnabled": false,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1g5ajrns73fufkv0"
        ]
      }
    }
  }
}
```

### qa-c15-acl-disable.json

```json
{
  "at": "2026-10-08T10:36:43.422Z",
  "httpStatus": 200,
  "request": {
    "operationName": "SetEnvironmentACL",
    "variables": {
      "id": "evm-db3muqt2gmjc73dioj60",
      "protectedStatus": "unprotected",
      "networkIsolationEnabled": false,
      "ipAllowListEntries": [
        {
          "cidrBlock": "0.0.0.0/0",
          "description": "Allow all (default)"
        },
        {
          "cidrBlock": "::/0",
          "description": "Allow all (default)"
        }
      ]
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetEnvironmentACL($id: String!, $protectedStatus: String!, $networkIsolationEnabled: Boolean!, $ipAllowListEntries: [IPAllowListEntryInput!]) {\n  setEnvironmentACL(\n    id: $id\n    protectedStatus: $protectedStatus\n    networkIsolationEnabled: $networkIsolationEnabled\n    ipAllowListEntries: $ipAllowListEntries\n  ) {\n    ...EnvironmentFields\n    __typename\n  }\n}\n\nfragment EnvironmentFields on Environment {\n  id\n  projectId\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  envGroupIds\n  protectedStatus\n  networkIsolationEnabled\n  ipAllowList\n  ipAllowListEntries {\n    cidrBlock\n    description\n    __typename\n  }\n  ipAllowListProxiedDomains\n  __typename\n}"
  },
  "response": {
    "data": {
      "setEnvironmentACL": {
        "__typename": "Environment",
        "createdAt": "2026-10-08T10:24:43Z",
        "databaseIds": [],
        "envGroupIds": [],
        "id": "evm-db3muqt2gmjc73dioj60",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "ipAllowListProxiedDomains": [],
        "keyValueIds": [],
        "name": "qa-20261008-c15-aenv-573a",
        "networkIsolationEnabled": false,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1052gmjc73dioj7g"
        ]
      }
    }
  }
}
```

### qa-c15-acl-enable-repeat.json

```json
{
  "at": "2026-10-08T10:40:10.853Z",
  "httpStatus": 200,
  "request": {
    "operationName": "SetEnvironmentACL",
    "variables": {
      "id": "evm-db3muqt2gmjc73dioj60",
      "protectedStatus": "unprotected",
      "networkIsolationEnabled": true,
      "ipAllowListEntries": [
        {
          "cidrBlock": "0.0.0.0/0",
          "description": "Allow all (default)"
        },
        {
          "cidrBlock": "::/0",
          "description": "Allow all (default)"
        }
      ]
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetEnvironmentACL($id: String!, $protectedStatus: String!, $networkIsolationEnabled: Boolean!, $ipAllowListEntries: [IPAllowListEntryInput!]) {\n  setEnvironmentACL(\n    id: $id\n    protectedStatus: $protectedStatus\n    networkIsolationEnabled: $networkIsolationEnabled\n    ipAllowListEntries: $ipAllowListEntries\n  ) {\n    ...EnvironmentFields\n    __typename\n  }\n}\n\nfragment EnvironmentFields on Environment {\n  id\n  projectId\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  envGroupIds\n  protectedStatus\n  networkIsolationEnabled\n  ipAllowList\n  ipAllowListEntries {\n    cidrBlock\n    description\n    __typename\n  }\n  ipAllowListProxiedDomains\n  __typename\n}"
  },
  "response": {
    "data": {
      "setEnvironmentACL": {
        "__typename": "Environment",
        "createdAt": "2026-10-08T10:24:43Z",
        "databaseIds": [],
        "envGroupIds": [],
        "id": "evm-db3muqt2gmjc73dioj60",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "__typename": "IPAllowListEntry",
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "ipAllowListProxiedDomains": [],
        "keyValueIds": [],
        "name": "qa-20261008-c15-aenv-573a",
        "networkIsolationEnabled": true,
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1052gmjc73dioj7g"
        ]
      }
    }
  }
}
```

### qa-c15-repeat-isolation-read.json

```json
{
  "at": "2026-10-08T10:40:51.738Z",
  "request": {
    "operationName": "QaC15IsolationRead",
    "query": "query QaC15IsolationRead($a: String!, $b: String!) { a:environment(id:$a) { id name projectId networkIsolationEnabled protectedStatus serviceIds ipAllowList ipAllowListEntries { cidrBlock description } } b:environment(id:$b) { id name projectId networkIsolationEnabled protectedStatus serviceIds ipAllowList ipAllowListEntries { cidrBlock description } } }",
    "variables": {
      "a": "evm-db3muqt2gmjc73dioj60",
      "b": "evm-db3muulajrns73fufkt0"
    }
  },
  "httpStatus": 200,
  "response": {
    "data": {
      "a": {
        "id": "evm-db3muqt2gmjc73dioj60",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "name": "qa-20261008-c15-aenv-573a",
        "networkIsolationEnabled": true,
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1052gmjc73dioj7g"
        ]
      },
      "b": {
        "id": "evm-db3muulajrns73fufkt0",
        "ipAllowList": [
          "0.0.0.0/0",
          "::/0"
        ],
        "ipAllowListEntries": [
          {
            "cidrBlock": "0.0.0.0/0",
            "description": "Allow all (default)"
          },
          {
            "cidrBlock": "::/0",
            "description": "Allow all (default)"
          }
        ],
        "name": "qa-20261008-c15-benv-573a",
        "networkIsolationEnabled": false,
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1g5ajrns73fufkv0"
        ]
      }
    }
  }
}
```

### qa-c15-repeat-rest-read.json

```json
[
  {
    "at": "2026-10-08T10:41:13.440Z",
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/environments/evm-db3muqt2gmjc73dioj60"
    },
    "httpStatus": 200,
    "response": {
      "id": "evm-db3muqt2gmjc73dioj60",
      "projectId": "prj-db3muhdajrns73fufks0",
      "name": "qa-20261008-c15-aenv-573a",
      "serviceIds": [
        "srv-db3n1052gmjc73dioj7g"
      ],
      "databaseIds": [],
      "keyValueIds": [],
      "databasesIds": [],
      "redisIds": [],
      "envGroupIds": [],
      "protectedStatus": "unprotected",
      "networkIsolationEnabled": true,
      "ipAllowList": [
        {
          "cidrBlock": "0.0.0.0/0",
          "description": "Allow all (default)"
        },
        {
          "cidrBlock": "::/0",
          "description": "Allow all (default)"
        }
      ]
    }
  },
  {
    "at": "2026-10-08T10:41:17.448Z",
    "request": {
      "method": "GET",
      "url": "https://api.bex.co/v1/environments/evm-db3muulajrns73fufkt0"
    },
    "httpStatus": 200,
    "response": {
      "id": "evm-db3muulajrns73fufkt0",
      "projectId": "prj-db3muhdajrns73fufks0",
      "name": "qa-20261008-c15-benv-573a",
      "serviceIds": [
        "srv-db3n1g5ajrns73fufkv0"
      ],
      "databaseIds": [],
      "keyValueIds": [],
      "databasesIds": [],
      "redisIds": [],
      "envGroupIds": [],
      "protectedStatus": "unprotected",
      "networkIsolationEnabled": false,
      "ipAllowList": [
        {
          "cidrBlock": "0.0.0.0/0",
          "description": "Allow all (default)"
        },
        {
          "cidrBlock": "::/0",
          "description": "Allow all (default)"
        }
      ]
    }
  }
]
```

### qa-c15-final-move-read.json

```json
{
  "at": "2026-10-08T10:45:57.096Z",
  "request": {
    "operationName": "QaC15AfterMove",
    "query": "query QaC15AfterMove($a: String!, $b: String!) { a:environment(id:$a) { id name projectId networkIsolationEnabled protectedStatus serviceIds } b:environment(id:$b) { id name projectId networkIsolationEnabled protectedStatus serviceIds } }",
    "variables": {
      "a": "evm-db3muqt2gmjc73dioj60",
      "b": "evm-db3muulajrns73fufkt0"
    }
  },
  "httpStatus": 200,
  "response": {
    "data": {
      "a": {
        "id": "evm-db3muqt2gmjc73dioj60",
        "name": "qa-20261008-c15-aenv-573a",
        "networkIsolationEnabled": true,
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1052gmjc73dioj7g"
        ]
      },
      "b": {
        "id": "evm-db3muulajrns73fufkt0",
        "name": "qa-20261008-c15-benv-573a",
        "networkIsolationEnabled": false,
        "projectId": "prj-db3muhdajrns73fufks0",
        "protectedStatus": "unprotected",
        "serviceIds": [
          "srv-db3n1g5ajrns73fufkv0"
        ]
      }
    }
  }
}
```

## Re-runnable fixture and packet checks

The created B command writes this CGI under `/tmp/qa-site/cgi-bin/probe` and marks it executable; root markers keep normal public health checks independent of the private probe. Substitute the new owned server hostname.

```sh
#!/bin/sh
printf "Content-Type: text/plain\r\n\r\n"
wget -T 3 -q -O - http://tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a.tea-d98210cbbpdc73dcrkvg.svc:3000 2>/dev/null || printf "BLOCKED\n"
```

### qa-c15-repeat-live-probe.json

```json
{
  "at": "2026-10-08T10:40:52.820711+00:00",
  "probes": [
    {
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co/cgi-bin/probe"
      ],
      "exitCode": 0,
      "response": "qa-c15-server-573a \nHTTP 200",
      "stderr": ""
    },
    {
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\\nHTTP %{http_code}",
        "https://qa-20261008-c15-a-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-server-573a \nHTTP 200",
      "stderr": ""
    },
    {
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-client-573a\n\nHTTP 200",
      "stderr": ""
    }
  ]
}
```

### qa-c15-sameenv-enforced-control.json

```json
{
  "at": "2026-10-08T10:44:20.681311+00:00",
  "probes": [
    {
      "at": "2026-10-08T10:44:21.228155+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co/cgi-bin/probe"
      ],
      "exitCode": 0,
      "response": "qa-c15-server-573a \nHTTP 200",
      "stderr": ""
    },
    {
      "at": "2026-10-08T10:44:21.792841+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-a-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-server-573a \nHTTP 200",
      "stderr": ""
    },
    {
      "at": "2026-10-08T10:44:22.330299+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-client-573a\n\nHTTP 200",
      "stderr": ""
    }
  ]
}
```

### qa-c15-crossenv-stale-labels.json

```json
{
  "at": "2026-10-08T10:45:44.705727+00:00",
  "probes": [
    {
      "at": "2026-10-08T10:45:48.247989+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co/cgi-bin/probe"
      ],
      "exitCode": 0,
      "response": "BLOCKED\n\nHTTP 200",
      "stderr": ""
    },
    {
      "at": "2026-10-08T10:45:48.796677+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-a-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-server-573a \nHTTP 200",
      "stderr": ""
    },
    {
      "at": "2026-10-08T10:45:49.336467+00:00",
      "command": [
        "curl",
        "-sS",
        "--max-time",
        "10",
        "-w",
        "\nHTTP %{http_code}",
        "https://qa-20261008-c15-b-573a.onbex.co"
      ],
      "exitCode": 0,
      "response": "qa-c15-client-573a\n\nHTTP 200",
      "stderr": ""
    }
  ]
}
```

## Runtime read projections

These are explicit projections of the captured Kubernetes JSON responses; they are not synthetic writes or proof from the desired API flag. The successful empty policy read, template labels, readiness, unchanged App generations and later idle stamp make the mechanism independently assessable.

```json
[
  {
    "capture": "qa-c15-repeat-live-probe.json",
    "at": "2026-10-08T10:40:52.820711+00:00",
    "projection": "Selected fields from the completed kubectl -o json reads; null policy requires exitCode=0 and empty stdout",
    "resources": [
      {
        "fixture": "A",
        "kind": "app",
        "exitCode": 0,
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 3,
          "networkIsolationLabel": "env-db3muqt2gmjc73dioj60",
          "lastActive": "2026-10-08T10:29:31Z"
        }
      },
      {
        "fixture": "A",
        "kind": "deploy",
        "exitCode": 0,
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 4,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "podTemplateLabels": {
          "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "app.bex.co/app-uid": "7c676032-93aa-4cf2-a250-6b4a509edaf4",
          "app.bex.co/container-policy": "image-v1",
          "app.bex.co/revision": "rev-3",
          "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
          "bex.co/app-id": "srv-db3n1052gmjc73dioj7g"
        },
        "readyReplicas": 1
      },
      {
        "fixture": "A",
        "kind": "networkpolicy",
        "exitCode": 0,
        "response": null
      }
    ]
  },
  {
    "capture": "qa-c15-sameenv-enforced-control.json",
    "at": "2026-10-08T10:44:20.681311+00:00",
    "projection": "Selected fields from the completed kubectl -o json reads; null policy requires exitCode=0 and empty stdout",
    "resources": [
      {
        "fixture": "A",
        "kind": "app",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "app",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 4,
          "networkIsolationLabel": "env-db3muqt2gmjc73dioj60",
          "lastActive": "2026-10-08T10:29:31Z"
        }
      },
      {
        "fixture": "A",
        "kind": "deploy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "deploy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 5,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "podTemplateLabels": {
          "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "app.bex.co/app-uid": "7c676032-93aa-4cf2-a250-6b4a509edaf4",
          "app.bex.co/container-policy": "image-v1",
          "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60",
          "app.bex.co/revision": "rev-4",
          "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
          "bex.co/app-id": "srv-db3n1052gmjc73dioj7g"
        },
        "readyReplicas": 1
      },
      {
        "fixture": "A",
        "kind": "networkpolicy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "networkpolicy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 1,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "spec": {
          "egress": [
            {
              "to": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "ingress": [
            {
              "from": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "podSelector": {
            "matchLabels": {
              "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a"
            }
          },
          "policyTypes": [
            "Ingress",
            "Egress"
          ]
        }
      },
      {
        "fixture": "B",
        "kind": "app",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "app",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "generation": 2,
          "networkIsolationLabel": "env-db3muqt2gmjc73dioj60",
          "lastActive": "2026-10-08T10:30:34Z"
        }
      },
      {
        "fixture": "B",
        "kind": "deploy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "deploy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "generation": 3,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "podTemplateLabels": {
          "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "app.bex.co/app-uid": "c05a43d5-c562-4f20-a96f-f64b9db00489",
          "app.bex.co/container-policy": "image-v1",
          "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60",
          "app.bex.co/revision": "rev-2",
          "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
          "bex.co/app-id": "srv-db3n1g5ajrns73fufkv0"
        },
        "readyReplicas": 1
      },
      {
        "fixture": "B",
        "kind": "networkpolicy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "networkpolicy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "generation": 1,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "spec": {
          "egress": [
            {
              "to": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "ingress": [
            {
              "from": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "podSelector": {
            "matchLabels": {
              "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a"
            }
          },
          "policyTypes": [
            "Ingress",
            "Egress"
          ]
        }
      }
    ]
  },
  {
    "capture": "qa-c15-crossenv-stale-labels.json",
    "at": "2026-10-08T10:45:44.705727+00:00",
    "projection": "Selected fields from the completed kubectl -o json reads; null policy requires exitCode=0 and empty stdout",
    "resources": [
      {
        "fixture": "A",
        "kind": "app",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "app",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 4,
          "networkIsolationLabel": "env-db3muqt2gmjc73dioj60",
          "lastActive": "2026-10-08T10:44:31Z"
        }
      },
      {
        "fixture": "A",
        "kind": "deploy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "deploy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 5,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "podTemplateLabels": {
          "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "app.bex.co/app-uid": "7c676032-93aa-4cf2-a250-6b4a509edaf4",
          "app.bex.co/container-policy": "image-v1",
          "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60",
          "app.bex.co/revision": "rev-4",
          "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
          "bex.co/app-id": "srv-db3n1052gmjc73dioj7g"
        },
        "readyReplicas": 1
      },
      {
        "fixture": "A",
        "kind": "networkpolicy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "networkpolicy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a",
          "generation": 1,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "spec": {
          "egress": [
            {
              "to": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "ingress": [
            {
              "from": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/network-isolation": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                },
                {
                  "podSelector": {
                    "matchLabels": {
                      "app.bex.co/environment-id": "env-db3muqt2gmjc73dioj60"
                    }
                  }
                }
              ]
            }
          ],
          "podSelector": {
            "matchLabels": {
              "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-a-573a"
            }
          },
          "policyTypes": [
            "Ingress",
            "Egress"
          ]
        }
      },
      {
        "fixture": "B",
        "kind": "app",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "app",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "generation": 2,
          "networkIsolationLabel": null,
          "lastActive": "2026-10-08T10:45:15Z"
        }
      },
      {
        "fixture": "B",
        "kind": "deploy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "deploy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "metadata": {
          "name": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "generation": 4,
          "networkIsolationLabel": null,
          "lastActive": null
        },
        "podTemplateLabels": {
          "app.bex.co/app": "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "app.bex.co/app-uid": "c05a43d5-c562-4f20-a96f-f64b9db00489",
          "app.bex.co/container-policy": "image-v1",
          "app.bex.co/revision": "rev-2",
          "app.bex.co/workspace": "tea-d98210cbbpdc73dcrkvg",
          "bex.co/app-id": "srv-db3n1g5ajrns73fufkv0"
        },
        "readyReplicas": 1
      },
      {
        "fixture": "B",
        "kind": "networkpolicy",
        "exitCode": 0,
        "command": [
          "kubectl",
          "--context",
          "hetzner-prod",
          "--request-timeout=15s",
          "get",
          "networkpolicy",
          "tea-d98210cbbpdc73dcrkvg-qa-20261008-c15-b-573a",
          "-n",
          "tea-d98210cbbpdc73dcrkvg",
          "-o",
          "json",
          "--ignore-not-found"
        ],
        "response": null
      }
    ]
  }
]
```
