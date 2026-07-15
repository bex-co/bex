# w5 · m31 — Environments UX round 2: ACLs, membership, confirmations, create-selectors

**Worker:** worker5 **Goal:** The dashboard catches up to the shipped Environments backend: ACLs become editable, env-group membership manageable, protected services deletable (via typed confirmation) instead of dead-ending, and datastores can be created directly into an environment. **Status:** done

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on                   |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | Capture Render's protected-environments dashboard UX live → `docs/render-artifacts/` — **DONE**                        | 30m | —                            |
| t002 | ACL editor on the environment card (protected toggle · network isolation · ipAllowList) over `setEnvironmentACL` — **DONE** | 45m | t001                         |
| t003 | Env-group membership dialog (`envGroupIds` full-replace, mirroring `manage-resources-dialog.tsx`) — **DONE**            | 30m | —                            |
| t004 | Thread `ProtectedConfirmation` through dashboard delete/suspend/deploy-override flows (typed-confirmation dialog) — **DONE** | 45m | t001                         |
| t005 | Project/Environment selectors on the Postgres + Key Value create pages (inputs already accept `environmentId`) — **DONE** | 30m | —                            |
| t006 | Render parity — UI vs Render behavior + cross-surface consistency check; refresh ADR018 — **DONE**                      | 20m | t002, t003, t004, t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed — **DONE**                                                  | 20m | t006                         |
| t008 | Test coverage — component/hook tests incl. the confirmation failure modes — **DONE**                                    | 30m | t006                         |
| t009 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                               | 10m | t008                         |

## Definition of done

In a real browser against the mock cluster: an environment's `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` are editable from its card; its env-group membership is manageable; deleting (and suspending/deploy-overriding) a protected member service prompts for and honors the typed `ProtectedConfirmation` instead of surfacing a raw error; creating a Postgres or Key Value can target a Project/Environment directly and the resource lands there (auto-joining the project). Verified end to end with a browser click-through.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` rounds 8–9 — `w6/m19` shipped protected-environment ACLs REST/GraphQL/MCP-only (its closeout records "no editing UX yet"), `w6/m24` shipped env-group↔environment linkage API-only, and `w5/014` (folded here as t005) filed the datastore-create selector gap. Verified 2026-07-15: zero dashboard hits for `ProtectedConfirmation`, no ACL-editing or membership component.
- **Goal linkage:** Render parity (pillar 1), Environments UI half — `docs/ADR018-render-parity.md` Environments row; docs/ADR032-environments.md.
- **Expected outcome:** the protected-service dead end (cannot delete/suspend from the browser at all) is repaired; the Environments feature is fully operable without curl.
- **Why now:** the backend shipped 2026-07-14/15 with the UI gaps recorded in the closeouts; every day is a live broken flow for protected services; w5's queue is empty.
- **Render parity closing task: included** — dashboard UI surface over existing REST/GraphQL/MCP verbs.

## Resolution

Shipped the complete Environments dashboard catch-up: Render-informed Environment settings, workspace-scoped Env Group membership, exact server-issued confirmation retries for delete/suspend/Blueprint sync, and a shared Project/Environment create selector for services and both datastores. The isolated mock-cluster browser pass persisted every ACL field, reopened the checked Env Group membership, placed Postgres and Key Value resources into the selected Environment, and completed all three protected actions. Cross-surface parity is documented in ADR018/ADR032; Render's broader inbound-rule enforcement/default difference is filed as `w4/018`.

Final verification: dashboard codegen, lint/typecheck, 1,132 tests, and production build passed; backend `go test ./...` and `make lint-backend` passed. Browser evidence is under `.playwright-mcp/`.
