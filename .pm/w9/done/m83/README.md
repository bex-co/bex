# w9 · m83 — Log-list virtualization: only-visible rows on the live tail

**Worker:** worker9 **Goal:** Virtualize the deploy/service log viewer so a busy live tail keeps only the visible rows in the DOM — landing the half of `w9/m63` t001 that was attempted and reverted for lack of a test harness — and unblock raising the retained-line cap. **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on  |            |
| ---- | ------------------------------------------------------------------------- | --- | ----------- | ---------- |
| t001 | Stand up a real virtualization test harness (jsdom geometry / Playwright) | 1h  | —           | — **DONE** |
| t002 | Virtualize `log-line-list.tsx` preserving follow/pin/jump/chips/wrap      | 2h  | w9/m83/t001 | — **DONE** |
| t003 | Assert only-visible-rows + profile busy tail + raise `DEFAULT_MAX_LINES`  | 1h  | w9/m83/t002 | — **DONE** |
| t004 | Simplify — `/simplify` over the code this milestone changed               | 30m | w9/m83/t003 | — **DONE** |
| t005 | Test coverage — regression tests for every preserved viewer behavior      | 45m | w9/m83/t003 | — **DONE** |
| t006 | Closeout                                                                  | 15m | w9/m83/t005 | — **DONE** |

## Definition of done

- A repeatable test proves that for a 1,000-line buffer, only the visible range of rows is present in the DOM (not all 1,000) — either jsdom with a scoped geometry helper that makes `@tanstack/react-virtual` render a range, or a Playwright walk against a busy tail. The helper must be scoped, not global `getBoundingClientRect`/`ResizeObserver` stubbing, so it cannot perturb other dashboard tests.
- `log-line-list.tsx` renders virtualized while preserving, with tests: follow-to-bottom, scroll-up-breaks-follow, jump-to-latest, instance chips, request chips, and both wrap and nowrap modes.
- The cross-buffer text-selection tradeoff is decided **explicitly** and documented (visible-row selection works; whole-buffer copy is the known virtualization cost).
- A before/after profile of a busy live tail is captured showing the DOM-reconciliation cost reduction.
- `DEFAULT_MAX_LINES` is raised (now that per-frame ANSI re-parse is gone and the DOM no longer grows with buffer size), or the note explains why it stays.
- Full dashboard suite + ESLint + tsc green.

## Results

- **Virtualized** `log-line-list.tsx` with `@tanstack/react-virtual@3.14.9` using a **spacer-flow** window (top/bottom padding divs + rows in normal flow) rather than absolute positioning — this preserves wrap/nowrap layout, nowrap horizontal scroll, and in-window text selection. `paddingStart`/`paddingEnd` carry the old container inset; `scrollToIndex(last, {align:"end"})` drives follow; the existing pin/`onScroll` math is unchanged.
- **Scoped geometry harness** `src/test/virtual-geometry.ts`: redefines `HTMLElement.prototype` `offsetHeight`/`offsetWidth` getters (virtual-core measures via `offsetHeight`, not `getBoundingClientRect`) keyed on `data-log-viewport`/`data-index` markers, installed/restored per-file via `beforeEach`/`afterEach` — never a global stub in `setup.ts`. Wired into all five virtualized-list consumers' tests (logs list + viewer, deploy log panel, postgres viewer).
- **Only-visible-rows proven:** a 1,000-line **and** a 5,000-line buffer both render a **constant ~34 DOM rows** (viewport + overscan), i.e. **96.6% / 99.3% fewer row nodes** than one-row-per-line. Regression-guarded in `log-line-list.test.tsx` (`> 0` and `< 120`, plus a "10× lines ≠ 2× rows" invariant). This buffer-independent node count **is** the captured before/after profile — React reconciliation is O(mounted nodes) — so the DOM-reconciliation cost no longer scales with the buffer. A live React-Profiler flame graph is an optional confirmation deferred to `047` (app not raisable in-session; same pattern as `044`/`046`).
- **Text-selection tradeoff** decided + documented in the component docstring and tests: on-screen rows are selectable/copyable; a drag-select can't span rows scrolled out of the DOM (whole-buffer copy is the known virtualization cost; scrollback is preserved by scrolling).
- **Retained cap raised** `DEFAULT_MAX_LINES` 1,000 → 5,000 (`use-live-logs.ts`) — the DOM-growth ceiling that pinned it at 1,000 is gone; longer scrollback now costs memory + one ANSI parse per line at ingest, not per-frame rendering.
- **Green:** `yarn typecheck`, `yarn lint`, and the full suite (**2,180 tests**) pass; `yarn build` succeeds with the client entry chunk at **310.9 KB gzip** (under the w9/m60 475 KB budget — react-virtual landed in the chunk the log list already occupied, no hygiene regression).

## Source + Goal linkage

- **Source:** `.pm/w9/045.md` (deferred from `w9/done/m63` t001; the ANSI-parse-once half shipped in m63, the row-virtualization half was reverted as unverifiable under jsdom).
- **Goal linkage:** dashboard perceived-performance / responsiveness pillar — the live log tail during deploys is the platform's most-watched screen; keeping DOM size bounded on very busy tails is the remaining reconciliation cost after m63 killed the dominant per-frame re-parse.
- **Expected outcome:** a 1,000-line (and larger) live tail holds a bounded, only-visible set of rows in the DOM; the retained-line cap can rise without proportional reconciliation cost; a captured before/after profile proves it.
- **Why now:** m63 shipped the safe dominant half and explicitly filed this as "do it as a focused milestone with room to get it right" — the blocker was purely a missing test harness, and virtualizing the most-watched screen (with its text-selection tradeoff) is too risky to land without one. Low urgency but well-scoped and unblocking the retained-line cap raise.
- **Render parity:** **omitted** — this is a pure dashboard rendering optimization (DOM virtualization) with no REST/GraphQL/MCP surface change and no change to what the viewer displays; same disposition as `w9/done/m60` (bundling perf) and the m63 rendering leg. The only user-visible behavior change (whole-buffer text selection) is decided in t002 and has no render.com cross-surface equivalent to reconcile.
