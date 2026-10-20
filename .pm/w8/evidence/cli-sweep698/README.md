# CLI sweep 698 — autoscaling read/write dialects (live, 2026-10-19)

Disposable free web service `qa698-web` (`srv-dbbbqnavfpec73dm0ks0`). Deleted and verified absent (REST 404).

- `GET /services/{id}/autoscaling` → 200 with the disabled default: `{"enabled":false,"min":0,"max":0,"criteria":{"cpu":{…0},"memory":{…0}}}`.
- Flat/wrong shapes (`targetCPUPercent`, unknown keys) → 400 `invalid request body at /criteria` (resp. `/enabled`); omitting the `memory` criterion → 400 at `/criteria/memory` (both criteria objects are schema-required).
- Fully valid enabled body on free → 400 `autoscaling requires a paid instance type` (the sweep-635 gate, after schema validation). No paid plan submitted.
- `DELETE /services/{id}/autoscaling` → 204.

No bug filed.
