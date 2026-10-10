# A one-sided CLI build filter is rejected before service create/update

**Severity:** major — an ordinary include-only or ignore-only setting cannot be created or updated through the supported CLI. A both-lists control succeeds.

**Attribution:** server compatibility failure rooted in a pinned upstream serializer/OpenAPI mismatch. Each independent CLI flag is accepted by the pinned command; its unused list becomes null. The literal pinned OpenAPI requires two arrays and does not allow null. Do not call that payload OpenAPI-valid or claim Render production accepts it. The proposed Bex correction is a focused server compatibility exception for the unchanged pinned CLI, with upstream drift documented. This is not a Bex launcher defect.

## Context and durable evidence

2026-10-09 UTC, production https://api.bex.co/v1/, QA workspace bex-canary (tea-daif693dqjvc73e7as3g), human device OAuth, isolated config, non-TTY/JSON. Published Bex v0.3.2 and unmodified Render v2.27.0, commit a764810a768202704e7206eb7b87a47211fcd98e, module v1.1.3-0.20260909214233-a764810a7682. Exact diagnosis snapshot: 30db645bf321627b083845528afd6e8265284c28. Deployed API revision unknown.

- [Actual CLI commands, stdout, stderr, exit and elapsed time](evidence/cli.json).
- [Actual same-pin loopback-intercepted requests](evidence/offline-requests.json): dummy API key, isolated mock endpoint; these are not original production packet captures.
- [Production POST replays of those exact bodies](evidence/create-replays.json): actual CLI User-Agent, selected workspace header, complete non-secret body/response, status and relevant headers.
- [API-only PATCH fragments and corrected-empty-array control](evidence/patch-diagnostics.json): reconstructed fragments, not intercepted CLI traffic.
- [Exact-source gate diagnostic](evidence/validator-probe.txt) and [probe source](evidence/validator-probe.go.txt): real request validator, matching templated PATCH route, mock terminal handlers. Four null cases fail expected acceptance; four empty-array controls reach the handler. This is not a shipped regression or full composed-server test.
- [Domain normalization diagnostic](evidence/domain-probe.txt) and [source](evidence/domain-probe.go.txt): the existing domain accepts nil unused slices and returns non-null arrays.
- [Runtime, unchanged settings and cleanup](evidence/control-and-cleanup.json).

## Reproduction

Use a private config with normal QA device login and explicit BEX_WORKSPACE. Persist fresh create intents and verify free capacity. No credentials belong in this record.

```sh
bex services create --name qa-20261009-829f64-opts --type web_service --runtime go --repo https://github.com/bex-co/bex --root-directory examples/hello-go --build-command 'go build -o app .' --start-command ./app --plan free --region frankfurt --env-var MESSAGE=qa-opts-829f64-雪 --auto-deploy=true --max-shutdown-delay 31 --build-filter-path 'examples/hello-go/**' --confirm -o json
```

Bex exits 1 in 3.365 s, stdout empty, full stderr:

```text
Error: received response code 400: invalid request body at /buildFilter/ignoredPaths
```

Fresh same-pin Render creates against Bex use the same free Go app, auto-deploy false, no env marker. Include-only qa-20261009-337383-filter exits 1 in 0.644 s with the same stderr. Ignore-only qa-20261009-775e64-filter exits 1 in 0.534 s with empty stdout and:

```text
Error: received response code 400: invalid request body at /buildFilter/paths
```

The full commands are retained in cli.json. All failed intents remain absent from lists. Exact offline-captured POST payload replays fail HTTP 400 in 0.592/0.568 s; the include-only complete response is:

```json
{
  "error": "invalid request body at /buildFilter/ignoredPaths",
  "id": "bad_request",
  "message": "invalid request body at /buildFilter/ignoredPaths"
}
```

The ignore-only response substitutes /buildFilter/paths in both strings. See create-replays.json for exact bodies, trailing newline, content type and CF-Ray values. No original production packet capture was made.

