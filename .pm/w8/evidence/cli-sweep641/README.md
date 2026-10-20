# CLI sweep 641 — `idleTTLSeconds` ceiling boundary (live, 2026-10-19)

Disposable free web service `qa641-web` (`srv-dbbb7aivfpec73dm0itg`). Deleted and verified absent (REST 404).

Nested `PATCH /services/{id}` with `{"serviceDetails":{"idleTTLSeconds":N}}`:

| Value | Result |
| --- | --- |
| 604800 | 200 accepted |
| 604801 | 400 `bad request: idleTTLSeconds must be 0-604800` |
| 604799 | 200 accepted |
| 0 | 200 accepted |

Readback note: the value is exposed as top-level `idleTTLSeconds` on `GET /services/{id}` (not inside `serviceDetails`); 604799 read back exactly. A final value of 0 reads back as absent (zero value), consistent with sweep 639/640 semantics.

Boundary is exact and the refusal names the field and range. No bug filed.
