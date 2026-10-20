# CLI sweep 1092 — webhook PATCH semantics (live, 2026-10-20)

One disposable webhook endpoint created, patched, and deleted (204). No new bug filed.

- PATCH renaming and replacing `eventFilter` → 200; the read shows the new name and the filter set (stored sorted). A bogus event type → 400; downgrading the URL to `http://` → 400 (HTTPS-only holds on update, not just create).
