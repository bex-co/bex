# CLI sweep 969 — env-group link semantics + periodic findings check (live, 2026-10-20)

Five disposable services and five env groups created across probes; all deleted (204s). No new bug filed.

- Periodic spot check at the unchanged pin `3ede47d3c5a9`: w8/137 → 405, w8/138 → override keys still lack `serviceId`, w8/139 → 400. All three stay open.
- Env-group link route is `POST /env-groups/{id}/services/{serviceId}` with **no body** (an empty JSON body gets the coded body-not-allowed refusal; the route table confirms the shape). Link → 200, unlink → 204.
- After linking, the group's GET shows `serviceLinks: [{id, name, type}]` and the link is authoritative from the group side. The service's own env-vars list still shows only its own vars — group vars are a separate resource applied at the runtime layer, not merged into the service's env list (same separation as Render; recorded, not filed).
- Group GET withholds var values (`envVars: [{key}]` only); values are read per-key.
