# w8 · m66 — Keep Blueprint service updates in canonical intent

**Worker:** worker8 **Goal:** an accepted Blueprint source/config update survives control-plane convergence and deploys the declared configuration. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist effective Blueprint intent before projector convergence | 55m | — |
| t002 | Audit owned fields, callers and release sequencing | 40m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout | 20m | t004, t005 |

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

No QA product implementation, commit/push or external submission is authorized by this filing.
