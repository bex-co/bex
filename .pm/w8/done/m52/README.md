# w8 · m52 — Keep a new service out of stale orphan cleanup

**Worker:** worker8 **Goal:** a successful configured create retains its original App and accepted settings through projector resyncs **Status:** done

**Size:** 3h40m across three implementation/acceptance tasks and four standing closing tasks. Severity major. One live occurrence; deterministic current-source reproduction; exact live deletion actor unobserved.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fence orphan deletion with fresh durable existence and observed identity — **DONE** | 45m | — |
| t002 | Verify the shared prune boundary and all create producers — **DONE** | 45m | w8/m52/t001 |
| t003 | Replay configured creates through a real API and pinned CLIs — **DONE** | 50m | w8/m52/t002 |
| t004 | Render parity — **DONE** | 20m | w8/m52/t003 |
| t005 | Simplify — **DONE** | 15m | w8/m52/t004 |
| t006 | Test coverage — **DONE** | 35m | w8/m52/t004 |
| t007 | Closeout — **DONE** | 10m | w8/m52/t005, w8/m52/t006 |

## Definition of done

- [ ] A row and complete App created between desired-row and CR snapshots survives two reconciles with the original UID and accepted settings.
- [ ] Cleanup requires fresh durable absence and observed own/legacy identity; query errors, pending creates and replacements cannot be deleted by stale absence. True orphan cleanup still converges.
- [ ] Five App types and REST/GraphQL/MCP/Blueprint creates are audited; real store/apiserver concurrency and native/image/env/files controls are retained.
- [ ] Bex and unmodified exact-pin Render against Bex create fresh free configured fixtures; marker logs, declared process port, HTTP200, settings and creation identity hold across resyncs. Failed predeploy preserves the prior release after wake convergence.
- [ ] Required tests/lint/parity/review and owned-resource cleanup pass. Live attribution is resolved or explicitly retained with evidence and limits.

## Source + Goal linkage

- **Source:** user-requested infinite qa-find-bugs-cli loop filing to w8, 2026-10-08 local; [sanitized evidence, traced mechanism and failing probe](finding.md). A fresh unmodified same-pin Render control passed and is retained alongside the Bex occurrence/control.
- **Goal linkage:** ADR008 hosting reliability and ADR003 source-of-truth projection. Successful creation must preserve the tenant’s runtime configuration.
- **Expected outcome:** ordinary resync cannot erase a newly created service’s health/predeploy/env references and leave its URL unavailable while its durable row survives.
- **Why now:** production showed the same service ID with a creation time changed 13 seconds after create and accepted settings absent. Current main deterministically reproduces the destructive interleaving; older image/clone/log repairs do not cover it.
- **Render parity included:** retention and runtime behavior are tenant-facing across REST, GraphQL, MCP and dashboard. Fix the server cleanup boundary while preserving the pinned CLI/auth/error contracts.
