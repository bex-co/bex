# m77 verification — 2026-09-05

The milestone board was stale: `1f1647b44` already shipped a partial implementation. This closeout completes and verifies that code rather than attributing all historical improvements to the final commit.

| Measurement | Original before `1f1647b44` | Starting partial implementation | Final |
| --- | --- | --- | --- |
| Driver: 9 representative parts, log opens | 9 | 1 | 1 |
| Driver: same fixture, log writes | 9 | 9 | 1 |
| Driver: same fixture, SSE encodes including eviction and 2 replays | 20 | 9 | 9 |
| Store: 96 parts, transactions/advisory locks/cursor queries | 96 each | 3 each, potentially concurrent/out of order | 3 each, serial writer |
| Store: same 96 parts, INSERTs | 96 | 96 | 96 |

The driver fixture covers start, text, reasoning, tool input, plan, diff, terminal output, and finish. It asserts literal live/replay bytes, record timestamps/turn/index, mode 0600, bounds, and redaction. `driver-before.json` retains the measured original output. PostgreSQL counters trace actual queries around the unchanged append allocator using batch sizes 1 and 32. Diagnostic local durations were 885.65 ms and 52.42 ms; timing is not a pass/fail threshold.

The gateway's deterministic gated-store test observes the first browser frame while PostgreSQL append is blocked and withholds `[DONE]` until terminal flush completes. This isolates forwarding from both model generation and Completer polling latency. A 100-part gateway fixture produces exactly 4 ordered append calls. Byte-budget, timer, cancellation, append-deadline, failure/harvest, concurrent replay, and quota tests pass under the race detector.

## Reproduce

- Driver: from `lego/agent-image/driver`, `node test/fixtures/hotpath-before.mjs` reconstructs and measures the historical source in temporary files; `npm test` checks the optimized path and `npm run build` checks compilation.
- Gateway: from `lego/backend`, `go test -race ./internal/sshgateway/agentattach -run 'Test(TranscriptBatcher|ForwardAgentTurnForwards|LiveSplice)' -count=5`.
- PostgreSQL: point `BEX_TEST_DB_URI` at a disposable database, then from `lego/backend` run `go test -race -p 1 ./internal/store ./internal/agentsessions -run '^(TestPGTranscript|TestCompleter)' -count=1 -v`. These tests migrate/create their fixtures; never use a production database.
- Whole backend: `go test ./...` and `BEX_TEST_DB_URI=... go test -p 1 ./...`.
- Backend lint: from `lego/operator`, `make lint-backend`.
- Consumers: from `dashboard`, `yarn test src/features/agent-sessions`; from `mobile`, `yarn test:unit`.
- Image: `docker build -f lego/agent-image/Dockerfile -t bex-agent-m77-final lego` from the repository root.

## Evidence and limits

All 103 driver tests and the driver build passed. Backend unit and real-PostgreSQL suites passed; backend lint reported zero issues. Gateway race fixtures passed five runs. PostgreSQL tests cover concurrent 7/32/96-part batch writers, rollback/retry, partial tee recovery, and exact-quota Completer completion; replacing the fix with the original Completer code makes the near-quota regression fail. Dashboard agent-session tests passed 242 tests across 22 files; all 342 mobile unit tests passed.

The image build validates exact package pins, executable permissions, installed versions, and binary ownership for Claude, Codex, and Gemini. Final local image digest: `5fd71756970779eb003d6dd4988a1f870e6fb9e83846133dd0d5f72895db37ab`. A network-disabled runtime probe verified all three executable/profile mappings. Shared invalid-profile fixtures exercise both Go and TypeScript validators; arbitrary profile/command/args/env overrides are rejected.

Simplify reuse, quality, and efficiency reviews completed. No harness package, extra bridge, sandbox provider, transport hop, or AI SDK major change was introduced. PostgreSQL remains transcript authority; the existing gateway credential boundary and bounded ADR051 log harvest remain intact.

Production model execution is deferred: this internal milestone's acceptance permits local fixtures and explicitly does not require a production model credential. These results prove deterministic stream/store/profile behavior and image construction, not a new production model E2E run. Existing dashboard/mobile wire consumers are unchanged and tested.

The legacy-gap regression starts with durable ordinals 0 and 2, harvests ordinal 1 once, and retains exact quota accounting. Historical sequence cursors remain unchanged (0, 2, 1 in allocation order); this repair does not renumber already-issued cursors. New live batch writers serialize their ordered parts, preventing that historical out-of-order allocation.

A repeated full-suite run against the reused local database failed four existing webhook replay tests because their fixed signing epochs had already been retired. Creating another database in that same cluster then exposed a retained cluster-wide gateway test role; the final full-suite run therefore uses an entirely fresh disposable PostgreSQL container with `-count=1`, matching CI. This was test-fixture state reuse, not an agent-transcript assertion failure.
