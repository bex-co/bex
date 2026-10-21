# CLI sweep 1288 — stock images under render-v1, and the create response's port (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `4b6687b5bc18`, bex-canary, HEAD-built pinned CLI.

- **Stock images now run.** `redis:7-alpine` with `--env-var PORT=6379` → `live` in 21 s ("Ready to accept connections tcp", raised open files to 10032). `postgres:17-alpine` with `PORT=5432` and a throwaway `POSTGRES_PASSWORD` → `live` in 11 s ("PostgreSQL init process complete … ready to accept connections"; the entrypoint's `chown`/user switch worked). Both confirm w6/m149/m150's render-v1 container policy live.
- **Create response port.** In all three creates (CLI `--env-var PORT=…` and REST `envVars`), the create response said `port: 3000` / `openPorts: [3000]` while the immediate `GET` said 8085 / 8086 / 6379. Filed as [w8/169](../../169.md).
- Recheck on this pin: w8/156 (aliases), w8/162 (external redirects) and w8/165 (`rootDir` trailing slash) still reproduce. `RateLimit-*` headers are still present (w8/147).
- Fixtures `srv-dbcc6mp59hnc73e5hnhg` (redis), `srv-dbcc6n4ekfmc73cr36jg` (pg), `srv-dbcc74cekfmc73cr36lg` and `srv-dbcc74h59hnc73e5hnjg` (echo) were deleted (204 → 404).
