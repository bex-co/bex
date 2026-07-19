# w2 · m61 — P2 operator cleanup and resilience

**Worker:** worker2 **Goal:** Make generated artifacts reclaimable, finalization durable, mutable credentials consistent with running workloads, and every operator HTTP dependency bounded. **Status:** done (2026-07-19)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and label the complete generated-artifact inventory — **DONE** | 45m | w2/m60/t009 |
| t002 | Reclaim cross-namespace build and kpack artifacts — **DONE** | 45m | t001 |
| t003 | Wait for registry, object, backup, and TLS cleanup — **DONE** | 60m | t001 |
| t004 | Make KeyValue password changes rollout-consistent — **DONE** | 45m | w2/m60/t009 |
| t005 | Bound registry and Prometheus HTTP calls — **DONE** | 30m | w2/m60/t009 |
| t006 | Verify Render parity for delete and credential lifecycle — **DONE** | 30m | t002, t003, t004, t005 |
| t007 | Simplify cleanup and client plumbing — **DONE** | 30m | t006 |
| t008 | Complete restart, residue, rotation, and timeout coverage — **DONE** | 60m | t007 |
| t009 | Close out m61 — **DONE** | 15m | t008 |

## Outcome and verification (2026-07-19)

- One exact-lifetime `ArtifactIdentity` now labels and inventories cross-namespace Jobs, Pods, Secrets, ServiceAccounts, NetworkPolicies, and kpack Images/Builds. Legacy mechanism labels are accepted only when creation time proves they belong to the current App; tests prove a recreated same-name App cannot adopt old residue.
- App finalization joins every error and waits for a fresh zero inventory, historical TLS Secret/private-key removal, Zot manifest absence, persisted static-object purge completion, and credential revocation. Database finalization similarly persists backup-purge success, then removes and observes the Job/Pods absent. Restart, failed Job, transient delete, removed-host, and registry second-read tests pin the behavior.
- KeyValue credentials have one immutable `<id>-auth` authority and one immutable connection projection. The Pod template carries a one-way revision; Ready status advances only after the current StatefulSet/Pod uses it, and backend connection-info returns a non-leaking conflict on any split. User-driven password rotation remains deliberately unsupported.
- Registry, Prometheus, image-verification, and OpenSandbox calls use bounded transports and response reads. Deterministic tests cover stalled connect, TLS, headers, body, size, and caller cancellation.
- REST/GraphQL/MCP views and the dashboard's shared data report deletion in progress rather than stale Ready/available state. Official CLI command and acknowledgement shapes remain unchanged; no cleanup-specific flag or workaround was introduced.
- Final gates passed: operator `make test` (codegen, fmt, vet, Kubernetes 1.35 envtest; controller coverage 76.5%), backend `go test ./...`, `make lint` (zero issues across both modules), targeted race suites for operator build/controller/httpclient/registry and backend apps/KeyValue/Postgres, `git diff --check`, and `bash -n scripts/delete-audit.sh`. The live delete audit requires explicit disposable resource names and was not run against an unknown current cluster.
- ADR039 closes O-07 through O-10. OpenChoreo remains only a future first candidate and Korifi a future second comparison; no PoC, PM milestone, adapter, or migration was started.

## Definition of done

Deleting and recreating an App leaves no copied Secret, ServiceAccount, Job, Pod, kpack Image/Build, registry manifest, static object, backup-purge work, or TLS private key behind; cleanup errors survive manager restart and keep the finalizer until absence is proven; a KeyValue password change cannot split client and server credentials; and stalled registry/Prometheus endpoints release reconcile workers within documented deadlines. All tenant-facing lifecycle behavior remains consistent across bex surfaces.

## Source + Goal linkage

- **Source:** `docs/ADR039-operator-audit-and-platform-reuse.md` O-07 through O-10. This explicitly follows up completed `w7/m12` and `w7/m39`: the new audit found artifact classes and early-finalizer paths outside their earlier acceptance inventory.
- **Goal linkage:** ADR008's reliable, secure hosting platform and `GOAL.md` #1/#2/#7 (service lifecycle, operations, security review).
- **Expected outcome:** Deletes are retryable and residue-free, KeyValue credentials have one observable version, and external control-plane latency cannot indefinitely pin reconciles.
- **Why now:** These Medium/Medium-high findings become easier and safer after m60 establishes correct deletion/status semantics and close residual operational leaks independently of any uncommitted future substrate evaluation. Render parity closing is included because delete completion and credential behavior are tenant-visible even though most implementation is operator-internal.
