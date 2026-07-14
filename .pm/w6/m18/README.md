# w6 · m18 — Deterministic ownerId threading: usage, API keys, GitHub connections

**Worker:** worker6 **Goal:** A multi-workspace caller can view/act on usage, API keys, and GitHub connections for a non-default workspace by naming it explicitly, on every surface — closing the read/bind-side gap `w6/m14` left when it fixed writes. **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | REST: `ownerId` param on `GET /v1/usage`, api-key list/create, GitHub connection endpoints → `core.WithWorkspace` | 40m | —          |
| t002 | GraphQL: matching `ownerId` arg on the same three surfaces (m14 pattern)                            | 35m | t001       |
| t003 | MCP: honor `core.SelectedWorkspace` precedence for `get_usage`, api-key tools, GitHub tools          | 35m | t001       |
| t004 | Fix `use-current-workspace.ts`/`team-panel.tsx`; thread `currentWorkspaceId` into Usage + API-keys pages | 40m | —          |
| t005 | Regression test: multi-workspace caller gets workspace-B's data when targeting B explicitly, workspace-A's by default (mirrors `TestMultiWorkspaceTargetingE2E`) | 30m | t001, t002, t003, t004 |
| t006 | Render parity — full-surface consistency check (REST/GraphQL/MCP/UI)                                 | 20m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                          | 20m | t006       |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                             | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                       | 10m | t008       |

## Definition of done

A caller belonging to 2+ workspaces can view/act on usage, API keys, and GitHub connections for a **non-default** workspace by naming it explicitly (REST `ownerId`, GraphQL arg, MCP selected-workspace), on every surface including the dashboard, with no regression to the default-workspace behavior for single-workspace callers.

## Source + Goal linkage

- **Source:** `w6/012.md`, filed during `w6/m14` (2026-07-13) — the read/bind-side residual `m14` explicitly left open. Proposed via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** w6's multi-tenant-security mandate — `m14` closed this gap for writes (membership-checked `ownerId` on create); usage, api-keys, and GitHub connections were the noted residual reads/binds still resolving the caller's default workspace only.
- **Expected outcome:** no more silent wrong-workspace answers for multi-workspace users/agents on usage, keys, or GitHub connection status; the dashboard's workspace switcher actually drives what these pages show.
- **Why now:** root cause and fix shape are already fully diagnosed and written up in `w6/012.md`; small, self-contained, no new dependencies — was just sitting in the inbox since `m14`.
- **Render parity closing task: included** — touches REST/GraphQL/MCP and the dashboard.
