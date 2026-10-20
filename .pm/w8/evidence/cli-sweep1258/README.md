# CLI sweep 1258 — environment-group link, update fan-out, precedence and unlink at runtime (live, 2026-10-20)

One environment group (`GRP_VAL=v1`, `SHARED=from-group`) created over REST, and one free echo-server service created with `bex services create … --env-var SHARED=from-service`. Both deleted (404). No new bug filed.

- Link → a `config_change` deploy; the process sees `GRP_VAL=v1`, and the service's own `SHARED=from-service` wins over the group's (Render's service-over-group precedence).
- `PUT /env-groups/{id}/env-vars/GRP_VAL {"value":"v2"}` → the linked service gets a `config_change` deploy, and the process reads `v2`.
- Unlink (204) → after the resulting deploy, `GRP_VAL` is gone from the process and `SHARED` stays `from-service`.
