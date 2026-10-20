# CLI sweep 663 — maintenance-mode plan gate (live, 2026-10-19)

Disposable free web service `qa663-web` (`srv-dbbbfmlfsg2s73ako7r0`), live at HTTP 200 before the probe. Deleted and verified absent (REST 404).

`bex services update <id> --maintenance-mode` on the free plan → 400 `maintenanceMode requires a paid web service plan`. The refusal is named, the service kept serving 200, and `maintenanceMode.enabled` stayed `false` on readback. No paid plan was submitted.

No bug filed.
