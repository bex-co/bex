# CLI sweep 1291 — stock images on pin `f64d4c85f2cd` (pod user namespaces everywhere) (live, 2026-10-21)

Production `https://api.bex.co/v1/`, bex-canary, HEAD-built pinned CLI. The only product change in this pin is w6/m151 t003–t004: render-v1 runtime pods run in a pod user namespace in every workspace. That change can break images that switch users or own files at startup, so it was regression-checked with stock images, each a free web service whose `PORT` points at the image's own listener:

| Image | `PORT` | Result |
| --- | --- | --- |
| `redis:7-alpine` | 6379 | `live` in 21 s, "Ready to accept connections" |
| `postgres:17-alpine` (+ throwaway `POSTGRES_PASSWORD`) | 5432 | `live` in 32 s; initdb, `chown` and the user switch all fine |
| `nginx:1.27-alpine` | 80 | `live`, `GET /` 200 "Welcome to nginx" (setting `PORT=80`, via w5/m149, now sidesteps the port-detection gap for this image) |
| `mysql:8.4` (+ throwaway root password) | 3306 | `live` in 42 s, "ready for connections" |

No regression. All four were deleted (204 → 404). w8/156, w8/162 and w8/165 still reproduce on this pin.
