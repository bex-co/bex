# w2 · m61 — P2 operator cleanup and resilience

**Worker:** worker2 **Goal:** Make generated artifacts reclaimable, finalization durable, mutable credentials consistent with running workloads, and every operator HTTP dependency bounded. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and label the complete generated-artifact inventory | 45m | w2/m60/t009 |
| t002 | Reclaim cross-namespace build and kpack artifacts | 45m | t001 |
| t003 | Wait for registry, object, backup, and TLS cleanup | 60m | t001 |
| t004 | Make KeyValue password changes rollout-consistent | 45m | w2/m60/t009 |
| t005 | Bound registry and Prometheus HTTP calls | 30m | w2/m60/t009 |
| t006 | Verify Render parity for delete and credential lifecycle | 30m | t002, t003, t004, t005 |
| t007 | Simplify cleanup and client plumbing | 30m | t006 |
| t008 | Complete restart, residue, rotation, and timeout coverage | 60m | t007 |
| t009 | Close out m61 | 15m | t008 |

## Definition of done

Deleting and recreating an App leaves no copied Secret, ServiceAccount, Job, Pod, kpack Image/Build, registry manifest, static object, backup-purge work, or TLS private key behind; cleanup errors survive manager restart and keep the finalizer until absence is proven; a KeyValue password change cannot split client and server credentials; and stalled registry/Prometheus endpoints release reconcile workers within documented deadlines. All tenant-facing lifecycle behavior remains consistent across bex surfaces.

## Source + Goal linkage

- **Source:** `docs/ADR039-operator-audit-and-platform-reuse.md` O-07 through O-10. This explicitly follows up completed `w7/m12` and `w7/m39`: the new audit found artifact classes and early-finalizer paths outside their earlier acceptance inventory.
- **Goal linkage:** ADR008's reliable, secure hosting platform and `GOAL.md` #1/#2/#7 (service lifecycle, operations, security review).
- **Expected outcome:** Deletes are retryable and residue-free, KeyValue credentials have one observable version, and external control-plane latency cannot indefinitely pin reconciles.
- **Why now:** These Medium/Medium-high findings become easier and safer after m60 establishes correct deletion/status semantics and close residual operational leaks independently of any uncommitted future substrate evaluation. Render parity closing is included because delete completion and credential behavior are tenant-visible even though most implementation is operator-internal.
