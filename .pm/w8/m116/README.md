# w8 · m116 — Return backward log pages most-recent first

**Worker:** worker8 **Goal:** `direction=backward` (the default) log pages list the most recent line first, as Render's API contract states. **Status:** todo

## Tasks (in order)

| id   | title                                                      | est | depends_on |
| ---- | ---------------------------------------------------------- | --- | ---------- |
| t001 | Order backward pages newest-first at the shared page verb | 40m | —          |
| t002 | Audit every log reader for order assumptions               | 30m | t001       |
| t003 | Verify Render parity across log surfaces                   | 25m | t002       |
| t004 | Simplify the ordering change                               | 15m | t003       |
| t005 | Test direction, paging, and hasMore end to end             | 40m | t003       |
| t006 | Accept the hosted replay and close milestone               | 20m | t004, t005 |

**Total:** 6 tasks / 170m.

## Definition of done

For a service with two planted lines A (older) and B (newer): `GET /v1/logs?direction=backward&limit=2` returns `[B, A]`; `direction=forward` returns `[A, B]`; the pinned CLI's `-o json` and `-o text` agree with the wire order per direction; cursor paging in both directions visits every line exactly once with no repeats or gaps, and `hasMore=true` is never followed by an empty page for these fixtures. Multi-resource and multi-instance merges keep one global order per direction; same-timestamp groups stay whole across page cuts. Dashboard history rendering is unchanged in appearance (no double reversal). Fail-before/pass-after tests exist at the page-verb layer and the hosted replay passes with owned-fixture cleanup.

## Source + Goal linkage

- **Source:** functional native CLI sweep465 (2026-10-18, `bex-canary`, human QA login): two planted app lines return oldest-first under both CLI directions and at the REST wire, while page **selection** and cursors are correct (backward `limit=1` walks newest→oldest and terminates). Only the within-page order diverges. Sanitized receipts in the sweep465 evidence record.
- **Goal linkage:** [ADR018](../../../docs/ADR018-render-parity.md) (the [CLI checklist](../../../docs/cli-compatibility-checklist.md) grades `--direction` working) and [ADR010](../../../docs/ADR010-observability.md). Render's [List logs](https://api-docs.render.com/reference/list-logs) oracle: "Backward will return most recent logs first. Forward will start with the oldest logs in the time range." Bex's own [live-deploy-following artifact](../../../docs/render-artifacts/live-deploy-following.md) likewise describes pages as "newest first".
- **Expected outcome:** `bex logs` (default direction) and every API consumer sees newest first, matching Render tooling expectations and the checklist grade.
- **Why now:** the default read of the most-used debugging surface silently diverges from the contract the pinned client was graded against; anything consuming `-o json` programmatically gets reversed pages.
- **Render parity:** included — REST/GraphQL/MCP share the page verb and the dashboard renders the same reads. Preserve envelope fields, cursor semantics, and entry identity; only entry order (and any lying `hasMore`) changes.
