# w1 · m176 — Land the dashboard's routine dependency updates, lucide 1.x and the test runtime

**Worker:** worker1 **Goal:** the dashboard takes Dependabot #83's minor and patch updates plus `lucide-react` 1.x, `jsdom` 29 and `@types/node` 26, with every guarded behavior still green. **Status:** done

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Re-run #83's minor/patch half; separate real regressions from load flakes and fix or hold each — **DONE** | 1h  | —          |
| t002 | `lucide-react` 1.x with a shared `GithubIcon` (7 files, 16 uses) — **DONE**                               | 30m | t001       |
| t003 | `jsdom` 29 and `@types/node` 26 — **DONE**                                                                 | 30m | t002       |
| t004 | Simplify — **DONE**                                                                                       | 15m | t003       |
| t005 | Test coverage — **DONE**                                                                                  | 20m | t003       |
| t006 | Closeout — **DONE**                                                                                       | 10m | t005       |

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

## Outcome (2026-10-15)

Dependabot closed #83 and reopened the group as #92 (69 updates); this milestone took #92's versions. 53 minor/patch entries landed (resolved to the newest in range on 2026-10-15), plus `lucide-react` ^1.25.0 (resolves 1.57.0), `jsdom` ^29.1.1 and `@types/node` ^26.1.1. `eslint-plugin-react-hooks` stays at ^7.0.1 for `w1/m177`.

What the updates broke, and the fix for each:

- **`@tanstack/react-router` 1.170.36 types a route error as `unknown`** (it was `Error`): `error-page.tsx` read `error?.message`. It now reads a string `message` off any thrown value, as `isStaleChunkError` already did, and otherwise shows the generic message.
- **The Path-filter test** (m174's consistent failure) fails because React 19.3 renders once more on the blur's same-value `setPath("/api")` before the clear. A guard on the blur made the extra render vanish, which confirmed the cause. The card's settled query is still unfiltered, so this is test fragility, not a product change. The test now judges the newest call for both request metrics, and fails if the clear is broken (checked by mutating the clear handler).
- **The Back/Forward test** also failed on `main`'s own dependencies at load ~50 (3/3 runs). The cause is a cold, code-split route chunk outlasting `findBy`'s 1 s window. Teammate commit `fc4ff2b66` fixed it the same day by awaiting `router.load()`, so this milestone keeps that fix and drops its own preload.
- **jsdom 29** replaces Node's `Blob` with its own, which undici's `Request` reads as an empty body. That broke `readBoundedFormData` in 50 tests, though only inside jsdom. The three server-function test files that exercise it now run in Vitest's `node` environment, matching production.
- **jsdom 29** also drops the non-standard `-webkit-text-security` style, so the masked env-value assertion could not see it. `AutoTextarea` now masks with the class `[-webkit-text-security:disc]`; the production CSS carries `.\[-webkit-text-security\:disc\]{-webkit-text-security:disc}`.
- **lucide 1.0** dropped brand icons. `GithubIcon` (`src/common/components/github-icon.tsx`) is 0.562's ISC path data, byte-identical, built with `createLucideIcon`. It replaces all 16 uses in the 7 files.

Verification in the worktree, at load 40–70: two full `yarn test` runs passed (4962/4962, then 4969/4969 with the new tests). `yarn lint` (typecheck, ESLint, knip), `yarn install --immutable`, the critical `yarn npm audit` gate and `yarn build` all passed. Simplify: the reuse and efficiency reviews came back clean. From the quality review, the error-page tests now share one router helper, the icon test checks only the local contract, and the metrics test has one latest-call helper. Unifying `messageOf` with `report-route-error`'s `errorMessage` was skipped on purpose: the page needs text a user can read, while the log takes any identifying text. New tests: `github-icon.test.tsx`, and the five error-page message cases.
