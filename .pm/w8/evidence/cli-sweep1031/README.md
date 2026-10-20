# CLI sweep 1031 — deploy into a suspended service conflicts (live, 2026-10-20)

One disposable free web service created, suspended, resumed, and deleted (204). No new bug filed.

- Triggering a deploy while suspended → coded 409 `conflict: service "…" is suspended`. The refusal names the state; resume afterwards works normally.
