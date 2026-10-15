# w1 · m176 — Land the dashboard's routine dependency updates, lucide 1.x and the test runtime

**Worker:** worker1 **Goal:** the dashboard takes Dependabot #83's minor and patch updates plus `lucide-react` 1.x, `jsdom` 29 and `@types/node` 26, with every guarded behavior still green. **Status:** todo

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Re-run #83's minor/patch half; separate real regressions from load flakes and fix or hold each | 1h  | —          |
| t002 | `lucide-react` 1.x with a shared `GithubIcon` (7 files, 16 uses)                               | 30m | t001       |
| t003 | `jsdom` 29 and `@types/node` 26                                                                 | 30m | t002       |
| t004 | Simplify                                                                                       | 15m | t003       |
| t005 | Test coverage                                                                                  | 20m | t003       |
| t006 | Closeout                                                                                       | 10m | t005       |

## Definition of done

- `dashboard/package.json` and `yarn.lock` carry #83's minor and patch versions (any held package named with its reason), `lucide-react` 1.x, `jsdom` 29 and `@types/node` 26; `yarn install --immutable` passes.
- `yarn lint` (typecheck, ESLint, unused code), the critical `yarn npm audit` gate and `yarn test` pass; each test that failed during the bisect has a recorded cause (product regression fixed or held, test fragility fixed, or load flake shown to pass repeatedly).
- The GitHub mark renders in all seven former `Github` call sites.

## Source + Goal linkage

- **Source:** `w1/125` (promoted 2026-10-15) with the user's decision to take the recommended split of Dependabot #83; evidence in `.pm/w1/done/125.md` and `.pm/w1/done/m174/` t005.
- **Goal linkage:** dependency hygiene for the dashboard; the dashboard has taken no routine update since #83 opened (2026-10-06).
- **Expected outcome:** current minor/patch dependencies and the two runtime majors that need no product migration; #83 shrinks to the deferred majors.
- **Why now:** the minor half is ten days stale and every later security update stacks on it; the lucide recipe and the bisect evidence are fresh.
- **Render parity omitted:** dependency updates with no REST/GraphQL/MCP/UI behavior change.
- **Not here:** the lint-tooling majors (`w1/m177`), `vite` 8 / `ai` 7 / `xterm` 6 (Dependabot's next quarterly batch, with live QA), `react-intl` 10 (waits on an `@ory/elements-react` release).
