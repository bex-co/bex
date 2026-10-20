# CLI sweep 647 — env-var name validation and PUT semantics (live, 2026-10-19)

Disposable free web service `qa647-web` (`srv-dbbb8h2vfpec73dm0j50`). Deleted and verified absent (REST 404).

Via `PUT /services/{id}/env-vars`:

| Probe | Result |
| --- | --- |
| `K1=V1` | 200 |
| `K2=` (empty value) | 200 accepted |
| `1BAD=x` | 400 `ENVIRONMENT_VARIABLE_INVALID: invalid environment variable name "1BAD": use letters, digits and underscores, not starting with a digit, at most 253 characters` |
| empty key | 400 same coded refusal |
| `K 3=sp` (space) | 400 same coded refusal |
| `K4=a=b=c` | 200; value preserved verbatim |

PUT replaces the whole set: after the final single-key PUT, `GET env-vars` listed only `K4` — Render's documented replace semantics. No bug filed.
