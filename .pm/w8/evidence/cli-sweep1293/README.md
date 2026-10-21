# CLI sweep 1293 — `bex psql` passthrough forms from Render's CLI reference (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, workspace tian-personal. A free Postgres 17 was created with `bex pg create --plan free --version 17 --ip-allow-list cidr=0.0.0.0/0,description=qa` (`dpg-dbce0af4vasc73dpoo2g`, `public: true`). Its `expiresAt` is `2026-11-26T00:00:00Z`, which is m126's rollout clamp (2026-10-27 + 30 days), deliberate.

| Command ([render.com/docs/cli-reference](https://render.com/docs/cli-reference) forms) | Result |
| --- | --- |
| `psql <id> -c "SELECT 1 AS one;" -o text` | exit 0, aligned table |
| `… -o json` | exit 0, `{"output": " one \n-----\n   1\n(1 row)\n\n"}` |
| `… -o json -- --csv -q` | exit 0, `{"output": "one\n1\n"}` |
| `… -o text -- --csv -q` | exit 0, `one` / `1` |
| `psql <id> -c "SELECT 1/0;" -o text` | exit 1, `Error: exit status 1: ERROR:  division by zero` |

All match the pinned CLI's documented behavior. The database was deleted (204 → 404). No new bug.
