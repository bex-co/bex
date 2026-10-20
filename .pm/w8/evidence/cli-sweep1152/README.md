# CLI sweep 1152 — autoscaling reads disabled-by-default and refuses on free (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `GET /services/{id}/autoscaling` → 200 with the full disabled shape (`enabled: false`, `min/max: 0`, cpu/memory criteria present but disabled) rather than a 404 or empty body: clients can render the feature state on any plan.
- `PUT` enabling autoscaling on the free plan → 400. The write gate fails closed; the read shape is unaffected.
