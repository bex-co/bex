# CLI sweep 1181 — image PATCH validates references (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed. (A first probe script had a syntax error and never sent a request; the redo above is authoritative.)

- PATCH `image.imagePath` to another public image → 200, read-back shows the new reference.
- Empty `imagePath` → 400; a garbage reference → 400 `image reference is malformed: invalid reference format` — references are parsed, not stored blind.
