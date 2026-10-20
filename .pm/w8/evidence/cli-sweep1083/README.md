# CLI sweep 1083 — closed-finding regression spot: w8/139 still fixed (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204; GET 404). No new bug filed.

- The w8/139 acceptance re-run on the current pin: hyphenated `allkeys-lru` create → 201, options read back `allkeys_lru` (and the list envelope shows the same). The fix holds after the pin settled.
