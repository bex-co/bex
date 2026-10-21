# CLI sweep 1294 — health-check request shape against Render's documented contract (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, bex-canary. A free `ealen/echo-server` with `--health-check-path /healthz-qa1294` (`srv-dbcegdciimkc73f6l190`) logs each request, including its headers:

- The health check is `GET /healthz-qa1294` with `host: qa-20261021-c0f901-hc.onbex.co`. This matches [Render](https://render.com/docs/health-checks): "Render uses the service's `onrender.com` subdomain" when there is no verified custom domain. `user-agent: kube-probe/1.34`; Render documents no user agent.
- Went live in 24 s.
- Informational: the first request-log entry after go-live came from `46.224.128.204` (a Hetzner address, not this client), presumably the platform's own post-deploy reachability check. Request-log `message` bodies are Traefik's raw JSON (`KubernetesIngressName`, `KubernetesServiceName`, namespace = workspace ID). Only the tenant's own identifiers appear, so not filed.
- Deleted (204 → 404). No new bug.
