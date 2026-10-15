# w1 · m177 — Take the dashboard's lint-tooling majors and fix what they find

**Worker:** worker1 **Goal:** the dashboard runs `eslint` 10, `@eslint/js` 10, `globals` 17, `eslint-plugin-react-refresh` 0.5 and `eslint-plugin-react-hooks` 7.1 with every new finding fixed in the code, none suppressed. **Status:** done

## Tasks (in order)

| id   | title                                                                                                  | est   | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | ----- | ---------- |
| t001 | `eslint` 10, `@eslint/js` 10, `globals` 17: fix `no-useless-assignment` and any new core findings — **DONE**       | 30m   | —          |
| t002 | `eslint-plugin-react-refresh` 0.5: name TanStack's `Route` export and move other non-component exports — **DONE**   | 1h    | t001       |
| t003 | `eslint-plugin-react-hooks` 7.1: refactor the 22 `set-state-in-effect`, 20 `refs` and 1 memoization finding — **DONE** | 2h30m | t002       |
| t004 | Simplify — **DONE**                                                                                               | 20m   | t003       |
| t005 | Test coverage — **DONE**                                                                                          | 40m   | t003       |
| t006 | Closeout — **DONE**                                                                                               | 10m   | t005       |

## Definition of done

- The five packages are at the majors above; `yarn lint` passes with no new `eslint-disable` and no rule turned off except `react-refresh`'s documented `allowExportNames: ["Route"]`.
- Every refactored component keeps its behavior: `yarn test` passes, and each refactor that changed render or effect order has a test asserting the user-visible behavior.

## Source + Goal linkage

- **Source:** `w1/125` (promoted 2026-10-15) with the user's decision; the finding counts come from m174 t005's trial upgrade.
- **Goal linkage:** dashboard correctness: the React Compiler rules flag state set synchronously in effects and refs read during render, both sources of extra renders and stale UI.
- **Expected outcome:** current lint tooling and a dashboard free of those patterns.
- **Why now:** Dependabot keeps proposing these majors; until the findings are fixed they block every dashboard dependency group.
- **Render parity omitted:** internal code quality, no surface change.

## Outcome (2026-10-15)

Installed at #92's ranges: `eslint` ^10.7.0, `@eslint/js` ^10.0.1, `globals` ^17.7.0, `eslint-plugin-react-refresh` ^0.5.7 and `eslint-plugin-react-hooks` ^7.1.1. `yarn lint` passes with no new `eslint-disable` and no rule turned off; m176's `use-service-name-draft` lost the one disable it had.

- **t001 (`no-useless-assignment`)**: the four findings are dead initial values, in `use-reauth-draft`, two in `hydra-consent` and one in the `global-search` test.
- **t002 (react-refresh 0.5)**: one configuration change from the plan. `allowExportNames: ["Route"]` silences the `Route` export but then reports every route file's local components, because 0.5 no longer counts `createFileRoute(...)(...)` as a component wrapper. The rule now lists TanStack's two route factories as `extraHOCs`, keeps the vite preset's `allowConstantExport` and `allowCompoundComponents`, and still reports any other non-component export from a route module. All 66 findings cleared without moving code.
- **t003 (react-hooks 7.1)**: three agents refactored the 43 findings in 19 files, each owning disjoint files, and all of it was reviewed here. Patterns used: state adjusted during render with the stored previous input, `useSyncExternalStore` for `useIsMobile`, the existing `useIsHydrated` for the sidebar, `useLayoutEffect` or `useEffectEvent` for latest-value refs, and id/generation-tagged state for revealed secrets. The refactors also removed several stale frames that the old effects painted for one render. Two fixes go beyond the plan:
  - Database and Key Value connection info could show a reveal under another id, or after access loss, when a response landed late; both now drop it.
  - A deploy-hook gap that predates this milestone is filed as `w1/130`.
- **t004 (Simplify)**: the review found one real defect. `editable-field-row` passed a fresh `options` array on every render, so the combobox's render-time highlight reset fired on every parent render. The prop is now `readonly` and documented as needing a stable reference. Other fixes from the review: the two connection-info hooks share `useConnectionReveal` (`features/capabilities/hooks`); `use-agent-sessions` derives `exhausted` from the request key it was found for, so a late page can no longer exhaust a new filter; a redundant eligibility reset is deleted; one comment is reworded.
- **t005 (Test coverage)**: new or extended tests cover every refactor that changed render or effect order. The suite grew from 4975 to 5018 tests. The agents confirmed each new test fails when its target refactor is broken, except one cron-runs test, which passes on old and new code.

Verification: `yarn lint`, two full `yarn test` runs (5018/5018, 529 files), `yarn install --immutable` and `yarn build`, all in the worktree; then install and lint again in the main tree, whose dashboard is identical to the worktree's base.
