# Implementation and verification — 2026-10-02

## Access-intent decision

An update omitting both fields preserves the resource. A present empty `ipAllowList` withdraws external publication; a nonempty list enables it for those sources. An explicit Bex `public` value wins in the same patch. Thus `public:true` with an empty list deliberately retains the legacy unrestricted resource layer, while environment rules still apply. Existing public/empty resources are not migrated. Internal access and HTTP/environment empty-layer predicates are unchanged.

Key Value applies this in its shared patch. The dedicated Postgres setter now delegates to the validated Postgres update path, so PATCH, dedicated PUT, GraphQL and MCP share the rule for both datastores. No imported CLI or proxy policy fork was introduced.

The final caller census additionally found both Blueprint datastore update sinks assigning rules without publication intent. They are included in this milestone, along with explicit-rule creation and export/adoption policy preservation. Exporting raw rules alone would disable a legacy public/empty endpoint or publish a private endpoint with retained rules after the new sync semantics; the export must represent effective external access. Omitted Bex-native create defaults stay private.

Both dashboard editors read public intent with their rules. They allow an unchanged-list save when doing so changes external access, including legacy public/empty resources; English/Chinese copy states what saving does without falsely labeling those legacy endpoints blocked. Service HTTP editing keeps empty-is-open behavior. ADR009, ADR021, MCP descriptions and the CLI checklist agree. Separate REST Postgres create-default/readback drift is filed as w2/038.

## Completed checks

- Backend access tests exercise all eight writer surfaces, omitted/empty/nonempty/explicit-public precedence across existing public/private and empty/nonempty resources, preview without writes, invalid CIDRs and authorization denial. All pass. Replacing only the three production implementation files with HEAD originals through a Go overlay makes the new tests fail for both datastores: every writer leaves external publication enabled after a clear, and private Postgres does not republish on later rules.
- Full backend `GOWORK=off go test ./...` passed (67 packages with tests), using isolated real Postgres 17, OpenFGA with the repository model, and OpenBao services. Focused datastore suites passed again after simplify test-only cleanup.
- Operator `make test` passed, including controller/envtest. Real verified-TLS proxy tests cover KV and PG primary/pooler/replica route admission, withdrawal, restoration, and direct backend preservation. IPv4/IPv6 rules and environment AND composition have route-predicate coverage. Controller tests retain internal endpoints and credential references. Proxy race tests and affected-package lint passed; the final PG test cleanup passed its focused rerun.
- Dashboard full `yarn test` passed: 458 files, 3,934 tests. `yarn typecheck` and `yarn lint` passed. All eight changed generated GraphQL types/documents match offline schema codegen. The final named-consumer test cleanup passed all 10 focused tests.
- Blueprint focused regressions passed for explicit-rule creation, public/private transitions, field absence, visible first-adoption normalization and repeated no-op plans. The shared generator preserves effective policy without changing resource entries, HTTP rules or inherited environment rules. Targeted apps lint passed; the final full backend rerun is pending. The earlier local CLI/protocol acceptance covers the eight direct API writers; Blueprint behavior is covered by these Go tests.
- All-module `make lint` passed, including whole-program dead-code analysis. Three simplify reviews completed: reused CIDR/schema helpers, removed the old fake MCP-shaped duplicate, and removed test-case ordering assumptions. No additional production abstraction was needed.

## Runtime acceptance

[Sanitized local acceptance](evidence/acceptance.md) passed with installed Bex v0.2.1 and unchanged pinned Render v2.27.0: 20 CLI commands and 122 real protocol checks, including 48 external denials and 14 internal successes, with zero timeouts. All eight API writers and omission/preview/invalid/anonymous/override/environment controls passed. Passwords and CA stayed valid across transitions. Owned records, route variants, Docker containers/network/volumes, processes, keys and sessions were cleaned up; the failed dev-2 startup was also removed.

The shared local cluster could not provision CNPG because its webhook and control plane were unhealthy. The fallback used real envtest storage, the composed API, production proxy binaries and real TLS Postgres/Valkey/PgBouncer peers, with seeded CR status/credentials and a test identity. Replica routing targeted the same Postgres peer; internal traffic used the private Docker network. Managed provisioning, production OAuth/OpenFGA, physical replication and Kubernetes NetworkPolicy were not tested by this acceptance. Backend authorization, operator reconciliation and IPv6 route predicates have separate automated coverage. No production rollout is claimed.

Closeout revalidated 2026-10-02: current backend Key Value, Postgres and Blueprint suites passed; independent review found no material gaps in Blueprint intent or dashboard handling. The existing full-suite and local protocol evidence above remains the acceptance record; no production rollout is claimed.
