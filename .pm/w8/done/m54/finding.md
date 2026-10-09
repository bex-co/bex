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

~~~sh
bex services create --name qa-20261009-829f64-opts --type web_service --runtime go --repo https://github.com/bex-co/bex --root-directory examples/hello-go --build-command 'go build -o app .' --start-command ./app --plan free --region frankfurt --env-var MESSAGE=qa-opts-829f64-雪 --auto-deploy=true --max-shutdown-delay 31 --build-filter-path 'examples/hello-go/**' --confirm -o json
~~~

Bex exits 1 in 3.365 s, stdout empty, full stderr:

~~~text
Error: received response code 400: invalid request body at /buildFilter/ignoredPaths
~~~

Fresh same-pin Render creates against Bex use the same free Go app, auto-deploy false, no env marker. Include-only qa-20261009-337383-filter exits 1 in 0.644 s with the same stderr. Ignore-only qa-20261009-775e64-filter exits 1 in 0.534 s with empty stdout and:

~~~text
Error: received response code 400: invalid request body at /buildFilter/paths
~~~

The full commands are retained in cli.json. All failed intents remain absent from lists. Exact offline-captured POST payload replays fail HTTP 400 in 0.592/0.568 s; the include-only complete response is:

~~~json
{"error":"invalid request body at /buildFilter/ignoredPaths","id":"bad_request","message":"invalid request body at /buildFilter/ignoredPaths"}
~~~

The ignore-only response substitutes /buildFilter/paths in both strings. See create-replays.json for exact bodies, trailing newline, content type and CF-Ray values. No original production packet capture was made.

The both-lists create adds --build-filter-path 'examples/hello-go/**' and --build-filter-ignored-path '*.md': exit 0 in 1.156 s, ID srv-db49ak3jrdls73co04p0, name qa-20261009-689277-filter. Readback has both arrays, autoDeploy yes and maxShutdownDelaySeconds 31. The Go service eventually serves HTTP 200 with qa-filter-689277-雪.

On that proven-owned fixture:

~~~sh
bex services update srv-db49ak3jrdls73co04p0 --build-filter-path 'examples/hello-go/*.go' --confirm -o json
bex services update srv-db49ak3jrdls73co04p0 --build-filter-ignored-path 'docs/**' --confirm -o json
~~~

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
