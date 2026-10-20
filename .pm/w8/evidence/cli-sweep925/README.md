# CLI sweep 925 — service read-object key inventory (live, 2026-10-20)

Disposable free web service created, read, deleted (204, then GET 404). No new bug filed.

- The single-GET read object carries both identity forms (`id`, `immutableName`, `slug` via the list shape) and both owner forms (`owner` object and `ownerId`), plus the dual notification fields — the merge-safe superset the stable-id transition documented (render.go's "every consumer keeps the name they already read" rule).
- A same-name create in a second workspace was deliberately **not** run: this loop mutates only `bex-canary`. Cross-workspace name scoping rests on sweep 855's availability reads plus sweep 856's same-workspace 409.
