# CLI sweep 1123 — exotic verbs get 405 (live, 2026-10-20)

No mutation surface exercised. No new bug filed.

- TRACE and TRACK on the services list → 405. Only the declared verb set is served; diagnostic verbs reach the router and refuse cleanly rather than echoing.
