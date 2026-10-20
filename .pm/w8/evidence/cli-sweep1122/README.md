# CLI sweep 1122 — HEAD and OPTIONS on read routes (live, 2026-10-20)

No mutation surface exercised. No new bug filed.

- HEAD on the services list and an owner read → 200 with an empty body (standard Go server behavior: the GET handler runs and the body is suppressed).
- OPTIONS on the list → 204, consistent with the CORS preflight handling from sweep 1086.
