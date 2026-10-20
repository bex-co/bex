# CLI sweep 657 — Postgres create validation dialects (live, 2026-10-19)

Refusal-only probes; nothing created (`postgres list` still shows only the baseline `qa-20261017-64552e-pg14`).

Via the pinned CLI (`postgres create --name qa657-pg`):

| Probe | Result |
| --- | --- |
| `--plan bogus` | server 400 `invalid request body at /plan` |
| `--disk-size-gb 3` | client `invalid --disk-size-gb 3: must be 1 or a multiple of 5` |
| `--version 99` | server 400 `invalid request body at /version` |
| `--connection-pool bogus` | client `invalid --connection-pool bogus: must be 'none' or 'pgbouncer'` |
| `--region bogus` | client enum refusal listing the five regions |
| `--ip-allow-list not-a-cidr` | client `must start with cidr=` |

Client and server validation layers are consistent and every refusal names the field. No bug filed.
