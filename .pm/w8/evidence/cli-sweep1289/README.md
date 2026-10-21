# CLI sweep 1289 — `PORT` edge values on pin `4b6687b5bc18` (live, 2026-10-21)

Four free `ealen/echo-server` web services were created via REST with `PORT` set to `abc`, `0`, `70000` and `80`. The process's view was read through `GET /?echo_env_body=PORT`. The three unusable values were accepted (201), are shown in `env-vars` as written, and are silently replaced by `3000` in the container, with no event. `80` works. Filed as [w8/170](../../170.md).

Fixtures `srv-dbcche159hnc73e5hnt0`, `srv-dbcche959hnc73e5hnv0`, `srv-dbccheh59hnc73e5ho10` and `srv-dbcchep59hnc73e5ho30` were deleted (204 → 404).
