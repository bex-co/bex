# Project rename can retain the previous heading and title
Why: a successful rename can leave the ready project page identifying the project by its previous name while its breadcrumb and API show the saved name.

Severity: **minor**. The name persisted correctly and the owned static site's placement/runtime stayed correct. This is a frontend freshness regression of the rename-title guarantee in w2/done/m54/t003, not a failed rename or a backend snapshot-isolation defect.

Source: continuous `$qa-find-bugs w4`, cycle 6, **2026-10-08 America/Los_Angeles / 2026-10-09 UTC**, credentials consumed privately from muse.env. Research main **6c50f19c3**. Workspace bex (`tea-d98210cbbpdc73dcrkvg`); only this run's Free qa-prefixed project/environments/static site were changed. No product fix is included in this filing.

## Repeatable journey and observations

1. Create an owned qa-prefixed project and environment and a Free static site from `https://github.com/bex-co/bex`, main, root `examples/static-site`, publish `.`, auto-deploy off. Wait for Running/Live and check its public URL.
2. Fresh-load `/project/<owned-id>?env=<owned-environment-id>`. As soon as the heading's Edit control is available, rename the project before the sidebar's Projects read has finished. Do not delay, mock, intercept or reorder requests in production.
3. Wait for the successful save and then for **Manage resources** and the real resource table, rather than treating the pending skeleton as a failure. Compare heading, breadcrumb, sidebar and browser title to a narrow direct API read.
4. Repeat after a fresh load with another distinct name. This run reproduced r2→r3 and r3→r4. Timing depends on a list read overlapping the mutation; the ordinary settled-page rename is a passing control.
5. Reload repairs the name. No claim is made that the stale state survives every later navigation or mutation.

Expected: once rename refresh settles, the heading, breadcrumb/sidebar and title identify the authoritative saved project name; the title keeps its existing `<name> ・ bex Dashboard` format.

Observed terminal state, after the environment table was ready:

```json
{
  "heading": "qa-20261008-loop-a6-project-r3",
  "breadcrumb": "qa-20261008-loop-a6-project-r4",
  "title": "qa-20261008-loop-a6-project-r3 ・ bex Dashboard"
}
```

The pre-save Projects read started at **918 ms**, rename started at **1272 ms** and acknowledged r4 at **1520 ms**, then Projects completed at **1533 ms** carrying r3. At five seconds the heading, breadcrumb and title all still showed r3; later polling corrected the breadcrumb/sidebar to r4 but the loader's heading/title remained r3. These are natural browser timings, not a test transport delay.

Passing control: rename on an already settled page returned r2, and the heading, breadcrumb and title all became r2. Fresh-load checks also agreed with the persisted name. These controls support an ordering-sensitive bug rather than a universal inability to rename.

## Durable API evidence

The following is the **complete HTTP request body and response body** of the successful UI mutation, POST `https://api.bex.co/graphql`; no cookies or authorization headers are included:

```json
{
  "request": [
    {
      "operationName": "RenameProject",
      "variables": {
        "id": "prj-db45d93fuh0c73ao9fcg",
        "name": "qa-20261008-loop-a6-project-r4"
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation RenameProject($id: String!, $name: String!) {\n  renameProject(id: $id, name: $name) {\n    ...ProjectFields\n    __typename\n  }\n}\n\nfragment ProjectFields on Project {\n  id\n  name\n  ownerId\n  createdAt\n  serviceIds\n  databaseIds\n  keyValueIds\n  __typename\n}"
    }
  ],
  "status": 200,
  "response": [
    {
      "data": {
        "renameProject": {
          "__typename": "Project",
          "createdAt": "2026-10-09T02:51:16Z",
          "databaseIds": [],
          "id": "prj-db45d93fuh0c73ao9fcg",
          "keyValueIds": [],
          "name": "qa-20261008-loop-a6-project-r4",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "serviceIds": [
            "srv-db45dmpgovas7388mfd0"
          ]
        }
      }
    }
  ]
}
```

Independent narrow POST `https://api.bex.co/graphql` after the page's resource table had settled, complete request and response:

```json
{
  "request": {
    "query": "query QAA6SettledRename{project(id:\"prj-db45d93fuh0c73ao9fcg\"){id name} service(id:\"srv-db45dmpgovas7388mfd0\"){id projectId environmentId}}"
  },
  "status": 200,
  "response": {
    "data": {
      "project": {
        "id": "prj-db45d93fuh0c73ao9fcg",
        "name": "qa-20261008-loop-a6-project-r4"
      },
      "service": {
        "environmentId": "evm-db45debfuh0c73ao9fdg",
        "id": "srv-db45dmpgovas7388mfd0",
        "projectId": "prj-db45d93fuh0c73ao9fcg"
      }
    }
  }
}
```

