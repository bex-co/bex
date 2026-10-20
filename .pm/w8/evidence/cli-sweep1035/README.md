# CLI sweep 1035 — post-fix regression: both KV enums hyphenated at once (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204). No new bug filed.

- On the new pin, a single create carrying **both** hyphenated spellings (`allkeys-lru` + `journal-snapshot`) → 201, reading back both normalized underscore forms in `options`. The w8/139 fix holds for the combined case, not just one field at a time.
