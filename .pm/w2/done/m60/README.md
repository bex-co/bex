# w2 · m60 — P1 operator correctness: lifecycle, storage, type, and health

**Worker:** worker2 **Goal:** Make deletion, grow-only storage, workload type, and child-resource health converge correctly under real Kubernetes event semantics. **Status:** done (2026-07-19)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Admit Database deletion and finalizer events — **DONE** | 30m | w2/m59/t009 |
| t002 | Preserve grow-only Postgres storage across plan changes — **DONE** | 45m | w2/m59/t009 |
| t003 | Implement grow-only Valkey PVC expansion — **DONE** | 60m | w2/m59/t009 |
| t004 | Make App workload type immutable and reject transitions — **DONE** | 45m | w2/m59/t009 |
| t005 | Reconcile status-only child health and rollout revisions — **DONE** | 60m | w2/m59/t009 |
| t006 | Verify Render parity across affected lifecycle surfaces — **DONE** | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify the correctness fixes — **DONE** | 30m | t006 |
| t008 | Complete lifecycle and failure-mode coverage — **DONE** | 60m | t007 |
| t009 | Close out m60 — **DONE** | 15m | t008 |

## Outcome and verification (2026-07-19)

- Database and App primary watches share a generation/deletion/finalizer predicate; a real manager/envtest proves a metadata-only Database delete reaches the finalizer and removes its CNPG child, while an injected cleanup failure proves the finalizer stays until retry succeeds.
- Postgres storage is the monotonic maximum of plan/explicit intent, persisted allocation, and live CNPG request. Plan downgrade never shrinks it; REST/CLI-shaped and Blueprint shrink attempts fail before mutation; all read adapters receive the effective accepted size.
- Valkey now patches the bound `data-<id>-0` PVC only after confirming its StorageClass supports expansion. `StorageReady` reports binding, controller/filesystem resize, unsupported class, and convergence; shrink/non-expandable cases leave the PVC unchanged and avoid hot loops.
- `App.spec.type` is immutable at CRD admission and in Blueprint Core. All 20 ordered transitions among web/private/worker/cron/static are rejected before mutation, while legacy empty ↔ `web_service` normalization and same-type sync remain valid.
- Deployment/StatefulSet, Pod, and PVC status-only events now enqueue their owning resource. App and KeyValue readiness requires current-generation/current-revision children plus live Pod readiness, with a 30-second safety requeue; crash-after-Ready and old-revision rollout regressions are covered.
- Render's current Blueprint, service-update, Postgres-storage, and CLI command contracts were rechecked. ADR004/009/018/021/039 and the CLI checklist record the result honestly as code/envtest plus command-schema evidence, not a new live CLI run. Dashboard status/error mappings already preserve failed/pending states, so no new internal-resize UI surface was added.
- Final gates passed: operator `make test` (fmt, vet, generated CRDs/RBAC, envtest), backend `go test ./...`, dashboard `yarn test` (1,613 tests), `make lint` (zero issues across operator/backend), targeted race tests for controller/apps/Postgres, `scripts/gitops-validate.sh`, and `git diff --check`.
- ADR039 closes O-03 through O-06. O-07 through O-10 remain m61 work; OpenChoreo is still only a future first candidate and Korifi a future second comparison, with no PoC, adapter, or migration started.

## Definition of done

A Database deletion always reaches and completes finalization; Postgres plan downgrades never request PVC shrink; Valkey storage increases patch and observe the real PVC; App type changes are rejected consistently before they can orphan another workload kind; and App/KeyValue status follows child crashes and failed rollouts without requiring a spec edit. Tests prove each failure mode and the Render-facing surfaces return consistent results.

## Source + Goal linkage

- **Source:** `docs/ADR039-operator-audit-and-platform-reuse.md` O-03 through O-06. This is a corrective follow-up to completed datastore/service milestones (`w1/m14`, `w1/m15`, `w8/m14`, `w7/m12`) based on newly demonstrated event/storage/transition gaps.
- **Goal linkage:** ADR008 pillar 1's reliable self-service deployment and `GOAL.md` #1/#4/#7 (service lifecycle, PostgreSQL, security/correctness review).
- **Expected outcome:** Kubernetes actual state, bex CR status, and Render-facing lifecycle responses converge after delete, resize, plan change, type update, crash, and rollout failure.
- **Why now:** O-03–O-06 are High correctness blockers and form the lifecycle acceptance baseline any future, explicitly approved OpenChoreo/Korifi adapter must also pass. Render parity closing is included because plan/type/lifecycle errors and observed health are tenant-visible across REST, GraphQL, MCP, CLI, and dashboard paths.
