# CLI sweep 1172 — Key Value lives at `/key-value`; w8/139 hyphen enums hold (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204, verified 404 and absent from the list). No new bug filed.

- `/keyvalue` and `/kv` → 404; the declared base is `/key-value`.
- Bogus `maxmemoryPolicy` → 400 at `/maxmemoryPolicy`; bogus `persistenceMode` → 400.
- The Render hyphen form `allkeys-lru` → 201 and reads back normalized as `allkeys_lru` in `options` — the w8/139 fix still holds at the current pin.
