# A project move refreshes Project but leaves the selected source Environment stale

Why: a completed move/remove should update the visible source membership after its refresh, rather than leave an enabled row and count claiming the old environment.

**Severity:** minor (temporary misleading membership, repaired by the next poll; no observed data loss or runtime outage). Continuous functional qa-find-bugs, 2026-10-11 UTC pass 155, user-selected w4, muse.env through qa-login.sh. Research main `1cc56f66a`. Extend the existing open m178; filing only.

## Reproduction and observed controls

Owned Free image web `qa-20261010-a155-place` (`srv-db5i5iagccos73bibo10`), BusyBox 1.37, port 3000; source project A `prj-db5i52imr7rc73alv3sg`, Stage `evm-db5i55amr7rc73alv3tg`; target project B `prj-db5i5kqgccos73bibo2g`, target environment `evm-db5i5n2mr7rc73alv3vg`. All six final fixture IDs, including the later owned group, are deleted. Recreate equivalent owned names.

1. Create the web from Stage's New Service link. Project and Environment are correctly preselected; creation returns A/Stage. Wait for Live and public HTTP200 `no-group`.
2. Fresh-load A with `?env=<Stage ID>`. Row actions → Move to project → B. At 05:48:21.970Z the single SetProjectServices mutation succeeds. At 05:48:43.155Z GraphQL shows A and Stage empty, service in B with null environment and Running rev-1; REST omits environmentId and the Stage-filtered service list is []. The source page still shows the row, “1 resource”, All (1), Services (1). By 05:49:15.754Z a later poll repairs it.
3. Reassign only the owned service through Stage → Manage resources → Services → Save. Fresh-load A/Stage and repeat with passive response capture. Mutation completes 05:50:40.435Z. A Projects + singular Project refresh returns at 05:50:40.930Z with the source Project's serviceIds empty. No Environments response is captured during the following observation window. Ready row/count remain at 4, 8, 12, 16 seconds and 05:51:00.860Z, when an independent GraphQL read again reports Stage empty and the service B/null-environment/Running/rev-1.
4. Reassign Stage, fresh-load A/Stage, and choose Remove from project. The one mutation returns source Project serviceIds [] at 05:52:17.344Z. At 05:52:29.648Z the service's projectId and environmentId are both null and Stage is empty, while the ready source row/count still show one.
5. Passing control: assign the ungrouped fixture to A through the same ordinary GraphQL project-membership mutation, fresh-load `?env=unassigned`, and Move to B. The source row and All/Services counts become zero at 05:53:37.740Z, 0.791 seconds after the successful write, without reload. Public HTTP200 `no-group` remains correct.

No request interception, injected latency, rate limiting or repeated failed membership write was used. Some cold-page selector waits were too short; those harness timeouts are excluded. This is a selected-environment display gap, not an authoritative placement defect or a recurrence of m178's now-passing Unassigned failure.

Later lifecycle control: assign the service to B/Target, create its own linked group with literal QA_A155_VALUE, wait for the configuration deploy to reach Live and public `qa-a155-linked-value`. Project Settings → Delete Project succeeds without unlinking. Project/Target return404, service and group survive unscoped; group link/value/content revision remain, service remains Running rev-8 and public value remains correct. This is partial m187 acceptance, not its full closeout.

## Producer, consumer and pinned framework

- `dashboard/src/features/projects/hooks/use-move-to-project.ts:99-115` refreshes ProjectsDocument and, on a cross-project move, the source ProjectDocument before router.invalidate. Neither document contains Environment memberships (`projects/api/projects.graphql:1-30`). Move calls it with from.id at :156; Remove calls it without a source ID at :184 because its mutation already returns the source Project.
- `dashboard/src/features/environments/components/environments-panel.tsx:103` separately watches useEnvironments(projectId). At :241-255 it passes the selected Environment to EnvironmentCard independently of the refreshed projectRows. `environment-card.tsx:149-178` builds the service/datastore/group rows and counts from that Environment's ID arrays and the workspace resource dictionaries. A fresh source Project cannot clear those arrays.
- `environments/hooks/use-environments.ts:115-134` maps only EnvironmentsDocument data, with cache-and-network and the 30,000ms poll in `common/lib/polling.ts:11`. Its loading flag is first-load-only; the observed table is ready, not a skeleton. The subsequent list poll repairs membership.
- Installed and locked Apollo Client **4.1.3**, `react/hooks/useQuery.js:31-61`, keeps the watchQuery observable for the same client/document. At :153-178 it reobserves for changed watch options; a Router invalidation does not itself change this projectId/options. Installed router-core **1.171.21**, `src/router.ts:2493-2535`, marks route matches invalid and calls its loader machinery; it does not explicitly refetch an unchanged Apollo Environment watcher. The capture has the fresh Project response predicted by the current fix, followed by the separate stale Environment consumer. No library defect is claimed.
- `common/apollo/cache.ts:6-13` has no list-membership derivation from Project to Environment. The existing typed EnvironmentsDocument already carries serviceIds/databaseIds/keyValueIds/envGroupIds and supports this re-read; no schema extension is needed.

