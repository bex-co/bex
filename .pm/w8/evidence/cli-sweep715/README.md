# CLI sweep 715 — service-name character edges (live, 2026-10-19)

The one created fixture (`qa715--double`) was deleted and verified absent (REST 404); the rest were refused.

| Name | Result |
| --- | --- |
| `-qa715x` / `qa715x-` | 400 `RESOURCE_NAME_INVALID` (start/end hyphen rule) |
| `QA715UPPER` | 400 same (lowercase rule) |
| `qa715_under` | 400 same (charset rule) |
| `qa715--double` | created — interior double hyphens are allowed; the message's rules are exactly the enforced ones |

No bug filed.