The both-lists create adds --build-filter-path 'examples/hello-go/\*_' and --build-filter-ignored-path '_.md': exit 0 in 1.156 s, ID srv-db49ak3jrdls73co04p0, name qa-20261009-689277-filter. Readback has both arrays, autoDeploy yes and maxShutdownDelaySeconds 31. The Go service eventually serves HTTP 200 with qa-filter-689277-雪.

On that proven-owned fixture:

```sh
bex services update srv-db49ak3jrdls73co04p0 --build-filter-path 'examples/hello-go/*.go' --confirm -o json
bex services update srv-db49ak3jrdls73co04p0 --build-filter-ignored-path 'docs/**' --confirm -o json
```

Both exit 1, stdout empty, in 2.319/1.312 s, with the corresponding unused-list pointer error above. Readback retains the original two arrays. API-only null PATCH fragments fail 400 in 0.903/1.074 s. Changing only the unused list to [] succeeds 200 in 1.364 s, with that empty list in readback. A native CLI both-lists update succeeds, exit 0 in 2.079 s. The repaired API payload diagnoses the boundary; it is not the desired CLI fix.

Adjacent controls pass: --auto-deploy=false --max-shutdown-delay 1 persists both explicit values. A combined rename and invalid shutdown delay 0 gets the named 1–300 refusal and preserves the old name/valid delay. These are not additional findings.

## Producer, gate and consumer

Pinned upstream paths are relative to the exact module above:

1. cmd/servicecreate.go:87–88 and cmd/serviceupdate.go:89–90 define independent repeatable flags. pkg/service/create.go:64–66 and pkg/service/update.go:57–59 use the same buildFilterFromInputs.
2. pkg/service/create.go:364–371 copies each list with append([]string(nil), values...). An unused list stays nil. pkg/client/types_gen.go:1878–1881 emits both fields without omitempty, making it JSON null. The loopback capture proves the complete serialization independently.
3. lego/backend/internal/api/openapi/render-public-api-1.json at /components/schemas/buildFilter requires paths and ignoredPaths as non-null arrays of strings. Exactly three refs point there: service response, servicePOST and servicePATCH. Mutating the component globally would change the response contract too.
4. lego/backend/internal/api/render_openapi.go:257–279,311–362 loads the schema and applies compatibility allowances. applyRenderSchemaCompatibility at 383–401 handles overlapping service-details branches, not filter nulls. The wrapper at 572–578 calls validateRenderRequest (602–624), returns this 400 and stops before the handler.
5. REST create threads BuildFilter at apps/rest.go:566; update at 1000 feeds the shared patch pipeline. apps/service.go:886–926 already decodes nil slices, validates globs through normalizeBuildFilter and projects non-null arrays. Two direct normalization callers: specFromCreate (2867) and checkBuildFilter (settings_check.go:158–162). The latter serves SetBuildFilter (service.go:3999–4009) and the patch step (settings.go:282–295). No domain conversion or body rewrite is needed.
6. apps/webhook.go:962–985 treats empty includes as all paths; ignored globs win. Updates replace the whole filter: a one-sided update should clear the unused list, not merge it with stored globs. Omitted top-level BuildFilter preserves existing update state; all-empty filters clear to nil.

## Proposed correction, blast radius and regression checks

Add a named compatibility overlay for only create-service and update-service: allow JSON null for either present buildFilter list as the pinned CLI empty-slice encoding. Clone the operation-local buildFilter object and both list schema refs/values before adding nullable; keep the embedded artifact and shared response component unchanged. Preserve required field presence, string item types, unrelated constraints and exact request body bytes. Existing domain normalization produces [] on readback. Avoid a CLI fork, global null-array allowance or merging with stored lists.

Two upstream builders and two REST operations are affected. The five repo-backed App kinds are web/private/worker/cron/static; only free web was exercised live. Image-backed filters retain the domain refusal. Postgres/Key Value have no filter field. GraphQL create/set/update, MCP create/update and dashboard calls share the neutral type/domain while bypassing this REST schema wrapper; no equivalent live defect was observed there. Verify their response arrays and filter semantics. Blueprint decode shares the neutral type; preserve its grammar and webhook/rootDir composition.

