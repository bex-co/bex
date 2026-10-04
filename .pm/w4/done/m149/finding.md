# Public Key Value deletion retains its TLS Secret; the deletion audit misses it

**Severity:** minor (tenant TLS-key/artifact retention; no surviving workload, endpoint or cross-tenant disclosure demonstrated). **Observed:** 2026-10-02 sweeps 9–10. **Workspace:** `tea-d98210cbbpdc73dcrkvg`. **Source:** user-requested continuous `qa-find-bugs` with `muse.env`, filing in w4. No product fixes were made.

## Repro and outcome

1. From a fresh `https://dashboard.bex.co/keyvalue/new` page, create a Free Key Value with Public access enabled. Sweep 10 created `qa-20261002-kv-tls-r10` / `red-davobqc5o9vs73dt7r30`.
2. Wait for Available and for the per-instance Certificate to be Ready. The KeyValue owns the Certificate and its auth/connection Secrets; the issued `<id>-kv-tls` Secret has no owner reference. The free KeyValue has no finalizer.
3. Freshly open its detail page, click Delete Key Value Instance, enter the displayed sudo phrase and confirm. GraphQL returns `deleteKeyValue:true`; the dashboard returns to Overview.
4. Fresh Overview omits the resource. REST returns 404, GraphQL returns null plus not found, and MCP `get_key_value` returns a not-found tool error. The KeyValue, Certificate, CertificateRequest, StatefulSet, pod, Service, PVC and immutable Secrets disappear. **The same TLS Secret UID remains with neither owner reference nor deletion timestamp**, including at 10:27:56Z, over two minutes after the 10:25:48Z delete.
5. Control: independently create a Free **private** Key Value, wait for Available, then delete it through the same fresh-page dialog. Its two owned Secrets and other artifacts disappear, with no TLS Secret present before or after.
6. Run the current `scripts/delete-audit.sh` Key Value checks for the deleted public ID, bound to this namespace/context and without credential-file loading or optional S3 checks. It exits **0: 11 passed, 0 failed**, while the TLS Secret remains.

Sweep 9 independently reproduced the public case on `red-davo3q45o9vs73dt7qt0`; its TLS connection supported authenticated PING/SET/GET before deletion. Its different Secret UID remained ownerless several minutes after delete. This is repeatable lifecycle residue, not an issuance failure or delayed PVC collection.

**Target:** deleting a KeyValue through an existing authorized delete path eventually removes its per-instance TLS Certificate and issued Secret, including Free and backup-disabled instances, while preserving current API acknowledgements and read filtering. Cleanup must stop the Certificate producer, observe it gone, then delete the matching Secret and observe absence before releasing the responsible finalizer. A private instance that never issued TLS still deletes normally. The audit must detect the currently surviving TLS Secret.

## Mechanism and concrete fix

Research/fetched HEAD: `a4993bc94a9f55ab0c6c0f244599bad64d29393d`. Production bex-api and `bex-controller-manager` both use `sha256:7bc6535cde1da49bcdc406781e2b2cfcd9e8a99a8bc8ba212d8418b84ae0951c` (GitOps pin from `e97ca42273a9`). The mechanism below remains on HEAD; no newer deletion fix was found.

