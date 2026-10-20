# CLI sweep 772 — metrics query-parameter validation (live, 2026-10-19)

Read-only against the baseline `hello-go`.

| Probe | Result |
| --- | --- |
| `aggregateBy=bogus` | 400 `aggregateBy must be one of statusCode\|host` |
| `statusCode=200` on http-requests | 400 `unsupported query parameter "statusCode" for get-http-requests` (filtering rides `aggregateBy`, not a status param) |
| `resolutionSeconds=abc` | 400 `invalid query parameter "resolutionSeconds"` |
| `startTime` after `endTime` | 400 `startTime must be before endTime` |

Every invalid parameter is refused by name. No bug filed.