Regression acceptance must cover actual Bex and same-pin Render include-only/ignore-only create/update, both-lists controls, replacement/omission/all-empty semantics, malformed list/item 400s, operation isolation and unchanged shared response schema. A real owned free fixture must serve its marker, then disappear from detail/list/runtime. Keep local five-kind/domain coverage distinct from production evidence. Relevant full backend checks and lint must pass.

## Dedupe, history and unverified behavior

Open/done board searches covered buildFilter plus null/ignoredPaths, both pointer errors and flag names. Targeted git history for ignoredPaths in render_openapi.go and buildFilterFromInputs in lego found no fix on main. w1/done/m34 supplied domain/adapter/matcher support; this is a CLI-acceptance gap at the later strict schema gate. Its deliberate rootDir AND composition is unrelated. w7/done/m41 tests wizard arrays; w4/done/m152's top-level buildFilter:null is an unrelated read response. ADR018/CLI parity and DO_NOT_DO permit the correction.

Render production was not called: its acceptance of the serializer shape is unverified. The deployed API revision is unknown. CLI failures, exact replays and the exact-source probe support the gate mechanism without identifying a deployed commit. One-sided runtime/apply and Git webhook behavior remain untested because create is blocked; the successful two-list runtime proves only that control. No Git push or webhook injection occurred. Other service kinds and GraphQL/MCP/UI effects are source analysis, not live coverage.

## Cleanup and handoff

The control and recorded deploy children dep-db49ak3jrdls73co04pg and dep-db49cq3jrdls73co04tg were removed through the exact owned service ID. CLI delete passed, detail and former runtime returned 404, list contained the same six baseline IDs and quota returned services 6/25, terminating 0, Postgres 0/1, Key Value 0/1. Failed create/replay intents remain absent. No fixture survives this finding.

Scheduled through /pm: seven tasks, 2h20m. No product fix, commit or push was performed by this hunt.

## Additional affected journey — cloning an include-only source

Sweep 27, 2026-10-09 UTC, same released Bex/pin/workspace/human QA context. HEAD 7e72b8ee677260f5661aef9ccefd579c699b3c35 contains the original filing through an external shared-workspace commit; this hunt invoked no commit/push. Relevant API/filter sources remain byte-identical to the diagnosis snapshot. Deployed revision stays unknown.

[Complete actual journey and cleanup](evidence/clone-journey.json), [actual same-pin loopback clone capture](evidence/clone-offline-request.json), and [production exact-body replay](evidence/clone-replay.json). The loopback uses only a dummy API key and recorded own-source metadata; it is not the original production packet capture or an authenticated Render-production test.

