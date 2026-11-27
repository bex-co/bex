# w4 · m688 — Every Render docs page has a disposition, and a weekly job fails when Render adds, removes or rewrites one

**Worker:** worker4 **Goal:** Render's documentation is watched like its changelog and its dashboard bundle: every page in `render.com/docs/llms.txt` has one disposition in a ledger, and a weekly job fails, naming the page and its pointer, when Render adds, removes or rewrites one. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | `scripts/render-docs-drift.sh` reads `llms.txt` and every page, and compares hashes with the ledger | 1h30m | — |
| [t002](t002.md) | The ledger: one row per page with its disposition; read every product page with at most one reference and file each unowned gap | 3h | t001 |
| [t003](t003.md) | A weekly `render-docs` job, and the ledger in ADR018's method paragraph | 45m | t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 30m | t003, t004 |
| [t006](t006.md) | Closeout | 10m | t005 |

Total: **6 tasks, about 6h10m**.

## Definition of done

- `docs/render-artifacts/docs-pages.md` has exactly one row per page in `llms.txt`, each with a disposition, a pointer and a normalized hash; every `gap` names an open or parked board item.
- Every product page that had at most one reference on 2026-11-26 was read in full, and each unowned gap it showed is filed.
- `scripts/render-docs-drift.sh` fails on a new page, a gone page and a changed page, and passes on a banner-only change, as its `.test.sh` proves.
- A weekly `render-docs` job runs it, a dispatched run on `main` passes, and ADR018 names the ledger and the job.
- No page body is committed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the aspect: what Render publishes outside its API pins that bex's parity ledger never read — its docs pages, its pricing page, and the Cloudflare headers its docs tell apps to use), filed to w4 by `/pm them to w4 and ship`. Re-checked at `origin/main` `f32c8164fb` before filing: no open, blocked or done item on any workstream owns it. Item 2.
- **Evidence:** 227 pages on 2026-11-26; 12 product pages had no reference anywhere in the repository and 31 at most one. Render changes behavior on docs pages without a changelog entry: ADR018's method paragraph lists six silent changes on the runtime version pages alone (2026-09-09 to 11-20), which is why w3/m637 watches those seven pages. The other pages have no monitor.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): parity reached stays parity when Render changes a page.
- **Expected outcome:** a Render docs change reaches the board within a week, with the ADR018 row to re-read, instead of surfacing in a customer report.
- **Why now:** the changelog (w4/m553) and dashboard bundle (w1/m645) ledgers are done, so the docs pages are the last Render source no job reads; one brainstorm's reading of three unread pages found two gaps.
- **Render parity:** omitted: this is tooling and documentation with no REST, GraphQL, MCP or dashboard change.
