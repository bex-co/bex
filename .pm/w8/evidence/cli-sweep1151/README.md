# CLI sweep 1151 — maintenance mode is PATCH-only and paid-gated (live, 2026-10-20)

Three disposable free web services created and deleted (204, verified 404). No new bug filed.

- `PUT /services/{id}/maintenance-mode` → 404: no dedicated subroute exists. The control is `serviceDetails.maintenanceMode` on the service PATCH.
- Enabling with an empty `uri` → 400 (the pair `enabled`+`uri` is required together); enabling with a uri on the free plan → 400 `maintenanceMode requires a paid web service plan`, and read-back stays `{enabled: false, uri: ""}`.
- Disabling (`enabled: false, uri: ""`) → 200.

The paid feature fails closed with an explicit plan refusal; no state changed and nothing was charged.
