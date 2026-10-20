# CLI sweep 643 — `services update` validation boundaries (live, 2026-10-19)

Disposable free web service `qa643-web` (`srv-dbbb7v5fsg2s73ako6vg`). Deleted and verified absent (REST 404).

Via the pinned CLI (`services update <id> --confirm -o json`):

| Probe | Result |
| --- | --- |
| `--plan bogus` | 400 `plan must be one of free\|starter\|standard\|pro\|pro_plus\|pro_max\|pro_ultra` |
| `--max-shutdown-delay -5` | 400 `maxShutdownDelaySeconds must be 1-300` |
| `--max-shutdown-delay 30` | 200; readback `maxShutdownDelaySeconds = 30` |
| `--health-check-path healthz` | 400 `health check path must start with /` |
| `--health-check-path /healthz` | 200; readback `"healthCheckPath":"/healthz"` |

Every refusal names the field and constraint; accepted values read back exactly. No bug filed.
