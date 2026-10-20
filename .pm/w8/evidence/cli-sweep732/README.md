# CLI sweep 732 — create-time field applicability on image services (live, 2026-10-19)

Nothing retained: the one accepted create was deleted and verified absent (REST 404, zero `qa732` services).

| Probe (`--image` web service) | Result |
| --- | --- |
| `--build-filter-path src/**` | server 400 `prebuilt image services cannot declare buildFilter; remove it or deploy from repo instead` |
| `--start-command /bin/sh` | client `--build-command and --start-command are only supported for native runtimes` |
| `--publish-directory .` | client `--publish-directory is not supported for web_service` |
| `--pre-deploy-command 'echo hi'` | **accepted** (created, then deleted by this sweep) — pre-deploy applies to image services too |

Applicability is enforced per-field, server or client as appropriate, with named reasons. No bug filed.