The following is the **Projects operation excerpt** extracted from a three-operation HTTP batch (`Projects`, `BillingReadiness`, `Workspaces`). It contains the complete Projects operation request/result, but is **not** the entire HTTP batch. The unrelated two operations are omitted; their omission is not a missing-field defect.

```json
{
  "startedMs": 918,
  "completedMs": 1533,
  "batchedOperationNames": [
    "Projects",
    "BillingReadiness",
    "Workspaces"
  ],
  "operationRequest": {
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
  "operationResponse": {
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
          "serviceIds": [
            "srv-d9bj8s3eg85c7390eb9g"
          ]
        },
        {
          "__typename": "Project",
          "createdAt": "2026-08-09T23:06:05Z",
          "databaseIds": [],
          "id": "prj-d9sgfnbjghus73cg6hg0",
          "keyValueIds": [],
          "name": "forums",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "serviceIds": [
            "srv-d9nqg9dcavls73fp8m2g"
          ]
        },
        {
          "__typename": "Project",
          "createdAt": "2026-10-09T02:51:16Z",
          "databaseIds": [],
          "id": "prj-db45d93fuh0c73ao9fcg",
          "keyValueIds": [],
          "name": "qa-20261008-loop-a6-project-r3",
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "serviceIds": [
            "srv-db45dmpgovas7388mfd0"
          ]
        }
      ]
    }
  },
  "status": 200
}
```

Local supporting artifacts, existence checked and screenshot viewed:

- `.playwright-mcp/qa-project-a6-rename-settled.png` (86,393 bytes): ready resource table, sidebar r4, heading r3. This supports the settled mismatch.
- `.playwright-mcp/qa-project-a6-fast-rename-repeat.png` (46,223 bytes): five-second, still-pending resource content. This supports timing only, not the terminal-state claim.
- `.playwright-mcp/qa-project-blueprint-a6-ledger.json`: 29 complete safe captures, passing settled rename, both fresh reproductions, Blueprint/static journeys, cleanup and logout. These files are gitignored; the durable requests/results above carry the issue without them.
- MCP console inspection before logout reported **0 errors, 0 warnings**. Journey GraphQL and Blueprint calls returned 200. Cleanup's four intentional REST404s are expected. Post-logout whoami401 is expected. Locator timeouts and fetch-response-method mistakes in exploratory snippets were harness errors and are not findings.

## Root cause and actual framework path

- `dashboard/src/features/projects/hooks/use-rename-project.ts:17–26`: awaits RenameProject, toasts success and returns true; no project-list read settlement or post-write freshness barrier.
- `dashboard/src/routes/project.$projectId.index.tsx:104–125`: success closes the dialog and starts resource refetches/router invalidation; it does not drain or refresh Projects. Settings at `project.$projectId.settings.tsx:60–83` uses the same hook and invalidates directly. Only Overview's fast path was reproduced live.
- `dashboard/src/routes/project.$projectId.tsx:29–45`: the parent loader snapshots ProjectDocument and derives the route head from that snapshot. The overview reads that parent loader result. `common/lib/document-head/index.ts:148–151` deliberately selects cache-first for retained `stay` matches.
- `features/projects/hooks/use-projects.ts:73–97`: Projects uses the primed cache policy and optional polling. `dashboard-breadcrumbs.tsx:331–349` reads that watcher for the project label; `project-sidebar.tsx:36–38` reads its polling version. A later cache broadcast can update those labels without updating the already published loader/head.
- `features/projects/api/projects.graphql:1–39` selects name in list/by-ID/mutation documents. `common/apollo/cache.ts:6–12` has no Project normalization exception. The producer `lego/backend/internal/projects/service.go:508–531` persists then re-reads the renamed project; List at 375 and Get at 387 read the store. ProjectView.name is a string (175–177). The new name is expressible and was correctly returned; no backend/schema change is proposed.

Versions opened from the lockfile and installed code: **Apollo 4.1.3**, **React Router 1.170.25**, **router-core 1.171.21**.

