# Successful Blueprint updates are reverted by the control-plane projector

Why: an accepted source/configuration update must remain the desired configuration and reach its deployment, rather than return200 and silently restore old intent.

**Severity:** major. **Ownership:** shared backend Blueprint update and canonical intent persistence. Functional `$qa-find-bugs-cli` w8 sweep230. Published Bex0.3.2 and unmodified Render2.27/a764810a7682; isolated QA human in bex-canary / tea-daif693dqjvc73e7as3g, non-TTY JSON. Current source232f5b90eddf; deployed revision unknown. [Durable evidence](evidence/cli-sweep230/README.md).

## Reproduction and result

Create a fresh ledger-owned Free Node service through a valid inline declaration, with literal environment/absolute commands. Reapply only that service with repo https://github.com/expressjs/express, branch master, rootDir empty and the original commands/literals. POST /v1/blueprints/deploy returns200. Its new owned Blueprint is renamed, autoSync disabled, and POST /v1/blueprints/{owned-id}/sync receives the exact provided manifest twice, returning200 with only that owned service listed. Native validation, preceding successful deploys and full harmless declarations are retained in the common runtime receipt.

Expected: settled service source remains Express/master and a deployment uses that source. Actual: a fresh GET returns old Bex/main while the owned Blueprint manifest still declares Express/master; all three changed-source deploy rows are Canceled. No Cancel request was made. Later native Bex and Render `deploys create <owned-id> --wait --confirm -o json` exit0/Live and report the old Bex commit. Those are default-source controls, not successful deployment of the requested repository. Exact native argv/results and API bodies/replies are retained; native POSTs were not intercepted.

## Producer and consumer

- `apps/deploy.go:3081–3143` builds the desired presence-aware spec and delegates existing updates to patchChangedStackService. The deployment-worthy branch at3279–3323 changes existing.Spec, opens a deploy via recordBlueprintRedeploy and patches the CR. It never writes the matching IntentStore fields.
- `apps/service.go:349` documents SetAppSource as the row write that prevents resync reversion; the ordinary source setter calls it at4135–4139. `store/store.go:1977–1991` atomically persists repo/image/branch/registry binding in SQL.
- `store/reconciler.go:683` calls actual applyOwnedSpec with projectSpec of the desired row. At2088–2143 it owns source, registry binding, tier, replicas, port, idle timeout, suspension, hosts and disk. Its deliberate restore bumps spec/generation when the stale row differs; comments at2077 explain why that can supersede a newly opened deploy. Exact cancellation interleaving is unobserved; source persistence loss is independently confirmed.
- Four feature-method states exercise the real current updater with fake Kubernetes and recording IntentStore: requested source/tier/replicas/idleTTL reaches the CR and one deploy opens, while matching canonical writes remain0. Diagnostic exit0 proves execution of the current bug, not fixed behavior or a full suite. Only repo/branch is changed live; paid plans/replicas remain local diagnostics.

## Fix and blast radius

Persist the Blueprint's effective projector-owned intent in the managed App row as part of the deployment-worthy update. Derive that effective state with the existing field-presence/omission rules; do not write raw zero defaults or blindly use save-only source semantics that suppress the requested deployment. Keep the row, CR, release stamp and deployment record consistent through write failures and projector races. Preserve direct/storeless App updates, metadata-only/maintenance/grouping no-ops, registry binding atomicity, validated domain claims and disk semantics. No CLI fork or projector ownership weakening.

The updater has one updateStackService caller, reached through applyStackService in shared DeployStack/sync. REST, GraphQL and MCP use those same service methods; dashboard review consumes them. Shared blast-radius audit must enumerate every applyOwnedSpec field and map it to the existing canonical setter/transaction. Live source confirms the defect; field siblings require appropriate behavior regressions and safe acceptance, not paid production fixtures.

## Dedupe and cleanup

Whole-board symbol/behavior and targeted SetAppSource history searches found no matching Blueprint canonical-intent repair. w8/done/m52 owns stale create-snapshot/existence projection, not this update's missing durable setter. w6/done/m97 owns legitimate clone-secret handling and w6/done/m100 owns terminal build generation attribution; their updater mentions do not implement this source write. w4/done/m146 owns empty-env no-op comparison, and w5/done/142 owns apply index reuse. w4/blocked/287 concerns a user Cancel losing an optimistic lock; no user Cancel is sent here. No duplicate is filed for the cancellation symptom or for hook-count measurement.

Four exact-owned services/twenty child deploy records are deleted; the new Blueprint is disconnected after membership enumeration and the existing Blueprint remains. Details/lists/former HTTP404, baseline6/quota6/term0 and comparison-config/finite-reader teardown are verified. Kubernetes descendant teardown and live GraphQL/MCP/dashboard writes remain unverified. This hunt writes planning/evidence only; no product fix, QA commit/push or external issue.
