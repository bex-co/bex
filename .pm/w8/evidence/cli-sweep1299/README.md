# CLI sweep 1299 — stock-image regression on pin `a4b29737afef` (w6/090: render-v1 by default everywhere) (live, 2026-10-21)

Production `https://api.bex.co/v1/`, bex-canary, HEAD-built pinned CLI. Each image ran as a free web service created with `bex services create`, with `PORT` set to the image's own listener where needed.

| Image | Result |
| --- | --- |
| `redis:7-alpine` (`PORT=6379`) | `live` in 11 s |
| `postgres:17-alpine` (`PORT=5432`, throwaway password) | `live` in 43 s |
| `nginx:1.27-alpine` (`PORT=80`) | `live`, `GET /` 200 |
| `ealen/echo-server:0.9.2` | `live`, `GET /` 200 |

No regression. All four were deleted (204 → 404).
