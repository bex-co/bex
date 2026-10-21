# CLI sweep 1277 — six free official `render-examples` Blueprints deployed live (2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, human device-login grant. These are the bex-valid official examples from sweep 1276 whose every service declares `plan: free` or is static, checked before connecting this time (see sweep 1275). Each was connected with `POST /v1/blueprints`, set to `autoSync: false`, followed to a terminal deploy, checked at `GET /`, then deleted.

| Example | Runtime | Result |
| --- | --- | --- |
| `go-web-server` | Go | `live` in 289 s, `/` 200 "Hello from Render!" |
| `vuepress` | static | `live` in 295 s, `/` 200 |
| `render-tanstack-demo` | static | `live` in 193 s, `/` 200 |
| `flue` | Node | `live` in 253 s. `/` returns the app's own JSON 404 (its agents are served on `POST /agents/…`) |
| `adminer` | Docker (`adminer:5.4.2`, listens on 8080 and ignores `$PORT`) | `update_failed` at the 15-minute rollout budget: TCP health check on port 3000. Render auto-detects the bound port; bex deliberately does not (ADR089 § Port slice: "undeclared-port discovery … stay explicit in ADR018"). The image declares `EXPOSE 8080`, which bex already reads for private services' image ports, but not for web services |
| `fastify-hello-world` | Node | `build_failed`: `package.json engines.node "18.x" is not a supported Node version; supported Node lines are 22, 24, 26`. This is the deliberate ADR060 rule (unsupported requests fail by name; `.nvmrc 18` is its own example) |

Not filed. Both failures are recorded divergences. They do mean two of Render's official free examples fail on bex as published, which matters for the port-detection and Node-line decisions.

**Cleanup:** all six Blueprints → 404; services `srv-dbc807m5…`, `srv-dbc882m5…`, `srv-dbc8ac65…`, `srv-dbc8bpm5…`, `srv-dbc8dqm5…` and `srv-dbc8g5m5…` each 204 → 404.