- Apollo `core/QueryInfo.js:205–231` writes mutation data; `QueryInfo.js:100–145` also writes successful query data. There is no mutation-age fence between these separate operations.
- `QueryManager.js:622–637` sends incoming query data into markQueryResult/cache writes. `QueryManager.js:1052–1060` returns a complete cache-first result without consulting the network.
- `ObservableQuery.js:366–396` forces network-only for refetch, but `QueryManager.js:513–515,558–572` deduplicates matching in-flight query/variables unless context overrides it. Thus a bare refetch does not guarantee a new post-write request; it may wait for the pre-save snapshot.
- `ApolloClient.js:450–471` exposes each refetch promise on the returned `.results`; the combined promise uses Promise.all and may reject before all other reads settle. Any drain must await all individual results, including rejection cases.
- Router `src/router.ts:1668` assigns retained matches `stay`; invalidate at 2462–2535 invalidates then reloads. Invalidation alone does not impose network freshness on the application's cache-first loader.

The capture also predicts the initially old breadcrumb and its later repair: both consume the normalized Project cache, while heading/title retain a route snapshot. The passing settled rename has no late old list result to overwrite the mutation before publication. This is an application ordering gap, not a claim that Apollo or Router violates its contract.

## Bounded fix to implement

Repair **useRenameProject and its two callers**, preserving the shared title caching policy and existing API contract:

1. Perform exactly one RenameProject mutation. Distinguish mutation rejection from later read-refresh failures.
2. After acknowledgment, settle active Projects reads before publishing the retained Project loader. A concrete available seam is `client.refetchQueries({include:[ProjectsDocument]})`, then **Promise.allSettled of its .results**, so a reused pre-save in-flight read finishes writing before the final read. Awaiting only the aggregate rejection is insufficient.
3. Only after that drain, perform a fresh typed **ProjectDocument by ID, network-only**, with per-request `context: {queryDeduplication:false}` if needed to avoid reusing an older by-ID operation; await its cache write before the caller invalidates. Do not start this final read concurrently with the old Projects read. Audit other active writers in t002 and prove the chosen ordering fences them; change this bounded strategy if that audit reveals another pre-save writer.
4. Keep busy state until refresh finishes or fails. Before settlement, show the existing saving/pending state; do not announce an old ready label as the completed rename. On refresh rejection after a successful mutation, preserve the acknowledged save, offer honest refresh/retry feedback, and release busy state; do not report a failed rename or send a second mutation. A successful newer server read is authoritative even if another actor renamed the project again. Preserve the existing authorization/error route handling.
5. A plain additional router.invalidate, concurrent Project/Projects refresh, global network-only title policy, or global query-deduplication disable is not sufficient scope.

The mechanism above is researched, **not implemented or verified**. The real-cache regression must hold a pre-save list result past the mutation and past a faster by-ID read, and prove the repair still publishes the saved name after the delayed result has drained. Test refresh rejection and inactive list watchers; an assertion that refetch/invalidate was called is inadequate.

## Blast radius, aliases and neighbouring classes

- **Two production hook callers**: Project Overview and Project Settings, named above. Routes are `/project/$projectId` (including the `?env=` view) and `/project/$projectId/settings`; no separate legacy project-rename route was found in the file-route inventory.
- **Eight production useProjects calls**, excluding its definition/tests: Overview (`routes/index.tsx:150`); Project/Environment selector (`project-environment-selector.tsx:32`); move hook (:62); global search (:105); project sidebar (:36); Service, Datastore and Project breadcrumbs (:82, :223, :333). Also audit one-off ProjectsDocument reads in the Overview loader and `service-detail-loader.ts:37`. Environment-group scope reads use a separate scope query and are not included in this eight-call count.
- **Sixteen titleLoaderFetchPolicy calls in fifteen production files**, excluding the definition/tests: fourteen route files plus two calls in service-detail-loader. Keep its policy intact; preserve the prefetch/retained-navigation controls covered by w9/done/m62 and m68.
- Family boundaries: web, static, cron, worker and private service menus plus Postgres/Key Value placement menus can consume shared project labels. Only the static site's project page was live-probed here. This does not assert seven independent rename bugs or justify changing service/datastore rename hooks.
- Mutation forbidden/unauthenticated/conflict/validation/transport failure remains a rejected save with the existing reason; do not convert it to a completed rename. A post-write refresh timeout/503 is a saved mutation with failed refresh. Refresh 401/403 keeps existing authz handling and must not disclose otherwise hidden data. Deletion/not-found during refresh is handled by existing route state, not fabricated as an empty-named project. No new taxonomy or existence oracle.

