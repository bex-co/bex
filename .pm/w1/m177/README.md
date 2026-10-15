# w1 · m177 — Take the dashboard's lint-tooling majors and fix what they find

**Worker:** worker1 **Goal:** the dashboard runs `eslint` 10, `@eslint/js` 10, `globals` 17, `eslint-plugin-react-refresh` 0.5 and `eslint-plugin-react-hooks` 7.1 with every new finding fixed in the code, none suppressed. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est   | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | ----- | ---------- |
| t001 | `eslint` 10, `@eslint/js` 10, `globals` 17: fix `no-useless-assignment` and any new core findings       | 30m   | —          |
| t002 | `eslint-plugin-react-refresh` 0.5: name TanStack's `Route` export and move other non-component exports   | 1h    | t001       |
| t003 | `eslint-plugin-react-hooks` 7.1: refactor the 22 `set-state-in-effect`, 20 `refs` and 1 memoization finding | 2h30m | t002       |
| t004 | Simplify                                                                                               | 20m   | t003       |
| t005 | Test coverage                                                                                          | 40m   | t003       |
| t006 | Closeout                                                                                               | 10m   | t005       |

## Definition of done

- The five packages are at the majors above; `yarn lint` passes with no new `eslint-disable` and no rule turned off except `react-refresh`'s documented `allowExportNames: ["Route"]`.
- Every refactored component keeps its behavior: `yarn test` passes, and each refactor that changed render or effect order has a test asserting the user-visible behavior.

## Source + Goal linkage

- **Source:** `w1/125` (promoted 2026-10-15) with the user's decision; the finding counts come from m174 t005's trial upgrade.
- **Goal linkage:** dashboard correctness: the React Compiler rules flag state set synchronously in effects and refs read during render, both sources of extra renders and stale UI.
- **Expected outcome:** current lint tooling and a dashboard free of those patterns.
- **Why now:** Dependabot keeps proposing these majors; until the findings are fixed they block every dashboard dependency group.
- **Render parity omitted:** internal code quality, no surface change.
