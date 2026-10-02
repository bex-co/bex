# w2/m165 acceptance — 2026-10-02

The environment selector defect is repaired and live-verified against the rebuilt local `dev-2` API (`http://localhost:54020`). Production was not changed or monitored during acceptance. The user's `$loopx w2` request authorized this implementation and its subsequent ship; the earlier QA filing itself authorized neither.

## Live CLI and identity evidence

- Current checkout Bex and unmodified Render v2.27.0 at `a764810a768202704e7206eb7b87a47211fcd98e` both passed the discovered-ID, name, and no-environment create/get/list journeys for Postgres and Key Value.
- The installed Bex on this machine was `vdev-140f1ef06`, embedding Render v2.26.0 at `6c0f561f8af9`, rather than the binary described in the original finding. It passed the same primary matrix; a separately built, unmodified upstream at that exact installed pin also passed ID/name read controls.
- **113 successful CLI commands**, **10 expected rejections**. Successful controls include `kv`/`pg` aliases and raw `services --environment-ids` filters with both public and legacy spellings. REST, GraphQL, and MCP return `evm-*` from both input spellings; actual name filtering retains name semantics.
- SQL IDs and Kubernetes App/Database/KeyValue labels remain `env-*`; no existing environment or resource was recreated. Anonymous reads return 401 before lookup, foreign-workspace aliases 403, and authenticated malformed/unknown IDs 404. Wrong-project name selectors and ID list filters refuse or return empty as appropriate.
- **Cleanup verified:** 20 datastores, five environments, five projects, two disposable accounts, their tenant namespaces, and the generated OAuth client removed; detail reads returned 404 and resource lists were empty. Setup retries corrected local harness assumptions (GraphQL Project lacks `environmentIds`, session authentication for ACL-bearing cleanup, and the KV name length); those preliminary fixtures were also removed.

The complete sanitized command exits, timings, identities, state checks, and cleanup records are in [evidence.jsonl](evidence.jsonl). Binary hashes and counted results are in [summary.json](summary.json). Neither contains credentials.

## Durable identity and surface regressions

- `internal/id/environment_test.go`: pinned CLI grammar, round trips, strict suffix bounds, and non-ID/name preservation.
- `internal/api/environment_alias_pg_test.go`: real PostgreSQL legacy rows and App foreign keys, composed REST/GraphQL/MCP reads, project/resource references, both filter forms, protected datastores, unchanged CR labels, direct/project-inline/Blueprint creation families, rejection of a second alias row, and refusal to validate an imported collision.
- `internal/environments/aliases_test.go`: ID/name separation, legacy/canonical pagination on all surfaces, workspace index, rename/update/delete, anonymous/foreign/missing neighbors, and create-assignment binding.
- Existing composed datastore placement/admission matrices now distinguish durable IDs from public selectors. PG/KV regressions verify filtering and protected actions; environment-group tests verify create/clone/move, metadata, linked-service and workspace checks; event tests preserve stored audit IDs while exposing canonical references.
- Dashboard URL parser and component tests show saved legacy project and creation links selecting the intended environment; wrong-project/unknown selections stay rejected.

## Shared CLI caller census

At the exact current pin, `pkg/validate/id.go:41` recognizes `evm-*`, and `pkg/resolve/resolve.go:225–268` owns ID-vs-name resolution plus project/workspace binding. KV create (`pkg/keyvalue/create.go:50`), KV list (`cmd/kvlist.go:101`), and KV get/update/delete/suspend/resume (`pkg/keyvalue/resolve.go:25`) converge there. PG create/list (`pkg/postgres/service.go:129,222`) and resource verbs (`pkg/postgres/resolve.go:74`) do likewise. `cmd/kv.go` declares `kv`/`keyvalue`, and `cmd/pg.go` declares `pg` aliases. This covers the 14 canonical datastore command paths without forking or rewriting the CLI.

## Limits

- The live run proves selectors, API semantics, and resource metadata; it does not claim Postgres/Valkey data-plane readiness or connectivity.
- Canonical-ID plus a wrong-project CLI flag was not separately invoked live. The pinned resolver's `validateEnvironmentBelongsToProject` call and backend binding regressions cover that path; wrong-project name selectors and API ID filters were exercised live.
- All 14 mutation variants were not individually executed. Remaining shared callers are supported by the source census and backend regressions.
- A literal old `env-*` selector remains a name to unchanged upstream clients. Existing API links work, and rediscovery supplies the canonical `evm-*` selector.

## Simplify review

Three independent reuse, quality, and efficiency reviews completed. Applied the two useful cleanups: HTTP constants in the composed API regression, and one normalization of a cloned environment-group filter slice per request. The latter also preserves caller-owned selectors. No additional abstraction or CLI patch was introduced.

## Final validation

- Backend: `GOWORK=off go test -p 1 ./...` passed with dedicated PostgreSQL, OpenFGA, and OpenBao test services. Package execution was serialized because store tests truncate their shared fixture database. The first full run exposed one preexisting project-response assertion comparing the outward ID to a stored ID; it now asserts the public alias, and the complete suite passed on rerun.
- Dashboard: all **457 test files / 3,831 tests** passed; `yarn lint` passed typecheck, ESLint, and knip.
- CLI: `go test ./...` passed, including the pinned-client contracts.
- `make lint` passed all four Go modules and whole-program dead-code checks. After the final environment-group simplification and test adjustment, `make lint-backend` passed again with zero issues.
- `bash scripts/skill-layout-validate.sh`, repository Markdown formatting, and `git diff --check` passed. Operator code was unchanged; its envtest suite was outside this change's validation scope.

The shipping pull fast-forwarded cleanly to `a9407d21d`, including upstream dashboard environment-group and event API changes. The combined tree then passed the full dashboard suite and lint again, `go test -p 1 ./internal/api ./internal/events` against fresh dedicated test services, and backend lint. No merge repairs were needed.

Local verification is complete. Shipping ends at a successful push; this record does not claim production rollout or post-push CI results.
