# Static route URL boundaries — implementation evidence

Implemented 2026-10-02. Configured destinations are parsed once; wildcard substitution changes only the decoded path. Redirect serialization escapes captured filename delimiters. Rewrites use the decoded path without query or fragment and retain rooted object normalization. Configured query/fragment tokens remain literal; incoming queries are not forwarded.

## Verification

- HTTP handler regressions exercise both aliases and GET/HEAD, reserved characters, Unicode, encoded slash/dot, repeated encoding, actual special-character origin keys, complete redirect components and following the Location. Configured encoded/query-bearing rewrite paths resolve the intended bytes.
- Forty-eight malformed/unsafe direct-CR cases return 400 without a Location or destination fetch. A genuine local object miss remains 404. Existing object precedence, first-match, error/header policy, revision/cache isolation and custom-host resolver controls pass.
- Original-code Go overlays fail the new runtime regressions and 22 backend refusal cases. The final static-server suite passes (1.153s).
- REST, GraphQL service/server and MCP route writes/reads, Create and parsed Blueprint application preserve ordered configured strings. Refused Create/SetRoutes writes preserve existing state. Shared validation reaches all three SetRoutes adapters and both creation paths. Existing authorization and non-static controls pass.
- Dashboard validator/editor checks pass 28 tests. Full dashboard suite passes 464 files / 4,012 tests. Dashboard lint (typecheck, ESLint, unused-code) passes.
- Full operator make test, full backend go test ./..., and all-module make lint pass. DB/OpenFGA integration cases retain their environment gates; this run does not claim a production or authenticated Render replay.
- Three simplify reviews completed. Redirect serialization is computed once; module boundaries remain intact.

## Caller and parity audit

There is one matchRoutes call, one expandDest call, two normalizePath calls and two matchPattern consumers (routes and the header matcher fallback introduced by m150). Header selection stays on immutable requestPath. Both dashboard route bases retain the same editor. Existing resolver tests cover platform/custom hosts and publication-preserving snapshot refresh.

ADR029 and ADR018 record the contract and official Render documentation comparison. External redirects, named placeholders, query forwarding and implicit SPA policy remain outside this change. Local paths containing hostname-like text are object keys; actual external destinations are rejected.

## Remaining release/QA gate

The release pipeline must deploy static-server, bex-api and dashboard. QA must replay the README's fresh owned Free site, encoded Location browser checks, GET/HEAD object comparisons, aliases, stored UI/REST/GraphQL/MCP reads and resolver refresh. Delete the fixture, verify exact resource/Secret absence and revoke its session. No production resources or sessions were created in this implementation run. t006/t007 remain blocked.
