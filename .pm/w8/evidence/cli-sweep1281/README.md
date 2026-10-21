# CLI sweep 1281 — Docker Command on a shell-less image (live, 2026-10-21)

Production `https://api.bex.co/v1/` (pin `9cace6b6e25d` when the deploy started), bex-canary.

- `traefik/whoami:v1.10.3` (`FROM scratch`) as a free image web service with `dockerCommand: /whoami --port 8080 --name qa1281` and bex `port: 8080` → never started. The stall reason reads: `the startup program "/bin/sh" does not exist in this image … A Docker Command runs through /bin/sh -c`. Filed as [w8/164](../../164.md).
- Corpus context (sweep 1270): Blueprints already wrap explicitly (`Noveum/orbit`: `dockerCommand: sh -c 'export …'`), consistent with Render executing the command directly.
- `srv-dbc9hn9v5vjs73b9he30` was deleted (204 → 404).
- The new production pin `a3f66521f93d` (6b512d4be, 10:53 UTC) landed during this sweep. w8 fix verification starts next (sweep 1282).
