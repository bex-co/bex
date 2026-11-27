# w3 · m637 — Render's per-language version pages are watched daily

**Worker:** worker3 **Goal:** when Render adds, moves or back-dates a default version on any of its seven per-language version pages, a daily job fails within a day, naming the page and the bex table that no longer matches. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Parse Render's version pages](done/t001.md) — **DONE** | 60m | — |
| t002 | [Compare with bex's tables in a Go test](done/t002.md) — **DONE** | 45m | — |
| t003 | [Daily drift job](done/t003.md) — **DONE** | 40m | t001, t002 |
| t004 | [Fixture self-tests](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Record the method](done/t005.md) — **DONE** | 20m | t003 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t004, t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t004, t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

## Definition of done

1. The daily job passes against Render's live pages with an empty exceptions list (after [m636](../m636/README.md)).
2. Fixture mutations each fail it with a message naming the page and the bex table: a moved current default, a new row, a back-dated row, a removed page, changed markup and an eighth page.
3. The job is scheduled daily, listed in `scripts/ci-schedule-drift.sh`, and its self-tests run in `scripts.yml`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 2. Render moved its native defaults on 2026-09-09, 09-17, 10-12, 11-03, 11-19 and 11-20 without a changelog entry (its newest defaults entry is 2026-10-22, row 31 of [render-changelog.md](../../../../docs/render-artifacts/render-changelog.md)). `scripts/render-changelog-drift.sh` reads only the changelog; nothing in the repo fetches the version pages.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); keeps ADR018's runtime row true without relying on chance.
- **Expected outcome:** a Render default move is on the board within a day. Today the 11-03 moves were found on 11-13 (w4/620), the 11-19 Ruby move by chance (w9/138), and services created on bex from 10-12 to 11-14 got a uv Render never gave them (w8/328).
- **Why now:** six silent moves since September and three in November; after m636 lands, bex's tables match Render's in full, and this keeps them matching.
- **Sequence:** file after m636, so the comparison starts with no exceptions.
- **Render parity omitted:** CI and docs only; no REST, GraphQL, MCP or dashboard change.

## Implementation — 2026-11-26 (t001–t007)

Render's seven pages, read 2026-11-26, carry the rows this milestone was filed with (Bun 1.4.5 from 2026-11-20, Ruby 3.4.13 from 2026-11-19, uv 0.12.24 from 2026-10-12), and since [m636](../m636/README.md)'s `41ff57632f` every bex table matches them with no exception.

- **Parser (t001):** `scripts/lib/render_version_pages.py`, run by `scripts/render-version-drift.sh` (`parse` prints the JSON). It takes the page list from `render.com/docs/llms.txt` (every `/docs/*-version(s).md` link), reads each page's `.md` copy and emits, keyed by page, the header's current default(s), the day the header dates them from, and the history rows. It fails naming the page on an eighth version page, a page llms.txt stops listing or that cannot be fetched, and markup it does not read: no `Service Creation Date` table, a row that is not `<date> | `<version>``, an `and later` or `Before` row out of place, rows out of date order, a version it does not recognise, or a header default or date the newest row does not repeat. uv's `Before 2025-06-12` row under a 2026-02-11 row is read as written; no comparison reaches it.
- **Comparison (t002):** `TestRenderVersionDrift` in `lego/operator/internal/build/render_version_drift_test.go` reads the JSON from `BEX_RENDER_VERSION_PAGES` (skips when unset) and compares, from the row in force on 2026-06-05, Node, Python, Ruby, Bun, uv and Elixir row by row with `exactDefaults`, `toolchains.PythonDefaults`, `bunDefaults` and `uvDefaults`; Erlang/OTP's header release and date with `erlangDefaults`' newest step (the page has no Erlang table; `28.0.2` and `28.0.2.0` compare as one release); Poetry's current default with `defaultPoetryVersion`. Each difference names the page, the row and the table: a moved current default, a new row, a back-dated row, a removed row, a page with no table, a table with no page. `renderVersionDriftExceptions` is empty; an entry must name its ADR018 row, and one that covers no difference fails.
- **Daily job (t003):** `.github/workflows/render-version-drift.yml`, cron `29 5 * * *` and `workflow_dispatch`, on `ubuntu-24.04-arm` with setup-go from `lego/operator/go.mod`; `scripts/ci-schedule-drift.sh` gives it the daily 40 h window. Each difference is a `::error` annotation. Run locally against the live pages on 2026-11-26: `every default from 2026-06-05 on matches bex's tables (7 pages)`.
- **Self-tests (t004):** `scripts/render-version-drift.test.sh`, in `scripts.yml` and its shellcheck list, over the seven pages saved byte for byte with a trimmed `llms.txt` in `scripts/fixtures/render-version-pages/` (the repository's fixture directory, not `scripts/testdata/`; excluded from prettier in `.prettierignore`). The real pages pass; a moved current default, a new row, a back-dated row, a moved Erlang/OTP default, a removed or unfetchable page, an eighth page, four kinds of changed markup and an unreachable llms.txt each fail with the message the test expects, and a change to a row older than any bex service passes. The check runs the Go comparison, so `scripts.yml` now sets up the operator's Go before that step (same cache key as `operator-test.yml`). The file's header gives the refresh command for when bex follows a Render move.
- **Docs (t005):** ADR018's Method paragraph, ADR060 D7 (a new bullet) and its runtime-lines addendum, and the intro of `docs/render-artifacts/render-changelog.md` name the job. The addendum's Elixir and uv rows still gave the cutoffs m636 removed; they now give Render's dated rows.
- **Simplify (t006):** the parser stops at an unreadable llms.txt instead of fetching every page as well, and two checks the comparison already made were dropped.
- **Tests (t007):** `TestRenderVersionDriftNamesEachKindOfMove` (12 cases on pages built from bex's own tables), `TestRenderVersionDriftIgnoresRowsNoBexServiceFallsIn` and `TestRenderVersionExceptions`, beside the shell cases above. Mutation spot checks, each reverted: skipping the eighth-page check fails the self-test's eighth-page case; accepting any version string fails its `lts` case; dropping the back-dated branch fails the Go back-dated case and the self-test's.

## Closeout — 2026-11-26 (t008)

Shipped in `c4431e3a1f`. A dispatched run of `render-version-drift.yml` on main against Render's live pages passed with the empty exception list ([run 45880011945](https://github.com/bex-co/bex/actions/runs/45880011945): `every default from 2026-06-05 on matches bex's tables (7 pages)`), so the definition of done holds: the job is scheduled daily and listed in `scripts/ci-schedule-drift.sh`, and its self-tests, each fixture mutation failing by name, run in `scripts.yml`.
