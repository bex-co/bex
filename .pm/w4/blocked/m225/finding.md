# A late detail read restores a service's previous name after a successful rename

**Severity:** minor functional display defect. The server retains the new label, and subsequent unmodified polling/reload recovers. No saved-label loss, wrong-target action, rollout or runtime change was observed. **Source:** functional qa-find-bugs w4, a879, 2026-10-17 UTC. **Research main:** 46084c87396de84b1c0ee1b94ce8abbb2691bb41. Filing only.

## Reproduction and observed result

Owned Free image web service **srv-db9le8ks300s73dskd3g**, created as **qa-20261017-a879-name**, image docker.io/library/node:24-alpine, port8080, workspace tea-d98210cbbpdc73dcrkvg. The sole deployment is **dep-db9le8ks300s73dskd40**. [evidence.json](evidence.json) contains the exact Node command, complete native rename transactions, complete independent narrow identity queries/responses, timed DOM projections and the executable browser probe.

1. Load `/services/srv-db9le8ks300s73dskd3g/settings` freshly. Edit Service Name and enter a new ordinary label.
2. In this browser only, hold one native Server(id) poll after fetching its actual successful upstream response. Do not change any response field. The held response contains the previous label.
3. Click the native Save changes button. Capture the HTTP200 SetDisplayName response and independently read the saved label through a narrow GraphQL request.
4. Release that already-obtained Server response unchanged. Capture the field, heading and title immediately, before the next ordinary poll can recover them.

Four successful replays establish the ordering. Times below are UTC on 2026-10-17; requests, errors and full narrow responses remain in the artifact.

| Replay | Old response held | Rename succeeded | Old response released | Observed field after release | Independent saved label |
| --- | --- | --- | --- | --- | --- |
| First mobile | 11:17:45.834 | 11:17:46.251 | 11:17:47.010 | Previous 100-code-point emoji label at 11:17:47.916 | qa-a879 after slow read |
| Fresh desktop | 11:19:11.006 | 11:19:11.448 | 11:19:12.070 | qa-a879 after slow read at 11:19:12.972 | qa-a879 desktop after read |
| Desktop screenshot replay | 11:21:13.354 | 11:21:13.756 | 11:21:14.410 | qa-a879 desktop after read at 11:21:15.314 | qa-a879 desktop captured |
| Fresh mobile screenshot replay | 11:23:34.854 | 11:23:35.210 | 11:23:35.837 | qa-a879 desktop captured at 11:23:36.740 | qa-a879 mobile captured |

The last two screenshots were captured inside the probe immediately after release and were viewed: `.playwright-mcp/qa-a879-stale-desktop.png` and `.playwright-mcp/qa-a879-stale-mobile.png`. Each shows the old name in both heading and Settings while its success toast names the new one. The mobile document remains390px wide. Their hashes and observations are durable in evidence.json; local PNGs are supplementary, gitignored artifacts.

The field shows the new label in the afterMutation snapshots before release, then returns to the old label. The heading can remain old throughout this short interval; the document title independently becomes new. Do not describe this as every surface first becoming new and then reverting. Different detail observers and the loader snapshot consume this state separately. The captured failure is the old field/heading beside a truthful new-name success, with the backend still new. Later unmodified reads recover: the first mobile page displays the new label at11:18:38.028, and the first desktop page at11:20:02.774. Screenshots named `qa-a879-mobile-after-read.png` and `qa-a879-desktop-after-read.png` were taken after recovery and are controls, not failure captures.

**Target:** once post-write publication finishes, render the name returned by an authoritative read made after relevant older metadata reads settle. A genuinely newer concurrent label wins. Do not make the API return the old label, permanently pin the submitted label over later edits, or wait for a routine poll/reload as the completion mechanism.

## Passing controls and exclusions

