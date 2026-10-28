---
name: bex-web-services
description: >-
  Configures bex web services—port binding, TLS, health checks, custom
  domains, auto-deploy, PR previews, persistent disks, and deploy lifecycle.
  Use when the user needs to set up a web service, fix health check failures,
  add a custom domain, configure zero-downtime deploys, or troubleshoot port
  binding issues.
license: MIT
compatibility: bex web services (native runtimes or Docker)
metadata:
  author: bex
  version: "1.0.0"
  category: compute
---

# bex Web Services

This skill covers **Web Service** behavior on bex: how traffic reaches your process, how deploys go live, and how optional features (domains, disks, auto-deploy) interact. Use it alongside Blueprint and networking skills when wiring `render.yaml` or Dashboard settings.

## When to Use

- Configuring or debugging **port binding**, **PORT**, or **multi-port** web services
- **TLS/HTTPS** expectations at the edge vs inside the container
- **Health checks** blocking or rolling back deploys
- **Custom domains**, DNS, and certificate provisioning
- **Auto-deploy**, **CI-gated deploys**, and **PR preview** generation
- **Persistent disks** and their impact on scaling and zero-downtime
- **Deploy lifecycle**: build, pre-deploy, swap, drain, **rollback**, shutdown delay

Deeper patterns live under `references/` (health checks, domains, deploy phases).

Before configuring or troubleshooting deploys and health checks, read `references/deployments.md`.
Before attaching or changing a persistent disk, read `references/persistent-disks.md`.
Before configuring or troubleshooting private connectivity or additional ports, read `references/private-networking.md`.

## Port Binding

- Listen on **`0.0.0.0`** (all interfaces). Binding only to **`localhost`** or **`127.0.0.1`** prevents bex’s proxy from reaching your app.
- Use the **`PORT`** environment variable for the HTTP listen port. bex sets it for you; the **default is often `10000`** and you can change the configured value in the service **Settings** in the Dashboard.
- **Reserved ports** (do **not** bind your application to these for normal traffic): **`18012`**, **`18013`**, **`19099`**.

### Multi-port Web Services

- Only **one** port receives **public** HTTP traffic: the port aligned with **`PORT`**.
- **Additional** open ports are reachable on bex’s **private network** only (not from the public internet through the same public URL pattern).

## TLS and HTTPS

- **TLS terminates at bex’s edge.** The edge speaks HTTPS to clients; your process typically receives **plain HTTP** on `PORT`.
- **HTTPS redirect** for clients is handled by the platform; users hitting HTTP are redirected appropriately at the edge.
- **Do not terminate TLS inside the app** for the primary public listener unless you have a rare, explicit need—standard Web Services assume HTTP behind the proxy.

## Health Checks

Follow `references/deployments.md` for current platform behavior. For endpoint implementations and common failure patterns, see `references/health-check-patterns.md`.

## Custom Domains

- Point DNS with a **CNAME** to **`[service-name].onbex.co`** (use your service’s hostname from the Dashboard).
- bex **automatically provisions and renews** TLS certificates for verified domains.
- **Apex** (root) domains need provider-specific **CNAME-like** or flattened records where plain CNAME at `@` is unsupported.
- **Wildcard** domains (e.g. `*.example.com`) are supported when configured and verified.
- Multiple custom domains per service are supported; Blueprints can list them under the **`domains`** field.

See `references/custom-domains.md` for Dashboard steps, verification, and troubleshooting.

## Auto-Deploy and PR Previews

- Follow `references/deployments.md` for auto-deploy behavior and manual triggers.
- **PR previews** are configured under Blueprint **`previews.generation`** (and related preview settings); generation behavior depends on repo integration and plan.

## Persistent Disks

- A disk-backed web service stays **single-instance** and cannot scale horizontally.
- Attaching a disk disables **zero-downtime deploys**.
- The disk is available only to the running service, not during builds or pre-deploy commands.
- If the workload needs horizontal scaling or uninterrupted deploys, use shared external storage instead of a service-attached disk.
- Follow `references/persistent-disks.md` for current support, configuration, mount-path, runtime-access, sizing, and snapshot behavior.

## Deploy Lifecycle

For paid web services, use `preDeployCommand` for database migrations and other release tasks that must succeed before the new version receives traffic. It runs after the build on separate compute; if it fails, the deploy is canceled and the previous live version continues serving. Follow `references/deployments.md` for the full build, readiness, traffic-switching, draining, rollback, restart, and deploy-hook behavior.

## Free Tier Notes

Free Web Services have **separate limits**: services **spin down after inactivity** (cold starts on the next request), and they do not support scaling beyond a single instance or persistent disks. Treat free-tier behavior as distinct from paid Web Service defaults when advising on uptime and scaling.

## References

| Topic | File |
|--------|------|
| Health check design, timeouts, pitfalls | `references/health-check-patterns.md` |
| Domains, DNS, TLS verification | `references/custom-domains.md` |
| Deploy lifecycle, health checks, rollback, triggers | `references/deployments.md` |
| Persistent disk constraints and configuration | `references/persistent-disks.md` |
| Private-network addressing, ports, discovery, and troubleshooting | `references/private-networking.md` |

## Related Skills

- **bex-deploy** — Blueprints, first-time deploy, `render.yaml` structure
- **bex-docker** — Docker-based Web Services and image/runtime details
- **bex-networking** — Private network, internal URLs, multi-port private listeners
- **bex-scaling** — Instance counts, plans, and scaling constraints (including disk interactions)

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
