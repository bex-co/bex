# CLI sweep 1262 — Key Value rename, suspend/resume and data persistence through the CLI (live, 2026-10-20)

One free Key Value (`bex-canary`, allow-list = this client's IPv4 /32) created with `bex kv create`, deleted with `bex kv delete --confirm` (GET 404). Data-plane access used private `redis-cli --tls --sni` over IPv4 with the server-issued credentials (never printed). No new item; a one-line note was added to w9/m164.

- `SET qa1262 persist-me` → OK. `bex kv update <id> --name <new>` → `bex kv get <new>` works, and `bex kv get <old>` fails at resolution (the documented upstream `render workspace set` wording).
- `bex kv suspend` → `suspended`. While suspended, `redis-cli` gets a bare `SSL_connect failed: unexpected eof while reading` and `connection-info` still answers 200. This is the same opaque front-door close as Postgres in sweep 1260, now noted in w9/m164's legibility addendum.
- `bex kv resume` → `available` in 29 s, and `GET qa1262` → `persist-me` (journal_snapshot data survived suspend/resume).
