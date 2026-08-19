# w1 · m75 — Architecture-survey bug sweep: five cross-module drift fixes

**Worker:** worker1 **Goal:** close the five latent bugs the 2026-08-19 architecture survey surfaced (excluding the pricing/tiers drift guard, owned elsewhere) — each one a place where hand-synced twins or copy-paste already diverged. **Status:** done

## Tasks (in order)

| id   | title                                                      | est | depends_on                   |
| ---- | ---------------------------------------------------------- | --- | ---------------------------- |
| t001 | Align build-Job name derivation across backend and operator | 45m | — — **DONE**                 |
| t002 | Default `BEX_RUNTIME` to `kubernetes`                       | 20m | — — **DONE**                 |
| t003 | Memoize agentcred's default SessionLimiter                  | 20m | — — **DONE**                 |
| t004 | Route KeyValue project/env setters through the patch path   | 40m | — — **DONE**                 |
| t005 | Map coded errors at the MCP tool-registration seam          | 60m | — — **DONE**                 |
| t006 | Render parity — verify surfaces stayed consistent           | 30m | t001–t005 — **DONE**         |
| t007 | Simplify — run /simplify over the changed code              | 30m | t006 — **DONE**              |
| t008 | Test coverage — meaningful tests for the shipped behavior   | 45m | t006 — **DONE**              |
| t009 | Closeout                                                    | 15m | t008 — **DONE**              |

## Definition of done

All met:

- Backend and operator derive identical build-Job names past 63 chars, pinned by identical literals on both sides (`lego/types/v1alpha1/jobnames_test.go`, `lego/operator/internal/build/jobname_contract_test.go`, `lego/backend/internal/deploys/buildjobname_test.go`).
- Unset `BEX_RUNTIME` resolves to `ModeKubernetes` (`cmd/manager/runtimemode_test.go`).
- `agentcred`'s nil-`Limits` fallback acquires and releases one shared limiter, shedding on the per-source dimension (`agentcred/limiter_test.go`).
- The three KeyValue setters merge-patch exactly as their Postgres twins do (`internal/api/datastore_twin_parity_test.go`).
- Coded errors survive to agents on every MCP tool; tool count and classification unchanged (175, `TestMCPParityInventory` green).
- `make test` (operator, incl. codegen), `go test ./...` (backend, types, cli), and `make lint` (all four modules) green.

## What shipped

Each of the five fixes has a regression test **proven red on the pre-fix code** (verified by reverting each fix in turn).

1. **Build-Job name fork** — the derivation moved into the contract module both sides import. `lego/types/k8sname` (new leaf) owns DNS-1123 truncation that binds the discarded tail to a hash of the whole name; `appv1alpha1.RevisionJobName` builds on it, and `BuildJobName`/`BuildRevision` are the cross-boundary contract. bex-api's `buildJobName` and the operator's `build.JobName` now both call it.
2. **`BEX_RUNTIME` default** — flipped to `kubernetes` behind a testable `runtimeMode()` seam.
3. **agentcred limiter** — memoized with `sync.Once`; also given explicit bounds (64/4) rather than inheriting `NewSessionLimiter`'s SSH-session defaults (100/5), which its own doc comment had contradicted. Both it and `modelproxy` now resolve the limiter once per request so a late `Limits` assignment can't split acquire from release.
4. **KeyValue patch path** — the three fan-out setters route through `patchKeyValueObj`, matching Postgres.
5. **MCP coded errors** — a new `internal/mcputil.AddTool` wraps every handler's error at registration, which is the last point the typed error still exists (the SDK converts it to text before any middleware runs). All 175 registrations across 24 adapters go through it, and a **forbidigo lint rule** now forbids the raw `mcp.AddTool` — it resolves through the type checker, so aliased imports and explicit generic instantiation are caught too. The 49 now-redundant per-handler `core.MCPError` calls were deleted; `MCPError` stays idempotent as insurance.

### Found and fixed while in there

- **`predeploy.JobName` had the same truncation defect** — and it is reachable, not theoretical: App CR names are tenant-prefixed (`tea-<xid>-<service>`), so any service name ≥23 chars overflowed and every revision collapsed to ONE name, silently **skipping the next revision's migration**. Fixed via the shared helper.
- **The rename opens a transition window**, which the fix closes: a migration already running under the old name would otherwise have been left in flight while a second Job ran the same migration concurrently — the newest-wins sweep is normally skipped once a revision has started. `reconcilePreDeploy` now also sweeps when the recorded Job name differs from the computed one (`predeploy_rename_test.go`, proven red).
- **`publish.JobName` was a third copy of the same bug** — a name hit is read as "already published", so colliding names would serve stale content. Routed through the shared helper.
- The `gen-<n>` revision spelling was itself still hand-synced on both sides; it is now `appv1alpha1.BuildRevision`.
- Added `core.PaymentRequiredCode` (the code was a bare literal at two sites).

### Deliberately not done

- **Other 63-char truncations** (`cleanup_job.go`, `keyvalue_backup.go`, `hostRedirectResourceName`, `platformAliasName`): all length-safe today, and renaming them would strand finalizer-awaited Jobs or cause a routing/redirect gap on live objects. Migrating them is a drain-gated change, not a cleanup.
- **`AppReconciler.Mode` zero value** still selects the non-kubernetes path when the struct is built without it (55 of 101 test constructions rely on this). Making the reconciler itself default to kubernetes is the right end state but has real blast radius; production always sets it. Filed as a follow-up.
- **Gateway limiter/credential-proxy consolidation** (`LazySessionLimiter`, the shared `serveGit`/`serveModel` skeleton) — the survey's §5 theme, deliberately a separate milestone.

## Source + Goal linkage

- **Source:** inbox notes `w1/052`–`w1/056` (moved to `done/` on promotion), from the 2026-08-19 architecture survey. The survey's pricing/tiers drift guard is excluded — another person owns it (user decision 2026-08-19).
- **Goal linkage:** platform correctness and Render compatibility (ADR006 — same error shapes across REST/GraphQL/MCP; ADR004 — deploy Cancel must address the Job the operator actually created).
- **Expected outcome:** deploy Cancel works for long App names; long-named Apps no longer skip pre-deploy migrations; an unconfigured manager can no longer silently run the untested host runtime; the gateway's fallback cap actually caps; KeyValue and Postgres share one write semantic; agents get machine-readable error codes from all 175 MCP tools instead of 45.
- **Why now:** three of the five were drift between hand-synced twins that had *already* diverged once — the class recurs until pinned by tests. Render parity was included because t004/t005 change user-facing surface behavior.