## Bounded target and family audit

After a successful Move or Remove, re-read the **source project's EnvironmentsDocument** and publish its actual member arrays before considering source freshness complete. Retain the existing cross-project singular Project-before-invalidate ordering. Remove's returned source Project is already normalized: preserve that passing control and avoid adding a redundant singular Project read. Keep exactly one membership write and preserve other target members.

Use the existing client/router singleton seam, which survives menu unmount. Settle older matching source Environments reads individually, then issue an explicit source-scoped network-only typed read with `context: {queryDeduplication:false}`; otherwise an in-flight pre-write poll can supply the old list or publish late. `projects/hooks/use-rename-project.ts:39-48` supplies the current settle-results-then-fresh-read pattern. This pass did not induce that overlap; w4/231 documents the adjacent create/list ordering trap. Do not globally change fetch policies, clear the cache, remount the panel or refresh unrelated projects.

Before settlement, retain the previous data during the pending refresh. After the authoritative read settles, source row/count are zero and the target remains assigned, without waiting for the next timer or a reload. Mutation refusal keeps placement and its existing error. Post-commit refresh rejection is a freshness failure, never a failed move or a reason to repeat the write; a retry re-reads only. Preserve existing not-found/forbidden/unauthenticated/timeout handling. These failure classes are verification work, not live failures in this pass.

Production grep found **one** hook consumer, MoveToProjectMenu, and **three** menu mounts: service-row-actions.tsx:252, database-row-actions.tsx:179, key-value-row-actions.tsx:103. The service family represents web/static/cron/worker/private; only web was exercised live. ResourceTable has **three** direct production mounts: workspace index.tsx:287, EnvironmentCard:379, UnassignedResources in EnvironmentsPanel:323. The selected EnvironmentCard and EnvironmentsPanel each have one direct mount; useEnvironments has four calls in three files (panel, project selector, two breadcrumbs). Fix the three-kind move/remove refresh seam; preserve Overview, Unassigned, selector/breadcrumb caches and other members. Env-group move uses a different path and is outside this fix.

User entrypoint here is the single `/project/$projectId` route, with selected-environment or unassigned search state. Membership entrypoints remain GraphQL setProjectServices/setProjectDatabases/setProjectKeyValues, REST PUT `/v1/projects/{id}/service-links`, `/database-links` and `/keyvalue-links`, and MCP `update_project` with present serviceIds/databaseIds/keyValueIds. The old MCP set_project_services/set_project_databases/set_project_keyvalues tools were retired, not retained as aliases (projects/mcp.go:44-56, :102-105; rest.go:217-219). Service detail aliases do not constitute separate Environment table implementations. No API wire/placement change is requested.

## Acceptance and meaningful regression

- Repeat the fresh A/Stage→B sequence twice. Once the post-write source Environment read completes, Stage's ready row and All/Services counts are zero without reload; service GraphQL is B/null environment and REST Stage-filtered list is []. Exactly one membership write.
- Repeat Remove from project from fresh A/Stage: source Environment row/count become zero after refresh, service is ungrouped, no second membership write or redundant singular Project read.
- Preserve the captured fresh A/Unassigned→B control: source row/count drop after its Project refresh; preserve public HTTP200 and the original Live deploy during placement-only operations. Inverse moves and sibling kinds are explicit further verification.
- Use real ApolloClient/InMemoryCache and the actual Environment membership consumer with polling disabled. Return a fresh empty source Project while a warmed Environment still contains the ID; current code must retain the row and fail the regression. Assert the row/count disappear after the source Environment read on the fix.
- Hold a pre-write Environment read until after the mutation; verify final post-write data wins. Cover menu unmount, Remove, mutation refusal versus refresh rejection, unrelated projects/target members, service/Postgres/Key Value branches and the passing Unassigned consumer. Counting refresh calls alone is insufficient.
- Delete owned fixtures by immutable ID, verify six inventory sets and session revocation, and keep the original milestone's cluster-cleanup acceptance outstanding until actually verified.

