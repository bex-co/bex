# CLI sweep 1209 — healthCheckPath must be an absolute path (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `healthCheckPath: /healthz` → 200 and reads back; the relative form `healthz` → 400 (leading slash required); empty string → 200 (clears the override, treated as unset like the empty rename in sweep 1168).
