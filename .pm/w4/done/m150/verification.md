# Static header patterns — local implementation evidence

Implemented 2026-10-02. A dedicated bounded string matcher serves the two header
consumers. The route caller and capture expansion retain their existing behavior.
Root file suffix selectors do not cross slashes; globstar selectors require a
nested path and also match trailing directory slashes. Exact/global/subtree
controls and last matching header name remain compatible. Other metacharacters
are literal; unsupported multi-star patterns retain the old exact or trailing
wildcard interpretation. No new dependency, regex, filesystem operation, origin
request, configuration field, or cache invalidation was introduced.

## Verification

- Restoring the original static server through a Go overlay makes the new root
  and nested response tests fail. The separate error-response overlay fails all
  22 new cases because selected headers are missing.
- GET/HEAD tests cover root/nested CSS and YAML files and misses, trailing
  directories, negative scopes, ordering, rewrites using the visitor path,
  existing object precedence, and unchanged route splat redirects.
- Resolved 400/404/413/502/503 cases retain selected headers, honest error body
  metadata and platform Retry-After. Existing unknown-host 404 and method 405
  controls stay headerless. Encoded Unicode filenames and literal metacharacters
  have positive/negative response checks.
- A real CachedResolver refresh test serves both platform and custom-domain
  aliases. The saved header becomes visible only after Refresh; revision,
  publication prefix and origin-fetch count remain unchanged.
- New backend tests preserve all five official pattern strings through REST
  PUT/GET, GraphQL set plus service/server reads, MCP update/list, shared create,
  and parsed Blueprint apply. Header-only writes retain RestartedAt. Existing
  API authorization, deleting-resource absence and dashboard tests also pass.
- `make test` in lego/operator passes, including generation and envtest.
  `go test ./...` in lego/backend passes. `make lint` passes for all four modules
  plus whole-program dead-code analysis. Final focused static-server tests pass.
  Existing dashboard static editor/page suites pass: four files, 17 tests.
- Three simplify reviews found no further useful change: header policy stays
  separate from route capture, shares the existing exact/subtree behavior, and
  introduces no per-request compilation or mutable matcher state.

## Consumers and scope

There remain three SetHeaders adapter callers and two headersFromViews writers
(create and setter), plus Blueprint projection. Two dashboard URL families share
one editor. At runtime two normal-header and two resolved-error call sites use
one header policy; only the single route caller uses matchPattern directly.
Static Apps alone enter this server. Web/private/worker/cron remain behind the
non-static gate; PostgreSQL and Key Value use separate services and protocols.
No backend production or dashboard source changed.

The [official Render header table](https://render.com/docs/static-site-headers)
was checked on 2026-10-02. Its root/nested examples define this implementation;
no authenticated Render execution is claimed. Existing bare `/blog` matching
for `/blog/*` is preserved and explicitly distinguished from documented subtree
examples in ADR029.

## Remaining gate

The release pipeline must deploy the changed static-server. QA must replay the
README's complete saved-pattern, settled GET/HEAD, exact-toggle, route-toggle,
fresh UI/API and deletion acceptance against a new owned Free fixture, then
verify exact artifact absence and revoke the session. No hosted fixture or
session was created in this implementation run. Tasks t003 and t006 retain
that deployed cross-surface acceptance; local implementation and checks are
complete. Do not infer production deployment from a successful push.
