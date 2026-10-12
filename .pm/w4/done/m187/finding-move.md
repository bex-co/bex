# Project move retains an environment from the former project

Why: a successful move leaves contradictory machine-readable placement and offers configuration from the old environment.

- **Severity / source:** major; live cycle24, 2026-10-09 UTC, workspace bex. Regression interaction with w4/m32's departure guarantee and w6/036's single-write move. Research HEAD is the milestone's.
- **Repro:** create owned sourceA/Stage and targetB/TargetEnv, create the Free image web in Stage. Link a synthetic group in Stage; fresh-load A/Stage and move its web row toB. ProjectA and Stage membership become empty, but service reads retain Stage. Unlink the group, wait for that config deploy's Live terminal state, reassign the service to Stage through Manage resources, fresh-load A/Stage again, and repeat Move toB. It fails the same way without any group link.
- **Times:** initial move09:41:23Z capture; independent reads still disagree09:43:01Z. Unlinked fresh repeat09:50:35Z; REST/GQL/MCP and filter09:51:13Z. Fresh service /env at09:52:41Z offers the old Stage group as Available to link (1). No wrong link was submitted.
- **Expected:** projectIdB, environment null/omitted; A/Stage excludes it and REST environment filter excludes it. Target-only assignment supplies no target environment.
- **Actual:** projectIdB with environmentId StageA on service detail/list; Stage's membership excludes it because it checks both columns. REST environment filter includes it. The UI correctly shows projectB but uses the returned old environment to offer StageA's group. No timing-only disagreement: independent fresh reads persist and the unlinked repeat reproduces it.

## Root cause and target fix

- lego/backend/internal/store/projects.go:186–221 replaces target membership. Its first UPDATE (:195) NULLs project/environment only for rows whose project already equals targetB. Its incoming UPDATE (:202) sets project_idB without clearing environment_id, so A/Stage→B/Stage is committed. placementDiffAround (:143) captures incoming rows, but placementChanges (:112) accurately reports EnvironmentFrom=Stage/EnvironmentTo=Stage.
- lego/backend/internal/projects/service.go:576–595 records the store diff and clears projected rules only for EnvironmentFrom nonnil→EnvironmentTo nil. The bad producer diff never reaches that consumer predicate.
- store/environments.go:223 and:246 require matching parent project for membership, while store/app_placements.go:35 reads the committed pair and apps/placement.go:78 hydrates Get/List. apps/rest.go:712 filters the hydrated EnvironmentID. The disagreement follows these separate predicates; do not hide it by filtering only display reads.
- store/store.go:1493 LEFT JOINs environment rules by environment_id alone; reconciler.go:1952 projects that environment label and:2019 copies its inbound-IP layer. Those are source-backed risks of stale policy, not measured stale enforcement in this run's allow-all/isolation-off fixture.
- dashboard/src/features/services/components/env-groups-panel.tsx:85–95 makes the service's environment authoritative; :189–205 filters available groups. Thus this UI consequence is traced independently from the SQL producer. The source project's initial brief stale row eventually disappeared; no additional cache finding.
- pgx/v5 v5.10.0 tx.go:398–450 was opened: BeginFunc commits a nil-error callback and rolls back callback failures. This is an incorrect successful SQL update inside the transaction, not a library retry or delayed commit.

**Fix:** atomically clear incompatible environment placement when a selected App changes projects, including incoming members of another project. Return the full before→after diff (A/Stage→B/null) so the existing projected-isolation/IP clear and service_moved facts see the departure. Retain the single target write, bounded tenant predicates, errors and authorization. Audit compatible retained target members and same-membership/no-op semantics explicitly; no live no-op claim this pass. Clear only inherited environment state, preserve service-owned allowlists/config and group content/links. A read-only normalizer would conceal drift and leave policy writers wrong.

**Existing drift:** these source rows remain wrong until an explicit valid assignment/delete repairs them. t004 inventories and conditionally repairs only provable managed same-tenant incompatible references with a reviewed retry-safe mechanism; no automatic broad production repair or obsolete backfill resurrection is authorized by this finding.

## Blast radius, aliases and controls

