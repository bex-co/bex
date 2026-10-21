# CLI sweep 1285 — prebuilt nginx, the 3000-vs-10000 port default, and a free-capacity stall (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `a3f66521f93d`, bex-canary, HEAD-built pinned CLI.

- `bex services create --type web_service --plan free --image docker.io/library/nginx:1.27-alpine` (`srv-dbcarnhakels738t2kp0`): nginx now starts cleanly (the sweep 454 `chown` failure is gone on this pin; workers spawn). The deploy still stalled after 307 s: "startup health check has not succeeded … a TCP connect to port 3000" (`port: 3000`, `openPorts: [{3000/TCP}]`). nginx listens on 80, and Render would detect it. This is the recorded port-detection divergence (ADR089), not filed again. Deleted (204 → 404).
- Default port: bex 3000 versus Render 10000, with 25 corpus files hardcoding 10000. Filed as [w8/168](../../168.md).
- `python:3.12-alpine` with `dockerCommand: python -m http.server 10000` (`srv-dbcauf9akels738t2l10`) did not schedule within 150 s. Stall reason: "no node currently has room for this service's instance (not enough cpu, memory free for its instance type)". Free capacity was briefly exhausted at about 12:26 UTC. The message is clear and actionable, so this is an operational observation, not a bug. Deleted (204 → 404).
