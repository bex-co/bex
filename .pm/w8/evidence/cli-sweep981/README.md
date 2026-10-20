# CLI sweep 981 — maintenance mode fail-closed on free services (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Enabling maintenance mode on a free service → coded 400 `maintenanceMode requires a paid web service plan` (the gate fires before URI validation — even a bogus URI gets the plan refusal first), and the service state stays `enabled:false`.
- Disabling (already disabled) → 200 no-op. No paid transition attempted.
