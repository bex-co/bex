# CLI sweep 1099 — service list cursor walk visits each once (live, 2026-10-20)

Read-only cursor walk over the canary service list. No new bug filed.

- Walking with `limit=3` and following the returned cursors visits all 7 services exactly once (7 walked, 7 unique, no repeats, no empty trailing page needed to terminate).