- Canceling a temporary draft and changing back to the exact original value produced zero SetDisplayName writes; the untouched Save button stayed disabled.
- Native save of padded `  qa-a879 Café 雪  ` sent the trimmed value `qa-a879 Café 雪` and succeeded. The generic row trims before sending; this does not independently prove backend handling of raw padding.
- Heading, Service Name, sidebar and title settled to that ordinary label. A same-document global-search cache had first been primed; searches by the new label, original name/slug and typed ID then found the same renamed service.
- On fresh390×844 Settings, `qa-a879-` followed by92 fish emoji is100 Unicode code points /192 UTF16 units. Save and an independent narrow read retained the exact value; the page remained390px wide. The home list link opened this service with the same label.
- A normal unheld clear at11:26:55.177 sent displayName="" and returned name="qa-20261017-a879-name". At11:26:57.413 the heading/field and independent read showed that original name. Exactly one original deploy remained, revisionrev-1, undeployedChangesfalse.
- Every sampled public runtime read returned the same markerqa-a879, pid1 and startedAt2026-10-17T11:08:20.765Z. Native name changes neither redeployed nor restarted the app.
- One harness lookup incorrectly assumed an aside element and timed out; the actual sidebar is a data-slot div. It was corrected and verified. It is not a product error. Later screenshot timing after normal recovery likewise is not a failure reproduction.

The network slice has336 nonstatic browser records after starting ID9000:311 HTTP200,12 HTTP204,12 failures (five aborted, seven connection-closed) and one record without a status. These transport records occurred amid navigation and local interception; their causes were not separately isolated. They are retained as observations, not an additional product finding. The four successful race receipts explicitly have HTTP200 responses and no captured GraphQL errors. Console errors:zero.

## Root cause and concrete fix

- `dashboard/src/features/services/hooks/use-display-name.ts:14–39`: after awaiting SetDisplayName, the hook immediately toasts success, returns true and clears busy. It has no ordering barrier for a pre-write detail read and no authoritative post-write publication.
- `components/display-name-row.tsx:29–49` derives the visible value from the fetched label/name and delegates to that hook. `routes/services.$serviceId.settings.tsx:84,110–115` observes Server without its own timer and calls `void router.invalidate()` after success. This does not await a fresh read after an older one. `lib/service-detail-loader.ts:87–101` uses the shared title fetch policy and separately warms Services; route invalidation is not a transport-order barrier.
- `hooks/use-server.ts:45–85` reads ServerDocument cache-first. `components/service-detail-layout.tsx:44` owns its normal visible poll; `common/lib/polling.ts` uses30s for settled resources. The field and header consume `toServiceView` (`lib/status.ts:34–43`), which prefers displayName. `components/service-detail-header.tsx:121` renders service.name. Correct conversion cannot recover freshness from an accepted old response.
- `components/editable-field-row.tsx:139–149,177–195` separates an editing draft from the persisted prop, seeds on Edit, restores on Cancel and closes after a successful save. There is no extra optimistic committed-name state to remove. Preserve its draft/focus semantics rather than changing this shared row to conceal old metadata.
- `api/display-name.graphql:4–9` returns Service/id/name/displayName. Server and Services select the same identity/label fields. `common/apollo/cache.ts` has no Service override; Apollo normalizes these records by typename/id. Generated ServerQuery and SetDisplayNameMutation labels are nullable strings; empty clearing remains a valid string. The publication fix fits the existing query/mutation contract.
- Actual deployed request extensions and the lockfile identify **Apollo Client4.3.3**. Primary package source inspected under `.playwright-mcp/qa-a720-apollo-source/package`: `cache/inmemory/helpers.js:5–20` supplies typename/id keys; `core/QueryInfo.js:92–132` writes an accepted query result to the cache; `core/ObservableQuery.js:395–431` makes refetch network-only; `core/QueryManager.js:515–588` can join an in-flight document/variables read unless context.queryDeduplication isfalse. Network-only alone does not establish response ordering. Source hashes/excerpts are in evidence.json. The locally installed4.1.3 package was not used as evidence of the deployed mechanism.
- Backend `lego/backend/internal/apps/service.go:4805–4864` authorizes, validates, writes the row projection and App.spec.DisplayName, preserving immutable identity. `apps/graphql.go:300–310,1484–1487` returns the display-label projection and wires the mutation. Both independent reads around release confirm it saved the intended label. No backend repair is indicated.