## Dedupe, goal linkage and limits

Scanned **76 non-done README titles** across workstreams (including board/dev READMEs, not 76 milestones), matching open/blocked/done records, DO_NOT_DO, last40 dashboard/lego commits and targeted hook history after pulling main to 1cc56f66a. No later fix refreshes the source Environment list.

- **m178 is the owner:** retain its shipped Project refresh (`3470efce4`), add t007/t008 and rerun the standing checks. Its original DoD: the A→B Unassigned control passed; inverse Unassigned, reload and Unassigned Remove were not fully replayed here; selected-environment Move/Remove are the added gaps; placement/public rev-1 controls passed before group configuration; logical cleanup/session revocation passed, cluster remnants unverified. The existing integration `move-refreshes-source-project.test.tsx:53-142` renders only source Project IDs and cannot exercise EnvironmentCard membership. This is an uncovered sibling consumer, not evidence that the Project fix regressed.
- **m187:** producer clear and linked-group project deletion passed in the described controls; it previously noted a brief stale source row separately. Other linked/unlinked move variants, direct REST/MCP deletion and remaining DoD clauses were not replayed, so it remains open.
- **231:** overlapping Environment create/list freshness is a separate mutation and remains open. Do not broaden this task into all eight environment mutation hooks.
- **w6/done/036:** retain its one-write membership guarantee. Prior w9/m89/m92 hunt records are precedent, not matching findings; no anti-goal conflicts.

