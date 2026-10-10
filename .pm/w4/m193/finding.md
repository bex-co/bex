# Default Free Key Value reports 10 GiB logical capacity for its 1 GB plan

**Severity:** major (misleading user-facing capacity). Functional live QA, 2026-10-10 pass a36 at `f397e4f35`. No incorrect charge, enforced quota or data loss was observed. This is a researched filing, not an implemented fix.

## Live reproduction

Create a new Free ($0/month, 128Mi/0.1 CPU) Valkey 8 store from the dashboard with Journal + Snapshot and **no storage override**. The complete creation capture below proves `storageGB` is omitted. Wait until Available. Its catalog entry says **storageGB 1**, but Metrics → Disk shows **Capacity 10 GiB**. Revisited at 07:23:44 UTC after suspend/resume and rename, with the same result. REST, GraphQL and MCP each return **10737418240 bytes**. Expected under the logical-capacity contract: **1073741824 bytes / 1 GiB**.

Owned identity: `qa-20261010-loop-a36-kv`, renamed `qa-20261010-a36-renamed`, `red-db4u78bvjfbs73b08h40`, workspace `tea-d98210cbbpdc73dcrkvg`. The default Free create request is the only storage intent submitted. No paid plan, storage resize or plan upgrade was selected.

Normal TLS client controls passed: PING, benign marker SET/GET and counter INCR (40→41). The two keys and the exact absolute expiry **1791623180212** survived suspend/resume and rename; TTL decreased from 7195 to 7030 to 6937 seconds. Renaming changed the display name immediately and after reload; ID/host stayed stable. These are not data-survival findings.

Read-only control: `beancount-forum-db` (`dpg-d9nqg95cavls73fp8m20`) Details shows **1 GB**, Metrics **1 GiB**, and GraphQL returns **1073741824 bytes**. It was neither edited nor connected to. The difference is explained by the distinct storage producers below, not by different metric serializers.

## Root cause and target

- Free plan floor is 1 GB (`lego/types/tiers/tiers.yaml:118`; ADR021’s plan/persistence contract). `keyvalue_controller.go:129–135` resolves that floor correctly.
- **Two physical-to-logical promotions in the Key Value producer:** `keyvalue_controller.go:409–411` passes `pvcCapacityStorageGB(pvc)` into grow-only logical intent; `:1204–1220` again sets `desiredGB = max(desiredGB, requestedGB, capacityGB)`, expands the request to that value, and calls `finalizeKeyValueStorage`. At `:1272` the status allocation is stamped from that inflated desired/request. `pvcCapacityStorageGB` (:1148) explicitly reads PVC status capacity.
- `storage.go:126–138` computes the maximum of supplied observed sizes. The behavior itself is useful for genuine accepted growth; the Key Value caller mixes physical capacity into it. The Postgres caller `database_controller.go:166–168` supplies **CNPG requested size**, rather than observed PVC capacity, then stamps that logical result at :964. This explains the passing Postgres control.
- Actual pinned **k8s.io/apimachinery v0.35.0** `pkg/api/resource/quantity.go:814–829` was opened: Quantity.Value returns bytes via ScaledValue. `storage.go` rounds those bytes up in GiB. A 10Gi physical observation therefore supplies integer 10, not a serializer or decimal/binary rounding artifact.
- `lego/types/tiers/tiers.go:334` resolves Valkey effective storage as `max(plan floor, spec, allocated)`; `lego/backend/internal/metrics/datastore.go:264,320–324` converts it to logical-capacity bytes. `dashboard/.../datastore-metrics-panel.tsx:81,100,140–154` uses that exact returned value for label and reference line. The API/UI consumer is behaving as implemented; changing only its display would mask the producer issue.
- The status field is `int32`, documented as accepted grow-only request (`lego/types/v1alpha1/keyvalue_types.go:210–225`), distinct from observed `StorageCapacityGB`; the generated CRD preserves both. Correct the logical producer while retaining the physical observation for readiness.

The chain above is source-confirmed. The live API/catalog discrepancy is directly reproduced. **Live CR/PVC values and the cluster’s exact allocation history were not read**, so the provider-floor origin of this fixture’s status is a code-supported inference, not a claimed kubectl observation. The 10 GiB Hetzner floor is previously recorded by m91. ADR021’s old grow-only paragraph also takes max(request/capacity), while the newer m91 and tiers comments explicitly require logical capacity. Reconcile that contract collision in the implementation/record update.

