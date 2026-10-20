# CLI sweep 770 — datastore metrics paths (live, 2026-10-19)

Read-only against the baseline Postgres.

- `GET /metrics/disk?resource=<pg>` → 200 time series (`unit:"bytes"`, instance/resource labels); `db-connections` → 200 (`unit:"count"`). The bex datastore-metric extensions (w3/m10) serve real data.
- `resource=` (empty) → 500, same w4/370 family on the datastore paths too.
- A web-service id on the disk path → 200 `[]` (wrong-kind resource reads as no data, not an error).

No new bug filed.