Goal: ADR008 reliable hosting and ADR032 truthful grouping. [Render Projects docs](https://render.com/docs/projects) describe individual moves and listing a project's resources. Authenticated Render timing was not measured. Preserve Bex's documented retain-and-detach deletion divergence from Render's destructive dashboard deletion.

**Unverified:** live sibling resource kinds, Overview refresh, inverse selected-environment move, narrow mobile/zh, old-poll/refresh-error races, move-time MCP placement, cluster remnants and authenticated Render timing. After project deletion, MCP get_service correctly omitted placement, but that is not a move-time MCP probe.

Evidence: viewed `.playwright-mcp/qa-a155-source-environment-stale.png` (77,826 bytes), viewed `qa-a155-environment-remove-stale.png` (77,826 bytes), complete local `qa-placement-refresh-a155-ledger.json`. All six fixture APIs and public service URL return404; six baseline ID sets exact; own logout succeeded, whoami401, cookies0, browser blank. Local screenshots are ignored; complete replayable wire/DOM probes below survive in the board.

## Durable probes

POST `https://api.bex.co/graphql`, application/json, existing browser session credentials included without recording cookies/tokens. Complete mutation and selected refresh request/response bodies and complete independent requested responses are retained below. Passive capture spans the stated observation window only.

```json
{
  "crossProjectRepeat": {
    "mutation": {
      "at": "2026-10-11T05:50:40.435Z",
      "status": 200,
      "request": [
        {
          "operationName": "SetProjectServices",
          "variables": {
            "id": "prj-db5i5kqgccos73bibo2g",
            "serviceIds": ["srv-db5i5iagccos73bibo10"]
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetProjectServices($id: String!, $serviceIds: [String!]!) {\n  setProjectServices(id: $id, serviceIds: $serviceIds) {\n    ...ProjectFields\n    __typename\n  }\n}\n\nfragment ProjectFields on Project {\n  id\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  __typename\n}"
        }
      ],
      "response": [
        {
          "data": {
            "setProjectServices": {
              "__typename": "Project",
              "createdAt": "2026-10-11T05:46:59Z",
              "databaseIds": [],
              "id": "prj-db5i5kqgccos73bibo2g",
              "keyValueIds": [],
              "name": "qa-20261010-a155-b",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": ["srv-db5i5iagccos73bibo10"]
            }
          }
        }
      ]
    },
    "projectRefresh": {
      "at": "2026-10-11T05:50:40.930Z",
      "timing": {
        "startTime": 1791697840450.472,
        "domainLookupStart": -1,
        "domainLookupEnd": -1,
        "connectStart": -1,
        "secureConnectionStart": -1,
        "connectEnd": -1,
        "requestStart": 0.121,
        "responseStart": 477.636,
        "responseEnd": 478.243
      },
      "status": 200,
      "request": [
        {
          "operationName": "Projects",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query Projects($ownerId: String!) {\n  projects(ownerId: $ownerId) {\n    ...ProjectFields\n    __typename\n  }\n}\n\nfragment ProjectFields on Project {\n  id\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  __typename\n}"
        },
        {
          "operationName": "Project",
          "variables": {
            "id": "prj-db5i52imr7rc73alv3sg"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query Project($id: String!) {\n  project(id: $id) {\n    id\n    name\n    ownerId\n    serviceIds\n    databaseIds\n    keyValueIds\n    __typename\n  }\n}"
        }
      ],
      "response": [
        {
          "data": {
            "projects": [
              {
                "__typename": "Project",
                "createdAt": "2026-07-18T04:56:00Z",
                "databaseIds": [],
                "id": "prj-d9dgeo0bd9nc73a0vh1g",
                "keyValueIds": [],
                "name": "bex.co",
                "ownerId": "tea-d98210cbbpdc73dcrkvg",
                "serviceIds": [
                  "srv-d9bkcspg9s7c73d0n8ug",
                  "srv-d9e40ei9086p3l1jri30"
                ]
              },
              {
                "__typename": "Project",
                "createdAt": "2026-07-19T05:14:27Z",
                "databaseIds": [],
                "id": "prj-d9e5qct5qe4s73b1mjn0",
                "keyValueIds": [],
                "name": "beancount.io",
                "ownerId": "tea-d98210cbbpdc73dcrkvg",
                "serviceIds": ["srv-d9bj8s3eg85c7390eb9g"]
              },
              {
                "__typename": "Project",
                "createdAt": "2026-08-09T23:06:05Z",
                "databaseIds": [],
                "id": "prj-d9sgfnbjghus73cg6hg0",
                "keyValueIds": [],
                "name": "forums",
                "ownerId": "tea-d98210cbbpdc73dcrkvg",
                "serviceIds": ["srv-d9nqg9dcavls73fp8m2g"]
              },
              {
                "__typename": "Project",
                "createdAt": "2026-10-11T05:45:46Z",
                "databaseIds": [],
                "id": "prj-db5i52imr7rc73alv3sg",
                "keyValueIds": [],
                "name": "qa-20261010-a155-a",
                "ownerId": "tea-d98210cbbpdc73dcrkvg",
                "serviceIds": []
              },
              {
                "__typename": "Project",
                "createdAt": "2026-10-11T05:46:59Z",
                "databaseIds": [],
                "id": "prj-db5i5kqgccos73bibo2g",
                "keyValueIds": [],
                "name": "qa-20261010-a155-b",
                "ownerId": "tea-d98210cbbpdc73dcrkvg",
                "serviceIds": ["srv-db5i5iagccos73bibo10"]
              }
            ]
          }
        },
        {
          "data": {
            "project": {
              "__typename": "Project",
              "databaseIds": [],
              "id": "prj-db5i52imr7rc73alv3sg",
              "keyValueIds": [],
              "name": "qa-20261010-a155-a",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": []
            }
          }
        }
      ]
    },
    "observations": [
      {
        "at": "2026-10-11T05:50:44.442Z",
        "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
        "rowVisible": true
      },
      {
        "at": "2026-10-11T05:50:48.451Z",
        "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
        "rowVisible": true
      },
      {
        "at": "2026-10-11T05:50:52.506Z",
        "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
        "rowVisible": true
      },
      {
        "at": "2026-10-11T05:50:56.511Z",
        "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
        "rowVisible": true
      }
    ],
    "authoritative": {
      "request": {
        "operationName": "QaA155MoveRead",
        "query": "query QaA155MoveRead($id:String!,$a:String!){service(id:$id){id projectId environmentId phase revision}environments(projectId:$a){id serviceIds}}",
        "variables": {
          "id": "srv-db5i5iagccos73bibo10",
          "a": "prj-db5i52imr7rc73alv3sg"
        }
      },
      "at": "2026-10-11T05:51:00.859Z",
      "status": 200,
      "response": {
        "data": {
          "environments": [
            {
              "id": "evm-db5i55amr7rc73alv3tg",
              "serviceIds": []
            }
          ],
          "service": {
            "environmentId": null,
            "id": "srv-db5i5iagccos73bibo10",
            "phase": "Running",
            "projectId": "prj-db5i5kqgccos73bibo2g",
            "revision": "rev-1"
          }
        }
      }
    },
    "final": {
      "at": "2026-10-11T05:51:00.860Z",
      "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
      "rowVisible": true
    }
  },
  "removeFromProject": {
    "mutation": {
      "at": "2026-10-11T05:52:17.344Z",
      "status": 200,
      "request": [
        {
          "operationName": "SetProjectServices",
          "variables": {
            "id": "prj-db5i52imr7rc73alv3sg",
            "serviceIds": []
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetProjectServices($id: String!, $serviceIds: [String!]!) {\n  setProjectServices(id: $id, serviceIds: $serviceIds) {\n    ...ProjectFields\n    __typename\n  }\n}\n\nfragment ProjectFields on Project {\n  id\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  __typename\n}"
        }
      ],
      "response": [
        {
          "data": {
            "setProjectServices": {
              "__typename": "Project",
              "createdAt": "2026-10-11T05:45:46Z",
              "databaseIds": [],
              "id": "prj-db5i52imr7rc73alv3sg",
              "keyValueIds": [],
              "name": "qa-20261010-a155-a",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": []
            }
          }
        }
      ]
    },
    "first": {
      "at": "2026-10-11T05:52:23.349Z",
      "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
      "rowVisible": true
    },
    "authoritative": {
      "request": {
        "operationName": "QaA155MoveRead",
        "query": "query QaA155MoveRead($id:String!,$a:String!){service(id:$id){id projectId environmentId phase revision}environments(projectId:$a){id serviceIds}}",
        "variables": {
          "id": "srv-db5i5iagccos73bibo10",
          "a": "prj-db5i52imr7rc73alv3sg"
        }
      },
      "at": "2026-10-11T05:52:29.648Z",
      "status": 200,
      "response": {
        "data": {
          "environments": [
            {
              "id": "evm-db5i55amr7rc73alv3tg",
              "serviceIds": []
            }
          ],
          "service": {
            "environmentId": null,
            "id": "srv-db5i5iagccos73bibo10",
            "phase": "Running",
            "projectId": null,
            "revision": "rev-1"
          }
        }
      }
    },
    "final": {
      "at": "2026-10-11T05:52:29.649Z",
      "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nqa-a155-stage\nNew Service\nNew Environment\nqa-a155-stage\n1 resource\nManage resources\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nSelected: 0\nChoose environment\nMove\n\tName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\t\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t",
      "rowVisible": true
    }
  },
  "unassignedControl": {
    "before": {
      "at": "2026-10-11T05:53:35.858Z",
      "url": "https://dashboard.bex.co/project/prj-db5i52imr7rc73alv3sg?env=unassigned",
      "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nUnassigned\nNew Service\nNew Environment\nUnassigned\nAll (1)\nServices (1)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\nName\tStatus\tRuntime\tRegion\tUpdated\t\nActions\n\n\nqa-20261010-a155-place\nWeb Service\n\tRunning\tImage\tfsn1\tnow\t"
    },
    "mutation": {
      "at": "2026-10-11T05:53:36.949Z",
      "status": 200,
      "request": [
        {
          "operationName": "SetProjectServices",
          "variables": {
            "id": "prj-db5i5kqgccos73bibo2g",
            "serviceIds": ["srv-db5i5iagccos73bibo10"]
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation SetProjectServices($id: String!, $serviceIds: [String!]!) {\n  setProjectServices(id: $id, serviceIds: $serviceIds) {\n    ...ProjectFields\n    __typename\n  }\n}\n\nfragment ProjectFields on Project {\n  id\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  __typename\n}"
        }
      ],
      "response": [
        {
          "data": {
            "setProjectServices": {
              "__typename": "Project",
              "createdAt": "2026-10-11T05:46:59Z",
              "databaseIds": [],
              "id": "prj-db5i5kqgccos73bibo2g",
              "keyValueIds": [],
              "name": "qa-20261010-a155-b",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": ["srv-db5i5iagccos73bibo10"]
            }
          }
        }
      ]
    },
    "after": {
      "at": "2026-10-11T05:53:37.740Z",
      "url": "https://dashboard.bex.co/project/prj-db5i52imr7rc73alv3sg?env=unassigned",
      "main": "qa-20261010-a155-a\nSearch\n⌘ K\nNew\nP\n\nPROJECT\n\nqa-20261010-a155-a\nEnvironments\nUnassigned\nNew Service\nNew Environment\nUnassigned\nAll (0)\nServices (0)\nDatabases (0)\nKey Values (0)\nEnv Groups (0)\n\nNo resources in this environment yet."
    },
    "removedWithoutReload": true
  }
}
```
