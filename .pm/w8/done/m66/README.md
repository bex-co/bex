# w8 · m66 — Keep Blueprint service updates in canonical intent

**Worker:** worker8 **Goal:** an accepted Blueprint source/config update survives control-plane convergence and deploys the declared configuration. **Status:** done — six tasks complete; canonical-intent implementation62c74aae6, inline sweep288 and persistent sweep330 with three native waited controls, no-ops and full cleanup.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist effective Blueprint intent before projector convergence — **DONE** | 55m | — |
| t002 | Audit owned fields, callers and release sequencing — **DONE** | 40m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 35m | t003 |
| t006 | Closeout — **DONE** | 20m | t004, t005 |

## Definition of done

- An own Free service changes repository/branch through inline Blueprint apply and an owned persistent provided-manifest sync; after projector convergence its metadata remains the declared source and its deployment uses that source's actual commit. No spontaneous cancellation caused by stale canonical intent.
- Native published/current Bex and pinned Render --wait controls deploy the declared source, preserving exact runtime literals; repeated identical apply is a no-op. Literal/omitted/explicit-empty policies, source/registry, managed/storeless and other projector-owned fields have meaningful persistence/convergence tests.
- Shared REST/GraphQL/MCP/UI readback and errors remain coherent; no pricing/CLI contract change or weakening of projector ownership. Canonical write failures and release sequencing retain truthful results.
- Affected required checks pass. Exact-owned fixture/child/Blueprint teardown, baseline presence and quota reconciliation are recorded. All tasks complete before moving to done.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs-cli` w8, sweep230. [Finding](finding.md) and [durable evidence](evidence/cli-sweep230/README.md).
- **Goal linkage:** ADR008 reliable hosting, ADR003 control-plane source of truth, ADR049 Blueprint declarative intent and ADR018 Render compatibility.
- **Expected outcome:** a successful source/config declaration remains durable instead of being restored from an unchanged SQL row.
- **Why now:** three accepted live source updates cancel and settle at the old repository; four actual current-source managed updates open a deploy and patch desired CRs with zero matching intent writes.
- **Sizing:** 185 minutes across six tasks. Shared-field persistence, failure sequencing and producer/projector integration exceed a sub-hour note.
- **Render parity included:** the shared API, Blueprint preview/readback and both native CLI clients expose the behavior. Only the source loss is live-proven; field siblings and non-REST surfaces remain scoped checks.

The original filing authorized planning/evidence. The maintained implementation and subsequent hosted acceptance below complete the scope; QA adds no product implementation.

## Historical canonical intent persisted — 2026-10-14 (`/loopx w8`)

t001–t005 are done; see `done/`. Parked in `blocked/`: closeout needs bex-api deployed, then QA replays sweep230. An own Free service changes repo/branch through inline apply and an owned persistent sync. After a resync it must still read Express/master, its deploy must build that source's commit without spontaneous cancellation, a repeated identical apply must be a no-op, and owned teardown must follow.

## Independent original canonical-intent replay — sweep282

[Current-source evidence](../../evidence/current-source-sweep282/README.md) reruns the original actual patch-path probe after `62c74aae6`: source, tier and replicas each produce one canonical write, the desired CR and one deploy row. The direct idleTTL helper case remains excluded because no supported Blueprint applier writes that field, as the completed field audit records. Newly committed disk, row-failure, no-op/unmanaged controls pass in the selected run. This is source acceptance; the hosted inline/persistent-sync gate remains open, with no production mutation or QA ship.

## Historical live inline acceptance — sweep288

[Complete native/API/runtime evidence](evidence/cli-sweep288/README.md) now passes inline Bex/main to Express/master persistence, exact source commit/content/literals, three native waited deployments (published Bex, current checkout and pinned Render), and identical-apply no-op. Both Free fixtures/seven deploy children are fully cleaned, baseline six/quota terminating0 preserved. Persistent setup did not create a Blueprint: the prior owned Express/master connection was disconnected and the current auto-upsert intentionally cannot revive it; the actual discarded store error remains unobserved. A retained invalid root-directory setup is excluded. No persistent sync was run. Keep t006 and this milestone blocked on that remaining live gate; no QA product change or ship.

## Historical persistent source-kind acceptance exposed a separate edge — sweep294

[Complete m70 evidence](../m70/evidence/cli-sweep294/README.md) establishes a fresh own persistent connection and successful provided-manifest sync, avoiding the prior disconnected tuple. The image→Git change retains both source fields and reports Live/new Git commit while serving the former image; the equivalent fresh inline path reproduces. Both native later deploys recover and exact runtime package/literals pass; identical reapplies are no-ops. Current managed-row probes show canonical writes are now correct but persist the earlier source fold’s wrong tuple. This is the separate [m70](../m70/README.md) producer defect, not regression of m66’s canonical write. Keep closeout blocked on the full live end state; no manual-recovery result closes automatic apply acceptance. Two services/seven children/one Blueprint are completely cleaned, baseline six/quota terminating0.

## Closed 2026-10-15 — sweep330

[Complete persistent acceptance](evidence/cli-sweep330/README.md) passes the first automatic correct Git artifact, durable metadata, six native reads, REST/GraphQL/MCP source/command readback, three native default-source waited controls and delayed identical-sync no-ops before and after them. Nine maintained tests/18 named events pass. All own resources and six deploy children are absent; original services/Blueprint remain and quota termination is0. Together with inline sweep288 this closes all six tasks. [m78](../m78/README.md) remains open for different containing/service sources. Serving SHA, browser writes and direct GraphQL/MCP mutations are not inferred.
