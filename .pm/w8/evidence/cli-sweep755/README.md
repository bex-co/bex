# CLI sweep 755 — maintenance-mode body dialects (live, 2026-10-19)

Disposable free web service `qa755-web` (`srv-dbbcmc54c21c738t03f0`). Deleted and verified absent (REST 404).

- `PATCH serviceDetails.maintenanceMode {enabled:true, uri:"https://…"}` on free → 400 `maintenanceMode requires a paid web service plan` (the sweep-663 gate fires before URI validation — the scheme-less URI got the same gate answer).
- `{enabled:"yes"}` (wrong type, no uri) → 400 `serviceDetails.maintenanceMode requires enabled and uri`.

No paid plan was submitted. No bug filed.
