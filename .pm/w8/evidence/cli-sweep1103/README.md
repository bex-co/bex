# CLI sweep 1103 — PATCH accepts both port placements, serviceDetails wins (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- PATCH top-level `port` → 200 (read 8081); PATCH with both placements → 200 and `serviceDetails.port` wins (read 8083), exactly the precedence documented in `rest.go` ("serviceDetails.port wins when both are sent"). The create-side asymmetry from sweep 1102 does not extend to PATCH.
