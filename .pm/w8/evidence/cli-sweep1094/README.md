# CLI sweep 1094 — dryRun create validates without persisting (live, 2026-10-20)

No fixture persisted (verified by name list). No new bug filed.

- `POST /services?dryRun=true` with a valid free image body → 200 with a preview service object (its `id` is the would-be name, not a minted `srv-` id), and the follow-up name list is empty. The bex `dryRun` query extension (registered in `renderQueryExtensions`) exercises the full create validation path without writing.
