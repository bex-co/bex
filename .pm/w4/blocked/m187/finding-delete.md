# Linked environment group blocks project and environment deletion

Why: an ordinary valid service/group setup makes authorized grouping deletion refuse with an unrelated move instruction.

- **Severity / source:** major; live cycle24, 2026-10-09 UTC, workspace bex. A separate teardown root cause from finding-move.md.
- **Repro:** after explicit target assignment has restored B/TargetEnv, move the unlinked group toTargetEnv, link only the owned web, wait for its config deploy to become Live. Group and service share the target environment. Fresh Project Settings → Delete Project → confirm refuses ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES. Reload and repeat: same refusal. No cross-project inconsistency remains before this candidate.
- **Times:** first UI refusal09:58:48Z. A fresh second capture encountered a harness error because the confirmation closed; by-id reads remained200. The complete fresh third repeat10:00:15Z records the refusal and visible toast. REST/MCP project deletion10:01:05Z and direct REST environment deletion10:01:37Z reproduce it.
- **Expected:** Bex preserve-and-detach deletion succeeds; project/child environment disappear, service survives unassigned, group survives workspace-scoped with contents and valid links intact. Direct environment deletion retains its service's parent project. This is Bex's established divergence, not Render's cascading deletion.
- **Actual:** project/environment still exist, service and group keep logical placement, and the UI toast says to move linked services or unlink before moving the group. Public200, saved group value/revision and existing deployment history remain at the sampled reads. No CR-field or no-side-effect claim.

## Root cause, consumer and bounded correction

- projects/service.go:537–545 calls Environments.ClearMembersForProject before Store.DeleteProject. environments/service.go:809 calls clearEnvironmentMembers for each child; its direct Delete (:817–832) reaches the same helper.
- clearEnvironmentMembers (:850–887) clears App isolation/IP projections first, then visits database/key-value/group resources. It does not clear committed App environment membership at that point; those rows are cleared by the later store cascade.
- envGroupResources.leave (:383–384) ignores the supplied expected environment and calls the public SetEnvironmentID(group,empty). envgroups/service.go:745 delegates to MoveEnvGroup; :771 validates current links inside mutateMetaCAS before committing metadata. :1385–1410 compares each linked App's environment label to empty and emits ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES.
- This public move guard is correct for standalone Move group. It is the wrong admission path during teardown of an environment whose own linked members are also planned to leave. The later store deletion that would complete the detach is never reached.
- clearEnvironmentMembers' pre-group writes can precede the failure by source ordering. The production run did not inspect ACL/isolation CR fields or quantify transient policy changes. Do not turn that inference into a claimed observed bypass.
- envgroups/meta.go:100–145 was opened: each CAS attempt rereads durable current metadata, reruns the mutation callback and fences deleted/foreign metadata. Preserve that behavior, concurrent fields, current links and explicit conflicts.

**Fix:** define a narrow, authorized grouping-teardown detach protocol using the expected deleted environment and planned member departures. It must reach service null environment and group workspace scope while retaining group contents/content revision and valid service references. Conditional/CAS clearing must leave a newer valid reassignment intact; preflight linked-group eligibility/permissions before earlier destructive projection writes and make interruption/retry explicit. Keep standalone MoveEnvGroup's current-link check and exact-scope guard; do not globally bypass it or require the user to unlink. Merely clearing CR labels before the SQL cascade lets a concurrent projector reassert them, so prove the store/projector/CAS ordering with a composed test rather than treating one extra label patch as the fix.

## Blast radius, aliases and controls

- clearEnvironmentMembers has2 production call sites: environment Delete and project cascade via clearMembersForProject. Both deletion paths were live probed. Three adapters each: REST DELETE /v1/projects/{id} or /v1/environments/{id}; GraphQL deleteProject/deleteEnvironment; MCP delete_project/delete_environment. Project deletion exercised all three; environment deletion only REST.
- envGroupResources.leave has2consumer sites: teardown and setResourceMembers (:1035) used by SetEnvGroups. Its public SetEnvironmentID seam also serves joins and Blueprint group assignment. Keep ordinary membership removal/move validation; a global change to leave or SetEnvironmentID would affect those controls. Audit the complete caller census in t003.
- Standalone Move group refusal was repeated from fresh group loads while its service was in another environment; no content/scope/link changed. An unlinked Move group succeeded preserving the value and egr1_AAAAAAAAAAE. Those controls verify why the validator remains required.
- Hard control: only unlink the group, wait its config deploy terminal Live, then the same UI Delete Project succeeds10:02:49Z. Project and environment GET404; service and group both envnull, service projectnull, content revision/value unchanged. Public URL still200 at10:03:18Z. The control uses the same helper with cur.links=[], which makes validateLinkedServiceEnvironments accept; not a route workaround.
- Postgres/KeyValue use separate conditional ClearEnvironmentID writers and retain resource-owned rules. No live datastore mutation this pass. Five App kinds are potential linked members; only image web observed.
- Adjacent classes: retain child ACL administrator preflight, service/group authorization, foreign/missing confidentiality, unauthenticated401, missing404, sensitive role403, metadata conflict and dependency errors. Auth refusal before writes differs from post-commit interruption. Do not promise a distributed transaction across SQL/Kubernetes/OpenBao or report failed teardown as successful.

