# w9 · m60 — Entry-chunk diet: strip the ~500 KB of dead weight route-splitting can't touch

**Worker:** worker9 **Goal:** shrink the dashboard entry chunk (post-w5/m67: ~5.5 MB raw / ~1.18 MB gzip) by removing the always-mounted dead weight that route code-splitting cannot reach — the full lucide-react icon barrel, dead i18n `description` strings, the eagerly-bundled non-default locale, and the react-intl stack — then run the analyzer-driven entry-attribution pass (absorbing `w5/043`) and add guardrails so the win sticks **Status:** done

## Results (measured 2026-08-16, apples-to-apples via stash-build of the pre-m60 tree)

**Initial JS (entry + eager vendor chunks): 1,148 → 475 kB gzip (−673 kB, −58.6%)** — far beyond the ≥400 kB target; the lucide barrel (t001) was the dominant win, as predicted. Total client JS is essentially unchanged — the removed weight (icons, descriptions, zh, react-intl) is gone, and the remainder is now split into cache-stable vendor chunks. Entry chunk alone: 1,148 → 305 kB gzip. Spot-checks all pass: unused lucide icons (`Sailboat`/`Banana`) absent; locale `description` strings absent from every chunk; zh catalog absent from the entry (only the `中文` language _name_ remains, correctly) and living in its own `resources-zh` async chunk; `IntlProvider`/formatjs absent from the entry. t009 attribution: **every** m67-allowlisted route page already splits into its own `?tsr-split` chunk (none entry-pinned), and every listed heavy lib (`@xterm`/`@tiptap`/`ai`/`@ory/elements-react`/markdown stack) is confirmed absent from the entry — nothing left to split, so the guard allowlist is unchanged.

## Tasks (in order)

| id   | title                                                                  | est | depends_on             |
| ---- | ---------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Replace the lucide-react namespace import with a static 5-icon map — **DONE** | 30m | —              |
| t002 | Strip locale `description` fields from the runtime bundle — **DONE**   | 45m | —                      |
| t003 | Lazy-load the non-default locale (zh) instead of bundling both eagerly — **DONE** | 45m | t002        |
| t004 | Lazy-mount OryToaster so react-intl/formatjs leaves the entry chunk — **DONE** | 30m | —              |
| t005 | Bundle guardrails: visualizer, vendor manualChunks, no public sourcemaps — **DONE** | 45m | t001, t002, t003, t004 |
| t009 | Entry attribution: verify route pages + libs (all already split) — **DONE** | 1h  | t005              |
| t006 | Simplify — **DONE**                                                    | 20m | t005, t009             |
| t007 | Test coverage — **DONE**                                               | 30m | t005, t009             |
| t008 | Closeout — **DONE**                                                    | 10m | t007                   |

## Definition of done

- The production entry chunk no longer contains: unused lucide icons (spot-check `Banana`/`Sailboat`/`Panda` absent), any locale `description` string (spot-check a known dev-note string absent), Chinese catalog strings for an English session, or `IntlProvider`/formatjs references.
- Measured before/after entry-chunk size (raw + gzip) recorded in this README against the post-m67 baseline (~5.5 MB raw / ~1.18 MB gzip); expected combined reduction ≥ ~400 KB gzip.
- The analyzer attribution (t009) has dispositioned every remaining graph-pinned route page from m67's allowlist (split or documented why not) and confirmed the heavy per-feature libs (`@xterm/*`, `@tiptap/*`, `ai`/`@ai-sdk/react`, `@ory/elements-react`, the markdown stack) are absent from the entry chunk.
- A dev-only bundle visualizer is wired (`yarn build` can emit a treemap) and vendor code is split via `manualChunks` so a deploy doesn't invalidate the React/Radix/Apollo cache.
- Production `.map` files are no longer served publicly (`sourcemap: "hidden"` or equivalent), with the change's debugging implication noted in `dashboard/CLAUDE.md`.
- No behavior change: all dashboard tests pass, `EmptyState` renders every icon name actually used app-wide, zh sessions still render fully translated after the lazy-load.

## Source + Goal linkage

- **Source:** perf sweep 2026-08-16 (follow-on to the w5/m67–m69 brainstorm, handed to w9 by user direction); **absorbs `w5/043`** (m67's own follow-up note calling for the analyzer/attribution/vendor-split/budget pass — this milestone is that work plus the concrete dead weight the sweep already attributed). Evidence from the pre-m67 production build `dashboard/.output/public/assets/index-*.js` (5.95 MB raw / 1.31 MB gzip; ~5.5 MB / ~1.18 MB after m67): `common/components/empty-state.tsx:1` does `import * as Icons from "lucide-react"` + dynamic index, shipping ~1,900 icons for the 5 names ever used; `src/i18n/index.ts` statically imports all 26 en + 26 zh namespaces and every entry's dead `description` field (~305 KB source); `root-provider.tsx:2` imports `react-intl` solely for `OryToaster`; `vite.config.ts` has no `manualChunks`, no visualizer, and `sourcemap: true`; per m67, ~21 route pages remain exported with graph-dependent entry pinning (`no-new-route-component-export.test.ts` allowlist).
- **Goal linkage:** ADR008 vision — the dashboard is the human surface of the Render alternative; the entry chunk gates every first load, and none of this weight is reachable by w5/m67's route splitting (it all sits in the always-mounted provider tree).
- **Expected outcome:** ~400–500 KB gzip off every user's initial JS, cache-stable vendor chunks across deploys, and a visualizer + guard so regressions are visible in review instead of accreting silently.
- **Why now:** w5/m67 just shipped (2026-08-16, entry 1,312 → 1,176 kB gzip) and filed `w5/043` for exactly this continuation while the measurement context is hot; the lucide fix alone (one file) likely outweighs all of m67's delta.
- **Render parity task:** **omitted** — pure bundling/load-performance work with no REST/GraphQL/MCP/UI behavior or data-surface change to compare against render.com.
