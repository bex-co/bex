# w2 · m82 — Controller ERROR spam: snapshot-pull label/RBAC + terminal Failed-build requeue

**Worker:** worker2 **Goal:** stop `bex-controller-manager` from ERROR-logging healthy platform work — sandbox snapshot-pull backfill must not fail reconcile, and terminal Failed builds must not re-`fail` forever. **Status:** done

## Tasks (in order)

| id                           | title                                                                                                      | est | depends_on  |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------- | --- | ----------- |
| t001                         | Snapshot-pull: stamp protected label on create, non-fatal backfill, ClusterRole `patch` — **DONE**         | 35m | —           |
| t002                         | App reconciler: once Failed Ready is recorded, halt without re-`fail` — **DONE**                           | 30m | —           |
| t006                         | Never re-dispatch a terminally Failed build after its Job is reaped                                        | 40m | w2/m82/t002 | — **DONE** |
| t007                         | Stop swallowing App status writes — the durable Failed marker must land                                    | 30m | —           | — **DONE** |
| t003 (standing closing task) | Simplify (standing)                                                                                        | 20m | w2/m82/t001, w2/m82/t002, w2/m82/t006, w2/m82/t007 | — **DONE** |
| t004 (standing closing task) | Test coverage (standing)                                                                                   | 35m | w2/m82/t001, w2/m82/t002, w2/m82/t006, w2/m82/t007 | — **DONE** |
| t005 (standing closing task) | Closeout (standing)                                                                                        | 15m | w2/m82/t004 | — **DONE** |

## Definition of done

On prod (or mock with the same RBAC), `sandboxnamespaceregistry` reconciles every `app.bex.co/regime=sandbox` namespace without Forbidden ERROR; new `bex-snapshot-pull` Secrets carry `app.bex.co/protected-from-tenant-mount=true` at create; an App already `phase=Failed` with Ready reason `BuildFailedUserError` (or infra/generic) for the current generation produces **no further** `Reconciler error` for that generation **and no re-created build Job after the failed Job's TTL reap or an operator restart** (2026-08-20 evidence: `market-size` failed 08-17 and `beancount-forum` failed 08-02 both re-ran full builds when the operator rolled); the Failed Ready marker survives concurrent App-CR rewrites (status writes retried on conflict, never `_ =`-swallowed); `bex-operator-snapshot-pull` is `get`+`create`+`patch` in gitops; unit/envtest coverage pins these behaviors; operator tests green.

## Source + Goal linkage

- **Source:** live prod log diagnosis 2026-08-20 (`hetzner-prod` `bex-system` / `bex-controller-manager`); user asked to diagnose platform services then hand off to `/pm for w2`.
- **Goal linkage:** platform operator reliability / observability hygiene — ERROR logs must mean real failures, not re-reported terminal tenant build outcomes or best-effort label backfill under least-privilege RBAC.
- **Expected outcome:** controller ERROR rate drops to near-zero for these two controllers under steady state; sandbox resume-pull Secrets stay mount-protected; Failed Apps stay Failed without requeue storms.
- **Why now:** both bugs are actively ERROR-spamming prod today (Bug A mitigated live with a ClusterRole patch + Secret labels; Bug B still live until the controller image ships). Draft code already exists uncommitted in the working tree from the diagnosis session — land, test, ship; do not rewrite from scratch.
- **Render parity omitted:** operator-internal mechanism + gitops RBAC only; no REST/GraphQL/MCP/UI wire change.
