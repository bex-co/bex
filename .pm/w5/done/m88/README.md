# w5 · m88 — Correct agent-turn latency and outcome metrics

**Worker:** worker5 **Goal:** Make agent lifecycle timing and outcomes trustworthy across all terminal paths, without resetting clocks on unrelated session writes. **Status:** done

**Estimate:** 3h 30m implementation; 5h 15m including standing closing tasks (7 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Anchor lifecycle timing to durable turn timestamps — **DONE** | 60m | — |
| [t002](done/t002.md) | Record outcomes consistently across terminal paths — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Emit bounded lifecycle metrics from successful transitions — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Update Grafana queries and lifecycle timing definitions — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 60m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 15m | t006 |

Task IDs in depends_on are relative to w5/m88 unless written as a full wN/mN/tNNN ID. Resolve completed dependencies through done/ locations; the ID remains stable when the file moves.

## Definition of done

- [x] Use durable per-turn timing anchors instead of a mutable session updated_at. Pinning or unpinning during a turn cannot change the measured start or duration.
- [x] Completed, driver-failed, vendor-auth-rejected, canceled, dispatch-failed/recovered, and lost-sandbox turns expose appropriate bounded outcomes. A turn that never ran has no fabricated running duration.
- [x] Only the successful terminal transition owns the observation. Concurrent terminalizers and retried callbacks cannot double-count, and stale generations cannot record against a newer turn.
- [x] Prompt-less rehydration, archive/pin changes, and idle hibernation do not create phantom turns or extra terminal observations.
- [x] Metrics state their start/end boundaries and ordinary Prometheus scrape/process-reset limitations. Missing legacy timestamps are handled explicitly without manufactured historical precision.
- [x] Real-Postgres/concurrency regression coverage and a dev-5 metrics walk demonstrate the matrix, including pinning during a known-duration turn. Corrected panels are readable through the provisioned Grafana queries.
- [x] Affected backend, metrics/manifest, and documentation checks pass and the standing closing tasks are complete.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 2 for w5, materialized 2026-09-08. [w5/m81](../done/m81/README.md) explicitly records the model-auth fast-fail observation gap. Current completion.go measures from session UpdatedAt, while store.SetAgentSessionPinned rewrites updated_at on a running session. These are code-confirmed findings; the pin-timing effect has not yet been measured live.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 and [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) operational reliability: agent failures and slow turns must be observable with truthful, bounded platform signals.
- **Expected outcome:** Pin/unpin cannot shorten the reported turn duration; every terminal path contributes the appropriate outcome and latency, and Grafana shows the corrected definitions.
- **Why now:** [w5/m86](../done/m86/README.md) has made the existing lifecycle metrics operationally visible. Correct the source signals before operators rely on them as an SLO baseline. This is independent of the m87/m89 instance work.
- **Render parity:** Omitted: only internal lifecycle timing, Prometheus series, and platform Grafana/ADR documentation change. REST/GraphQL/MCP, tenant dashboard fields, and mobile semantics are outside this milestone. If implementation requires a tenant-facing contract change, add the parity closing task through pm before shipping.

## Scope and constraints

- Keep the m80 failure behavior, m85 dispatch recovery, and existing turn/generation custody intact; instrument their outcomes instead of reimplementing those mechanisms.
- Precise first-token tracing and provider-token accounting remain deferred. Do not add another model proxy or tenant billing surface.
- Reuse durable turn records where possible; add only the timing/outcome fields needed for this contract and define rolling-upgrade/legacy behavior.
- State the distinction between acceptance-to-terminal, running-to-terminal, and provisioning latency. A provisioning failure contributes an outcome and its measurable latency, not a made-up running duration.
- Keep session/workspace/repository/error text out of metric labels. This is operational telemetry, not a lossless billing ledger or a new telemetry outbox.
- An existing benchmarked warm-node measurement is historical evidence, not a production latency guarantee.

## Verification record

Shipped 2026-09-08 as `38e1082b1c86f3c571e4b985fe3df8c4dd3bad8f`:

- Durable `agent_session_turns.started_at` (migration 0107); terminal CAS returns `TerminalTurnFact`.
- Outcomes via `bex_agent_session_turn_outcomes_total{outcome}` and running-only `bex_agent_session_turn_duration_seconds{outcome}` (started_at → terminal); never-running failures omit duration samples.
- Paths covered: Completer complete/fail/lost, Cancel, Abandon/dispatch recovery, ModelAuthFailer (`vendor_auth_rejected`).
- Unit regressions: pin-during-turn duration, double-finalize once, never-running dispatch, cancel idempotency (`m88_test.go`).
- Grafana agent panels + ADR047/ADR088 updated; `scripts/obs-coverage-check.sh` PASS.
- Suites: `go test ./internal/{store,agentsessions,agentsession,sshgateway/modelproxy}/ ./cmd/api/`.
- Limitation: live dev-5 registry/panel walk deferred (kubectl/dev-5 unavailable at closeout); contract proven in unit tests including pin-timing.
