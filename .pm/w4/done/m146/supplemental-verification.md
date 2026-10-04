# Supplemental m146 verification — 2026-10-02

Upstream commit `683012552` landed the same canonical empty-environment correction while this session was verifying it. Its five-kind serialized regressions and validation-adapter matrix cover the original false update. Local duplicate production code and overlapping tests were removed after rebase to `c7773a005`.

The retained tests add the following independent checks:

- REST and MCP direct apply twice with unchanged ID/App and zero extra deploys; a real environment update keeps the service ID and opens one deploy.
- REST, GraphQL and MCP connected Blueprint create/preview/manual-sync aliases use a JSON-serialized App read and preserve the App across two syncs. The actual auto-sync worker consumes a queued intent and records completed sync history without App/redeploy writes.
- Real environment-group and service-secret handlers, backed by an in-memory secret store, keep a group-only service unchanged. Seed-only repeats retain a generated value and a user's later edit of a sync:false value. These checks strengthen the existing callback-fake coverage.

The JSON read wrapper exercises Get and List, matching the omitted-field representation consumed by apply and plan. Generation and serving state in these tests are seeded; unchanged resourceVersion guards against actual App writes. No live operator, Git provider, production API/UI or external HTTP acceptance is inferred from these fixtures. The milestone's existing t003/t006 hosted gates remain open.

Before-fix evidence remains in `/tmp/bex-m146-core-before.log`: the original helper produced envVars updates and two extra deploys/restart writes. The earlier full backend suite and full workspace lint passed before rebase. The final merged checks are recorded below when complete. Optional real Postgres/OpenFGA integrations were unset.

Three simplify perspectives completed. Reuse and quality found no required changes. Efficiency found unused transports; the retained adapter tests now initialize only their selected REST/GraphQL/MCP transport. The harness rejected a new third thread, so efficiency reused an existing agent after a usage-limit retry. The removed overlapping tests remain represented by upstream's stronger five-kind matrix.

Primary Render lifecycle/spec documents were rechecked today; ADR049 now states the explicit-empty environment ownership/serialization rule. Both existing dashboard plan consumers already count only non-noop actions. No new frontend code was needed.

## Final merged checks

Full backend `go test ./...`: PASS (`/tmp/bex-m146-supplement-backend.log`). Backend module lint: PASS, zero issues (`/tmp/bex-m146-supplement-backend-lint.log`). The three retained test groups compile and pass within that suite. Markdown formatting and diff whitespace checks are completed before shipping. The source patch for empty environments is already shipped in `683012552`; this commit adds test coverage and documentation.
