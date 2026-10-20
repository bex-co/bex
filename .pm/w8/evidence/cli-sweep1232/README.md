# CLI sweep 1232 — Key Value `0.0.0.0/0` refuses IPv6 (live, 2026-10-20)

`bex-canary`'s single Key Value slot was taken mid-sweep by another session's fixture (`400 WORKSPACE_RESOURCE_LIMIT`), so this sweep used `tian-personal`. `bex kv create --name qa-20261020-e09404-kv --plan free --ip-allow-list cidr=0.0.0.0/0,description=everywhere --confirm -o json` → `red-dbbge4h9a20c7385rahg`, available in 56 s.

- The KV host publishes `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1`. With TLS, SNI and private credentials (never printed), `redis-cli` gets `PONG` over IPv4 and `SSL_connect failed: unexpected eof while reading` over IPv6. By hostname from this dual-stack client, both `redis-cli` and `psql` (against the sweep 1230 Postgres) fail with that EOF. Filed as [w8/140](../../140.md).
- Cleanup: `bex kv delete <id> --confirm` → exit 0, GET 404.
