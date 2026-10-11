# A project move succeeds but the source project keeps the moved service until reload

Why: a completed move should change where the resource appears; an old source row and count undermine the dashboard's placement controls.

**Severity:** major (misleading membership after a successful move; no observed data loss or runtime outage). Live QA on 2026-10-07 UTC, continuous `qa-find-bugs` sweep 8, `muse.env`, workspace `tea-d98210cbbpdc73dcrkvg`. Research main `2892c2091`. Filing only. Work estimate ~2h 5m includes the bounded refresh fix, family audit, real Router/Apollo regression, parity and live closeout.

## Reproduction and observed target

Owned projects A `qa-20261007-r8-project-a` (`prj-db31k5j6phbs73c99m10`) and B `qa-20261007-r8-project-b` (`prj-db31n8hppijc73ae6td0`); Free image web service `qa-20261007-r8-web` (`srv-db31ktr6phbs73c99m3g`), BusyBox 1.37, port 3000. Docker command:

```sh
mkdir -p /tmp/qa-r8; printf 'qa-r8 project membership probe\n' > /tmp/qa-r8/index.html; exec httpd -f -p 3000 -h /tmp/qa-r8
```

1. Prepare the Running service as an A member with no environment. In this sweep, creation selected A and its owned environment; deleting that environment later retained the service in A's Unassigned view, as the dialog promised.
2. Fresh-load `/project/prj-db31k5j6phbs73c99m10?env=unassigned`. On the fixture's row, Actions → Move to project → B. Mutation succeeds. After 10 seconds the source still shows the row and Services (1), although GraphQL A has no service IDs and the service's projectId is B. Reloading A removes it.
3. Independently fresh-load B's Unassigned view and move the same fixture back to A through its row menu. The captured mutation returns A with the fixture. The subsequent Projects refresh returns B with `serviceIds: []`. Source B still shows the row at 3, 6, 9, 12, 15 and 18 seconds; a later independent GraphQL read also confirms B empty and A assigned while the row remains. The viewed screenshot shows B's row and a toast saying the fixture moved to A.
4. Reload B: zero fixture rows. No mutation or redeploy was needed to repair the display.

**Expected target:** retain the outgoing snapshot while a fresh membership read is pending, then publish the source project's authoritative empty membership when that read settles. The row and count leave the source without requiring navigation/reload; the resource remains assigned to the target and keeps serving. Preserve correct backend placement rather than making API reads match the stale UI.

**Controls actually observed:** Remove from project on fresh A updated its source table to zero rows; its mutation returned A, unlike a cross-project move's target-only response. Environment-to-environment bulk move settled to source 0 / target 1, with deny-all inheritance returning HTTP403 and the same Running revision 1. Rename persisted after reload; protected Suspend returned the exact named confirmation and cancel kept Running. Deleting an environment and then a project cleared inherited deny-all, returning HTTP200 without a new deploy, and preserved the child service. These controls do not prove other kinds' project move views.

## Producer, consumer and framework trace

- `dashboard/src/features/projects/hooks/use-move-to-project.ts:131` makes one atomic target membership mutation. That response normalizes the **target** Project; it does not include the source Project. At :103-108 the refresh starts `client.refetchQueries({include:[ProjectsDocument]})` and `router.invalidate()` independently. The invalidate can consume the old source before the Projects network response updates it.
- `dashboard/src/routes/project.$projectId.tsx:29-40` reruns the singular Project loader with `titleLoaderFetchPolicy(cause)`. `dashboard/src/common/lib/document-head/index.ts:148-151` deliberately maps retained `stay` matches to cache-first.
- `dashboard/src/routes/project.$projectId.index.tsx:87-98` reads a Router loader snapshot, maps it to the grouping input, and does not subscribe that Project snapshot to later Apollo entity writes. `dashboard/src/features/projects/hooks/use-grouped-resources.ts:117-139` renders rows from the snapshot's member ID arrays. The later correct Projects response can refresh the entity cache while the displayed snapshot retains the moved ID.
- Framework versions were read from both yarn.lock and installed package metadata: react-router 1.170.25, router-core 1.171.21, Apollo Client 4.1.3. Opened actual router-core `src/router.ts:1668` (an existing match receives stay), :2462-2535 (invalidate marks matches then calls load), and `src/load-client.ts:587` (loader receives match.cause). Opened Apollo `core/QueryManager.js:1052-1060`: a complete cache-first read returns the cache without a link fetch. `core/ApolloClient.js:450-469` returns a Promise for the selected refetches. The app's `common/apollo/cache.ts` has no Project keyFields override. The captured network sequence has SetProjectServices and Projects, both HTTP200, and no singular Project network read during the repeated move. This matches the ordering explanation.
- Existing hook tests at `projects/hooks/__tests__/use-move-to-project.test.ts:7-25` mock both the client and Router; counting invalidate calls cannot verify that a real cache-first loader publishes the refreshed membership.