Target: a new default Free store retains logical allocation 1 GB independently of physical floor/capacity. Genuine explicit or plan-driven logical growth remains monotonic. Never shrink an existing PVC or reset an ambiguous historical high-water. Address both Key Value promotion sites and distinguish physical convergence from accepted logical size. If existing inflated allocations need adoption, only adjust bookkeeping with evidence separating floor inflation from real requested growth; ambiguous records require an explicit disposition, not a silent decrease. Keep resize binding/class/quota/failure conditions and asynchronous restart readiness meaningful.

## Shared callers, family and aliases

Exhaustive production greps (tests excluded): `growOnlyIntent` has **2 callers** (Key Value + Postgres); `expandPVCTo` has **2 callers** (Key Value + App disk); `tiers.Valkey.EffectiveStorageGB` has **1 backend caller**, datastore metrics. Postgres EffectiveStorageGB has **3 backend callers** (admission, view and metrics). Preserve the passing Postgres path rather than changing shared arithmetic blindly.

`DatastoreMetrics` has **5 production calls**: REST handler, GraphQL resolver, MCP handler, activeConnections helper and datastoreAppMetric helper. Only the capacity metric’s logical producer changes; connection/memory mappings stay intact. The shared dashboard panel has **3 production mounts** (Postgres detail, Key Value detail, App Disk section).

The App-disk producer independently has the same source concern at `operator/internal/controller/disk.go:236–244` (`max(spec, request, capacity, allocated)`). **Service disk live behavior was not exercised**; it is audit work with its own source trace, not a second reproduced finding. Family: web/private/worker can carry service disks; cron/static disk support is constrained by the existing feature contract; Postgres and Key Value have their own producers. No claim that all resource kinds failed.

REST `/v1/metrics/disk-capacity` supports inferred red-/dpg-/srv- kinds and explicit `kind`; GraphQL uses `datastoreMetrics(query:{kind,resource,name:"DISK_CAPACITY"})`; MCP extension `get_datastore_metrics` accepts resourceId/resource and resolution/resolutionSeconds aliases and `metricTypes:["disk_capacity"]`. Preserve those spellings and existing empty/failure behavior; no new API field or type is required. Dashboard `/r/<id>` redirects to `/keyvalue/<id>`; `/d/<id>` redirects to Postgres. Service aliases resolve to their canonical service route.

## Prior guarantees and dedupe

Open/done PM content and every open milestone title were searched, followed by the last 40 dashboard/lego commits and symbol history. No open logical-capacity fix covers this gap; `.pm/DO_NOT_DO.md` permits it. The original producer arrived in **df2b96a05, 2026-07-19**, `w2/done/m60`, before **a17bd3f48**, m91’s 2026-09-02 metrics fix. This is an **uncovered producer residual of w4/m91**, not evidence of a new rollback or deployment lag.

Complete m91 DoD disposition:

| old guarantee | this pass |
| --- | --- |
| 1 GB Postgres reads 1 GiB | passing UI/GraphQL control below; Details also 1 GB |
| 5 GB Postgres reads 5 GiB | not exercised; verify in t002 without editing it |
| REST/GraphQL/MCP agree on logical capacity | all three agree on **10 GiB** for default Free KV; agreement alone is insufficient |
| Key Value equals its logical StorageGB | fails: catalog/create intent 1 GB versus actual 10 GiB |
| Disk USED unchanged | plotted benign fixture usage; source remains kubelet used bytes; no independent live baseline comparison of used data |
| Ops alert stays physical | carried as a regression boundary; m91’s physical-alert test exists, no live alert exercise here |
| Regression pinned for both datastores | opened `metrics/datastore_test.go:579–609`: logical inputs are seeded, but the test does not let the operator derive allocation from a 10 GiB floor; add a producer-to-consumer case |

