# CLI sweep 705 — nested `serviceDetails` PATCH field set (live, 2026-10-19)

Disposable free web service `qa705-web` (`srv-dbbbu9avfpec73dm0l50`). Deleted and verified absent (REST 404).

`PATCH {"serviceDetails":{…}}`:

| Field | Result |
| --- | --- |
| `runtime:"bogus"` | 400 `unknown field "runtime"` (not a PATCH-carried field) |
| `region:"bogus"` / `"oregon"` | 400 `unknown field "region"` (placement is installation-selected, ADR018) |
| `url:"https://evil.example.com"` | 400 `unknown field "url"` (computed, read-only) |
| `plan:"bogus"` | 400 `plan must be one of free\|starter\|…` |
| `healthCheckPath:"/h"` | 200 accepted |

Readback stayed `region:"fsn1"`, `url:"https://qa705-web.onbex.co"`. No bug filed.
