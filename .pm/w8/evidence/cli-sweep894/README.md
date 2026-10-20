# CLI sweep 894 — `openPorts` is not a web-service create field (live, 2026-10-20)

Refused create only; nothing created. No new bug filed.

- `serviceDetails.openPorts` on a `web_service` create → 400 `unknown field "openPorts"` — in the pinned schema the port list belongs to private-service details (a web service's single HTTP port is the separate `port` field, sweep 852), and private services are paid-gated (sweep 819), so the positive path could not be exercised on the free tier. Strict decoding names the misplacement instead of ignoring it.
