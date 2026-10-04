# Static route URL semantics — implementation verification

## Runtime and configuration contract

The runtime parses a configured local destination before expanding its decoded
path. `:splat` and a trailing `/*` substitute only in that path. A redirect
serializes the resulting URL; a rewrite normalizes its decoded path for the
revision-scoped object key. Literal captured question marks, hashes, percent
signs, spaces and Unicode cannot become query/fragment syntax. Configured query
and fragment remain literal; placeholders there are not expanded. Incoming
redirect queries are not newly forwarded. Rewrites exclude URL query/fragment
from the object key and do not reparse captured filename data.

Raw, decoded and expanded local-target guards remain, with the final serialized
redirect guard retained at the response boundary. Backend create/update and UI
validation now refuse malformed path/query/fragment escapes and decoded unsafe
local paths. Direct CR input is independently checked at runtime and a matching
invalid rule returns 400 with request-path error headers. No external fetch or
redirect support was added. Named placeholders remain unsupported.

Configured destinations retain their 2,048-byte backend/runtime budget. Before
allocating repeated capture expansion, an 8 KiB decoded expanded-path cap
bounds amplification; invalid/over-budget matched rules return 400. Existing
request/object-key limits still return 404 for overlong keys. Original request
normalization, header matching, object-before-rule precedence, first rule,
implicit SPA behavior and cache/revision isolation remain unchanged.

## Regression and browser evidence

Real HTTP tests failed against the original code: both redirect aliases emitted
query/fragment delimiters from filenames, a captured percent produced a Location
the HTTP client could not parse, and accepted rewrite URLs returned 404 while
the direct destination served 200. The fixed complete static-server package
passed (1.119s), including GET/HEAD followed redirects to actual special-key
objects, request-path headers, configured components, one/double decoding,
encoded slash/dot/backslash cases, bounded expansion, and existing error,
precedence, cache and isolation controls.

A disposable loopback server used the actual static Handler with an in-memory
origin. Headless Chrome performed 12 real navigations: both redirect aliases
for filenames containing `?`, `#`, `%`, space and Unicode, plus query-bearing and
encoded rewrites. Every redirect landed on the expected encoded pathname with
empty query/fragment and served the actual special-key bytes. Both rewrites
served identical YAML bytes with 200 and retained their original browser URL.
The browser, exact owned listener and temporary Go fixture were removed.
Supplementary local results: `/tmp/bex-m154-browser-results.json`; no browser
errors. This is a real browser/local-handler test, not a hosted-site claim.

## Shared consumer census

There is one matchRoutes call and one expandDest call. normalizePath retains
two consumers: the original request path and the decoded rewrite path. Following
m150, matchPattern has two consumers (route matching and header compatibility),
not the finding's original three; both header emission paths use the dedicated
header matcher against the original request path. The local-target predicate
now guards four runtime boundaries: raw config, decoded config, expanded path,
and serialized Location.

SetRoutes still has three write adapters (REST, GraphQL, MCP); routesFromViews
and validateRoutes each retain create/set callers. New local API tests cover
ordered roundtrips across REST, GraphQL service/server and MCP, create and
Blueprint good/bad destinations, unchanged restart state, malformed refusals
across all three adapters and preserved saved rules. The dashboard shares one
RoutesEditor through /services and /static; validation tests preserve accepted
encoded and double-encoded paths and configured components. Canonical route
composition was source-inspected, not a new hosted browser visit.

Only static Apps enter this handler. Existing non-static resolver tests cover web/private/worker/cron refusals; the route API regression explicitly exercises web, with the shared static-only type gate source-audited for the other App types; Postgres and Key Value have separate CR
kinds/controllers/protocols. No live probes of those sibling services are
claimed. Authorization, unknown-host/method refusals and origin-failure classes
remain in the existing regression suites.

## Parity and remaining gate

Official Render redirect/rewrite documentation describes URL destinations,
wildcard substitution and existing-file precedence. Authenticated Render URL
edge cases were not exercised; bex retains its local-only destination policy,
path-only capture grammar and implicit SPA behavior. ADR018/029 record the
contract and evidence limit.

The release pipeline must deploy operator/static-server, backend and dashboard
validation. QA then replays all saved-row GET/HEAD and browser probes on an
owned Free site, compares fresh API/UI route reads after resolver refresh,
checks custom-domain/sibling controls where assigned, deletes only the owned
fixture, verifies exact API/resource/Secret absence and revokes its session.
No hosted fixture or QA session was created by this implementation run.

Three simplify reviews completed: existing parser/serializer and safety
predicates are reused, no extra runtime layer was introduced, and preflight
expansion remains bounded. Equivalent API/UI checks stay within their module
boundaries rather than adding a cross-module dependency.

## Final local checks

- Full operator `make test` passed, including codegen/envtest (controller
  133.562s; static-server 0.901s).
- Full backend `go test -p 2 ./...` passed (Apps 12.194s; API 25.988s).
- Full dashboard passed 466 files / 4,019 tests (140.95s).
- All-module Go lint/dead-code analysis passed with zero issues; dashboard
  typecheck, ESLint and knip passed.
- The final added existing-App Blueprint regression passed (0.600s): a valid
  encoded URL/query/fragment change persists, then malformed query-percent
  reapply is refused with the entire App spec and resource version unchanged.
  It adds no production change after the full-suite runs.

Local adapter checks use their normal fakes/in-memory protocol fixtures;
environment-gated backend tests retain their normal local skips. No new real
Postgres, hosted API or deployment claim is made.

## Concurrent implementation reconciliation

Commit `703068251` independently shipped the same URL component fix during our verification. Its runtime/API/UI implementation and stronger all-control-character guards are retained. Our addition bounds configured and expanded runtime path allocations; duplicate adapter/Recorder tests were consolidated while retaining real-wire/cache and existing-App Blueprint evidence. Prior full-suite results above describe the independently verified implementation; final merged checks follow.

Final merged verification passed with both upstream and supplemental tests: complete static-server package 0.931s, complete Apps package 8.173s, and all-module lint/dead-code analysis with zero issues. Handler-level expansion tests prove both redirect and rewrite refuse amplified paths before any destination or SPA fallback fetch.
