# w5 · m14 — Delete service: dashboard danger-zone action

**Worker:** worker5 **Goal:** Give the dashboard the one core-lifecycle destructive action it's missing — a Settings-tab danger zone with type-to-confirm delete, wired to w2/m4's `deleteService` mutation, matching the pattern w6/m3/t004 establishes for workspace delete and `use-delete-database.ts` establishes for databases. **Status:** todo (gated on w2/m4)

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | GraphQL document + `useDeleteService` hook: mutation, cache eviction from the services list, redirect on success | 30m | w2/m4      |
| t002 | Settings-tab danger zone: delete button + type-to-confirm dialog (service name must match)     | 35m | t001       |
| t003 | Acceptance: create → delete → gone from list + services query, redirected with toast, no dangling row | 25m | t002       |
| t004 | i18n (en/zh) for the danger-zone copy + confirm dialog                                         | 15m | t002       |
| t005 | Render parity — dashboard danger-zone UX vs Render's service Settings delete flow              | 20m | t003, t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                    | 20m | t005       |
| t007 | Test coverage — meaningful tests for the delete flow (confirm-guard, cache eviction, redirect)  | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/`                                                 | 10m | t007       |

## Definition of done

On dashboard.bex.co, a service's Settings tab has a danger zone; typing the service's exact name and confirming calls `deleteService`, the service disappears from the list within one refetch, and the user lands back on the services list with a success toast. Wrong-name input keeps the button disabled. `yarn lint && yarn typecheck && yarn test` green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm new milestones` 2026-07-09 — gap analysis against `docs/render-parity.md`'s "Delete service" row (UI ✖, only `w2/m4` as owner, which is backend-only per its own README); `GOAL.md` item 1 names Delete explicitly ("Suspend. Delete. Create.").
- **Goal linkage:** V0 roadmap item 1 (Create/Suspend/Delete — the last unbuilt verb of the three); pillar 1 (Render dashboard parity).
- **Expected outcome:** service deletion becomes possible without `kubectl`/`curl`/MCP — the last resource type (services) catches up to databases/workspaces, which already have this pattern.
- **Why now:** w2/m4 is about to build the backend verb this gap analysis surfaced as still-missing; wiring the UI as a sibling milestone means the feature ships whole instead of backend-then-forgotten.
- **Render parity task included:** yes — this is dashboard surface work; t005 compares against Render's service Settings danger zone.

## Note

Originally materialized as `w5/m13` 2026-07-09; renumbered to `w5/m14` on rebase after `w1/m18`'s companion `w5/m13` (Build & Deploy settings / Root Directory) landed on `origin/main` first under the same number. No content otherwise changed.
