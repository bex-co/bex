# CLI sweep 878 — disk is create/subresource-only, not a PATCH field (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH {"serviceDetails":{"disk":{…}}}` → 400 `unknown field "disk"` — a disk attaches at create (the `serviceDisk` create field) or via `POST /disks` (paid-gated, sweep 804); it is not a mutable PATCH field, and strict decoding says so by name rather than ignoring the block.
