# w1 · m61 — Workspace-delete teardown: close the three resource-deletion gaps

**Worker:** worker1 **Goal:** deleting a workspace reliably tears down _everything_ the tenant owns — retryable after transient failures, sandboxes stopped and their tenant key revoked, Stripe subscription cancelled — with no permanent orphans. **Status:** done

## Tasks (in order)

| id   | title                                                                   | est | depends_on       | status       |
| ---- | ----------------------------------------------------------------------- | --- | ---------------- | ------------ |
| t001 | Make the workspace-delete purge phase re-drivable after partial failure | 45m | —                | — **DONE**   |
| t002 | Cascade `sandbox_tenant_keys` on tenant delete (migration 0056)         | 20m | —                | — **DONE**   |
| t003 | Sandbox WorkspacePurger: stop the workspace's sandboxes on delete       | 45m | t001             | — **DONE**   |
| t004 | Stripe teardown on workspace delete: cancel the metered Subscription    | 45m | t001             | — **DONE**   |
| t005 | Render parity: delete-verb behavior consistent across surfaces          | 20m | t002, t003, t004 | — **DONE**   |
| t006 | Simplify: `/simplify` over the code this milestone changed              | 20m | t005             | — **DONE**   |
| t007 | Test coverage: purge retry, sandbox teardown, Stripe cancel, FK cascade | 45m | t005             | — **DONE**   |
| t008 | Closeout                                                                | 15m | t006, t007       | — **DONE**   |

## Definition of done

A workspace delete (a) survives a mid-purge transient failure — a retry of the same API call completes the teardown with zero orphaned `Database`/`KeyValue` CRs, OpenBao paths, or env groups (proven by a test that fails a purger once, retries, and asserts nothing is left); (b) leaves no `sandbox_tenant_keys` row (FK cascade proven by a store test: `WorkspaceForSandboxKey` returns `ErrNotFound` after tenant delete) and stops the workspace's running OpenSandbox sandboxes through the API (fake-client test); (c) cancels the workspace's active Stripe Subscription when billing is enabled (stub-Stripe test) and is byte-identical with `BEX_STRIPE_SECRET_KEY` unset. Backend suite + lint green.

## Source + Goal linkage

- **Source:** 2026-07-31 code-review session ("when delete workspace — will all the tenant's resources be deleted?"): three verified gaps in `workspaces.Service.Delete` (`lego/backend/internal/workspaces/service.go:712-760`). ADR045 Finding 6 fixed the adjacent namespace-prune bug (w7/m62) but these were out of its window.
- **Goal linkage:** tenant lifecycle correctness underpins Render parity (workspace delete is a Render-compatible verb, `docs/render-artifacts/workspace-lifecycle.md`) and tenant isolation (ADR043): an orphaned live CNPG cluster burns real capacity, an orphaned sandbox tenant key is a live credential for a dead tenant, an orphaned Stripe subscription is a billing-integrity hole (ADR040).
- **Expected outcome:** workspace delete is a complete, idempotent, retryable teardown across the DB cascade, Kubernetes CRs, OpenSandbox, and Stripe; no class of tenant resource can be permanently orphaned by a transient failure.
- **Why now:** the gaps are live on prod today — every workspace delete since w3/m32 leaves an orphaned resolvable sandbox key, and any delete under an OpenBao/k8s blip strands a running CNPG cluster with no recovery path. Render parity task **included**: the fix touches the tenant-facing delete verb (GraphQL + dashboard danger zone) and its retry/error semantics.
