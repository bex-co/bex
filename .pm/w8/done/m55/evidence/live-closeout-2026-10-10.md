# m55 t007 live closeout — 2026-10-10 UTC

Production `https://api.bex.co/v1/`, workspace `bex-canary` / `tea-daif693dqjvc73e7as3g`, after deploy run for `1899e517d` succeeded.

**Clients**

- Checkout `bex` build of `5b91fcdab`. The launcher has no change on this path, so published v0.3.2 sends the same bytes.
- Unmodified Render built from the exact pin module `v1.1.3-0.20260909214233-a764810a7682`.

**Auth.** The QA human Kratos session was carried by a private loopback proxy that drops the CLI's placeholder bearer and attaches the session cookie. Device OAuth was not exercised: the shared Playwright browser was held by another session, and production re-authentication requires the password, which this run does not handle.

**Baseline.** Six services, listed in the original finding.

| Step | Result |
| --- | --- |
| Create Free static `qa-20261010-a49562-src` → `srv-db4ut996ogks73finjd0` | exit 0; deploy Live; URL 200 |
| Rename by ID to `qa-20261010-a49562-csv,雪%+&=` | exit 0; Live; URL 200 |
| `bex services update '<comma name>' --auto-deploy=false --confirm -o json` | exit 0, 1.4s, returns `srv-db4ut996ogks73finjd0` |
| Render, same command | exit 0, 1.3s, same ID |
| Wire query (proxy log) | `GET /v1/services?name=qa-20261010-a49562-csv%2C%E9%9B%AA%25%2B%26%3D&ownerId=…&limit=100` |
| `bex services create --from '<comma name>' --name qa-20261010-a49562-cl-bex` | exit 0 → `srv-db4utmp6ogks73finjfg`; Live; HTTP 200 `bex static site`; root `examples/static-site`, publish `.`, build `''` |
| Render, `--name qa-20261010-a49562-cl-render` | exit 0 → `srv-db4utn16ogks73finjhg`; Live; HTTP 200; same settings |

**API replays** (same session; the expected-match column is the last six characters of each matched ID):

| Query | Matches |
| --- | --- |
| single comma name | `finjd0` |
| ordinary + comma name (raw comma) | `finjd0`, `finjfg` |
| repeated keys | `finjfg`, `finjhg` |
| `%252C` literal | none |
| comma fragment alone | none |
| two names + `type=static_site&limit=1` | `finjfg` |
| no-such name | none |

**Cleanup.** Both clones were deleted before the source, by exact ID, all exit 0. All three detail reads return 404, and all three former URLs return 404. The native list equals the six-ID baseline. Limits read services 6/25 with 0 terminating, Postgres 0/1 and Key Value 0/1. Kubernetes descendants were not inspected.