- One production Store.SetProjectServices call site: projects.Service.SetServices. Three adapters: GraphQL setProjectServices; REST PUT /v1/projects/{id}/service-links; MCP update_project with serviceIds (the retired set_project_services is not a live alias). Only GraphQL mutation was live exercised; all three service read protocols were.
- The dashboard hook has one MoveToProjectMenu consumer, mounted by3 row-action files (service, database, key-value). The service branch reaches this SQL; datastore branches have separate SetProjectID writers. Five App kinds share the untyped apps SQL: web, static, cron, worker, private. Only image web was live mutated. t003/t007 verify siblings and separate PG/KV controls.
- Two production readViews call sites: apps.Get and apps.List. One production ClearServiceEnvironmentLayer caller. placementDiffAround also serves SetEnvironmentServices; retain the environment-assignment control.
- Live passing controls: Stage→Prod assigned the proper environment; deleting Prod retained parentA and cleared env; A→B/B→A from Unassigned and Remove from project returned proper null placement; assigning toStage joins parentA. After failing move, explicit target-environment assignment yielded B/TargetEnv, proving the producer-specific difference. Placement-only checks served HTTP200; captured deploy IDs/revisions unchanged within those probes, not continuous pod/uptime monitoring.
- Adjacent errors: retain existing unauthenticated401, role/foreign authorization refusals, missing-resource404 and unavailable/source errors. A failed SQL transaction must not publish move facts or partial placement; post-commit projection failure is a distinct explicit outcome and must be retryable without clearing a newer valid assignment.

