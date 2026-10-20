# CLI sweep 1230 — free Postgres psql round trip, allow-list edits, and the IPv6 lockout (live, 2026-10-20)

HEAD checkout build `94533d795`, pinned Render CLI v2.27.0, deployed pin `94533d795067`, human device-login grant. `bex-canary` refused a second Postgres (`400 WORKSPACE_RESOURCE_LIMIT: workspace is limited to 1 Postgres database …`; its slot is a baseline fixture), so this sweep used `tian-personal` (`tea-da2isimlm39c739m4ofg`, zero Postgres, one untouched pre-existing service).

- `bex pg create --confirm --name qa-20261020-9286a3-pg --plan free --database-name qa9286a3 --database-user qa9286a3 -o json` → `dpg-dbbg9e19a20c7385raa0`, PG 18, `available` in ~140 s. (The CLI's `-o json` envelope is `{"data": …}`.)
- `bex psql <id> -c …` in `json` and `text` modes returns `current_user`/`current_database` `qa9286a3` and server 18.6. `select 1/0` → exit 1 with `ERROR:  division by zero`. `create table … insert … select` and `\dt` work.
- Allow-list edits through `bex pg update`: a `203.0.113.0/24` list and `--clear-ip-allow-list` are refused **client-side** by the pinned CLI (`IP address (<v4>) not in allow list …`, using the api.ipify.org IPv4).
- **Finding → [w8/140](../../140.md):** after restoring `cidr=0.0.0.0/0`, `bex psql` failed for 180 s with `SSL error: unexpected eof while reading` at the AAAA address. A private probe shows IPv4 `hostaddr` answers and IPv6 `hostaddr` gets the EOF. Parent cause: w9/m164.
- Cleanup: `bex pg delete <id> --confirm` → exit 0, GET 404; `tian-personal` is back to its single pre-existing service.