- **Creation:** `lego/operator/internal/controller/keyvalue_controller.go:331–354` creates an owned Certificate whose `spec.secretName` is `<kv.Name>-kv-tls` (name derived at :538). `database_controller.go:686–697` stamps the owner reference on that Certificate, not on the issued Secret. The Valkey pod consumes the Secret at `keyvalue_controller.go:691`.
- **Framework:** production and `deploy/helm-artifacts.lock:21` pin cert-manager **v1.20.3**. Its deployed controller arguments omit `--enable-certificate-owner-ref`. The actual pinned [SecretsManager source](https://github.com/cert-manager/cert-manager/blob/v1.20.3/pkg/controller/certificates/issuing/internal/secret.go#L109) adds a Secret owner reference only when that option is enabled (:112–119); its comment at :55–59 states the default. The [flag binding](https://github.com/cert-manager/cert-manager/blob/v1.20.3/cmd/controller/app/options/options.go#L171) is at :171–173. [Upstream documentation](https://cert-manager.io/docs/usage/certificate/#cleaning-up-secrets-when-certificates-are-deleted) confirms Secret retention is the default. This is expected cert-manager behavior; bex must account for it in its own resource lifecycle.
- **Deletion:** `keyvalue_controller.go:758–778` dispatches deletion but stamps `kvFinalizer` only when paid backups are enabled. `keyvalue_backup.go:113–115` excludes Free; :334–377 handles backup CronJobs, Jobs and object-store purge, then removes that finalizer. It never cleans TLS. Even paid instances are reasoned to have the TLS gap; they were not purchased or probed.
- **Correct layer:** `lego/backend/internal/keyvalue/service.go:512–528` authorizes and deletes the CR, relying on cascade. Preserve that service-level boundary; put generated TLS teardown beside its operator-side producer. Use `KeyValueReconciler.secretClient()` (:261–270,288–291) for tenant Secrets. The shared informer intentionally cannot list Secrets cluster-wide.
- **Finalizer design:** register a TLS cleanup obligation before issuing a Certificate, independently of backup eligibility. Preserve it after public access is withdrawn; include existing instances and legacy ownerless Secrets in the compatibility path. A separate TLS finalizer is a direct way to avoid turning every Free deletion into a backup purge. Simply registering today's backup finalizer unconditionally would reach `keyvalue_backup.go:360` and change Free cleanup when the backup store is configured.
- **Ownership and ordering:** derive the exact per-instance Secret name from the same intent as issuance; establish canonical namespace and lifetime ownership before destructive calls. Use Certificate owner UID plus explicit Secret provenance for new artifacts, and a documented conservative check of the exact reserved name and cert-manager certificate/issuer/DNS metadata for legacy artifacts. Reject conflicting/foreign ownership. Retain the finalizer on read/delete errors or pending child deletion; do not broaden the Secret cache/RBAC or sweep every `*-kv-tls` name in a namespace. The pinned [CertificateSecretTemplate type](https://github.com/cert-manager/cert-manager/blob/v1.20.3/pkg/apis/certmanager/v1/types_certificate.go#L719) supports labels/annotations, **not ownerReferences**, so a proposed provenance label can fit there but an owner-reference-only template fix cannot.
- **Existing ordering to reuse carefully:** App cleanup calls `quiesceTLSProducers` before `deleteTLSSecrets` at `app_controller.go:5895–5906`; :6131–6163 explicitly observes Ingress/Certificate absence before Secret deletion. `cleanup_job.go:141–157` supplies the delete/observe primitive. It is not an ownership guard by itself. Do not wait for owner-reference GC while holding the parent finalizer: explicitly quiesce the owned producer first.
- **Audit blind spot:** `scripts/delete-audit.sh:273–336` checks the KV CR, StatefulSet, PVCs, two immutable Secrets (:307–316), backup resources and optionally S3. It omits the Certificate and `-kv-tls` Secret. Add both per-instance TLS artifacts to this same acceptance inventory; a genuine leftover must make it fail. The live safe harness and full output below demonstrate the current false pass.

## Consumers, aliases and adjacent cases

Exhaustive production grep found **one caller** each of `reconcileKeyValueTLS` and `handleKeyValueDeletion`, both in KeyValue Reconcile. `DeleteKeyValue` has **two transport callers**: REST `DELETE /v1/key-value/{id}` (`rest.go:180–188`) and GraphQL `deleteKeyValue` (`graphql.go:212–222`). The dashboard has **two** `useDeleteKeyValue` mounts, detail Danger Zone and row actions; the hook sends that GraphQL mutation at `dashboard/src/features/keyvalue/hooks/use-delete-key-value.ts:29–42`. Only the detail action was clicked here.

The workspace purge is an additional CR-deletion entrypoint: `keyvalue/purger.go:35–43` → `core/purge.go:57–75`; it needs the same operator guarantee. Direct authorized CR deletion also reaches the operator. Neither was run against production. A workspace namespace being deleted can own namespace-local garbage collection; preserve the existing namespace-teardown/RBAC boundary rather than pinning deletion on a permission that has already been withdrawn.

The Key Value MCP feature registers seven tools and **no delete tool** (`keyvalue/mcp.go:105 onward`); do not invent one for parity. Its existing `get_key_value`/list reads are the deletion controls. UI aliases `/r/<id>` redirect to the same Key Value page; they create no independent delete mechanism.

Resource family: **Key Value is the observed failure**. Web/static services use the App TLS teardown; worker/private/cron share the App mechanism where applicable, usually without a public certificate. Postgres uses CNPG-generated TLS rather than this cert-manager path. App TLS and Postgres implementations were read; their full deletion matrices were not re-run here. Sweep 9 Postgres cleanup did remove its own recorded cluster/Secret inventory.

Required verification work beyond the executed DoD: paid + backup-disabled configurations, public→private before delete, never-issued Certificate, legacy live instances, interrupted issuance/deletion, foreign same-name artifacts, authorization/namespace deletion failures, manager restart, API/row/purge entrypoints, and any shared helper changed. Preserve backup retention/purge and immutable auth/connection ownership. Changing public-access retention while the store remains alive is outside this delete-only finding.

Before query settlement, current dashboard/loading behavior stays intact; successful delete remains an acknowledgement, with terminating resources hidden by existing reads. Permission/confirmation refusals remain at the API gates. Missing TLS artifacts are idempotent success; Forbidden or transient read/delete failure is not evidence of absence and must retain cleanup state.

## Dedupe and prior guarantees

Searched open/blocked/done items for Key Value TLS deletion, orphan Secrets, `kv-tls`, Certificate owner refs and finalizers; scanned open milestones across all workstreams, DO_NOT_DO and the latest 40 dashboard/lego commits, plus `git log -S` for the TLS and backup-finalizer functions. No open item or already-landed fix covers this residue. No anti-goal applies.

This is a **residual of w7/done/m12's zero-leftover goal**, not evidence that a previously working Key Value TLS fix regressed: its TLS task `t005` addressed App hosts; its KV task addressed PVCs. Its acceptance script still omits this artifact. Full original DoD walk:

| Original w7/m12 requirement | Sweep 10 evidence/disposition |
| --- | --- |
| Repo-built App with env vars/custom host/completed build; no build Jobs/kpack Images | Not re-run; App TLS ordering read as a code comparison only. |
| No App repository in Zot | Not re-run; unaffected external-artifact path. |
| Static-site prefix removed | Not re-run; unaffected S3 path. |
| No OpenBao env-var/secret-file entries or materialized App env Secret | Not re-run; unrelated secret store. |
| No host TLS Secrets | Two public KeyValue lifetimes retain their TLS Secret. The App-specific implementation is a distinct path, not shown failing. |
| Managed Postgres with backup retention per recorded policy | Not re-run here; sweep 9 checked only its Free cluster/PVC/Secret cleanup, not paid backups. |
| Managed KeyValue deletion leaves no PVC | **Pass** for public and private controls, along with CR/workload/auth/connection Secret removal. TLS remains the missing class. |
| Repeatable t008 zero-leftover audit | **Gap reproduced:** current KV section passes 11 checks despite the TLS residue; optional S3 was explicitly skipped. |

Related **w2/done/m61** full DoD disposition: App delete/recreate artifact inventory (copied Secret/ServiceAccount/Job/Pod/kpack/registry/static/backup/TLS) not re-probed; restart/error retention not re-probed; KeyValue immutable credential consistency not altered/probed here; bounded registry/Prometheus calls not re-probed; tenant-facing delete/read shape passes for the two owned KV fixtures. Its TLS guarantee is App-specific. Preserve its mechanism, do not reopen its unrelated work.

`w1/blocked/m166` covers readiness/proxy routing, `w4/blocked/m116` covers CLI/publication behavior, and `w7/blocked/m151` covers placement references. This finding concerns what survives a completed delete. `w7/done/m84` audited redundant Certificate deletes during reconciliation, not issued-Secret teardown.

[Render's delete reference](https://api-docs.render.com/reference/delete-key-value) specifies deletion by ID and a 204 response. That confirms the public REST shape to preserve; Render's internal TLS-key retention is not observable from that reference and was not tested. bex's existing zero-leftover goal supplies the teardown expectation.

## Durable evidence

No passwords, cookies, connection URLs or Secret data are included. Kubernetes records below deliberately select metadata/type only for Secrets. Empty JSONPath output strings mean an absent metadata field. Screenshots are local supplements; exact requests/responses and inventory evidence below are the handoff.

Inspected screenshot: `.playwright-mcp/qa-r10-public-kv-delete.png` shows the Available public fixture and its destructive-confirmation dialog. It does not itself prove Secret retention.

### API creates, readiness and UI deletes

Mutations used `POST https://api.bex.co/graphql`, JSON content type and the signed-in session. Each block is the complete captured request and response.

**r10-create-public-keyvalue — 2026-10-02T10:22:33.387Z**

```json
{
  "request": [
    {
      "operationName": "CreateKeyValue",
      "variables": {
        "name": "qa-20261002-kv-tls-r10",
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "plan": "free",
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
          "id": "red-davobqc5o9vs73dt7r30",
          "name": "qa-20261002-kv-tls-r10",
          "plan": "free",
          "projectId": null,
          "status": "creating"
        }
      }
    }
  ]
}
```

**r10-create-private-keyvalue-control — 2026-10-02T10:23:00.659Z**

```json
{
  "request": [
    {
      "operationName": "CreateKeyValue",
      "variables": {
        "name": "qa-20261002-kv-private-r10",
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "plan": "free",
        "public": false,
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
          "id": "red-davoc16de41s73canudg",
          "name": "qa-20261002-kv-private-r10",
          "plan": "free",
          "projectId": null,
          "status": "creating"
        }
      }
    }
  ]
}
```

**r10-both-readiness — 2026-10-02T10:24:10.413Z**

```json
{
  "request": {
    "query": "query($a:String!,$b:String!){a:keyValue(id:$a){id name plan status public externalHost} b:keyValue(id:$b){id name plan status public externalHost}}",
    "variables": {
      "a": "red-davobqc5o9vs73dt7r30",
      "b": "red-davoc16de41s73canudg"
    }
  },
  "status": 200,
  "response": {
    "data": {
      "a": {
        "externalHost": "red-davobqc5o9vs73dt7r30.kv.bex.co",
        "id": "red-davobqc5o9vs73dt7r30",
        "name": "qa-20261002-kv-tls-r10",
        "plan": "free",
        "public": true,
        "status": "available"
      },
      "b": {
        "externalHost": "",
        "id": "red-davoc16de41s73canudg",
        "name": "qa-20261002-kv-private-r10",
        "plan": "free",
        "public": false,
        "status": "available"
      }
    }
  }
}
```

**r10-delete-public-keyvalue-ui — 2026-10-02T10:25:48.758Z**

```json
{
  "request": [
    {
      "operationName": "DeleteKeyValue",
      "variables": {
        "id": "red-davobqc5o9vs73dt7r30"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteKeyValue($id: String!, $confirm: String) {\n  deleteKeyValue(id: $id, confirm: $confirm)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteKeyValue": true
      }
    }
  ]
}
```

**r10-delete-private-keyvalue-control-ui — 2026-10-02T10:26:28.942Z**

```json
{
  "request": [
    {
      "operationName": "DeleteKeyValue",
      "variables": {
        "id": "red-davoc16de41s73canudg"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteKeyValue($id: String!, $confirm: String) {\n  deleteKeyValue(id: $id, confirm: $confirm)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteKeyValue": true
      }
    }
  ]
}
```

### All three read surfaces after deletion

At 2026-10-02T10:26:45.957Z, a freshly reloaded Overview contained zero exact-name matches for either fixture. The following complete captures show not-found semantics on all three APIs. MCP requests use Accept: application/json,text/event-stream.

```json
{
  "url": "https://api.bex.co/v1/key-value/red-davobqc5o9vs73dt7r30",
  "method": "GET"
}
```

HTTP 404:

```json
{
  "error": "not found",
  "id": "not_found",
  "message": "not found"
}
```

```json
{
  "url": "https://api.bex.co/graphql",
  "method": "POST",
  "body": {
    "query": "query($id:String!){keyValue(id:$id){id name status}}",
    "variables": {
      "id": "red-davobqc5o9vs73dt7r30"
    }
  }
}
```

HTTP 200:

```json
{
  "data": {
    "keyValue": null
  },
  "errors": [
    {
      "message": "not found",
      "locations": [
        {
          "line": 1,
          "column": 20
        }
      ],
      "path": ["keyValue"]
    }
  ]
}
```

```json
{
  "url": "https://api.bex.co/mcp",
  "method": "POST",
  "body": {
    "jsonrpc": "2.0",
    "id": 101,
    "method": "tools/call",
    "params": {
      "name": "get_key_value",
      "arguments": {
        "keyValueId": "red-davobqc5o9vs73dt7r30"
      }
    }
  }
}
```

HTTP 200:

```text
event: message
data: {"jsonrpc":"2.0","id":101,"result":{"content":[{"type":"text","text":"not found"}],"isError":true}}
```

```json
{
  "url": "https://api.bex.co/v1/key-value/red-davoc16de41s73canudg",
  "method": "GET"
}
```

HTTP 404:

```json
{
  "error": "not found",
  "id": "not_found",
  "message": "not found"
}
```

```json
{
  "url": "https://api.bex.co/graphql",
  "method": "POST",
  "body": {
    "query": "query($id:String!){keyValue(id:$id){id name status}}",
    "variables": {
      "id": "red-davoc16de41s73canudg"
    }
  }
}
```

HTTP 200:

```json
{
  "data": {
    "keyValue": null
  },
  "errors": [
    {
      "message": "not found",
      "locations": [
        {
          "line": 1,
          "column": 20
        }
      ],
      "path": ["keyValue"]
    }
  ]
}
```

```json
{
  "url": "https://api.bex.co/mcp",
  "method": "POST",
  "body": {
    "jsonrpc": "2.0",
    "id": 102,
    "method": "tools/call",
    "params": {
      "name": "get_key_value",
      "arguments": {
        "keyValueId": "red-davoc16de41s73canudg"
      }
    }
  }
}
```

HTTP 200:

```text
event: message
data: {"jsonrpc":"2.0","id":102,"result":{"content":[{"type":"text","text":"not found"}],"isError":true}}
```

### Metadata recorder and full output

The following read-only recorder was run before and twice after deletion. Its Secret calls select only metadata and type, and address only these two owned fixture IDs. Replace the IDs for a fresh reproduction.

```python
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

namespace = 'tea-d98210cbbpdc73dcrkvg'
ids = ['red-davobqc5o9vs73dt7r30', 'red-davoc16de41s73canudg']
base = ['kubectl', '--context', 'hetzner-prod', '--request-timeout=20s', '-n', namespace]
kinds = 'keyvalues.app.bex.co,certificates.cert-manager.io,certificaterequests.cert-manager.io,sts,pods,svc,ingress,pvc'
result = subprocess.run(base + ['get', kinds, '-o', 'json'], check=True, capture_output=True, text=True)
data = json.loads(result.stdout)
items = []
for x in data['items']:
    if not any(i in x['metadata']['name'] for i in ids):
        continue
    items.append({'kind': x['kind'], 'metadata': {k: x['metadata'].get(k) for k in ['name', 'uid', 'creationTimestamp', 'deletionTimestamp', 'ownerReferences', 'finalizers']},
                  'phase': x.get('status', {}).get('phase'),
                  'conditions': x.get('status', {}).get('conditions') if x['kind'] in ['KeyValue', 'Certificate'] else None})
secrets = []
for ident in ids:
    for name in [ident, ident + '-auth', ident + '-kv-tls']:
        # Select metadata and type explicitly: never request Secret data in output.
        fields = ['.metadata.name', '.metadata.uid', '.metadata.creationTimestamp', '.metadata.deletionTimestamp', '.metadata.ownerReferences', '.metadata.finalizers', '.type']
        template = ''.join('{' + field + '}{"\\n"}' for field in fields)
        r = subprocess.run(base + ['get', 'secret', name, '--ignore-not-found', '-o', 'jsonpath=' + template], check=True, capture_output=True, text=True)
        if not r.stdout.strip():
            continue
        values = r.stdout.splitlines()
        secrets.append(dict(zip(['name', 'uid', 'createdAt', 'deletionTimestamp', 'ownerReferences', 'finalizers', 'type'], values)))
out = {'at': datetime.now(timezone.utc).isoformat(), 'namespace': namespace, 'ids': ids, 'items': items, 'secrets': secrets}
if len(sys.argv) > 1:
    Path(sys.argv[1]).write_text(json.dumps(out, indent=2) + '\n')
print(json.dumps(out, indent=2))
```

**qa-r10-before-delete.json**

```json
{
  "at": "2026-10-02T10:24:34.158469+00:00",
  "namespace": "tea-d98210cbbpdc73dcrkvg",
  "ids": ["red-davobqc5o9vs73dt7r30", "red-davoc16de41s73canudg"],
  "items": [
    {
      "kind": "KeyValue",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30",
        "uid": "f1130ca5-9355-4647-8aff-3c2e1a68c86e",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": null,
        "finalizers": null
      },
      "phase": "Ready",
      "conditions": [
        {
          "lastTransitionTime": "2026-10-02T10:22:36Z",
          "message": "Valkey PVC capacity is 10 GB",
          "observedGeneration": 1,
          "reason": "StorageProvisioned",
          "status": "True",
          "type": "StorageReady"
        },
        {
          "lastTransitionTime": "2026-10-02T10:23:21Z",
          "message": "valkey ready",
          "observedGeneration": 1,
          "reason": "Provisioned",
          "status": "True",
          "type": "Ready"
        }
      ]
    },
    {
      "kind": "KeyValue",
      "metadata": {
        "name": "red-davoc16de41s73canudg",
        "uid": "994b7a5e-3661-43d5-b6bc-a5e29e233e8e",
        "creationTimestamp": "2026-10-02T10:23:00Z",
        "deletionTimestamp": null,
        "ownerReferences": null,
        "finalizers": null
      },
      "phase": "Ready",
      "conditions": [
        {
          "lastTransitionTime": "2026-10-02T10:23:04Z",
          "message": "Valkey PVC capacity is 10 GB",
          "observedGeneration": 1,
          "reason": "StorageProvisioned",
          "status": "True",
          "type": "StorageReady"
        },
        {
          "lastTransitionTime": "2026-10-02T10:23:34Z",
          "message": "valkey ready",
          "observedGeneration": 1,
          "reason": "Provisioned",
          "status": "True",
          "type": "Ready"
        }
      ]
    },
    {
      "kind": "Certificate",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30-kv-tls",
        "uid": "ed3ab20e-a3cd-48c8-8f90-f5b38d3f043b",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "app.bex.co/v1alpha1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "KeyValue",
            "name": "red-davobqc5o9vs73dt7r30",
            "uid": "f1130ca5-9355-4647-8aff-3c2e1a68c86e"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": [
        {
          "lastTransitionTime": "2026-10-02T10:22:59Z",
          "message": "Certificate is up to date and has not expired",
          "observedGeneration": 1,
          "reason": "Ready",
          "status": "True",
          "type": "Ready"
        }
      ]
    },
    {
      "kind": "CertificateRequest",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30-kv-tls-1",
        "uid": "fa6ed363-fffa-4d6d-b6a0-45bccd64c002",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "cert-manager.io/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "Certificate",
            "name": "red-davobqc5o9vs73dt7r30-kv-tls",
            "uid": "ed3ab20e-a3cd-48c8-8f90-f5b38d3f043b"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": null
    },
    {
      "kind": "StatefulSet",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30",
        "uid": "4bcfcd1c-be9c-4d52-9ff0-9364900ec38b",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "app.bex.co/v1alpha1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "KeyValue",
            "name": "red-davobqc5o9vs73dt7r30",
            "uid": "f1130ca5-9355-4647-8aff-3c2e1a68c86e"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": null
    },
    {
      "kind": "StatefulSet",
      "metadata": {
        "name": "red-davoc16de41s73canudg",
        "uid": "0a1f453d-f146-4663-9ba9-91591b96e356",
        "creationTimestamp": "2026-10-02T10:23:00Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "app.bex.co/v1alpha1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "KeyValue",
            "name": "red-davoc16de41s73canudg",
            "uid": "994b7a5e-3661-43d5-b6bc-a5e29e233e8e"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": null
    },
    {
      "kind": "Pod",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30-0",
        "uid": "ae81d19e-669a-4db5-934a-7d05ef4f59cd",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "StatefulSet",
            "name": "red-davobqc5o9vs73dt7r30",
            "uid": "4bcfcd1c-be9c-4d52-9ff0-9364900ec38b"
          }
        ],
        "finalizers": null
      },
      "phase": "Running",
      "conditions": null
    },
    {
      "kind": "Pod",
      "metadata": {
        "name": "red-davoc16de41s73canudg-0",
        "uid": "29e653de-75d3-45ae-b916-635bfbe0da05",
        "creationTimestamp": "2026-10-02T10:23:00Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "StatefulSet",
            "name": "red-davoc16de41s73canudg",
            "uid": "0a1f453d-f146-4663-9ba9-91591b96e356"
          }
        ],
        "finalizers": null
      },
      "phase": "Running",
      "conditions": null
    },
    {
      "kind": "Service",
      "metadata": {
        "name": "red-davobqc5o9vs73dt7r30",
        "uid": "cab9ec74-970e-4ca7-96a8-1592a5aa6a40",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "app.bex.co/v1alpha1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "KeyValue",
            "name": "red-davobqc5o9vs73dt7r30",
            "uid": "f1130ca5-9355-4647-8aff-3c2e1a68c86e"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": null
    },
    {
      "kind": "Service",
      "metadata": {
        "name": "red-davoc16de41s73canudg",
        "uid": "35018ddf-6488-46c7-9be7-2b5de2d812e0",
        "creationTimestamp": "2026-10-02T10:23:00Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "app.bex.co/v1alpha1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "KeyValue",
            "name": "red-davoc16de41s73canudg",
            "uid": "994b7a5e-3661-43d5-b6bc-a5e29e233e8e"
          }
        ],
        "finalizers": null
      },
      "phase": null,
      "conditions": null
    },
    {
      "kind": "PersistentVolumeClaim",
      "metadata": {
        "name": "data-red-davobqc5o9vs73dt7r30-0",
        "uid": "86be95a8-6840-4d8f-9685-965c1188be83",
        "creationTimestamp": "2026-10-02T10:22:33Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "StatefulSet",
            "name": "red-davobqc5o9vs73dt7r30",
            "uid": "4bcfcd1c-be9c-4d52-9ff0-9364900ec38b"
          }
        ],
        "finalizers": ["kubernetes.io/pvc-protection"]
      },
      "phase": "Bound",
      "conditions": null
    },
    {
      "kind": "PersistentVolumeClaim",
      "metadata": {
        "name": "data-red-davoc16de41s73canudg-0",
        "uid": "c51b75a0-2a49-4dc7-95ad-7519b724f88e",
        "creationTimestamp": "2026-10-02T10:23:00Z",
        "deletionTimestamp": null,
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "StatefulSet",
            "name": "red-davoc16de41s73canudg",
            "uid": "0a1f453d-f146-4663-9ba9-91591b96e356"
          }
        ],
        "finalizers": ["kubernetes.io/pvc-protection"]
      },
      "phase": "Bound",
      "conditions": null
    }
  ],
  "secrets": [
    {
      "name": "red-davobqc5o9vs73dt7r30",
      "uid": "d9a6b256-e0da-4687-9616-4d37412e35c8",
      "createdAt": "2026-10-02T10:22:33Z",
      "deletionTimestamp": "",
      "ownerReferences": "[{\"apiVersion\":\"app.bex.co/v1alpha1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KeyValue\",\"name\":\"red-davobqc5o9vs73dt7r30\",\"uid\":\"f1130ca5-9355-4647-8aff-3c2e1a68c86e\"}]",
      "finalizers": "",
      "type": "Opaque"
    },
    {
      "name": "red-davobqc5o9vs73dt7r30-auth",
      "uid": "095d8720-feb2-4d96-bb34-1acf63f67119",
      "createdAt": "2026-10-02T10:22:33Z",
      "deletionTimestamp": "",
      "ownerReferences": "[{\"apiVersion\":\"app.bex.co/v1alpha1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KeyValue\",\"name\":\"red-davobqc5o9vs73dt7r30\",\"uid\":\"f1130ca5-9355-4647-8aff-3c2e1a68c86e\"}]",
      "finalizers": "",
      "type": "Opaque"
    },
    {
      "name": "red-davobqc5o9vs73dt7r30-kv-tls",
      "uid": "2dab3f40-8ab4-4f56-b0ac-087a4ecb99fb",
      "createdAt": "2026-10-02T10:22:58Z",
      "deletionTimestamp": "",
      "ownerReferences": "",
      "finalizers": "",
      "type": "kubernetes.io/tls"
    },
    {
      "name": "red-davoc16de41s73canudg",
      "uid": "3943dc51-0e66-4745-8193-1a400bb58616",
      "createdAt": "2026-10-02T10:23:00Z",
      "deletionTimestamp": "",
      "ownerReferences": "[{\"apiVersion\":\"app.bex.co/v1alpha1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KeyValue\",\"name\":\"red-davoc16de41s73canudg\",\"uid\":\"994b7a5e-3661-43d5-b6bc-a5e29e233e8e\"}]",
      "finalizers": "",
      "type": "Opaque"
    },
    {
      "name": "red-davoc16de41s73canudg-auth",
      "uid": "5c769571-36c0-4c9d-b254-dd9b9d4f12f1",
      "createdAt": "2026-10-02T10:23:00Z",
      "deletionTimestamp": "",
      "ownerReferences": "[{\"apiVersion\":\"app.bex.co/v1alpha1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KeyValue\",\"name\":\"red-davoc16de41s73canudg\",\"uid\":\"994b7a5e-3661-43d5-b6bc-a5e29e233e8e\"}]",
      "finalizers": "",
      "type": "Opaque"
    }
  ]
}
```

**qa-r10-after-delete-first.json**

```json
{
  "at": "2026-10-02T10:26:37.167541+00:00",
  "namespace": "tea-d98210cbbpdc73dcrkvg",
  "ids": ["red-davobqc5o9vs73dt7r30", "red-davoc16de41s73canudg"],
  "items": [],
  "secrets": [
    {
      "name": "red-davobqc5o9vs73dt7r30-kv-tls",
      "uid": "2dab3f40-8ab4-4f56-b0ac-087a4ecb99fb",
      "createdAt": "2026-10-02T10:22:58Z",
      "deletionTimestamp": "",
      "ownerReferences": "",
      "finalizers": "",
      "type": "kubernetes.io/tls"
    }
  ]
}
```

**qa-r10-after-delete-settled.json**

```json
{
  "at": "2026-10-02T10:27:56.143571+00:00",
  "namespace": "tea-d98210cbbpdc73dcrkvg",
  "ids": ["red-davobqc5o9vs73dt7r30", "red-davoc16de41s73canudg"],
  "items": [],
  "secrets": [
    {
      "name": "red-davobqc5o9vs73dt7r30-kv-tls",
      "uid": "2dab3f40-8ab4-4f56-b0ac-087a4ecb99fb",
      "createdAt": "2026-10-02T10:22:58Z",
      "deletionTimestamp": "",
      "ownerReferences": "",
      "finalizers": "",
      "type": "kubernetes.io/tls"
    }
  ]
}
```

### Existing audit false pass

The script's KV check body was unchanged. Only the .env loader and its cwd bootstrap were omitted from an in-memory invocation; kubectl was explicitly bound to hetzner-prod with a 20-second request timeout, APPS_NS was the owned workspace, and backup endpoint/destination were unset. No product script was edited. The optional S3 check is not a passed guarantee.

```json
{
  "command": "scripts/delete-audit.sh --kv red-davobqc5o9vs73dt7r30",
  "harness": "Original script passed to bash stdin; .env loader omitted, repo cwd preserved, kubectl bound to hetzner-prod/20s, own APPS_NS and backup endpoint unset. Read-only KV section unchanged.",
  "exit": 0,
  "stdout": "=== delete-audit: checking for zero leftovers after deletion ===\n\n── Key Value: red-davobqc5o9vs73dt7r30 ──\n  PASS  KeyValue red-davobqc5o9vs73dt7r30 is absent from tea-d98210cbbpdc73dcrkvg\n  PASS  no PVCs for KeyValue red-davobqc5o9vs73dt7r30 in tea-d98210cbbpdc73dcrkvg\n  PASS  StatefulSet red-davobqc5o9vs73dt7r30 gone from tea-d98210cbbpdc73dcrkvg\n  PASS  KeyValue Secret red-davobqc5o9vs73dt7r30 is absent\n  PASS  KeyValue Secret red-davobqc5o9vs73dt7r30-auth is absent\n  PASS  no keyvalue-backup cronjobs remain for red-davobqc5o9vs73dt7r30\n  PASS  no keyvalue-backup jobs remain for red-davobqc5o9vs73dt7r30\n  PASS  no keyvalue-backup pods remain for red-davobqc5o9vs73dt7r30\n  PASS  no keyvalue-backup-purge cronjobs remain for red-davobqc5o9vs73dt7r30\n  PASS  no keyvalue-backup-purge jobs remain for red-davobqc5o9vs73dt7r30\n  PASS  no keyvalue-backup-purge pods remain for red-davobqc5o9vs73dt7r30\n  SKIP  KeyValue S3 check (red-davobqc5o9vs73dt7r30) — backup endpoint/destination or aws CLI not configured (prerequisite not configured)\n\n=== result: 11 passed, 0 failed ===\n",
  "stderr": ""
}
```

### Independent sweep 9 case

```json
{
  "observedAt": "2026-10-02T10:16:00Z",
  "kind": "Secret",
  "name": "red-davo3q45o9vs73dt7qt0-kv-tls",
  "uid": "dddb8da2-1b1d-4be6-a009-d458b571253f",
  "namespace": "tea-d98210cbbpdc73dcrkvg",
  "createdAt": "2026-10-02T10:05:57Z",
  "keyValueDeletedAt": "2026-10-02T10:12:36Z",
  "ownerReferences": null,
  "deletionTimestamp": null,
  "type": "kubernetes.io/tls",
  "annotations": {
    "cert-manager.io/certificate-name": "red-davo3q45o9vs73dt7qt0-kv-tls",
    "cert-manager.io/alt-names": "red-davo3q45o9vs73dt7qt0.kv.bex.co",
    "cert-manager.io/issuer-name": "letsencrypt-prod"
  },
  "relatedReferences": [],
  "notes": [
    "Metadata only inspected; certificate/private-key bytes never read or printed.",
    "Deletion accepted by GraphQL; REST404 and Overview absence; workload, PVC, auth secrets, Certificate and CertificateRequest removed.",
    "Second independent fixture reproduction pending; dedupe w7/done/m12 zero-leftover guarantee and shared App TLS teardown before filing.",
    "Exact own fixture Secret removal initiated after no references confirmed; not a platform infrastructure change."
  ]
}
```

**r9-project-only-keyvalue-create-result — 2026-10-02T10:05:29.012Z**

```json
{
  "request": [
    {
      "operationName": "CreateKeyValue",
      "variables": {
        "name": "qa-20261002-kv-project-r9",
        "ownerId": "tea-d98210cbbpdc73dcrkvg",
        "plan": "free",
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
          "id": "red-davo3q45o9vs73dt7qt0",
          "name": "qa-20261002-kv-project-r9",
          "plan": "free",
          "projectId": null,
          "status": "creating"
        }
      }
    }
  ]
}
```

**r9-delete-own-keyvalue-ui — 2026-10-02T10:12:36.595Z**

```json
{
  "request": [
    {
      "operationName": "DeleteKeyValue",
      "variables": {
        "id": "red-davo3q45o9vs73dt7qt0"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteKeyValue($id: String!, $confirm: String) {\n  deleteKeyValue(id: $id, confirm: $confirm)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteKeyValue": true
      }
    }
  ]
}
```

## Cleanup and limits

Both sweep 10 KeyValues were deleted through the dashboard. The private fixture needed no manual cleanup. For the public fixture, after confirming the resource/Certificate/workload were absent, the exact remaining Secret was removed as owned-fixture cleanup after rechecking UID `2dab3f40-8ab4-4f56-b0ac-087a4ecb99fb`. No other resources, cluster settings or cert-manager flags changed. Final inventory:

```json
{
  "at": "2026-10-02T10:29:29.164043+00:00",
  "namespace": "tea-d98210cbbpdc73dcrkvg",
  "ids": ["red-davobqc5o9vs73dt7r30", "red-davoc16de41s73canudg"],
  "items": [],
  "secrets": []
}
```

The sweep 9 orphan was similarly removed in that sweep. This run's Kratos session was revoked (`ok logged-out`), cookie jar/state files removed and browser cookies cleared. The two captured console errors are the expected final REST 404 checks. Creates/deletes succeeded; there was no production product-code change, paid action or bulk orphan sweep.

Live coverage is Free public and Free private deletion through the detail UI, followed by all three API reads and cluster metadata inspection. The untested cases listed above belong in verification tasks, not claims that this sweep passed them. Product tests were not run for this board-only filing.
