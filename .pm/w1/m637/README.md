# w1 · m637 — Render's per-language version pages are watched daily

**Worker:** worker1 **Goal:** when Render adds, moves or back-dates a default version on any of its seven per-language version pages, a daily job fails within a day, naming the page and the bex table that no longer matches. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Parse Render's version pages | 60m | — |
| t002 | Compare with bex's tables in a Go test | 45m | — |
| t003 | Daily drift job | 40m | t001, t002 |
| t004 | Fixture self-tests | 45m | t003 |
| t005 | Record the method | 20m | t003 |
| t006 | Simplify | 15m | t004, t005 |
| t007 | Test coverage | 30m | t004, t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

1. The daily job passes against Render's live pages with an empty exceptions list (after [m636](../m636/README.md)).
2. Fixture mutations each fail it with a message naming the page and the bex table: a moved current default, a new row, a back-dated row, a removed page, changed markup and an eighth page.
3. The job is scheduled daily, listed in `scripts/ci-schedule-drift.sh`, and its self-tests run in `scripts.yml`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 2. Render moved its native defaults on 2026-09-09, 09-17, 10-12, 11-03, 11-19 and 11-20 without a changelog entry (its newest defaults entry is 2026-10-22, row 31 of [render-changelog.md](../../../docs/render-artifacts/render-changelog.md)). `scripts/render-changelog-drift.sh` reads only the changelog; nothing in the repo fetches the version pages.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); keeps ADR018's runtime row true without relying on chance.
- **Expected outcome:** a Render default move is on the board within a day. Today the 11-03 moves were found on 11-13 (w4/620), the 11-19 Ruby move by chance (w9/138), and services created on bex from 10-12 to 11-14 got a uv Render never gave them (w8/328).
- **Why now:** six silent moves since September and three in November; after m636 lands, bex's tables match Render's in full, and this keeps them matching.
- **Sequence:** file after m636, so the comparison starts with no exceptions.
- **Render parity omitted:** CI and docs only; no REST, GraphQL, MCP or dashboard change.
