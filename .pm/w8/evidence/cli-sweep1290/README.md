# CLI sweep 1290 — Render default variables as a running process sees them (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `4b6687b5bc18`, bex-canary. A free `python:3.12-alpine` web service (`srv-dbccs7159hnc73e5ho6g`) served its `RENDER*`/`PORT`/`WEB_CONCURRENCY` environment and resolved three of the hostnames:

- Correct: `RENDER=true`, `IS_PULL_REQUEST=false`, `RENDER_SERVICE_ID`, `RENDER_SERVICE_NAME`, `RENDER_SERVICE_TYPE=web`, `RENDER_EXTERNAL_HOSTNAME=<slug>.onbex.co`, `RENDER_EXTERNAL_URL=https://<slug>.onbex.co`, `PORT=3000`. `RENDER_DISCOVERY_SERVICE=<slug>-discovery` resolves to the pod IP, `RENDER_SERVICE_NAME` resolves to the ClusterIP, and `RENDER_EXTERNAL_HOSTNAME` resolves to the public edge.
- `RENDER_CPU_COUNT=1` on free (Render documents `"0.5"`-style plan values). Filed as [w8/171](../../171.md).
- `RENDER_INSTANCE_ID` is the raw pod name, which is w4/386.
- `os.cpu_count()` is 4 (the node's cores), which matches what containers usually see. Informational only.
- Deleted (204 → 404).
