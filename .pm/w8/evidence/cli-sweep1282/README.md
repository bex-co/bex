# CLI sweep 1282 — live verification of the w8 fixes on pin `a3f66521f93d` (2026-10-21)

The new production pin `a3f66521f93d` (6b512d4be, 10:53 UTC) reached bex-api at about 11:00 UTC, when `RateLimit-*` headers first appeared. Workspaces: bex-canary, plus tian-personal for the datastores. Human device-login grant, HEAD-built pinned CLI. Every fixture was deleted by ID with a 404 proof.

| Item | Live check | Result |
| --- | --- | --- |
| **w8/147** RateLimit headers | `GET /v1/services?ownerId=…&limit=1` | ✅ `ratelimit-limit: 500`, `ratelimit-remaining: 499`, `ratelimit-reset: 1` |
| **w8/m126** free Postgres expiry | read-only `GET /v1/postgres/dpg-db9r654flioc739mcpbg` (baseline) | ✅ `expiresAt: 2026-11-26T00:00:00Z` (clamped rollout date + 30 days) |
| **w8/142** bare IPs in Blueprint allow lists | `bex blueprints validate` on a Key Value with `source: 198.51.100.1` and `2001:db8::1` plus a database with `203.0.113.7` | ✅ `valid: true` (sync/apply round trip not exercised) |
| **w8/m122** image digest | fresh `bex services create --image docker.io/ealen/echo-server:0.9.2` | ✅ deploy row `image: {ref, sha: "sha256:006b92e1…4790e2"}` on REST and in `bex deploys list -o json`; a second `deploys create --wait` row carries its own `sha` |
| **w8/m125** `Rndr-Id` / `requestID` | `GET /<marker>` with a forged `Rndr-Id` | ✅ response and app both see the same edge-minted UUID, the forged value is replaced, the request log carries label `requestID` = that UUID, and `bex logs --text <id>` returns the lines. The static site behaves the same |
| **w8/141** deploy-hook vocabulary | hook with `ref`, foreign `imgURL`, malformed `imgURL` | ✅ messages name `ref`/`imgURL`: `ref is not supported for image-backed services — deploy an image tag or digest with imgURL`, `imgURL must use the service's configured image …`, `imgURL: image reference is malformed …` |
| **w8/m124** same-name recreate | delete the echo service, recreate the same name | ✅ request, app and unfiltered logs contain none of the predecessor's marker; instances list only new pods; cpu/memory/instance-count points start at the new `createdAt`, and every memory series' `instance` is a new-service pod |
| **w8/146** static recreate cache | `mdn/beginner-html-site`, `GET /`, delete, recreate the same name from `beginner-html-site-styled` | ✅ the recreated site's first live deploy serves its own `index.html` (sha `5d04139b…`, styled CSS link) instead of the predecessor's (`e999a69c…`). The name was held about 60 s (four 409 retries; known, not filed) |
| **w8/140** `0.0.0.0/0` with IPv6 | free PG18 and free Key Value, `--ip-allow-list cidr=0.0.0.0/0` | ✅ `bex psql <id> -c 'select 42'` by hostname three times; `redis-cli PING` over the AAAA and the A address both `PONG`. Control: Key Value restricted to `203.0.113.0/24` refuses both families |
| **w8/m128** `dockerContext` relative to `rootDir` | sweep 1266's failing shape (`rootDir: example-pserv`, `dockerContext: .`) | ✅ `COPY index.html` now succeeds; `PATCH envSpecificDetails.dockerContext` → 200 (was 400 unknown field) |
| **w8/m127** free SMTP egress | free image web service probing from inside | ✅ `smtp.gmail.com:25/465/587` and `aspmx.l.google.com:25` blocked (`Network unreachable`); `www.google.com:443` connects |
| **w8/m123** client-identity headers | web service: sweep 1266 (GitOps-live) | ✅ (static site and custom domain not re-probed) |

Still open after this pin: w8/154 reproduces on `a3f66521f93d`. A Docker service created without `dockerfilePath` reads back `{dockerCommand: "", dockerContext: ".", dockerfilePath: ""}`, so the context now has Render's default and the Dockerfile path still does not. Fixture created and deleted (204 → 404).