The owned free Go source srv-db49tnjjrdls73co05k0 was created with both lists, then set to paths=[examples/hello-go/**], ignoredPaths=[] through an API-only setup PATCH. Native CLI rename changed its display name to qa-20261009-944f02-clone-src-雪. Readback preserved the include-only filter. Its deploy dep-db49tnjjrdls73co05kg went live and served HTTP 200 with qa-clone-src-944f02.

```sh
bex services create --from qa-20261009-944f02-clone-src-雪 --name qa-20261009-198179-clone --region frankfurt --env-var MESSAGE=qa-clone-198179 --confirm -o json
```

This ordinary clone exits 1 in 2.402 s, stdout empty, stderr exactly Error: received response code 400: invalid request body at /buildFilter/ignoredPaths followed by newline. The unique clone intent remains absent. The explicit region handles the existing documented clone-region residual; the env marker avoids claiming that the metadata-only upstream clone copies env values.

The unmodified same-pin Render CLI against the loopback fixture performs GET /v1/services with the encoded renamed-name filter and selected ownerId, then GET /v1/services/srv-db49tnjjrdls73co05k0, then POST /v1/services. Its captured body has buildFilter={paths:[examples/hello-go/**],ignoredPaths:null}, with the expected copied Go build/start/health/shutdown/free settings. An exact production replay returns HTTP 400 in 5.243 s, full response:

```json
{
  "error": "invalid request body at /buildFilter/ignoredPaths",
  "id": "bad_request",
  "message": "invalid request body at /buildFilter/ignoredPaths"
}
```

Fresh valid-name control qa-20261009-3a9739-clone-ok adds only a nonempty --build-filter-ignored-path '\*.md' override alongside its fresh name/marker. It exits 0 in 1.651 s, ID srv-db4a0g5lm2ps739a416g. Readback retains Go runtime, source/build/start commands, health path /, maxShutdownDelaySeconds 31, plan free, auto-deploy off and both filter arrays. Deploy dep-db4a0g5lm2ps739a4170 reaches live; runtime serves HTTP 200 with qa-clone-control-3a9739. This control verifies named lookup and copied configuration; its nonempty ignore list is a different filter configuration, not proof that the failed include-only clone works.

The additional producer is pkg/service/clone.go:127–133: extractCloneSourceDefaults copies only nonempty lists. mapSourceDefaultsToServiceInput at 77–78 copies onto nil slices; applyBuildFilterDefaults at 282–290 retains that nil unused list. The same existing BuildCreateRequest/buildFilterFromInputs then emits null. There are still two upstream builders and two REST operations, now explicitly covering three CLI paths: ordinary create, create --from, and update. The same scoped create-operation correction fixes this clone path; no separate implementation bug or milestone is needed. Ignore-only cloning was still source-only at sweep 27; the live mirror comparison below covers it.

After marker checks, delete the control clone first, enumerate source dependents (all this run's failed intents or already deleted clone), then delete the exact source ID. Both CLI deletes succeeded, both details and former URLs return 404, failed intents remain absent, and the final list/quota returns the six baseline services with terminating 0 and PG/KV 0/1. All recorded deploy children are removed with their owned services. No fixture survives.

Future acceptance also requires cloning a valid one-sided source by its renamed name without a partner filter flag; preserve the known region override and explicit env marker, and test include-only/ignore-only variants. This extends t002/t003's caller checks, with the existing seven tasks and 2h20m estimate unchanged. Earlier overlength control names were setup errors and were excluded from product conclusions.

## Live same-pin mirror comparison — ignore-only source

Sweep 28, 2026-10-09 UTC. [Complete actual Render/Bex journey and cleanup](evidence/clone-mirror-journey.json), [actual same-pin loopback capture](evidence/clone-mirror-offline.json), [exact production replay](evidence/clone-mirror-replay.json). The unmodified pinned Render CLI targets Bex with the same human QA authority in a separate private config; no Render-production account or endpoint is used. Its copied config was removed immediately after the two create comparisons without logging out the shared ongoing grant.

Native Bex creates owned free Go source srv-db4a6sdlm2ps739a41e0 (qa-20261009-a1756c-invsrc), then an API-only setup saves paths=[], ignoredPaths=[**/*.md]. Source deploy dep-db4a6sdlm2ps739a41eg goes live and serves HTTP 200 with qa-invsrc-a1756c. The same-pin Render command is:

```sh
render services create --from srv-db4a6sdlm2ps739a41e0 --name qa-20261009-913a90-mir-fail --region frankfurt --plan free --env-var MESSAGE=qa-mirror-913a90 --confirm -o json
```