Implement publication in the service-name write path: await relevant outstanding same-resource metadata work, then issue a genuinely new Server read with network-only and queryDeduplication:false, and only then invalidate the retained route/head using that fresh cache result. Await each selected read independently with allSettled; an aggregate rejection must not let another old response arrive after the final read. Account for the workspace Services document as well as the captured Server document, because both select normalized Service labels. Existing active Services observers and the breadcrumb loader's one-shot warming reads need an explicit disposition in t002; do not claim a Server-only change covers them without the test.

Use `features/projects/hooks/use-rename-project.ts:25–48` and `services/hooks/use-cron-job.ts:49–75` as ordering references. The cron helper is private to its own writer; its pending-read filter and tests are not a proven service-name solution. Its integration test constructs a real Apollo client without the application's defaultOptions at `components/__tests__/cron-late-read-rollback.test.tsx:243–249`. New coverage must use actual4.3.3 plus `apolloDefaultOptions` (notifyOnNetworkStatusChange:false), a real cache and controlled transport with the real poll, not infer pending work from a spinner or copy the loading predicate untested. Preserve the existing cron/project behavior if any helper is extracted; no new cron failure is claimed here.

**Before settle:** retain the accepted editing draft/busy control or an honest refreshing presentation with the current row geometry; do not announce a completed ready state backed by the older label. Once the write has committed, a read failure is a publication failure, not a failed rename. Keep an honest warning and offer a read-only refresh retry; keep stale editing unavailable until publication succeeds. Never retry SetDisplayName merely to repair the read. Route/resource/workspace changes must not publish a previous resource's name or toast into the new resource.

Mutation validation/conflict/forbidden/unauthenticated/not-found/timeout keep their existing refusal behavior and draft. A post-commit transport timeout, GraphQL error, forbidden or unauthenticated refresh stays an explicit error/recovery state; a missing resource stays genuinely missing. Do not turn these into an empty label, success or an invented not-found. Existing session/sign-in and load-error handling owns those distinctions.

## Shared callers and aliases

Production-source census excluding tests/definitions/comments is durable in evidence.json:

- **One useDisplayName caller:** DisplayNameRow. **One production JSX mount:** ServiceSettingsPage. It serves web/private/worker/cron under `/services/:id/settings`; static Settings reuses it under `/static/:id/settings`. The `/web`, `/pserv`, `/worker`, `/cron` and `/static` compatibility entry paths redirect to canonical service trees through common/lib/render-alias.ts and service-detail-loader.ts. Postgres and Key Value have distinct rename hooks/rows and are outside the writer allowlist.
- **15 useServer consumers:** Settings, Env, Plan, Metrics, Scaling, Disk, Events, service sidebar, breadcrumbs, deploy list, static edge-rule page, env-groups panel, detail layout, nonstatic route guard and service shell page. Keep their loading/error/not-found distinctions, one intended polling owner and ordinary external-update behavior. The census is an audit requirement, not15 live failure claims.
- **Nine useServices consumers:** home, env-group list/detail, project overview, breadcrumbs, global search, push settings, webhook deliveries and Blueprint generator. Server and Services are the two query documents selecting Service.displayName. Services reads and breadcrumb warming can also write Service.name; an old list arrival was not independently delayed in this run and belongs in the compatibility test.
- SyncBlueprint returns Service/id/name in its mutation response (`features/blueprints/api/blueprints.graphql:236–258`, generated SyncBlueprintMutation), another source of actual authoritative name updates. Preserve those updates; it is not the old-read transport reproduced here. Usage's similarly named services node is quota counters, not Service metadata.

Prefer a local name-publication helper; a shared extraction is acceptable only with the cron/project caller audit and regressions. Do not change global cache normalization/default fetch policy, poll cadence, all generic setting writers, the name validator or the generic EditableFieldRow.

## Deduplication and original guarantees