## Dedupe and predecessor guarantee walk

Searched distinctive rename/stale/title/breadcrumb terms and useRenameProject across open, done and blocked board entries; scanned open milestone READMEs across workstreams and re-read DO_NOT_DO. Checked the latest 40 dashboard/lego commits and targeted useRenameProject history. No pending issue or already-landed rename freshness repair covers this writer.

- **w4/blocked/m178** is related, not duplicate: its scope is membership after SetProject* / cross-project moves. Its landed hook is useMoveToProject, with closeout still blocked. It does not refresh useRenameProject or own the late pre-save list race. Reuse its real Apollo/Router test setup, but do not reopen its tasks or claim this repro tested its move fix.
- **w4/done/134** covers accessible rename-field names; **w4/done/144** datastore breadcrumbs; **w4/done/212** environment-group breadcrumbs. These do not repair this mutation's ordering.
- **w2/done/m54** promised rename updates in t003/t009. This finding is a residual/regression of that guarantee; no introducing commit was established. Its complete DoD was walked as follows:

| Original m54 guarantee | This run's result / remaining scope |
| --- | --- |
| Checked-in route classification for all HTML/inherited/redirect/API/error routes | Existing route inventory read; no new route proposed. Full current classification audit unverified. |
| Initial authenticated SSR and client titles use loaded names | Narrow one-project SSR/direct-load controls agreed with r4; client fast rename fails. Full SSR/client route matrix unverified. |
| Static/resource/service-tab/project-settings hierarchy; opaque IDs only as fallbacks | Overview title format passes when settled. Other resource/tab/settings formats and error fallbacks unverified this run; preserve their existing tests. |
| Generic root description/OG/Twitter, origin/locale/viewport/favicon; no private-name leak | Narrow project metadata spot-check remained generic. Whole metadata/self-hosted/locale matrix unverified. No root metadata change proposed. |
| No new canonical/robots/social-image policy | No such policy change proposed; not a new QA finding. |
| Redirect/API no competing title; errors/not-found clear prior title; deterministic loading | Fast rename shows stale ready snapshot. Redirect/API/error/not-found matrix unverified; preserve existing head tests and saving-state semantics. |
| Typecheck/lint/tests/build and recorded live matrix | Product code unchanged; no dashboard suite/build rerun for this filing. Milestone tests and live replay are still required; this is not a full re-closeout of m54. |

w9/done/m89 and m92 were reviewed as live-evidence/observable-DoD precedent; their auth/agent findings are unrelated. No anti-goal applies.

Render documents project renaming in [Projects and Environments](https://render.com/docs/projects#modify-a-project) and project navigation through [dashboard breadcrumbs](https://render.com/docs/render-dashboard#navigate-the-dashboard). These establish the user-facing surface, not Render's request ordering. Authenticated Render race behaviour is **unverified**. The immediate target is bex's persisted saved name and its own m54 promise.

## Other journeys, cleanup and limits

Healthy this pass: public Git Blueprint preview (create/static/missing-file/invalid-path); owned static creation and serving; Generate Blueprint copy/download, no-op validation/apply without a second deploy; header update planned as update and served `X-QA-Blueprint: qa-a6-v1`; project/environment creation, environment rename, and moving the site between two owned environments. The static response's 1,548 bytes matched the local fixture SHA256 `1eb4041e1174aaf5fc0d19dc504fccca37749fd03e73974ed230433614d7e8bb`.

Not verified: Settings fast rename, mobile/zh, inactive/failed refresh and concurrent second-client rename; other resource title families; MCP rename; authenticated Render; Git-connected Blueprint sync. Preview manifests create unprefixed resources, so no such production deployment was submitted. REST applying the generated owned-resource manifest tested plan/apply, not a connected Blueprint's later Git sync.

Cleanup succeeded: service `srv-db45dmpgovas7388mfd0`, environments `evm-db45debfuh0c73ao9fdg` / `evm-db45lmjfuh0c73ao9fk0`, and project `prj-db45d93fuh0c73ao9fcg` deleted. Each logical REST read returned 404; workspace lists returned baseline **3 projects / 5 services**, and UI owned-label count was zero. Public `https://qa-20261008-loop-a6-static.onbex.co` returned HTTP404. The run's Kratos session was revoked, whoami returned 401, browser redirected to login, and only this run's cookie jar was removed. Cluster-object cleanup was not separately inspected. Nothing owned remains live.
