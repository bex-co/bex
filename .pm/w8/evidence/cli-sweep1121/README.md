# CLI sweep 1121 — `_method` tunneling has no effect (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `?_method=DELETE` → 400 (unsupported query parameter), POST with a `_method` body field → 405 (no POST on the resource path), and the service survives both. Verb tunneling through parameters or body fields does not exist.
