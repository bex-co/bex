# Environment-group polling defeats the dashboard's revision check

- **Severity:** major — a successful saved value is silently replaced by another editor's stale draft.
- **Observed:** 2026-10-03 UTC (2026-10-02 local), production dashboard, QA workspace tea-d98210cbbpdc73dcrkvg. Owned unlinked group qa-20261002-concurrent-r49, evg-db09u9atm2ss7389qn2g; only synthetic QA_CONFLICT values.
- **URL:** https://dashboard.bex.co/env-groups/evg-db09u9atm2ss7389qn2g (deleted after testing).
- **Artifacts:** .playwright-mcp/qa-r49-api-captures.json, qa-r49-stale-draft.png, qa-r49-console.txt, qa-r49-network.txt and qa-muse-r49-ledger.json. Screenshot inspected: second-tab-a remains in the editor with no conflict warning after the metadata timestamp updates. It supports draft retention, not persisted-value proof; exact API evidence below supplies that proof. Files are local/gitignored; the record is independently runnable.

## Reproduction and target

Create an unlinked group with QA_CONFLICT=initial. Open its detail in tabs A and B. In A choose Edit and type tab-a. In B choose Edit, type tab-b and Save only. Wait for A's normal EnvGroup poll (30 seconds), without discarding its draft, then choose Save only in A.

First run: B sends revision egr1_AAAAAAAAAAE and succeeds with egr1_AAAAAAAAAAI. A's old draft sends **AI**, not its original AE, and succeeds with AM. Readback is tab-a. Second run after reloading both pages: B sends AM and saves second-tab-b at AQ. A's poll receives AQ, its textbox still holds second-tab-a, then A sends AQ and succeeds at AU. The final read is second-tab-a. No intercepted or fabricated responses, no forced cache writes, no role changes.

Expected: pin the revision when Edit creates the draft. A must send its original revision or refuse locally with an explicit stale-draft notice; to retain the existing server contract, the preferred path sends that original token and surfaces ENV_GROUP_REVISION_CONFLICT. Keep the typed draft on refusal, preserve B's persisted value/revision, and require an explicit discard/fresh edit before adopting new state. Do not silently refresh the token and retry the old patch.

## Root cause and consumer trace

1. dashboard/src/features/env-groups/hooks/use-env-groups.ts:129–146 reads EnvGroup with cache-first plus RESOURCE_POLL_INTERVAL_MS; common/lib/polling.ts:11 is 30,000 ms. mapEnvGroup at :55 includes the new opaque revision.
2. components/env-group-editors.tsx:27 passes group.revision into useEnvGroupEnvironmentPatch. hooks/use-env-groups.ts:418–429 updates revisionRef.current on **every** revision prop change, then chooses that ref at Save. There is no draft boundary in this hook. The exact assignment already exists in original commit 4f3619d1f.
3. services/components/service-environment-editor.tsx:280 and :356–369 keep a separate EnvironmentDraft initialized on Edit. Polling changes props without recreating it. commit at :555–572 calls save(patch, choice); it supplies no base revision. Thus the patch and token describe different user observations. A success clears the draft; a thrown error preserves it and shows the server reason.
4. The backend is a hard control, not the fix target: envgroups/patch.go:135 authorizes the group, :161–180 compares ExpectedRevision against the logical generation **before** reading/patching content maps. The three stale probes below all reject and final value/revision remain unchanged.
5. The wire can express the fix today: GraphQL patchEnvGroupEnvironment expectedRevision is String at graphql.go:292, forwarded at :300–303; generated dashboard definitions.ts:646–651 accepts a string or null. REST contents at rest.go:110–115 and MCP patch_env_group_environment at mcp.go:199–208 call the same PatchEnvironment. No schema or server conflict relaxation is required.

### Actual dependency behavior

The lockfile and installed packages agree: @apollo/client **4.1.3**, React/react-dom **19.2.3**. Apollo core/ObservableQuery.js:844–894 polls using network-only (unless explicitly no-cache), respecting skipPollAttempt. Its react/hooks/useQuery.js:113–149 subscribes through useSyncExternalStore; a changed result updates resultData.current and calls handleStoreChange. notifyOnNetworkStatusChange:false suppresses loading-only churn, not delivery of changed data. React DOM cjs/react-dom-client.development.js:8641–8656 compares effect dependencies and schedules the effect when revision changes. Therefore the effect assigning the ref is reached by an ordinary poll. This is not an Apollo mutation replay or a backend CAS failure.

