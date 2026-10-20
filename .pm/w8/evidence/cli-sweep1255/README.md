# CLI sweep 1255 — a service's real egress address is in its outbound-ips (live, 2026-10-20)

One free image web service (`srv-dbbk6ehq56vc73bn8eug`, `python:3.12-alpine` with a Docker Command serving a tiny app that fetches `api.ipify.org` and `api64.ipify.org`) was created over REST and deleted (GET 404). REST was needed because the pinned CLI refuses `--start-command` for image runtimes. No new bug filed.

- `GET /v1/services/{id}/outbound-ips` → `{"type":"shared","ips":["116.202.29.102","178.105.115.93","178.105.209.30","46.225.152.95"]}`. The process's observed egress was `178.105.115.93` for both the IPv4-only and the dual-stack lookup (three tries), so egress is IPv4 and inside the advertised set. This is the ADR018 “truthful shared-egress” contract, and an external allowlist built from this list admits the app.
