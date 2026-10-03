# Static header wildcard matching — verification

## Scope and grammar

Both normal and resolved-error header emission use one header matcher. Route
capture matching and destination expansion retain their existing implementation.
The matcher uses bounded string scans with no regular expression, filesystem
interpretation, per-request compilation or new dependency.

Exact paths, global `/*` and literal subtree `/blog/*` retain existing behavior,
including the existing bare `/blog` match. A literal directory prefix followed by `*<literal suffix>` matches one
file segment (including root `/*.css` and `/assets/*.css`); nested `/**/*<literal suffix>` requires at least two slashes.
Thus `/*.css` excludes nested CSS, `/**/*.css` excludes root CSS, and `/**/*`
includes a trailing nested directory slash. Suffixes containing another star
or slash do not enter this grammar. Unsupported arrangements retain existing
literal/exact or literal-prefix behavior; question marks, brackets, braces,
backslashes and Unicode are not a new escaping or character-class language.

Patterns still receive the normalized original request path, independently of
which object a rewrite serves. Header order, last matching name, error-body
metadata exclusions, platform Retry-After and the existing API limits remain.
Header configuration still becomes visible on resolver refresh without a new
publish revision. This fix does not change route grammar or URL destination
semantics; the separate URL-path milestone remains m154.

## Evidence boundary

The official Render header pattern table was checked during implementation.
Its root/nested examples are the source contract; no authenticated Render
execution was performed. Local handler/origin/resolver tests are not claims of
hosted deployment.

## Runtime regressions and checks

Restoring the two original header consumers through a Go overlay reproduces
missing root/nested markers in served GET/HEAD success, route, error and resolver
refresh cases. Exact/global/subtree controls continue working. With the fix,
the full static-server package passes, including root/nested positive and
negative scopes, directory requests, original-request versus rewrite headers,
existing-file precedence, last matching header name, resolved 400/404/413/502/503,
body-metadata protections and platform Retry-After. Unknown-host and method
refusals remain headerless. Custom-host refresh and single/double-encoded slash
and Unicode checks preserve normalized request-path behavior. Four published
non-static App types remain unknown hosts with no origin reads.

`GOWORK=off GOFLAGS=-p=2 make test` passed, including code generation/envtest
(controller 93.319s; static-server 0.785s). The final static-server rerun after
fixture preallocation passed (0.746s). Matcher boundary/allocation checks pass;
a diagnostic 100-rule/2048-character benchmark measured approximately 62µs with
zero allocations on the local machine, not a hosted latency guarantee.

The first complete backend run encountered an unchanged metrics oversized-body
test returning an unexpected EOF from its local HTTP fixture. The exact test
passed ten repeats. Final metrics/full-backend outcomes are recorded below. All-module `make lint`
passed with zero issues across the four modules and successful dead-code analysis. No unrelated metrics code changed.

Three simplify reviews completed. The tests reuse the existing GraphQL schema
builder and serving fixtures; duplicate test-file documentation was removed.
The implementation introduces no resolver state, dependency or broad glob
framework.

## Remaining acceptance

The release pipeline must deploy the static-server. QA then repeats the saved
eight-row configuration, same-rule exact/wildcard toggle, settled GET/HEAD
root/nested success and 404 responses, rewrite remove/restore and request-path
controls. Fresh dashboard and all three API reads must preserve the patterns,
with no rebuild/revision change. Delete the owned static fixture, verify its
API/list and exact-identity artifact absence, then revoke only that QA session.
No hosted fixtures were created by this implementation run.

## Configuration and consumer verification

`static_header_pattern_contract_test.go` adds behavior checks at the existing backend
boundaries; no API/UI production code changed. The ordered fixture preserves
`/*.css`, `/**/*.css`, `/**/*`, `/blog/*`, `/*` and a literal Unicode/metacharacter
path, duplicate header names and value whitespace. Each adapter's write/read
result and persisted App agree, and header updates leave `spec.restartedAt`
unchanged. This verifies persistence, not an immediately refreshed public URL.

