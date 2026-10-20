# CLI sweep 749 — Key Value policy PATCH cycle (live, 2026-10-19)

Disposable free Key Value `qa749-kv` (`red-dbbcl2i720pc73c0kmg0`). Deleted and verified absent (zero `qa749` in list).

- `PATCH {"maxmemoryPolicy":"bogus"}` → 400 `invalid request body at /maxmemoryPolicy`; `{"bogus":1}` → 400 `request body contains unknown field "bogus"`.
- `PATCH {"maxmemoryPolicy":"volatile_lru"}` → 200, status cycled `config_restart` → `available`, and `keyvalues get` read back `volatile_lru`.

No bug filed.
