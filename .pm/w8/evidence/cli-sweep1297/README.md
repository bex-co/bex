# CLI sweep 1297 — `bex kv-cli` passthrough forms from Render's CLI reference (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, workspace tian-personal. A free Key Value was created with `bex kv create --plan free --ip-allow-list cidr=0.0.0.0/0,description=qa` (`red-dbcft74iimkc73f6l1i0`).

- Without a TTY, `bex kv-cli <id> -- PING` (and every other form) → exit 1, `` `bex kv-cli` can only be used in interactive mode ``. This is the pinned CLI's non-TTY guard, not a server defect.
- In a PTY: `kv-cli <id> -- PING` → `PONG`, `-- SET qa1297 hello` → `OK`, `-- GET qa1297` → `"hello"`, each exiting 0. `redis-cli` prints its standard warning that the password is passed with `-a`/`-u`. That is upstream's invocation, the same against Render.
- On the first run per config, the upstream telemetry notice ("The Render CLI collects usage data … No data has been sent to Render yet") printed with Render copy. This is a documented residual (`docs/cli-compatibility-checklist.md:73`, w5/m92: analytics go to bex-api, and the notice is not reworded).
- Deleted (204 → 404). No new bug.