Searched all9,685 PM Markdown files including done/blocked, indexed277 non-done READMEs, reviewed DO_NOT_DO, the distinctive SetDisplayName/use-display-name hits and the open rename/stale-read matches. Pulled main successfully; last40 dashboard/lego commits and targeted hook history/-S refetchQueries contain no repair. The hook's history is its original rename implementation; there is no current ordered service-name publisher. No matching open owner or already-landed deploy-lag fix was found.

This is an uncovered completion-ordering path, not evidence that an older milestone's full DoD regressed. The following complete guarantee dispositions prevent reopening unrelated work:

| Prior owner | Original guarantees and this pass's disposition |
| --- | --- |
| w8/done/m8 | Set/change and read displayName via REST/GraphQL/MCP independently of immutable identity, keeping k8s name, cert naming and subdomain fallback. GraphQL set/read/clear and identity/URL/no-rollout controls pass here; REST/MCP write paths and cert metadata during rename were not retested. The old DoD does not order overlapping dashboard responses. |
| w6/done/m101 | Next subscribed webhook uses the new label: untested, no webhook created. Backend renamed-app feed test: existing, not rerun. Every service_name query arm uses label fallback: unchanged, not newly audited. Immutable apps.name is preserved: the narrow identity control passes, direct SQL not run. This is a UI response-order gap, not proof that the webhook projection broke. |
| w8/done/m47 | Rename/create uniqueness and concurrent single winner: untested. CLI by-displayed-name resolution without a wrong target: untested. Historical duplicate audit/resolution: untested. Our ordinary labels and server writes succeed; no uniqueness or addressing defect is claimed. |
| w4/done/225 | Clear-name success copy/fallback. A normal clear returns the original name and preserves this behavior. The new issue is the successful read arriving late, not clear-copy text. |
| w4/done/147 | Search includes immutable name/slug and Blueprint indexing. The three owned service searches pass; Blueprint search was not retested. No search-index repair is requested. |
| w4/done/276 and w4/done/m186 | Cron sibling-field overwrite and project rename ordering have separate hooks/documents. They provide prior patterns; neither owns SetDisplayName. No cron/project live regression is claimed. |
| w4/blocked/281 and w4/blocked/293 | Auto-Deploy retains redundant local state; generic row cancellation loses focus. This row follows new props normally and has no redundant committed-value state. Preserve their fixes; do not add this finding to those unrelated owners. |

Other open matches w4/319 and w4/m203 concern stale log time windows, not rename reads. No anti-goal applies to truthful completion of this existing hosting control.

[Render's update-service reference](https://api-docs.render.com/reference/update-service) documents PATCH service.name. It does not specify authenticated dashboard arrival ordering; no Render account experiment was run. [Apollo's refetch documentation](https://www.apollographql.com/docs/react/data/refetching) explains selective refetch and individual query results; actual4.3.3 source establishes the dedup/cache mechanism here. Bex's immutable-name/displayName extension remains intentional. No platform/API contract expansion is needed.

## Evidence and cleanup

The delayed Server captures deliberately persist an explicit safe projection only; complete full-body recordings are not needed and can include connection-related fields. The original successful API response was fulfilled unchanged, as the probe source shows. Do not relabel the projection as a full response. Complete native SetDisplayName and complete independent narrow identity requests/responses are durable in evidence.json, alongside controls, source hashes/excerpts and cleanup receipts.

Native DeleteService returned true at11:27:05.865. At11:27:26.173 REST service, deploy list and original-deploy detail each returned404 and all seven inventory-ID sets matched baseline. At11:27:49.290815 exact App/Deployment/Service/Ingress/certificate names were absent, and exact app/build/publish/purge/predeploy pod/job label queries across all namespaces returnedzero. Public URL returned404 at11:29:08.092503. No broad cluster mutation or foreign-resource mutation was performed.

Only this pass's session was revoked; whoami401, cookieszero, one about:blank page at11:27:31.913, and owned logout handles removed. The QA account password was consumed privately by the existing login helper and is absent from the filing.
