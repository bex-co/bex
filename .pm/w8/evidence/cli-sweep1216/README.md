# CLI sweep 1216 — branch PATCH on an image service is stored, not rendered (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `branch: main` on an image-backed service → 200, yet the read shape carries no `branch` value: with no repo attached there is nothing to render it against. This matches the earlier finding that branch renders only once a repo exists — accepted at write, absent at read until it is meaningful.
