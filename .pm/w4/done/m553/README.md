# w4 · m553 — The weekly Render drift job reads Render's changelog and fails on an entry with no disposition

**Worker:** worker4 **Goal:** every entry on Render's changelog has a recorded disposition (an ADR018 row, an owning board item, a non-goal or n/a), and the weekly Render drift workflow fails when a new entry has none, so a dashboard-only Render feature can no longer go unnoticed. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | scripts/render-changelog-drift.sh: Render's changelog feed against a disposition table — **DONE** | 1h | — |
| [t002](done/t002.md) | Disposition every Render changelog entry since 2023-03-31 — **DONE** | 2h | t001 |
| [t003](done/t003.md) | Run the changelog check in the weekly Render drift workflow — **DONE** | 30m | t001 |
| [t004](done/t004.md) | ADR018's Method names the changelog check — **DONE** | 15m | t002 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t003, t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 4h40m**.

## Definition of done

- `scripts/render-changelog-drift.sh` exits 0 against Render's live feed with the committed table, and non-zero, naming the entry, against a fixture feed with an unknown entry; `scripts/render-changelog-drift.test.sh` runs in `scripts.yml`.
- `docs/render-artifacts/render-changelog.md` has exactly one row for every entry on render.com/changelog since 2023-03-31, and every gap row names a board item.
- The weekly `render-schema-drift.yml` workflow runs the check, and a run on `main` is green.
- ADR018's Method paragraph names the check.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-20, item 4. Filed by `/pmx them to w4`.
- **Evidence:** the weekly drift jobs (`scripts/render-schema-drift.sh`, `scripts/render-mcp-drift.sh`, `.github/workflows/render-schema-drift.yml`, cron `17 6 * * 1`) watch only API contracts. The same brainstorm found three dashboard-only Render features with no owner, now w4/m550–m552: bulk suspend/resume (2025-08-27), Postgres admin apps (2024-06-04) and the log line detail view (2025-12-17). Render publishes an Atom feed at `https://render.com/changelog/feed.xml` (HTTP 200, `application/atom+xml`, the latest 10 entries, each `<id>` the entry URL; checked 2026-11-20).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); it makes [ADR018](../../../../docs/ADR018-render-parity.md)'s coverage checkable for the surfaces no API pin sees.
- **Expected outcome:** a new Render feature turns the weekly job red within a week, with its entry named, instead of waiting for a brainstorm to stumble on it.
- **Why now:** Render published four changelog entries in November 2026 alone, and the three gaps above went 11 to 29 months unnoticed.
- **Render parity:** omitted: repository tooling and docs only, with no REST, GraphQL, MCP or UI change.

## Closeout (2026-11-22)

The definition of done, replayed:

- `scripts/render-changelog-drift.sh` against Render's live feed and page, with the committed table: exit 0, `Render changelog: all 175 entries dispositioned (10 in the feed, 175 on the page)`, on macOS and in `buildpack-deps:bookworm-scm`. Against the fixture feed with an unknown entry: exit 1, `no disposition: 2026-11-20 Bulk suspend services <https://render.com/changelog/bulk-suspend-services>`. `scripts/render-changelog-drift.test.sh` (23 cases) runs in `scripts.yml`.
- `docs/render-artifacts/render-changelog.md` has one row for each of the 175 entries from 2023-03-31 to 2026-11-18: 114 matched, 18 partial, 5 gap, 18 non-goal, 20 n/a. The script refuses a gap row without a board item. The five gaps no item owned are filed as w4/717–w4/721 (`c2e2f076e3`).
- Dispatched run [45095344162](https://github.com/bex-co/bex/actions/runs/45095344162) of `render-schema-drift.yml` on `main` (`0ae12fe0ca`): the `render-changelog` job succeeded. In the same run, `render-schema` failed on upstream drift no board item owns yet: Render's OpenAPI operations and schemas moved, and `render-oss/cli` v2.38.0 is past the v2.36.0 pin. It was already red on the 2026-11-16 schedule, so it predates this milestone.
- ADR018's Method paragraph names the table, the `render-changelog` job and the script.
