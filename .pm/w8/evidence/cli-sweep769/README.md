# CLI sweep 769 — empty-string query parameters (live, 2026-10-19)

Read-only.

| Probe | Result |
| --- | --- |
| `GET /logs?resource=` | 400 `invalid query parameter "resource"` |
| `GET /services?ownerId=` | 400 `invalid query parameter "ownerId"` |
| `GET /metrics/instance-count?resource=` | 500 `internal error` (the w4/370 family, sweep 768) |
| `GET /metrics/cpu?resource=%20` | 404 `NOT_FOUND` (a space is a lookup miss, not the empty case) |

The 500 is specific to exactly-empty metrics `resource` values; sibling validators reject empties by name.

No new bug filed.
