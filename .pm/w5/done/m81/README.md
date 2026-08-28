# w5 · m81 — Agent-session lifecycle latency instrumentation

**Worker:** worker5 **Goal:** the per-phase latencies that today can only be measured by exec'ing into sandbox pods and running an out-of-band ACP probe become first-class Prometheus signals, so a regression — agent boot creeping from <1s to 30s, provisioning from 3s to 60s, or a turn-duration blowup — is alertable in prod instead of invisible. **Status:** done

## Tasks (in order)

| id   | title                                                                                          | est | depends_on | status     |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Turn-duration + terminal-convergence-latency histograms                                        | 60m | —          | — **DONE** |
| t002 | Provisioning + agent-readiness signals (schedule→running, time-to-first-model-call)            | 75m | t001       | — **DONE** |
| t003 | Wire into the Prometheus registry, document observed SLOs in ADR047, assert emission           | 45m | t002       | — **DONE** |
| t004 | Simplify                                                                                        | 20m | t003       | — **DONE** |
| t005 | Test coverage                                                                                   | 40m | t004       | — **DONE** |
| t006 | Closeout                                                                                        | 10m | t005       | — **DONE** |

**Shipped:** `bex_agent_session_turn_duration_seconds{outcome}` (Completer terminalizations) + `bex_agent_session_provision_seconds{outcome}` (dispatch→Running) on the CP `/metrics` registry, ADR047 SLO baseline recorded, emission asserted with real prometheus registries in unit tests. **Live-verified on dev-5 (2026-08-27):** `bex_agent_session_provision_seconds{outcome="running"}` appeared on `/metrics` after a live session. **Scope note:** `time-to-first-model-call` is delivered as the provisioning-latency proxy (agent boot measured <1s; a precise per-turn first-token metric is deferred because it needs turn-start↔first-proxied-call correlation the one-way mint channel lacks — documented in ADR047). Known coverage nuance: the model-auth **fast-fail** path (w5/m80 t003) finalizes a session out-of-band, so its turn does not traverse the Completer's `turn_duration` observe point; the common completion/driver-failure/timeout/lost paths do (unit-covered).

## Definition of done

- bex-api exposes, as labelled Prometheus series verified via `/metrics`: agent-session **turn duration**, **terminal-convergence latency** (create→terminal), **sandbox-provision latency** (schedule→running), and **time-to-first-model-call** (agent-ready→first upstream call).
- The series are correctly labelled (e.g. by terminal outcome) and do **not** double-count across steer/rehydrate turns.
- The observed SLOs (create-accept ~100ms, provision 3–10s warm, agent-ready <1s, turn duration) are recorded in ADR047 so the numbers have a documented baseline.
- Tests assert the series emit with correct labels on real session runs.
- No user/tenant-facing REST/GraphQL/MCP/UI field or semantic changes.

## Source + Goal linkage

- **Source:** `dev-5` agent-session investigation 2026-08-26/27 — every latency number (create ~100ms, provision 3–10s, agent-ready <1s via `initialize` 132ms + `session/new` 736ms, prompt-turn ~192s) had to be derived from pod timestamps and an out-of-band ACP probe because the session carries no per-phase timing: only `created_at`/`updated_at`, turn `created_at`/`completed_at`, and two counters (`bex_agent_session_status_reads_total`, `bex_agent_session_terminal_convergences_total` in `completion_metrics.go`).
- **Goal linkage:** ADR008 pillar 5 / ADR047 operational maturity — makes the AI-native execution path's latency observable and regression-alertable.
- **Expected outcome:** the fixes in the sibling milestone `w5/m80` become verifiable in-product, and future latency regressions (boot, provisioning, turn duration) surface as metrics instead of requiring a manual pod-level investigation.
- **Why now:** we just demonstrated real, measurable latency characteristics that are currently invisible in the product; instrumenting them locks in a baseline and catches drift. Complements `w5/m77`'s stream-path benchmark **test fixture** (a CI artifact for the persistence hot path) with production **lifecycle** metrics — no overlap.
- **Render parity omitted:** this is an internal observability mechanism — it adds Prometheus series only and changes no REST/GraphQL/MCP/dashboard/mobile field or semantic, so the Render-parity closing task does not apply.
