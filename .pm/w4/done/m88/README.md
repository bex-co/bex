# w4 · m88 — Platform dashboard observability: error logs + Loki ship

**Worker:** worker4 **Goal:** make `dashboard.bex.co` debuggable from k9s/`kubectl logs` and durable in Loki when SSR or edge requests fail — today the pod prints only a listen line and platform traffic is deliberately dropped from the shipper. **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Emit structured prod SSR/route error logs from the dashboard — **DONE**             | 45m | —          |
| t002 | Ship `dashboard` namespace pod logs into Loki via Alloy — **DONE**                   | 45m | t001       |
| t003 | Retain Traefik access lines for `dashboard.bex.co` (host-labeled) — **DONE**        | 40m | t002       |
| t004 | Simplify — **DONE**                                                                 | 20m | t003       |
| t005 | Test coverage — **DONE**                                                            | 40m | t003       |
| t006 | Closeout — **DONE**                                                                 | 10m | t004, t005 |

## Definition of done

- A deliberate SSR/route failure on the live dashboard writes a JSON line to stdout with `level` (error) and bounded request identity (`path`/`status`/`msg`) — visible in k9s/`kubectl logs` without dumping GraphQL variables or cookies.
- Alloy ships `dashboard` namespace container logs into Loki under a non-tenant label set (not a fake `app.bex.co/app` App); a Loki query can select them after a pod restart.
- Traefik JSON access lines for host `dashboard.bex.co` are retained under bounded `service=dashboard` (+ method/status); **`host` stays line-only** (ADR010 cardinality). Other non-App edge hosts stay dropped unless explicitly listed.
- No change to tenant App log attribution, bex-api's tenant log API contract, or Apollo DEV-only verbose SSR logging of operation variables.

## Source + Goal linkage

- **Source:** live prod investigation 2026-08-19 (`hetzner-prod` / `dashboard/dashboard-d8f788c5c-dbtbj`): k9s-equivalent logs show only `Listening on: http://localhost:3000/`; traffic to `/healthz` and `/auth/login` produced no pod lines; `dashboard/src/common/apollo/factory.server.ts` gates Apollo SSR logging on `import.meta.env.DEV`; `deploy/gitops/base/log-shipper.yaml` drops pods without `app.bex.co/app` and drops Traefik services that are not `default-<app>-…@kubernetes` (explicitly naming the dashboard).
- **Goal linkage:** platform operability for the human dashboard surface (ADR008 human pillar + ADR010 observability). Operators cannot triage dashboard.bex.co incidents from the same tools used for tenant Apps.
- **Expected outcome:** after ship, a dashboard SSR error and a Traefik 5xx for `dashboard.bex.co` are visible in cluster logs and Loki without inventing a tenant App identity.
- **Why now:** repeated dashboard image rollouts today leave operators blind (restart wipes the only stdout line); the investigation already proved Traefik edge is healthy (200/307) while the app plane itself emits nothing — the gap is observability, not an unknown outage.
- **Render parity:** omitted — this milestone changes platform dashboard stdout, Alloy shipper rules, and Traefik access retention only; it does not change REST/GraphQL/MCP or any tenant-facing dashboard feature contract.
