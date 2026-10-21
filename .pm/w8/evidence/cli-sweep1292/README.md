# CLI sweep 1292 — free-form service names through the pinned CLI's selectors (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, bex-canary, HEAD-built pinned CLI. This exercises w5/m153 (free-form names on every create) from the CLI side.

- `bex services create --name "QA Free Form 53fde7 Café" --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2` → `srv-dbcdjksiimkc73f6l120`, `name` kept verbatim, `slug: qa-free-form-53fde7-caf` (the `é` is dropped rather than transliterated; non-Latin names are already w4/385), URL `https://qa-free-form-53fde7-caf.onbex.co`, `live` in 33 s.
- Name selectors resolve the free-form name: `services instances "<name>"`, `deploys list "<name>"` and `logs -r "<name>"` all exit 0 with data. The slug works as a selector too.
- `bex restart "<name>"` → `failed to restart resource: unknown resource type`. This is upstream behavior: `restart` takes a resource ID (its help says "Restart a service by resource ID") and dispatches on the ID prefix. `bex restart <srv-id>` is the supported form. Not filed.
- `bex restart <srv-id>` → exit 0.
- Small shape note: the create response omits the bex-extension `immutableName`, which the next `GET` reports as `qa-free-form-53fde7-caf`. It is an extension field, so not filed.
- Deleted (204 → 404). No new bug.
