# CLI sweep 642 — scaling dialect and `numInstances` boundaries (live, 2026-10-19)

Disposable free web service `qa642-web` (`srv-dbbb7ltfsg2s73ako6t0`). Deleted and verified absent (REST 404).

## Dialect

- `PATCH /services/{id}` with `numInstances` (nested in `serviceDetails` or top-level): `400 request body contains unknown field "numInstances"` for every value (1, 2, 0, -1, 100). The pinned CLI likewise has no scale subcommand; `GET` exposes `serviceDetails.numInstances` read-only.
- Correct dialect is Render's `POST /services/{id}/scale`:

| Body | Result |
| --- | --- |
| `{"numInstances":1}` | 202 accepted |
| `{"numInstances":2}` on free plan | 400 `the free plan is limited to 1 instance(s) and cannot scale to 2` |
| `{"numInstances":0}` | 400 `numInstances must be 1-100` |
| `{"numInstances":-1}` | 400 `numInstances must be 1-100` |

Strict unknown-field PATCH refusal is consistent with sweep 639, and every scale refusal names the constraint. No bug filed.