| Path | Evidence | Scope |
| --- | --- | --- |
| REST collection | `TestStaticHeaderLiteralOrderedContractAcrossAdapters/REST` | PUT and GET `/v1/services/site/headers`, exact ordered fields |
| GraphQL | `TestStaticHeaderLiteralOrderedContractAcrossAdapters/GraphQL` | `setStaticHeaders`, both `service` and legacy `server` read aliases |
| MCP | `TestStaticHeaderLiteralOrderedContractAcrossAdapters/MCP` | In-memory real protocol `update_static_headers` and `list_static_headers` |
| Create | `TestStaticHeaderWildcardPersistenceAcrossAdapters` | Shared `Service.Create` writes wildcard headers into App spec; not a separate transport create test |
| Blueprint | `TestStaticHeaderWildcardPersistenceAcrossAdapters` | `DeployStack` compiles a static manifest and persists its wildcard headers |
| Authorization/deletion/type refusals | `TestStaticHeaderAuthorizationAndDeletionPreserveRules` and upstream `TestStaticHeaderOperationsRejectOtherAppTypes` | Both Set/List refuse denied auth, terminating static resources and web/private/worker/cron types, preserving existing rules |
| Dashboard | Existing `static-site-section`, `use-static-site`, `static-rule-validation`, `non-static-route` suites | 4 files/17 tests pass (2.84s); editor persistence/read-back, validation and non-static UI restrictions. No new UI logic or snapshots |

Current source census: the old three `matchPattern` call sites become one
route call (`matchRoutes`) and one compatibility call inside
`matchHeaderPattern`; both header consumers call the new matcher.
`applyHeaders` still has two serving callers (redirect/object), and
`applyErrorHeaders` still has two (resolved error/busy). REST, GraphQL and MCP
remain the three `SetHeaders` callers. `headersFromViews` remains used by
create-spec preparation and `SetHeaders`; Blueprint compilation/projection
feeds that same stored header contract.

`ServiceHeadersPage` mounts one `HeadersEditor` keyed by service id. Both
`/services/$serviceId/headers` and `/static/$serviceId/headers` route families
reuse it and `useStaticSiteMutations.setHeaders`. The dashboard does not parse
or reinterpret wildcard syntax. These aliases were source-inspected; the
existing tests do not constitute a fresh browser visit to both routes.

Seven-family boundary: static Apps enter the resolver; its type gate excludes
web, private, worker and cron Apps, and the new refusal test exercises all four
API type gates. Database/Postgres and KeyValue/Valkey are distinct CR kinds,
controllers and serving protocols; they never enter the static App resolver.
No new database or KeyValue runtime probe is claimed.

The official [Render static-header table](https://render.com/docs/static-site-headers)
was read during this run. Its root `/*.css`, nested `/**/*.css`, nested-all
`/**/*`, global and subtree examples match the target grammar. It also says
paths apply across attached custom domains, names are case-insensitive and
values are sent as supplied. The documentation does not specify a full glob
language. Unsupported layouts retain their previous literal behavior; this
change does not claim wildcard classes, escaping or generalized globstar.
Authenticated Render execution remains unverified. ADR018 and ADR029 now
separate the prior persisted-but-unmatched gap from local fix evidence and
outstanding hosted acceptance.

The targeted backend static/header/deletion subset passes (0.685s). The first
full backend run passed Apps (6.077s), API (13.663s) and all other packages except
`TestPrometheusFilterValuesRejectsOversizedResponse`: its local HTTP stream ended
with `unexpected EOF` before the unchanged bounded reader could classify the
64 MiB overflow. The failing test then passed 10 repetitions, and the complete
metrics package passed 10 repetitions (2.314s). No metrics code changed; the
first failure remains recorded rather than treated as a passing gate. The full backend rerun passed, as recorded below.

Simplify efficiency review found no further changes: matching uses bounded
string scans without compilation or allocations; resolver/cache/update behavior
is unchanged; route and header compatibility share the existing matcher without
new adapters. Reuse review replaced duplicate GraphQL test-schema construction
with the existing `mustSchema` helper.

The full backend rerun completed successfully: `go test -p 2 ./... -count=1`
exited 0, including Apps (8.096s), API (16.803s) and metrics (0.655s). The first-run EOF failure and its focused repetition evidence remain recorded. Environment-gated external-service
tests retain their normal local skips; no hosted or real-Postgres claim is made.

## Concurrent implementation reconciliation

Commit `ccf6478f7` landed independently while this drain verified the same item.
Its implementation is retained, including direct-child suffix selectors such
as `/assets/*.css`; these do not cross another slash. The earlier local grammar
kept that form literal, so its boundary test was updated to protect the already
shipped behavior. Existing upstream scope/route/refresh and create/Blueprint
regressions remain; supplemental tests avoid duplicating those broad matrices.
After reconciliation, the complete static-server and Apps packages passed with both upstream and supplemental tests present. The
initial independent runs above remain historical evidence, not a claim that
all test files retained their pre-merge names or layout.

Final merged verification: static-server 1.099s, Apps 7.175s; all-module `make lint` completed successfully with zero issues and dead-code analysis.
