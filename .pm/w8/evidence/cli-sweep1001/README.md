# CLI sweep 1001 — plan PATCH stays on the refusal side (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- PATCH plan to the same `free` → 200 no-op. PATCH to a bogus plan → coded 400 enumerating the plan vocabulary; the service still reads `free` afterwards.
- Per the sweep-777 lesson, no real paid plan value was ever submitted — paid transitions succeed by design and are never used as a "probably refused" probe.