[Router mutation/invalidation documentation](https://tanstack.com/router/latest/docs/guide/data-mutations) describes background revalidation with the previous result visible while loading. [Apollo's client reference](https://www.apollographql.com/docs/react/api/core/ApolloClient#refetchqueries) documents refetchQueries as asynchronous. The pinned local implementations above establish this finding's mechanism; no library bug is claimed.

## Bounded fix and family scope

Make the source Project cache fresh before publishing its next loader snapshot after a confirmed cross-project move. An explicit network-only Project read by source ID, completed before route invalidation, is supported by the existing typed ProjectDocument and client.query. Do not rely solely on an active Projects watcher surviving dropdown unmount. Keep the refresh on client/router singletons and separate its failure from the already committed mutation; do not add a second membership write or turn a successful move into a failed-move toast.

A production grep found one useMoveToProject caller (MoveToProjectMenu) and **three** MoveToProjectMenu mounts: `service-row-actions.tsx:252`, `database-row-actions.tsx:179`, `key-value-row-actions.tsx:103`. The shared service family represents web/static/cron/worker/private; only web was exercised live. Project-detail and workspace Overview both render these actions, with different data consumers. Task t002 audits each family and preserves the already passing Remove/Overview cases.

The shared title policy has **16 production call sites in 15 files**: agent detail/list, blueprint detail/list, environment-group detail/list, Project, Postgres, Key Value, webhook detail/list, Billing, Notifications, Overview, and two service-detail-loader sites. Apply a **project refresh fix**, retaining the shared policy and ordinary tab/search/preload behavior. A global switch back to network-only would discard the earlier navigation work and is outside this filing.

Pre-settle state may show the previous snapshot while refreshing. Mutation failure must preserve source membership and retain the named error; unauthenticated/forbidden/not-found handling stays with the existing loaders. A post-success refresh rejection must not retry the membership write or relabel the successful move as failed. Test this separately from mutation rejection and menu unmount.

## Dedupe and precedent

Searched open, blocked and done items across .pm, all open milestone titles, DO_NOT_DO, the latest 40 dashboard/lego commits, and targeted Project/cache/history searches after fetching and fast-forwarding main to 2892c2091. No open item covers this ordering regression; no later fix changes the hook/loader path.

- **Regression of `bb2ce9d74` (2026-07-18):** that dedicated source-table refresh commit added Router invalidation while its loader was network-only. `f01a6b970` (2026-07-31 navigation fix) later introduced cache-first retained matches. The old commit has no linked PM milestone/DoD; its concrete source-row removal guarantee fails for cross-project move here, while the Remove control passes.
- **w6/done/036:** its single-write guarantee remains satisfied: exactly one SetProjectServices mutation; source empty, target assigned. No orphaning or second-write failure is reported.
- **w6/done/054 → w6/m134:** event emission is a different surface and was not audited here; this finding concerns the displayed membership snapshot.
- **w4/done/173:** creation hint correctly explains that an explicit environment assigns the parent project; this sweep selected an environment and verified both IDs. The stale row appears later during a real move, not creation-copy ambiguity.
- **w9/done/m62 and m68:** retain primed cache-first mount, ordinary retained-navigation request savings, hover prefetch and visibility gating. Their timing/polling DoD bullets were not remeasured in this sweep; t002/t005 must guard relevant behavior rather than claiming this hunt reverified every bullet. Prior hunt m89/m92 records contain no matching project finding.

Governing contract: ADR032's Project/Environment placement and lifecycle guarantees; ADR008 dependable hosting. [Render Projects docs](https://render.com/docs/projects) describe individual moves and listing a project's own resources. No fresh authenticated Render timing experiment was run. Bex's documented retain-services delete behavior differs from Render and passed here; do not change it in this fix. REST/GQL/MCP schemas and authoritative placement stay unchanged. The recorded REST source Project read returned 200 but does **not** expose serviceIds, so it is not an independent membership proof; the complete GraphQL read below is that proof.

**Unverified:** live Postgres/Key Value and static/cron/worker/private project moves; Overview move rendering; post-mutation refresh rejection; other shared policy callers; authenticated Render timing; MCP membership reads. These are verification work, not observed bugs.

## Durable wire evidence

Authenticated browser POST `https://api.bex.co/graphql`, application/json, browser credentials included; no cookie/token is recorded. The following contains the complete captured mutation request/response, complete subsequent Projects request/response, and complete independent read. The two sources are sequenced as observed; cache publication ordering is traced above, not asserted from a timestamp absent in the capture.

```json
{
  "mutation": {
    "request": [
      {
        "operationName": "SetProjectServices",
        "variables": {
          "id": "prj-db31k5j6phbs73c99m10",
          "serviceIds": ["srv-db31ktr6phbs73c99m3g"]
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
    "body": [
      {
        "data": {
          "setProjectServices": {
            "__typename": "Project",
            "createdAt": "2026-10-07T10:08:22Z",
            "databaseIds": [],
            "id": "prj-db31k5j6phbs73c99m10",
            "keyValueIds": [],
            "name": "qa-20261007-r8-project-a",
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "serviceIds": ["srv-db31ktr6phbs73c99m3g"]
          }
        }
      }
    ]
  },
  "projectsRefresh": {
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
      }
    ],
    "status": 200,
    "body": [
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
              "createdAt": "2026-10-07T10:08:22Z",
              "databaseIds": [],
              "id": "prj-db31k5j6phbs73c99m10",
              "keyValueIds": [],
              "name": "qa-20261007-r8-project-a",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": ["srv-db31ktr6phbs73c99m3g"]
            },
            {
              "__typename": "Project",
              "createdAt": "2026-10-07T10:14:58Z",
              "databaseIds": [],
              "id": "prj-db31n8hppijc73ae6td0",
              "keyValueIds": [],
              "name": "qa-20261007-r8-project-b",
              "ownerId": "tea-d98210cbbpdc73dcrkvg",
              "serviceIds": []
            }
          ]
        }
      }
    ]
  },
  "independentRead": {
    "request": {
      "query": "query QAProjectMove{service(id:\"srv-db31ktr6phbs73c99m3g\"){id name phase revision environmentId projectId} source:project(id:\"prj-db31n8hppijc73ae6td0\"){id name serviceIds} target:project(id:\"prj-db31k5j6phbs73c99m10\"){id name serviceIds}}"
    },
    "status": 200,
    "response": {
      "data": {
        "service": {
          "environmentId": null,
          "id": "srv-db31ktr6phbs73c99m3g",
          "name": "qa-20261007-r8-web",
          "phase": "Running",
          "projectId": "prj-db31k5j6phbs73c99m10",
          "revision": "rev-1"
        },
        "source": {
          "id": "prj-db31n8hppijc73ae6td0",
          "name": "qa-20261007-r8-project-b",
          "serviceIds": []
        },
        "target": {
          "id": "prj-db31k5j6phbs73c99m10",
          "name": "qa-20261007-r8-project-a",
          "serviceIds": ["srv-db31ktr6phbs73c99m3g"]
        }
      }
    }
  }
}
```

## Evidence and cleanup

Verified local files: `.playwright-mcp/qa-20261007-pass8-stale-project-move.json`, `qa-20261007-pass8-stale-source-project.png` (viewed), `qa-20261007-pass8-stale-move-authoritative.json`, and `qa-20261007-pass8-stale-move-reload-control.json`, all under .playwright-mcp. They supplement the durable probe above.

All six owned logical resources (two projects, three environments, one web service) return REST404 after cleanup. Public fixture URL returns HTTP404. Final read-only inventory of 1,215 cluster objects, across namespaces including Secrets/middlewares, has no fixture name/ID; the normal terminating pod was waited to deletion. Only this sweep's Kratos session was revoked, and its jar removed. No product code was changed.