The screenshot and poll are consistent: ordinary EnvGroup reads include names/revision, **not secret values**; they cannot refresh the draft's value while preserving the user's intended edit. The batch also carried a Services read. The poll evidence below contains the complete EnvGroup request/response entry, explicitly extracted from that batch; unrelated service inventory is omitted.

## Proposed fix and boundaries

Carry a group-only base revision as part of the editor's draft lifecycle, captured synchronously with Edit's names snapshot. Extend the shared editor contract to return that captured revision to the group save wrapper; make it optional for the service wrapper. Keep the latest query revision separately for read mode and future drafts. A poll, refetch, permission refresh or ordinary rerender must never substitute a new base for an active draft, even one currently clean.

EnvironmentDraft (services/lib/environment-draft.ts:65–68) currently has only envVars/secretFiles; add narrowly scoped revision metadata or a typed draft envelope. The save callback type at service-environment-editor.tsx:197–209 can carry the token without adding it to the sparse operations. Preserve it through reauthentication persistence (:335–353), and **update reidentifyDraft** (environment-draft.ts:118–131), which explicitly reconstructs only the two arrays and would otherwise drop new metadata. use-reauth-draft.ts stores generic JSON, so the envelope is expressible. A restored legacy group draft with no base revision must remain visible but fail closed for save until the user explicitly discards/restarts; never attach today's revision to yesterday's edits. No new plaintext reveal or storage is needed.

On conflict, retain the draft/base token and explain refresh/discard; don't drop to null (the server's optional token permits unchecked writes) and don't auto-replay. On accepted save, retire that draft. A fresh edit uses the latest successfully read state. Keep the returned committed revision separate for rollout-only retries; retries must remain empty patches, with current conflict/recovery behavior, never reapply old contents. Preserve saved-but-refresh-failed handling.

Pre-settle: initial loading/missing group revision cannot open a revision-aware group draft; read mode stays masked. Poll-in-flight with existing data retains the active draft, focus and revision. Explicit discard clears both edits and their base before a fresh edit; changing group identity must not carry either across resources. Authentication expiry uses existing redirect/restoration; forbidden, missing group, unavailable storage, invalid token and repair-required/busy errors retain their existing server classes. No existence-disclosing errors or authorization changes.

### Blast radius / aliases

Exhaustive source grep found **one production call** of useEnvGroupEnvironmentPatch, in EnvGroupEditors, and **two production JSX callers** of EnvironmentEditor: EnvGroupEditors and ServiceEnvironmentEditor. reidentifyDraft and createEnvironmentDraft each have one production caller in EnvironmentEditor; remaining hits are declarations/tests. The change is group-revision-aware, not a global CAS rewrite. The service wrapper currently calls useEnvironmentDraftSave without expectedEnvRevision; do not claim service concurrency is fixed or silently add that separate contract.

Group route /env-groups/$groupId is the affected UI. API aliases are the REST contents route, GraphQL mutation and MCP tool named above; legacy item/replace writes remain compatible. Resource-family disposition: group contents can feed web, static, cron, worker and private services, but this run used **zero links** and asserts no live rollout result for those families. Postgres and Key Value have separate configuration surfaces. Shared-editor service behavior, manifest-owned keys, import/export, generation, secret files and rollout retry require regression controls.

## Controls, history and dedupe

The direct stale GraphQL/REST/MCP attempts below used AM while current state was AU. All rejected; final value second-tab-a and revision AU were unchanged. This isolates the dashboard's token selection. Sweep 48 separately passed unlinked import, duplicate validation, Cancel, Save-only preservation of untouched masked values, exact multiline files and clone contents; those are regression controls, not live concurrency proof for files or links.

68 open README records were scanned before filing (workstreams/dev records included), including all 46 open milestones at that point. Subsequent pull added w3/m172 billing reclamation, unrelated. Searches covered open and done expectedRevision, revisionRef, lost-update, stale-draft and concurrent-group terms. No open owner covers this editor token lifecycle. w4/m130 is the **service backend** returning conflict after an applied write; w4/m97 is backend **metadata** CAS. Neither changes this dashboard hook. w4/m159 concerns permission refresh dismissing service confirmations. Anti-goals were re-read: no conflict. git log -40 plus targeted -S history found no pending fix; pull to 0e1d657e2 changes neither implicated dashboard file.