It exits 1 in 0.947 s, stdout empty, complete stderr Error: received response code 400: invalid request body at /buildFilter/paths followed by newline. The failed intent is absent. The loopback capture of this actual pinned command with dummy auth and own-source metadata contains buildFilter={paths:null,ignoredPaths:[**/*.md]}. Its exact production replay returns HTTP 400 in 0.960 s:

```json
{
  "error": "invalid request body at /buildFilter/paths",
  "id": "bad_request",
  "message": "invalid request body at /buildFilter/paths"
}
```

Fresh Render control qa-20261009-b1e9c0-mir-ok adds --build-filter-path '**', exits 0 in 1.732 s and creates srv-db4a82bjrdls73co05tg. It reads back paths=[**], ignoredPaths=[**/*.md], copied native Go/build/start/health settings, maxShutdownDelaySeconds 23, plan free and auto-deploy off. Deploy dep-db4a82bjrdls73co05u0 becomes live and HTTP returns 200 with qa-mirror-b1e9c0. The include-all control confirms the create path and preserved configuration; Git webhook/filter execution remains untested.

This closes the source-only gap for the ignore-only clone branch and compares the unmodified client live against Bex. Original production packets were not intercepted; the full recorded wire is an explicitly labeled exact replay of the loopback capture. The same clone/default/builder/schema mechanism above applies, so no separate issue or implementation task is warranted.

Delete the control clone first and then the source after its only dependent is absent; both exact-ID CLI deletes succeed, details and former URLs return 404, final list/quota is the six unchanged baseline IDs with services 6/25, terminating 0 and PG/KV 0/1. Their recorded deploy children are gone. No fixture survives. HEAD during this sweep is 132ec2fe8160e410f38f14a0052dc12403c56de5, which adds the unrelated m52 store fix; relevant filter/API sources are unchanged and deployed API revision remains unknown.

## Sweep 29 production recheck after the source fix

Main now includes `b9c54e155436a717da79a9e93571de0e1cc679cc`, authored outside this QA hunt. The published Bex v0.3.2 one-sided update on a fresh owned free web fixture still exits 1 with the same `/buildFilter/ignoredPaths` 400. The filter remains absent after the refusal, and the fixture and deploy child were deleted with detail/list/runtime absence and quota back at baseline. See [the sanitized live recheck](evidence/live-after-source-fix.json). Production revision remains unobserved; the new implementation is not assessed as broken and t003 remains pending.

## Sweep 39 independent production recheck after rollout

Published Bex v0.3.2 and the unmodified same-pin Render v2.27.0 both pass fresh include-only and ignore-only Go web-service creates against Bex. For each client/filter pair, native updates switch to the opposite one-sided filter, both lists, and the original filter; readback always replaces the unused list with `[]`. An unrelated shutdown-delay update (first pair) or auto-deploy update (other pairs) preserves the filter. Invalid `[broken` globs return exit 1/HTTP 400 with the existing configuration and creation timestamp unchanged.

Each source is renamed, then cloned by its new display name using an explicit free plan, Frankfurt region, fresh harmless env marker, and auto-deploy off. All four clone commands succeed without a partner filter flag. Fresh readback confirms the copied filter, runtime, build/start commands, health path, shutdown delay, plan, and region. Identity-derived public/internal addresses differ as expected. All eight source/clone services reach marker HTTP 200, and cleanup's fresh deploy enumeration records Live children before deletion. See [complete commands, readbacks, runtime observations and cleanup](evidence/production-recheck-after-rollout.json).

All-empty clearing is explicitly **API-only**: `PATCH /v1/services/{id}` with `buildFilter:{paths:[],ignoredPaths:[]}` succeeds and removes the response filter; a subsequent native one-sided update restores it. An exploratory empty-flag command was refused locally by the pinned client and left state unchanged; it does not establish a native clear operation or a product defect. One clone deploy-list read had a TLS handshake timeout; a fresh read after a human-paced pause succeeded. Initial 404/503 observations converged to Live and matching runtime rather than being filed as failures.

Every exact owned fixture was deleted clone-first. Detail/list/former-runtime absence and the six immutable baseline IDs were verified; final quota is services 6/25, terminating 0, Postgres/Key Value 0/1. Delayed quota reconciliation used reads, never duplicate deletes. No fixture survives. The copied Render configuration was removed without logging out the ongoing Bex grant.

This fills the original live unmodified-Render and one-sided clone gaps independently of the external closeout. The actual deployed revision remains unobserved. Other service kinds, Git webhook/filter execution, GraphQL/MCP/dashboard behavior, and Render-production acceptance were not exercised in this sweep. No product code, commit, or push was performed by this hunt.
