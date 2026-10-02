# m153 current implementation — 2026-10-02

This implementation supersedes the alternative archived `implementation.patch`; that patch and `verification.md` remain historical evidence and must not be applied over this change. Production acceptance is still open.

## Authority and transition table

One init container, using the service's pinned Valkey image and existing PVC, owns conversion before the network-serving container starts. A synced atomic `.bex-persistence-mode` marker identifies the committed disk format. The desired mode and a successful PING do not establish data authority.

| Source | Target | Authoritative input and completion |
| --- | --- | --- |
| Journal + Snapshot | Journal + Snapshot | Retain active journal files; validate an active manifest record or legacy AOF; retain committed marker. |
| Journal + Snapshot | Snapshot | Load journal privately, SAVE current dataset, shut down, commit snapshot marker. |
| Snapshot | Journal + Snapshot | Load RDB with AOF disabled, enable AOF, require enabled/finished/unscheduled/successful rewrite and write status, SAVE, shut down, commit journal marker. Never load stale AOF. |
| Snapshot | Snapshot | Retain initialized RDB and committed marker. |
| Either durable mode | Off | Commit destructive boundary first, remove only known engine files; no old data can resurrect after interruption. |
| Off | Either durable mode | Finish discarding prior files, initialize an empty durable engine, commit target. |
| Off | Off | Finish discarding engine files. |
| Legacy empty mode | Any | Normalize to Journal + Snapshot, subject to trustworthy bootstrap below. |
| Fresh storage | Any | Initialize before the first client serves; unfinished initialization can retry or reverse safely. |
| Unknown retained storage | Durable | Refuse startup and preserve recovery data; a valid on-volume marker overrides the unknown bootstrap. Explicit Off remains a destructive reset. |

Legacy adoption does not restart an unchanged fleet. At the next actual template change, bootstrap uses the exact owned running Pod through the uncached API reader, rejects a pending legacy rollout, and freezes its source annotation. It never treats the desired StatefulSet flags as applied. Retained PVCs without trustworthy evidence fail closed. The init helper has only a private mode-0600 Unix socket, no TCP listener and no credentials to disclose. Conversion failures preserve the last committed dataset and prevent serving. Readiness requires the fetched StatefulSet generation to match the generation just written, plus the existing rollout checks.

## Verified scope

- Production mechanism: `lego/operator/internal/controller/keyvalue_persistence.go` and `keyvalue_controller.go`; manager wires the uncached reader. No backend dependency or CRD schema change.
- All public writes converge through `KeyValuePatch` (REST, GraphQL, MCP); Blueprint's existing-resource spec writer and direct CR changes reach the same operator mechanism. Backend normalization, nil/omitted values, validation and auth remain unchanged. Backend KeyValue suite and Blueprint KeyValue spec tests passed.
- Native real Valkey **7.2.14 and 8.1.9**, final expanded matrix: **PASS**, 119.68 seconds. The old startup projection demonstrably loses the SAVE-persisted marker and rolls counter 23 back to 1. The production script preserves exact values, counters, non-expiring keys and expiration deadlines through durable transitions, ordinary restarts, unsaved graceful snapshot shutdown, no-prior-AOF initialization, AOF-only writes and kill/retry.
- Each major passes all nine mode pairs, nine first-boot desired-mode reversals, failed journal creation/retry, interrupted Off reset/reversal, unknown retained-storage refusal/reset, and missing/corrupt/comment-only/history-only manifest refusal. Recoverable AOF byte checksums and the committed marker remain unchanged; repairing the manifest recovers newer journal-only writes.
- Controller regressions cover owned-pod bootstrap, stale rollouts, retained claims, unchanged legacy templates, frozen source, suspension and direct CR/readiness. A weakened generation guard fails the new stale-readiness regression (mutation check).
- Rewrite active/scheduled/disabled/zero-rewrite/error and shutdown failures use explicitly simulated CLI responses. They are not claims of real-engine fault injection for every predicate.
- Dashboard: typed confirmation for both directions involving Off; canceled, stale, failed and repeated saves covered. Full dashboard suite **458 files / 3931 tests passed**, typecheck and lint passed. English/Chinese copy distinguishes durable conversion from destructive Off; ADR018/ADR021 document intentional durable Free support.
- Render comparison uses current official Key Value documentation, not a paid Render UI mutation. The public APIs continue reporting desired persistence plus existing transition/readiness status; hosted cross-surface acceptance remains open.
- Simplify review found no additional production abstraction changes; Docker container/volume cleanup callbacks were separated so one failure cannot skip the other.

## Final checks

Full operator `make test` (codegen, vet, unit tests and envtest) passed after merging current main. Full workspace lint/deadcode passed earlier; final changed operator/manager and fault-harness scoped lint passed with zero issues. Skill-layout and workflow validation passed. The final merged dashboard persistence tests (5) and typecheck passed.

One full operator run failed only the seven simulated shell cases under host process pressure; all 185 envtest specs passed. An isolated rerun passed, then serial cases and in-shell simulated timing commands removed unnecessary forks while retaining the 45-second watchdog. The full suite subsequently passed. Real-engine final matrices ran independently of these simulations.

## Remaining gates

The shared local Docker daemon stalls container operations. Native runs prove actual engine/filesystem behavior but do **not** certify pinned Linux images, BusyBox commands or uid/fsGroup behavior. The operator CI workflow now runs the pinned-image matrix on its Linux Docker host; that result remains required.

After release of the operator and dashboard, QA must replay both exact Free/public TLS/UI sequences in the milestone README, verify REST/GraphQL/MCP transition and completion states, rename/connection identity, delete all owned resources and revoke the session. No production fixture or session was created by this implementation run. t003, t004, t006 and t007 retain those unobserved criteria and remain open.