w2/m73's seven DoD guarantees, item by item: (1) generated-secret custody unchanged/unprobed here; (2) name uniqueness unchanged/unprobed; (3) delete convergence passes for this fixture, linked rollout notices unprobed; (4) scope/move/link unchanged and unprobed; (5) staged revision-aware contents **fails across polling**, while sweep48 opaque-value/Cancel controls pass; (6) sweep48 import/copy/clone passed, remaining export/search variants unprobed; (7) no new authenticated Render comparison or whole-repo gate claimed, own cleanup passed. This is an uncovered residual of the original frontend assignment, not a claimed regression in backend CAS.

w1/m153's full original DoD disposition: environment settings dialog, Manage resources dialog, skeleton and source-picker focus are unprobed here and must retain prior behavior; service-editor poll control is unprobed; its subsequently added group-editor draft/focus behavior remains correct in the inspected screenshot. Never restore poll-driven unmounts to prevent this lost update. w4/m97's seven DoD items all concern backend metadata/claims/scope/link/tombstone/interleavings plus API parity; none is re-established by these UI probes and none requires reopening.

## Render and unverified scope

[Render environment-group documentation](https://render.com/docs/configure-environment-variables#environment-groups), checked 2026-10-03 UTC, describes shared values and group edits. It does not establish a concurrent-tab revision protocol. This is Bex's own documented revision-aware correctness guarantee (ADR006:556–558), not a claim that Render exposes the same tokens.

Unverified live: file-only conflicting edits, different-key concurrent edits, all linked-service rollout modes, rollout-only retry, reauthentication restore, legacy stored drafts, permission loss, resource switches and service-local CAS. Cover these in implementation tests; do not turn them into observed DoD claims. No production fixtures remain. API404 and exact projected-Secret inventory empty; logout returned ok logged-out.

## Durable requests and complete responses

Every mutation below is an actual dashboard batch with its complete response; only synthetic fixture values occur. The two rejected controls and MCP result are included exactly. The separate poll entry is extracted as explained above.

```json
{
  "captures": [
    {
      "request": [
        {
          "operationName": "CreateEnvGroup",
          "variables": {
            "name": "qa-20261002-concurrent-r49",
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "initial"
              }
            ],
            "secretFiles": [],
            "serviceIds": [],
            "environmentId": null
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation CreateEnvGroup($name: String!, $ownerId: String, $envVars: [EnvGroupVarInput!], $secretFiles: [EnvGroupSecretFileInput!], $serviceIds: [String!], $environmentId: String) {\n  createEnvGroup(\n    name: $name\n    ownerId: $ownerId\n    envVars: $envVars\n    secretFiles: $secretFiles\n    serviceIds: $serviceIds\n    environmentId: $environmentId\n  ) {\n    id\n    name\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"createEnvGroup\":{\"__typename\":\"EnvGroup\",\"id\":\"evg-db09u9atm2ss7389qn2g\",\"name\":\"qa-20261002-concurrent-r49\"}}}]\n"
    },
    {
      "tab": "B",
      "request": [
        {
          "operationName": "PatchEnvGroupEnvironment",
          "variables": {
            "id": "evg-db09u9atm2ss7389qn2g",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "tab-b"
              }
            ],
            "secretFiles": [],
            "saveMode": "save_only",
            "expectedRevision": "egr1_AAAAAAAAAAE"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation PatchEnvGroupEnvironment($id: String!, $envVars: [EnvGroupVarPatchInput!], $secretFiles: [EnvGroupSecretFilePatchInput!], $saveMode: String!, $expectedRevision: String) {\n  patchEnvGroupEnvironment(\n    id: $id\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n    expectedRevision: $expectedRevision\n  ) {\n    envVarKeys\n    secretFileNames\n    revision\n    affectedServiceIds\n    failedServiceIds\n    rolledOut\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"patchEnvGroupEnvironment\":{\"__typename\":\"EnvGroupEnvironmentPatchResult\",\"affectedServiceIds\":[],\"envVarKeys\":[\"QA_CONFLICT\"],\"failedServiceIds\":[],\"revision\":\"egr1_AAAAAAAAAAI\",\"rolledOut\":false,\"secretFileNames\":[]}}}]\n"
    },
    {
      "tab": "A",
      "request": [
        {
          "operationName": "PatchEnvGroupEnvironment",
          "variables": {
            "id": "evg-db09u9atm2ss7389qn2g",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "tab-a"
              }
            ],
            "secretFiles": [],
            "saveMode": "save_only",
            "expectedRevision": "egr1_AAAAAAAAAAI"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation PatchEnvGroupEnvironment($id: String!, $envVars: [EnvGroupVarPatchInput!], $secretFiles: [EnvGroupSecretFilePatchInput!], $saveMode: String!, $expectedRevision: String) {\n  patchEnvGroupEnvironment(\n    id: $id\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n    expectedRevision: $expectedRevision\n  ) {\n    envVarKeys\n    secretFileNames\n    revision\n    affectedServiceIds\n    failedServiceIds\n    rolledOut\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"patchEnvGroupEnvironment\":{\"__typename\":\"EnvGroupEnvironmentPatchResult\",\"affectedServiceIds\":[],\"envVarKeys\":[\"QA_CONFLICT\"],\"failedServiceIds\":[],\"revision\":\"egr1_AAAAAAAAAAM\",\"rolledOut\":false,\"secretFileNames\":[]}}}]\n"
    },
    {
      "tab": "B2",
      "request": [
        {
          "operationName": "PatchEnvGroupEnvironment",
          "variables": {
            "id": "evg-db09u9atm2ss7389qn2g",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "second-tab-b"
              }
            ],
            "secretFiles": [],
            "saveMode": "save_only",
            "expectedRevision": "egr1_AAAAAAAAAAM"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation PatchEnvGroupEnvironment($id: String!, $envVars: [EnvGroupVarPatchInput!], $secretFiles: [EnvGroupSecretFilePatchInput!], $saveMode: String!, $expectedRevision: String) {\n  patchEnvGroupEnvironment(\n    id: $id\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n    expectedRevision: $expectedRevision\n  ) {\n    envVarKeys\n    secretFileNames\n    revision\n    affectedServiceIds\n    failedServiceIds\n    rolledOut\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"patchEnvGroupEnvironment\":{\"__typename\":\"EnvGroupEnvironmentPatchResult\",\"affectedServiceIds\":[],\"envVarKeys\":[\"QA_CONFLICT\"],\"failedServiceIds\":[],\"revision\":\"egr1_AAAAAAAAAAQ\",\"rolledOut\":false,\"secretFileNames\":[]}}}]\n"
    },
    {
      "tab": "A2",
      "request": [
        {
          "operationName": "PatchEnvGroupEnvironment",
          "variables": {
            "id": "evg-db09u9atm2ss7389qn2g",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "second-tab-a"
              }
            ],
            "secretFiles": [],
            "saveMode": "save_only",
            "expectedRevision": "egr1_AAAAAAAAAAQ"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation PatchEnvGroupEnvironment($id: String!, $envVars: [EnvGroupVarPatchInput!], $secretFiles: [EnvGroupSecretFilePatchInput!], $saveMode: String!, $expectedRevision: String) {\n  patchEnvGroupEnvironment(\n    id: $id\n    envVars: $envVars\n    secretFiles: $secretFiles\n    saveMode: $saveMode\n    expectedRevision: $expectedRevision\n  ) {\n    envVarKeys\n    secretFileNames\n    revision\n    affectedServiceIds\n    failedServiceIds\n    rolledOut\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"patchEnvGroupEnvironment\":{\"__typename\":\"EnvGroupEnvironmentPatchResult\",\"affectedServiceIds\":[],\"envVarKeys\":[\"QA_CONFLICT\"],\"failedServiceIds\":[],\"revision\":\"egr1_AAAAAAAAAAU\",\"rolledOut\":false,\"secretFileNames\":[]}}}]\n"
    },
    {
      "request": [
        {
          "operationName": "DeleteEnvGroup",
          "variables": {
            "id": "evg-db09u9atm2ss7389qn2g"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation DeleteEnvGroup($id: String!) {\n  deleteEnvGroup(id: $id)\n}"
        }
      ],
      "status": 200,
      "body": "[{\"data\":{\"deleteEnvGroup\":true}}]\n"
    }
  ],
  "pollEnvGroupBatchEntry": {
    "request": {
      "operationName": "EnvGroup",
      "variables": {
        "id": "evg-db09u9atm2ss7389qn2g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "query EnvGroup($id: String!) {\n  envGroup(id: $id) {\n    id\n    name\n    ownerId\n    environmentId\n    createdAt\n    updatedAt\n    revision\n    availability\n    serviceLinks\n    envVars {\n      key\n      __typename\n    }\n    secretFiles {\n      name\n      __typename\n    }\n    __typename\n  }\n}"
    },
    "status": 200,
    "response": {
      "data": {
        "envGroup": {
          "__typename": "EnvGroup",
          "availability": "",
          "createdAt": "2026-10-03T06:22:31.735854104Z",
          "envVars": [
            {
              "__typename": "EnvGroupVar",
              "key": "QA_CONFLICT"
            }
          ],
          "environmentId": null,
          "id": "evg-db09u9atm2ss7389qn2g",
          "name": "qa-20261002-concurrent-r49",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "revision": "egr1_AAAAAAAAAAQ",
          "secretFiles": [],
          "serviceLinks": [],
          "updatedAt": "2026-10-03T06:24:16.699898315Z"
        }
      }
    }
  },
  "firstRead": {
    "request": {
      "query": "query { envGroupVar(id:\"evg-db09u9atm2ss7389qn2g\",key:\"QA_CONFLICT\") { key value } }"
    },
    "status": 200,
    "body": "{\"data\":{\"envGroupVar\":{\"key\":\"QA_CONFLICT\",\"value\":\"tab-a\"}}}\n"
  },
  "controls": [
    {
      "path": "/graphql",
      "request": {
        "query": "mutation($id:String!,$envVars:[EnvGroupVarPatchInput!],$saveMode:String!,$expectedRevision:String){patchEnvGroupEnvironment(id:$id,envVars:$envVars,saveMode:$saveMode,expectedRevision:$expectedRevision){revision}}",
        "variables": {
          "id": "evg-db09u9atm2ss7389qn2g",
          "envVars": [
            {
              "key": "QA_CONFLICT",
              "value": "stale-must-not-land"
            }
          ],
          "saveMode": "save_only",
          "expectedRevision": "egr1_AAAAAAAAAAM"
        }
      },
      "status": 200,
      "body": "{\"data\":{\"patchEnvGroupEnvironment\":null},\"errors\":[{\"message\":\"the environment group changed; refresh it before saving again\",\"locations\":[{\"line\":1,\"column\":100}],\"path\":[\"patchEnvGroupEnvironment\"],\"extensions\":{\"code\":\"ENV_GROUP_REVISION_CONFLICT\"}}]}\n"
    },
    {
      "path": "/v1/env-groups/evg-db09u9atm2ss7389qn2g/contents",
      "request": {
        "envVars": [
          {
            "key": "QA_CONFLICT",
            "value": "stale-must-not-land"
          }
        ],
        "saveMode": "save_only",
        "expectedRevision": "egr1_AAAAAAAAAAM"
      },
      "status": 409,
      "body": "{\"code\":\"ENV_GROUP_REVISION_CONFLICT\",\"error\":\"the environment group changed; refresh it before saving again\",\"id\":\"conflict\",\"message\":\"the environment group changed; refresh it before saving again\",\"params\":null}\n"
    },
    {
      "path": "/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 49,
        "method": "tools/call",
        "params": {
          "name": "patch_env_group_environment",
          "arguments": {
            "id": "evg-db09u9atm2ss7389qn2g",
            "envVars": [
              {
                "key": "QA_CONFLICT",
                "value": "stale-must-not-land"
              }
            ],
            "saveMode": "save_only",
            "expectedRevision": "egr1_AAAAAAAAAAM"
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":49,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"ENV_GROUP_REVISION_CONFLICT: the environment group changed; refresh it before saving again\"}],\"isError\":true}}\n\n"
    },
    {
      "request": {
        "query": "query { envGroup(id:\"evg-db09u9atm2ss7389qn2g\") { id revision } envGroupVar(id:\"evg-db09u9atm2ss7389qn2g\",key:\"QA_CONFLICT\") {key value} }"
      },
      "status": 200,
      "body": "{\"data\":{\"envGroup\":{\"id\":\"evg-db09u9atm2ss7389qn2g\",\"revision\":\"egr1_AAAAAAAAAAU\"},\"envGroupVar\":{\"key\":\"QA_CONFLICT\",\"value\":\"second-tab-a\"}}}\n"
    }
  ],
  "cleanup": {
    "status": 404,
    "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
  }
}
```
