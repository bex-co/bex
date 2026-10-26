# w8/m132 live DoD — sweep f446 (2026-10-26)

Production `https://api.bex.co/v1/`, platform pin `b4726243e74f` (contains `7d2171384`), HEAD-built bex `4070d0e4f` with pinned Render CLI v2.27.0 `a764810a7682`, workspace `bex-canary`, human device login. Fixture: a fresh Free static site `qa-20261026-f446-st` (`srv-dbftdddn83uc738utt60`) built from `render-examples/vue-hello-world` (`yarn; yarn build`, publish `dist`), deleted by ID afterwards (`GET` 404).

Asset `/js/chunk-vendors.5a9ba7b9.js`, identity 72,341 bytes:

| `Accept-Encoding` | `content-encoding` | bytes | check |
| --- | --- | --- | --- |
| `gzip` | gzip | 26,085 | `gunzip` output is byte-identical to identity; `vary: Accept-Encoding` |
| `gzip, deflate` | gzip | 26,085 | byte-identical after decode |
| `br` | br | 25,310 | unchanged |
| `gzip, br` | br | 25,310 | br preferred |
| `gzip;q=0` | identity | 72,341 | |
| `identity` | identity | 72,341 | |
| `deflate` | identity | 72,341 | |
| `zstd` | identity | 72,341 | |

- gzip ETag `W/"sNBmSmcFKqrR4WPZpQJPNw"` with `If-None-Match` → `304`.
- `Range: bytes=0-99` with `Accept-Encoding: gzip` → `206`, `content-encoding: gzip`, `content-range: bytes 0-99/26085`, and the 100 bytes equal the first 100 bytes of the full gzip body.
- `/index.html` (802 bytes, under the minimum) → identity.