**Render:** [Render projects documentation](https://render.com/docs/projects) documents dashboard cascade deletion and scoped groups. ADR032 explicitly chooses Bex preserve/detach; correcting Bex must not import Render's resource deletion semantics. No authenticated Render deletion probe.

**Dedupe:** open/done search plus74 non-done README scan and DO_NOT_DO review found no owner of linked-group grouping teardown. w4/m32 explicitly repaired project-cascade group clearing in its t005 resolution; the stronger current public scope validator was introduced4f3619d1f, and current leave adapter c00b83129 still routes cleanup through it. w7/m152 admission guarantee is a guard to keep, not this link eligibility defect. w4/m97 and w2/m73 require safe ordinary group scope changes. Last40product commits and targeted validator/leave history show no fix waiting for deployment.

**Original DoD / unverified:** the milestone audits all m32/m131 clauses. m97's seven clauses cover controlled metadata interleavings, rename claims, scope validation, App-reference outcomes/partial failure, content/tombstones, dev-4 drill and transport/tests: this pass verifies serial scope guards and service/group logical links/content only; no two-instance races, rename-claim races, tombstone/content compensation, App Secret-reference inspection or dev-4 drill was performed. Preserve all seven in t003/t007's current regression matrix rather than claiming this deletion test proves them. Restricting/deny-all/isolation-on, non-admin/foreign/error injection, multi-child cascades, concurrent reassignment, mobile/zh and recovery remain future tests.

## Durable probes

Complete requested GraphQL and complete small refusal protocol bodies follow. No auth headers/cookies. Recorded IDs were deleted.

### Fresh UI repeat and current valid placement

```json
{
  "at": "2026-10-09T10:00:15.494Z",
  "request": [
    {
      "operationName": "DeleteProject",
      "variables": {
        "id": "prj-db4b2qjqi37c73d1e12g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteProject($id: String!) {\n  deleteProject(id: $id)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteProject": null
      },
      "errors": [
        {
          "message": "move the linked services to the target Environment or unlink them before moving this group",
          "locations": [
            {
              "line": 2,
              "column": 3
            }
          ],
          "path": ["deleteProject"],
          "extensions": {
            "code": "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES",
            "serviceIds": ["srv-db4b3uvpcudc738ddrrg"],
            "targetEnvironmentId": ""
          }
        }
      ]
    }
  ],
  "ui": {
    "snapshot": "- main:\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261009-loop-a24-target\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - heading \"Project settings\" [level=1]\n  - text: Project Name A unique name for your project\n  - textbox \"Project Name\" [disabled]: qa-20261009-loop-a24-target\n  - button \"Edit\"\n  - text: Delete Project Its environments and their configuration are deleted with it. Its services, databases, and key value stores are not deleted — they become unassigned. This action cannot be undone.\n  - button \"Delete Project\"",
    "toasts": [
      "Move the linked services to the target Environment or unlink them before moving this group"
    ],
    "confirmOpen": 0
  },
  "state": {
    "query": "query {service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision projectId environmentId undeployedChanges}envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name environmentId revision serviceLinks}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger}}",
    "status": 200,
    "body": {
      "data": {
        "deploys": [
          {
            "id": "dep-db4bk8db00tc739pb4l0",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4bgrdb00tc739pb4g0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4baptb00tc739pb4bg",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4b3uvpcudc738ddrs0",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": ["srv-db4b3uvpcudc738ddrrg"]
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "revision": "rev-6",
          "undeployedChanges": false
        }
      }
    }
  }
}
```

### REST and MCP project deletion

```json
{
  "at": "2026-10-09T10:01:05.763Z",
  "rest": {
    "method": "DELETE",
    "url": "https://api.bex.co/v1/projects/prj-db4b2qjqi37c73d1e12g",
    "status": 409,
    "body": {
      "code": "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES",
      "error": "move the linked services to the target Environment or unlink them before moving this group",
      "id": "conflict",
      "message": "move the linked services to the target Environment or unlink them before moving this group",
      "params": {
        "serviceIds": ["srv-db4b3uvpcudc738ddrrg"],
        "targetEnvironmentId": ""
      }
    }
  },
  "mcp": {
    "request": {
      "jsonrpc": "2.0",
      "id": 25,
      "method": "tools/call",
      "params": {
        "name": "delete_project",
        "arguments": {
          "id": "prj-db4b2qjqi37c73d1e12g"
        }
      }
    },
    "status": 200,
    "body": {
      "jsonrpc": "2.0",
      "id": 25,
      "result": {
        "content": [
          {
            "type": "text",
            "text": "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES: move the linked services to the target Environment or unlink them before moving this group"
          }
        ],
        "isError": true
      }
    }
  },
  "state": {
    "query": "query {service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision projectId environmentId undeployedChanges}envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name environmentId revision serviceLinks}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger}}",
    "status": 200,
    "body": {
      "data": {
        "deploys": [
          {
            "id": "dep-db4bk8db00tc739pb4l0",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4bgrdb00tc739pb4g0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4baptb00tc739pb4bg",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4b3uvpcudc738ddrs0",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": ["srv-db4b3uvpcudc738ddrrg"]
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "revision": "rev-6",
          "undeployedChanges": false
        }
      }
    }
  }
}
```

### Direct REST environment deletion (same helper, separate observed route)

```json
{
  "at": "2026-10-09T10:01:37.397Z",
  "rest": {
    "method": "DELETE",
    "url": "https://api.bex.co/v1/environments/evm-db4b2v8b7jis73b70oig",
    "status": 409,
    "body": {
      "code": "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES",
      "error": "move the linked services to the target Environment or unlink them before moving this group",
      "id": "conflict",
      "message": "move the linked services to the target Environment or unlink them before moving this group",
      "params": {
        "serviceIds": ["srv-db4b3uvpcudc738ddrrg"],
        "targetEnvironmentId": ""
      }
    }
  },
  "state": {
    "query": "query {service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision projectId environmentId undeployedChanges}envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name environmentId revision serviceLinks}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger}}",
    "status": 200,
    "body": {
      "data": {
        "deploys": [
          {
            "id": "dep-db4bk8db00tc739pb4l0",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4bgrdb00tc739pb4g0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4baptb00tc739pb4bg",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4b3uvpcudc738ddrs0",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": ["srv-db4b3uvpcudc738ddrrg"]
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": "evm-db4b2v8b7jis73b70oig",
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "revision": "rev-6",
          "undeployedChanges": false
        }
      }
    }
  }
}
```

### Passing control after unlink and terminal rollout

```json
{
  "at": "2026-10-09T10:02:49.416Z",
  "before": {
    "data": {
      "deploys": [
        {
          "id": "dep-db4bn40ec12s73fjudvg",
          "status": "live",
          "trigger": "config_change"
        },
        {
          "id": "dep-db4bk8db00tc739pb4l0",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db4bgrdb00tc739pb4g0",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db4baptb00tc739pb4bg",
          "status": "deactivated",
          "trigger": "config_change"
        },
        {
          "id": "dep-db4b3uvpcudc738ddrs0",
          "status": "deactivated",
          "trigger": "create"
        }
      ],
      "envGroup": {
        "environmentId": "evm-db4b2v8b7jis73b70oig",
        "id": "evg-db4baolb00tc739pb4b0",
        "name": "qa-20261009-loop-a24-group",
        "revision": "egr1_AAAAAAAAAAE",
        "serviceLinks": []
      },
      "envGroupVar": {
        "key": "QA_PLACEMENT",
        "value": "qa-a24-group-saved"
      },
      "service": {
        "environmentId": "evm-db4b2v8b7jis73b70oig",
        "id": "srv-db4b3uvpcudc738ddrrg",
        "name": "qa-20261009-loop-a24-web",
        "phase": "Running",
        "projectId": "prj-db4b2qjqi37c73d1e12g",
        "revision": "rev-8",
        "undeployedChanges": false
      }
    }
  },
  "request": [
    {
      "operationName": "DeleteProject",
      "variables": {
        "id": "prj-db4b2qjqi37c73d1e12g"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation DeleteProject($id: String!) {\n  deleteProject(id: $id)\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "deleteProject": "prj-db4b2qjqi37c73d1e12g"
      }
    }
  ],
  "state": {
    "query": "query {service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision projectId environmentId undeployedChanges}envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name environmentId revision serviceLinks}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger}}",
    "status": 200,
    "body": {
      "data": {
        "deploys": [
          {
            "id": "dep-db4bn40ec12s73fjudvg",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4bk8db00tc739pb4l0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4bgrdb00tc739pb4g0",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4baptb00tc739pb4bg",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "id": "dep-db4b3uvpcudc738ddrs0",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": null,
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": []
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": null,
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "projectId": null,
          "revision": "rev-8",
          "undeployedChanges": false
        }
      }
    }
  },
  "checks": [
    {
      "url": "https://api.bex.co/v1/projects/prj-db4b2qjqi37c73d1e12g",
      "status": 404,
      "body": {
        "code": "NOT_FOUND",
        "error": "project not found",
        "id": "not_found",
        "message": "project not found",
        "params": null
      }
    },
    {
      "url": "https://api.bex.co/v1/environments/evm-db4b2v8b7jis73b70oig",
      "status": 404,
      "body": {
        "code": "NOT_FOUND",
        "error": "environment not found",
        "id": "not_found",
        "message": "environment not found",
        "params": null
      }
    }
  ]
}
```

## Evidence and cleanup

.playwright-mcp/qa-a24-project-delete-linked-group-refusal.png was opened and shows the truthful error toast next to the promised delete behavior. Complete probes above are durable; ignored screenshots and ledger stay local.

All seven fixtures were removed and only this run's session revoked; details and inventory proof in the milestone. No Kubernetes object-absence claim.
