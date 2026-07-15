# w3 · m15 — Configurable service notification policy

**Worker:** worker3 **Goal:** Service Settings offers Render's notification-policy selector, with failure-only as the workspace default and correct delivery behavior for every deploy lifecycle event. **Status:** done — Render-compatible per-service notification policy shipped across REST, GraphQL, MCP, delivery, and dashboard.

## Tasks (in order)

| id   | title                                                        | est | depends_on    |
| ---- | ------------------------------------------------------------ | --- | ------------- |
| t001 | Capture the Render contract and compatibility plan            | 45m | — **DONE** |
| t002 | Persist workspace default and per-service notification policy | 60m | t001          |
| t003 | Enforce the effective policy for start/success/failure mail    | 60m | t002          |
| t004 | REST, GraphQL, and MCP notification-policy surfaces           | 60m | t002          |
| t005 | Service Settings four-state notification selector             | 60m | t003, t004    |
| t006 | Render parity                                                 | 30m | t003, t004, t005 |
| t007 | Simplify                                                      | 30m | t006          |
| t008 | Test coverage                                                 | 45m | t006          |
| t009 | Closeout                                                      | 15m | t007, t008    |

## Definition of done

Service Settings displays and persists `Use workspace default (Only failure notifications)`, `All notifications`, `Only failure notifications`, and `None`. A service using the default sends no deploy-started or deploy-succeeded email and sends deploy-failed email; each explicit override governs all three lifecycle emails exactly as labeled. The effective policy is consistent across REST, GraphQL, MCP, and dashboard UI, legacy `notifyOnFail` values and notification settings migrate without surprise, focused suites pass, and the selector is verified through save and full-page reload in a real browser.

## Source + Goal linkage

- **Source:** User request on 2026-07-15 and `/Users/tianpan/Desktop/Screenshot 2026-07-15 at 12.31.46 PM.png`; follow-up to `w3/done/m9`, `w3/done/005.md`, and `w4/done/m21`.
- **Goal linkage:** Pillar 1 and Render parity: make deploy lifecycle signals actionable without routine notification noise.
- **Expected outcome:** New services inherit failure-only notifications, while a user can choose all, failures only, or none per service from Service Settings and obtain the same behavior through every API surface.
- **Why now:** Repeated hourly deploy-start emails exposed the all-enabled default as operationally noisy. The current worktree has a partial failure-only default migration but no service-level four-state model; landing them together avoids shipping a hard-coded policy that immediately needs replacement.
- **Render parity closing task:** Included because this changes tenant-facing REST, GraphQL, MCP, and dashboard behavior and intentionally replaces `w4/m21`'s documented simplification with Render's richer override model.

## Closeout evidence

- Backend: `go test ./...` passes, including policy, adapter, compatibility, and migration coverage. The real-Postgres migration case is gated by `BEX_TEST_DB_URI` and runs in the repository's container-backed CI; that variable was unavailable locally.
- Operator/types: `make test` passes after CRD/deepcopy regeneration.
- Dashboard: `yarn typecheck`, `yarn lint`, 1,088 tests, and `yarn build` pass.
- Browser: real headless Chrome on the local bex stub verified all four labels, default failure-only copy, `None` mutation + success toast, persistence across reload, and restoration/persistence of workspace default.
- Simplify: the final pass preserved legacy `notifyOnFail` semantics when the richer field is absent and reused the existing `notify_on_fail_changed` activity event instead of creating a duplicate vocabulary entry.