m91’s closeout explicitly carried live KV acceptance as unverified. `w4/done/071` later verified a fresh **Postgres** control, not KV. m60’s complete DoD (Database finalization, monotonic Postgres storage, real KV expansion, immutable App kind, readiness from child events and consistent surfaces) must remain intact; this work touches the allocation/resize guarantee, not deletion or App type policy. Other lifecycle controls pass above. `w1/done/m155` concerns Render metric naming, not capacity values; neither it nor the storage-grow milestones duplicate this finding.

[Render’s Key Value docs](https://render.com/docs/key-value) state Free instances have no disk persistence; bex deliberately provides a durable 1 GB Free disk (ADR021). This finding rests on bex’s own logical-storage contract, not a claimed Render Free disk-size match. [Kubernetes expansion docs](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims) describe grow-only expansion; retain physical volume requests rather than shrinking them to repair a label.

## Complete durable probes

Creation, GraphQL HTTP 200; the actual frontend uses a one-operation batch. No storageGB variable is present:

```json
{
  "request": [
    {
      "operationName": "CreateKeyValue",
      "variables": {
        "name": "qa-20261010-loop-a36-kv",
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "plan": "free",
        "version": "8",
        "public": true,
        "maxmemoryPolicy": "allkeys-lru",
        "persistenceMode": "journal-snapshot"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation CreateKeyValue($name: String!, $ownerId: String, $environmentId: String, $plan: String, $version: String, $storageGB: Int, $public: Boolean, $maxmemoryPolicy: String, $persistenceMode: String) {\n  createKeyValue(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    plan: $plan\n    version: $version\n    storageGB: $storageGB\n    public: $public\n    maxmemoryPolicy: $maxmemoryPolicy\n    persistenceMode: $persistenceMode\n  ) {\n    id\n    name\n    plan\n    status\n    projectId\n    environmentId\n    __typename\n  }\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "createKeyValue": {
          "__typename": "KeyValue",
          "environmentId": null,
          "id": "red-db4u78bvjfbs73b08h40",
          "name": "qa-20261010-loop-a36-kv",
          "plan": "free",
          "projectId": null,
          "status": "creating"
        }
      }
    }
  ]
}
```

Catalog and complete responses for all three capacity surfaces:

```json
{
  "request": {
    "operationName": "QaA36FreeCatalog",
    "query": "query QaA36FreeCatalog{keyValueInstanceTypes{id name cpu memory storageGB monthlyUsd}}"
  },
  "status": 200,
  "response": {
    "data": {
      "keyValueInstanceTypes": [
        {
          "cpu": "100m",
          "id": "free",
          "memory": "128Mi",
          "monthlyUsd": "0.00",
          "name": "Free",
          "storageGB": 1
        },
        {
          "cpu": "100m",
          "id": "starter",
          "memory": "256Mi",
          "monthlyUsd": "7.00",
          "name": "Starter",
          "storageGB": 1
        },
        {
          "cpu": "500m",
          "id": "standard",
          "memory": "1Gi",
          "monthlyUsd": "21.00",
          "name": "Standard",
          "storageGB": 5
        }
      ]
    }
  }
}
```

GraphQL HTTP 200:

```json
{
  "at": "2026-10-10T07:22:53.390Z",
  "request": {
    "operationName": "QaA36DiskCapacity",
    "query": "query QaA36DiskCapacity($query:DatastoreMetricsQueryInput!){datastoreMetrics(query:$query){unit labels{field value} values{time value}}}",
    "variables": {
      "query": {
        "kind": "keyvalue",
        "resource": "red-db4u78bvjfbs73b08h40",
        "name": "DISK_CAPACITY"
      }
    }
  },
  "status": 200,
  "response": {
    "data": {
      "datastoreMetrics": [
        {
          "labels": [
            {
              "field": "resource",
              "value": "red-db4u78bvjfbs73b08h40"
            }
          ],
          "unit": "bytes",
          "values": [
            {
              "time": "2026-10-10T07:22:53Z",
              "value": 10737418240
            }
          ]
        }
      ]
    }
  }
}
```

REST HTTP 200:

```json
{
  "url": "https://api.bex.co/v1/metrics/disk-capacity?resource=red-db4u78bvjfbs73b08h40&kind=keyvalue",
  "status": 200,
  "response": [
    {
      "labels": [
        {
          "field": "resource",
          "value": "red-db4u78bvjfbs73b08h40"
        }
      ],
      "unit": "bytes",
      "values": [
        {
          "timestamp": "2026-10-10T07:18:30Z",
          "value": 10737418240
        }
      ]
    }
  ]
}
```

MCP POST `https://api.bex.co/mcp`, Accept `application/json, text/event-stream`, protocol 2025-06-18, HTTP 200. Complete request and full JSON message decoded from SSE:

```json
{
  "request": {
    "jsonrpc": "2.0",
    "id": "a36-disk",
    "method": "tools/call",
    "params": {
      "name": "get_datastore_metrics",
      "arguments": {
        "resourceId": "red-db4u78bvjfbs73b08h40",
        "metricTypes": ["disk_capacity"]
      }
    }
  },
  "status": 200,
  "response": {
    "jsonrpc": "2.0",
    "id": "a36-disk",
    "result": {
      "content": [
        {
          "type": "text",
          "text": "{\"series\":[{\"labels\":{\"metric\":\"disk_capacity\",\"resource\":\"red-db4u78bvjfbs73b08h40\"},\"points\":[{\"timestamp\":\"2026-10-10T07:25:09Z\",\"value\":10737418240}],\"unit\":\"bytes\"}]}"
        }
      ],
      "structuredContent": {
        "series": [
          {
            "labels": {
              "metric": "disk_capacity",
              "resource": "red-db4u78bvjfbs73b08h40"
            },
            "points": [
              {
                "timestamp": "2026-10-10T07:25:09Z",
                "value": 10737418240
              }
            ],
            "unit": "bytes"
          }
        ]
      }
    }
  }
}
```

Postgres control, complete queried response:

```json
{
  "request": {
    "operationName": "QaA36PgCapacityControl",
    "query": "query QaA36PgCapacityControl($query:DatastoreMetricsQueryInput!){datastoreMetrics(query:$query){unit labels{field value} values{time value}}}",
    "variables": {
      "query": {
        "kind": "database",
        "resource": "dpg-d9nqg95cavls73fp8m20",
        "name": "DISK_CAPACITY"
      }
    }
  },
  "status": 200,
  "response": {
    "data": {
      "datastoreMetrics": [
        {
          "labels": [
            {
              "field": "resource",
              "value": "dpg-d9nqg95cavls73fp8m20"
            }
          ],
          "unit": "bytes",
          "values": [
            {
              "time": "2026-10-10T07:24:49Z",
              "value": 1073741824
            }
          ]
        }
      ]
    }
  }
}
```

## Artifacts, anomalies and cleanup

Ignored `.playwright-mcp/qa-kv-a36-ledger.json` retains full snapshots, benign client results, inventory comparisons and cleanup. Path-checked and viewed screenshots: `.playwright-mcp/qa-kv-a36-capacity.png` visibly reads **Capacity 10 GiB**; `qa-kv-a36-pg-control.png` visibly reads **Capacity 1 GiB** and 1 GB current storage. These are supplemental; the probes above survive the handoff.

The initial hand-built GraphQL metric used REST’s `disk-capacity` spelling and the first Postgres probe used REST’s `postgres` kind. Both produced expected validation errors; corrected GraphQL `DISK_CAPACITY` / `database` calls are shown above. A client read attempted during config_restart and a mislabeled browser selector were harness/transient observations, not filed bugs. Delete response capture missed the frontend’s batch shape; cleanup is established by 404/list comparison, not an invented captured mutation body.

The only owned store, `red-db4u78bvjfbs73b08h40`, was deleted through its typed UI confirmation. At 07:29:49 UTC the REST read returned 404 and all six inventory ID sets matched the baseline: services 5, databases 4, Key Values 2, projects 3, env groups 1, Blueprints 1. Other workers’ resources were left alone. This pass’s Kratos logout succeeded; old-session whoami returned 401 at 07:30:07 UTC, browser cookies went from 7 to 0, and the owned jar/state files were removed. Production CR/PVC/TLS inventory was not directly read; API/list removal is the cleanup evidence.

Unverified: raw production storage fields, actual billing/used-byte charges, legacy allocation repair, non-Free growth and plan changes, service disk live capacity, 5 GB Postgres, paid-plan backups and physical PVC/TLS deletion. No customer workload was changed; only the named fixture was created/renamed/suspended/resumed/deleted.