**Render:** [Projects and Environments](https://render.com/docs/projects) puts environments inside projects and limits scoped groups to their environment. Bex's projectId extension and optional REST/MCP environment omission are ADR032's documented wire contract. No authenticated Render timing/move measurement.

**Dedupe / original DoD:** see the milestone's complete m32/m131 audit and cross-board scan. The retained incoming SQL dates to41e9b612e; environment clearing added9ef8b64b8 only on current-target departures. Targeted histories and last40product commits show no pending fix. w7/m151 covers datastore writers; m178 covers UI refresh; 217/220 cover UI scope resolution, not this writer.

**Unverified:** CR layer enforcement/protection, other App kinds, PG/KV live moves, privileged/foreign/errors, no-op/retained targets, races/recovery, mobile/zh and actual group-value delivery. Keep them as task verification work.

## Durable probes

POST https://api.bex.co/graphql from an authorized browser. Fixture IDs are deleted; recreate owned equivalents. Complete Apollo mutation request/response and complete requested GraphQL response follow. REST/MCP service bodies below are explicitly placement projections to avoid persisting unrelated sensitive fields; they are not complete service responses.

### Fresh repeat with no group link

```json
{
  "at": "2026-10-09T09:50:35.498Z",
  "request": [
    {
      "operationName": "SetProjectServices",
      "variables": {
        "id": "prj-db4b2qjqi37c73d1e12g",
        "serviceIds": ["srv-db4b3uvpcudc738ddrrg"]
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
  "status": 200,
  "response": [
    {
      "data": {
        "setProjectServices": {
          "__typename": "Project",
          "createdAt": "2026-10-09T09:18:34Z",
          "databaseIds": [],
          "id": "prj-db4b2qjqi37c73d1e12g",
          "keyValueIds": [],
          "name": "qa-20261009-loop-a24-target",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "serviceIds": ["srv-db4b3uvpcudc738ddrrg"]
        }
      }
    }
  ],
  "state": {
    "query": "query {service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision plan projectId environmentId undeployedChanges}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger createdAt startedAt finishedAt failureReason}a:project(id:\"prj-db4b1qdlm2ps739a4270\"){id name serviceIds}b:project(id:\"prj-db4b2qjqi37c73d1e12g\"){id name serviceIds}aEnvs:environments(projectId:\"prj-db4b1qdlm2ps739a4270\"){id name serviceIds databaseIds keyValueIds envGroupIds}bEnvs:environments(projectId:\"prj-db4b2qjqi37c73d1e12g\"){id name serviceIds databaseIds keyValueIds envGroupIds} envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name ownerId environmentId serviceLinks revision}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}}",
    "status": 200,
    "body": {
      "data": {
        "a": {
          "id": "prj-db4b1qdlm2ps739a4270",
          "name": "qa-20261009-loop-a24-source-renamed",
          "serviceIds": []
        },
        "aEnvs": [
          {
            "databaseIds": [],
            "envGroupIds": ["evg-db4baolb00tc739pb4b0"],
            "id": "evm-db4b2job7jis73b70ohg",
            "keyValueIds": [],
            "name": "qa-20261009-loop-a24-stage",
            "serviceIds": []
          }
        ],
        "b": {
          "id": "prj-db4b2qjqi37c73d1e12g",
          "name": "qa-20261009-loop-a24-target",
          "serviceIds": ["srv-db4b3uvpcudc738ddrrg"]
        },
        "bEnvs": [
          {
            "databaseIds": [],
            "envGroupIds": [],
            "id": "evm-db4b2v8b7jis73b70oig",
            "keyValueIds": [],
            "name": "qa-20261009-loop-a24-target-env",
            "serviceIds": []
          }
        ],
        "deploys": [
          {
            "createdAt": "2026-10-09T09:48:29.917793Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:48:44.050128Z",
            "id": "dep-db4bgrdb00tc739pb4g0",
            "startedAt": "2026-10-09T09:48:30.921299Z",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "createdAt": "2026-10-09T09:35:35.030714Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:36:00.926711Z",
            "id": "dep-db4baptb00tc739pb4bg",
            "startedAt": "2026-10-09T09:35:43.898176Z",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "createdAt": "2026-10-09T09:20:59.15293Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:21:13.011148Z",
            "id": "dep-db4b3uvpcudc738ddrs0",
            "startedAt": "",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": "evm-db4b2job7jis73b70ohg",
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": []
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": "evm-db4b2job7jis73b70ohg",
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "plan": "free",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "revision": "rev-5",
          "undeployedChanges": false
        }
      }
    }
  }
}
```

### Fresh three-protocol reads and old-environment filter

```json
{
  "at": "2026-10-09T09:51:13.803Z",
  "gql": {
    "request": {
      "query": "query {services(ownerId:\"tea-d98210cbbpdc73dcrkvg\"){id projectId environmentId} service(id:\"srv-db4b3uvpcudc738ddrrg\"){id name phase revision plan projectId environmentId undeployedChanges}deploys(serviceId:\"srv-db4b3uvpcudc738ddrrg\"){id status trigger createdAt startedAt finishedAt failureReason}a:project(id:\"prj-db4b1qdlm2ps739a4270\"){id name serviceIds}b:project(id:\"prj-db4b2qjqi37c73d1e12g\"){id name serviceIds}aEnvs:environments(projectId:\"prj-db4b1qdlm2ps739a4270\"){id name serviceIds databaseIds keyValueIds envGroupIds}bEnvs:environments(projectId:\"prj-db4b2qjqi37c73d1e12g\"){id name serviceIds databaseIds keyValueIds envGroupIds} envGroup(id:\"evg-db4baolb00tc739pb4b0\"){id name ownerId environmentId serviceLinks revision}envGroupVar(id:\"evg-db4baolb00tc739pb4b0\",key:\"QA_PLACEMENT\"){key value}}"
    },
    "status": 200,
    "body": {
      "data": {
        "a": {
          "id": "prj-db4b1qdlm2ps739a4270",
          "name": "qa-20261009-loop-a24-source-renamed",
          "serviceIds": []
        },
        "aEnvs": [
          {
            "databaseIds": [],
            "envGroupIds": ["evg-db4baolb00tc739pb4b0"],
            "id": "evm-db4b2job7jis73b70ohg",
            "keyValueIds": [],
            "name": "qa-20261009-loop-a24-stage",
            "serviceIds": []
          }
        ],
        "b": {
          "id": "prj-db4b2qjqi37c73d1e12g",
          "name": "qa-20261009-loop-a24-target",
          "serviceIds": ["srv-db4b3uvpcudc738ddrrg"]
        },
        "bEnvs": [
          {
            "databaseIds": [],
            "envGroupIds": [],
            "id": "evm-db4b2v8b7jis73b70oig",
            "keyValueIds": [],
            "name": "qa-20261009-loop-a24-target-env",
            "serviceIds": []
          }
        ],
        "deploys": [
          {
            "createdAt": "2026-10-09T09:48:29.917793Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:48:44.050128Z",
            "id": "dep-db4bgrdb00tc739pb4g0",
            "startedAt": "2026-10-09T09:48:30.921299Z",
            "status": "live",
            "trigger": "config_change"
          },
          {
            "createdAt": "2026-10-09T09:35:35.030714Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:36:00.926711Z",
            "id": "dep-db4baptb00tc739pb4bg",
            "startedAt": "2026-10-09T09:35:43.898176Z",
            "status": "deactivated",
            "trigger": "config_change"
          },
          {
            "createdAt": "2026-10-09T09:20:59.15293Z",
            "failureReason": "",
            "finishedAt": "2026-10-09T09:21:13.011148Z",
            "id": "dep-db4b3uvpcudc738ddrs0",
            "startedAt": "",
            "status": "deactivated",
            "trigger": "create"
          }
        ],
        "envGroup": {
          "environmentId": "evm-db4b2job7jis73b70ohg",
          "id": "evg-db4baolb00tc739pb4b0",
          "name": "qa-20261009-loop-a24-group",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "revision": "egr1_AAAAAAAAAAE",
          "serviceLinks": []
        },
        "envGroupVar": {
          "key": "QA_PLACEMENT",
          "value": "qa-a24-group-saved"
        },
        "service": {
          "environmentId": "evm-db4b2job7jis73b70ohg",
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "phase": "Running",
          "plan": "free",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "revision": "rev-5",
          "undeployedChanges": false
        },
        "services": [
          {
            "environmentId": null,
            "id": "srv-d9bkcspg9s7c73d0n8ug",
            "projectId": "prj-d9dgeo0bd9nc73a0vh1g"
          },
          {
            "environmentId": null,
            "id": "srv-d9bj8s3eg85c7390eb9g",
            "projectId": "prj-d9e5qct5qe4s73b1mjn0"
          },
          {
            "environmentId": null,
            "id": "srv-d9nqg9dcavls73fp8m2g",
            "projectId": "prj-d9sgfnbjghus73cg6hg0"
          },
          {
            "environmentId": null,
            "id": "srv-d9ndt8hmcglc739fkp50",
            "projectId": null
          },
          {
            "environmentId": null,
            "id": "srv-d9e40ei9086p3l1jri30",
            "projectId": "prj-d9dgeo0bd9nc73a0vh1g"
          },
          {
            "environmentId": "evm-db4b2job7jis73b70ohg",
            "id": "srv-db4b3uvpcudc738ddrrg",
            "projectId": "prj-db4b2qjqi37c73d1e12g"
          }
        ]
      }
    }
  },
  "rest": {
    "url": "https://api.bex.co/v1/services/srv-db4b3uvpcudc738ddrrg",
    "status": 200,
    "projection": {
      "id": "srv-db4b3uvpcudc738ddrrg",
      "name": "qa-20261009-loop-a24-web",
      "projectId": "prj-db4b2qjqi37c73d1e12g",
      "environmentId": "evm-db4b2job7jis73b70ohg",
      "environmentIdPresent": true
    }
  },
  "restFilter": {
    "url": "https://api.bex.co/v1/services?ownerId=tea-d98210cbbpdc73dcrkvg&name=qa-20261009-loop-a24-web&environmentId=evm-db4b2job7jis73b70ohg",
    "status": 200,
    "projection": [
      {
        "service": {
          "id": "srv-db4b3uvpcudc738ddrrg",
          "name": "qa-20261009-loop-a24-web",
          "projectId": "prj-db4b2qjqi37c73d1e12g",
          "environmentId": "evm-db4b2job7jis73b70ohg",
          "environmentIdPresent": true
        },
        "cursor": "qa-20261009-loop-a24-web"
      }
    ]
  },
  "mcp": {
    "request": {
      "jsonrpc": "2.0",
      "id": 24,
      "method": "tools/call",
      "params": {
        "name": "get_service",
        "arguments": {
          "serviceId": "srv-db4b3uvpcudc738ddrrg"
        }
      }
    },
    "status": 200,
    "projection": {
      "id": "srv-db4b3uvpcudc738ddrrg",
      "name": "qa-20261009-loop-a24-web",
      "projectId": "prj-db4b2qjqi37c73d1e12g",
      "environmentId": "evm-db4b2job7jis73b70ohg",
      "environmentIdPresent": true
    }
  },
  "snapshot": "- main:\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261009-loop-a24-target\"\n    - button \"qa-20261009-loop-a24-web\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261009-loop-a24-web\" [level=1]\n  - text: Service Running Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db4b3uvpcudc738ddrrg\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261009-loop-a24-web.onbex.co\":\n    - /url: https://qa-20261009-loop-a24-web.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261009-loop-a24-web\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-5\n  - term: Created\n  - definition:\n    - time: 30m\n  - text: Deploys\n  - paragraph: 3 deploys\n  - textbox \"Search loaded deploys and commits\":\n    - /placeholder: Search loaded deploys and commits…\n  - combobox \"Filter by status\": All statuses\n  - table:\n    - rowgroup:\n      - row \"Deploy Trigger Duration Actions\":\n        - columnheader \"Deploy\"\n        - columnheader \"Trigger\"\n        - columnheader \"Duration\"\n        - columnheader \"Actions\"\n    - rowgroup:\n      - row \"Live dep-db4bgrdb00tc739pb4g0 Deployed 2 minutes ago Config Change 13s\":\n        - cell \"Live dep-db4bgrdb00tc739pb4g0 Deployed 2 minutes ago\":\n          - link \"Live dep-db4bgrdb00tc739pb4g0\":\n            - /url: /services/srv-db4b3uvpcudc738ddrrg/deploys/dep-db4bgrdb00tc739pb4g0\n          - time: Deployed 2 minutes ago\n        - cell \"Config Change\"\n        - cell \"13s\"\n        - cell\n      - row \"Deactivated dep-db4baptb00tc739pb4bg Deployed 15 minutes ago Config Change 17s Roll back to dep-db4baptb00tc739pb4bg\":\n        - cell \"Deactivated dep-db4baptb00tc739pb4bg Deployed 15 minutes ago\":\n          - link \"Deactivated dep-db4baptb00tc739pb4bg\":\n            - /url: /services/srv-db4b3uvpcudc738ddrrg/deploys/dep-db4baptb00tc739pb4bg\n          - time: Deployed 15 minutes ago\n        - cell \"Config Change\"\n        - cell \"17s\"\n        - cell \"Roll back to dep-db4baptb00tc739pb4bg\":\n          - button \"Roll back to dep-db4baptb00tc739pb4bg\": Rollback\n      - row \"Deactivated dep-db4b3uvpcudc738ddrs0 Deployed 30 minutes ago First Deploy — Roll back to dep-db4b3uvpcudc738ddrs0\":\n        - cell \"Deactivated dep-db4b3uvpcudc738ddrs0 Deployed 30 minutes ago\":\n          - link \"Deactivated dep-db4b3uvpcudc738ddrs0\":\n            - /url: /services/srv-db4b3uvpcudc738ddrrg/deploys/dep-db4b3uvpcudc738ddrs0\n          - time: Deployed 30 minutes ago\n        - cell \"First Deploy\"\n        - cell \"—\"\n        - cell \"Roll back to dep-db4b3uvpcudc738ddrs0\":\n          - button \"Roll back to dep-db4b3uvpcudc738ddrs0\": Rollback"
}
```

## Evidence

.playwright-mcp/qa-a24-cross-project-stale-scope-available.png shows the enabled old group after the fresh service page; the full accessibility tree and complete queries above establish its scope. The earlier qa-a24-cross-project-stale-scope.png covers the page header, not the below-fold group. Both screenshots were opened and files verified. Durable probes are the handoff evidence; ignored screenshots/ledger are local.

Cleanup is recorded in the milestone; no live fixture remains.
