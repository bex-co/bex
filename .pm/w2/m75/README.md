# w2 · m75 — ADR055 F2/F3: workspace-scoped registry and static-prefix identity migration

**Worker:** worker2 **Goal:** retire workspace-local `App.Name` as a global identity at the two remaining shared sinks — Zot repositories/users/ACLs and static-site S3 prefixes — so two same-named Apps in different workspaces can never collide; new Apps are born on the workspace-scoped scheme, legacy Apps keep working via dual-read, and a verified migration tool + gated runbook move the old artifacts. **Status:** todo

## Tasks (in order)

| id                                | title                                                                                                    | est | depends_on               |
| --------------------------------- | -------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001                              | Identity-scheme design note: workspace-scoped repo, user/ACL, and S3-prefix formats                       | 45m | —                        |
| t002                              | Operator: new Apps mint workspace-scoped Zot repos/users/ACLs/pull credentials; dual-read legacy fallback | 60m | w2/m75/t001              |
| t003                              | Static-site publish/serve S3 prefixes dual-path (new scheme for new publishes, legacy fallback on read)   | 45m | w2/m75/t001              |
| t004                              | Migration tool: enumerate, retag/copy, verify, tombstone legacy repos and prefixes                        | 60m | w2/m75/t002, w2/m75/t003 |
| t005                              | Zot ACL/retention/purge and future build-cache naming on the new scheme                                   | 45m | w2/m75/t002              |
| t006                              | Mock-cluster end-to-end: same-named Apps in two workspaces, plus legacy App via dual-read                 | 45m | w2/m75/t004, w2/m75/t005 |
| t007                              | Prod migration runbook with explicitly authorization-gated cutover + rollback                             | 30m | w2/m75/t006              |
| t008                              | Docs: annotate ADR055/ADR067/ADR069 residual-register lines and any env/table changes in CLAUDE.md        | 30m | w2/m75/t007              |
| t009 (standing closing task)      | Simplify: run /simplify over the code this milestone changed                                              | 30m | w2/m75/t008              |
| t010 (standing closing task)      | Test coverage: meaningful tests for scheme minting, dual-read, migration verification, collision prevention | 45m | w2/m75/t008            |
| t011 (standing closing task)      | Closeout: verify DoD, mark done, move milestone to done/                                                  | 15m | w2/m75/t010              |

## Definition of done

On the mock cluster, two Apps with the same name in different workspaces coexist end to end (build → push → pull → serve, and static publish → serve) on workspace-scoped Zot repositories and S3 prefixes; a pre-existing legacy-named App continues to work unchanged through dual-read; the migration tool moves a legacy App's artifacts with digest/object verification and tombstones the legacy location; the prod cutover runbook exists with an explicitly authorization-gated step; the ADR055 F2/F3 residual-register lines are annotated with the new state.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 1, item 2) — the oldest unowned security deferral: ADR055 F2/F3, re-confirmed through ADR056 (findings 1/3), ADR057 (8/10), ADR061, ADR067 (finding 1's decomposition), and ADR069's open questions; referenced from w7/m85 but owned nowhere.
- **Goal linkage:** tenant isolation (ADR022/ADR043) — this is the root-cause class of ADR055 F1–F5 ("workspace-local `App.Name` used as global identity at shared sinks").
- **Expected outcome:** registry and static-origin identity is workspace-scoped for all new Apps immediately, with a verified migration path for legacy artifacts; the security lineage's most re-reported code deferral finally has an owner and a closure path.
- **Why now:** w7/m86 (per-App Zot build-cache repos, ADR060 D3) is queued and would mint MORE repos on the legacy bare-name scheme; landing the identity scheme first means those `-cache` repos are born workspace-scoped instead of doubling the future migration surface.
- **Render parity closing task omitted:** no user-facing REST/GraphQL/MCP/UI surface changes — this is internal registry/object-store identity; wire shapes are untouched.
