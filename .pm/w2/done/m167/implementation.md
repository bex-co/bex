# Implementation and consumer audit — 2026-10-02

## Contract

`LogQuery.Search` carries all terms. Normalization copies the caller-owned slice, lowercases and deduplicates nonempty terms. Whitespace remains literal. No effective terms means no text filter. Go evaluates any term as a substring; Loki escapes every term before regex alternation. Text OR still composes with other filters using AND. No public GraphQL schema or CLI changes.

## Consumer census

| Consumer | Path | Verification |
| --- | --- | --- |
| REST history, subscribe, label values | `logs/rest.go` `parseLogParams` retains `v["text"]` | Adapter and subscription regressions; label discovery retains its explicit stream-index limitation |
| MCP list logs, label values | `logs/mcp.go` `logFilters.query` retains `Text` | MCP regression; same label-discovery limitation |
| GraphQL logs and label values | `logs/graphql.go` `logQueryFromArgs` wraps scalar | Existing scalar schema unchanged; generic regression suites |
| Dashboard | `dashboard/src/features/logs/api/logs.graphql` scalar `$text: String` | Source audit; no dashboard changes needed |
| PG/KV convenience endpoints | `api/server.go` shared datastore adapter wraps scalar | Composed API suite and existing datastore suites |
| Stored App/request/build and generic PG/KV | `logs/loki.go` escaped OR predicate and bounded scan | Literal/multi-term tests plus existing selector, cursor, deadline and instance tests |
| Pod App/predeploy and datastore fallback | `service.go` `filterAndCap`/`keep` | App/PG/KV multi-term regressions; predeploy shares the same filter |
| Supported App/build tails | `service.go` two producer keep sites | Multi-term follow regressions |
| Stored/followed build progress | `progress.go` both keep sites | Multi-term progress regression |
| Multi-resource pagination, equal timestamps, partial scan | Shared `queryLogPage`, cap and scan code unchanged | Multi-term continuation test plus existing paging/budget suite |

All five Go `keep` call sites consume a query normalized at the service boundary. `hasFilters` and `lokiScansLines` see the whole normalized set. Authorization runs before resolving App/Database/KeyValue selectors; no resource selector, instance translation, authorization or continuation logic changed. Unsupported datastore tails and structured live-tail filters retain named refusals.

## Render parity

Checked the official [List logs API](https://api-docs.render.com/reference/list-logs) and [CLI reference](https://render.com/docs/cli-reference#logs) on 2026-10-02: API text is an array and CLI text accepts comma-separated values. The reference currently documents v2.28.0; acceptance uses the repository pin v2.27.0, whose exact serializer and array builder are recorded in finding.md. The existing Bex ADR010 OR-within/AND-across contract is retained. Full regex support remains outside scope.

## Verification

Current logs and composed API suites passed after the model/adapters changed; the full logs suite passed again with the new regressions. Backend lint passed with zero issues. A Go overlay restoring first-only REST/MCP reductions failed nine App/Postgres/Key Value scenarios, demonstrating that the tests catch the original omission. Multi-resource equal-timestamp pages, partial-history continuation, and literal scalar GraphQL have additional behavioral coverage. Local CLI/transport acceptance passed all 17 cases against real isolated Loki and an owned stdout emitter; see evidence/local-acceptance.md for captures, cleanup and substrate limits. Three simplify reviewers completed reuse, quality and efficiency reviews. Removed duplicate builder normalization, reused REST-page decoding, and strengthened upper-time-bound and decoded-tail assertions. Final logs tests pass; the logs race suite also passed. Full backend integration checks are being completed before closeout.

## Extended acceptance and simplify

[Extended local-process acceptance](evidence/extended/acceptance.md) passed all 54 assertions against the rebuilt composed API and real Loki. It covers both Bex/Render term orders, REST/MCP arrays, scalar GraphQL, literal/empty/duplicate/AND controls, bidirectional multi-resource paging, time bounds, anonymous/unknown-resource refusals and existing label-index/tail limitations. Six production source hashes were unchanged after the final API build. Both owned App CRs, isolated envtest/API/emitter processes, uniquely labeled Loki container, all listener ports and private client credentials were cleaned up. Seeded metadata, fixed local identity and direct Loki ingestion are explicit runtime limits; no production rollout is claimed.

[The counted consumer matrix](consumer-matrix.md) separates runtime, automated and source-only coverage. Private overlays restoring the old REST/MCP first-term reductions fail the new adapter regressions for all three resource families. A separate type-compatible first-term model fails the pre-deploy/build/progress/datastore-source regressions. These are controlled mutation proofs, not a complete checkout/build of the old scalar model.

Three simplify reviews found no material production issue. Tests now reuse the existing REST page decoder, retain one stored-progress case, and combine continuation checks into the stronger multi-resource/two-order matrix with a both-term row. The pre-deploy text fixture uses a sufficient raw buffer rather than implying recovery beyond kubelet's bounded tail. Final focused text regressions passed after those changes. The later simplify cleanup removed the redundant private Loki-builder normalization while retaining normalization at the source boundary; no cached parallel state was introduced. This behavior-preserving cleanup followed the recorded runtime build and is covered by the subsequent logs tests and lint.

The source audit separately reproduced ignored instance filters in the live-only pre-deploy collector. It is tracked in w2/039, outside this text-array repair. Dashboard/scalar APIs retain their schemas, and full Unicode folding equivalence between Go substring matching and Loki's existing regex engine is not claimed by the ASCII/literal controls.

## Integration test isolation

The first complete backend run reused m166's PostgreSQL state and failed four replay-ledger tests because their fixed signing epochs had been retired by the prior run. A new database exposed a second test-fixture assumption: the SSH role is cluster-wide and still had grants in the older database. These failures are unrelated to log matching. Validation was restarted with a new owned PostgreSQL container while retaining the isolated OpenFGA/OpenBao support services. Running packages concurrently then exposed their documented shared-table resets (missing rows and a deadlock). The final retry resets only that owned PostgreSQL fixture and follows the actual CI command, `GOWORK=off go test -p 1 ./...`, which serializes integration packages as required by `.github/workflows/backend-test.yml`. No product code changed to mask these harness failures. The final result is recorded below after the run completes.

Final closeout verification (2026-10-02): detached `GOWORK=off go test -p 1 -json ./...` passed all 67 backend packages with tests, using fresh isolated Postgres, seeded OpenFGA, and OpenBao. Exit 0; owned containers and volumes removed. Optional external/live acceptance and preview-generation tests stayed skipped; no DB/FGA/OpenBao integration gate was skipped. Final backend lint reported zero issues. Local log acceptance passed 17 cases; logs race and post-simplify tests passed.
