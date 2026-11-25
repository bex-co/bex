---
name: bex-networking
description: >-
  Connects bex services over the private network—internal DNS, service
  discovery, and cross-service communication. Use when the user needs to wire
  services together, resolve internal hostnames, troubleshoot connectivity
  between services, configure environment isolation, or understand which
  services can reach each other.
license: MIT
compatibility: bex services in the same region and workspace
metadata:
  author: bex
  version: "1.0.0"
  category: networking
---

# bex private networking

bex’s **private network** lets services talk to each other without exposing traffic on the public internet. Use this skill when users need internal connectivity, discovery across scaled instances, or correct URL/port behavior for Blueprints and the Dashboard.

## When to Use This Skill

- Designing or debugging **service-to-service** traffic on bex
- Questions about **internal hostnames**, **internal URLs**, or **Connect > Internal** in the Dashboard
- **Service discovery** across multiple instances (custom load balancing, mesh-style setups)
- **Port limits**, reserved ports, or **multi-port** web services (public vs private)
- **Free-tier** web services and **who can send vs receive** private traffic
- **Environment isolation** (with a Pro workspace or higher) or **AWS PrivateLink** for private egress/ingress patterns

Before designing, configuring, or troubleshooting private connectivity, read `references/private-networking.md`. For architecture examples and Blueprint patterns, see `references/communication-patterns.md`.

## Private Connection Essentials

- If the service types or traffic direction are unclear, establish which service initiates the connection and which service receives it before proposing an address.
- Private-network peers must be in the same workspace and region. Use the destination's address from **Connect > Internal** in the Dashboard.
- Construct a complete URL when the client expects one, such as `http://[internal-hostname]:[port]/path`; a bare hostname does not supply the protocol.
- Web services and private services can receive private traffic. Background workers, cron jobs, and workflow runs can initiate outbound private connections but have no internal hostname and cannot receive them.
- In a gateway pattern, a public web service calls a private service over the private network; the private service does not need a public endpoint.

## Service Discovery

Use a service's normal internal hostname for ordinary service-to-service traffic. When an application specifically needs every active instance—for custom load balancing, per-instance metrics, or similar logic—use the service's discovery hostname, conventionally `[internal-hostname]-discovery`, which resolves to all active instance IPs. Each web or private service receives its own discovery hostname in `RENDER_DISCOVERY_SERVICE`. Do not persist the resolved IPs because they can change between deploys.

## Common Patterns

Short summaries; full diagrams and Blueprint notes live in `references/communication-patterns.md`.

1. **Web gateway + private backends** — Public Web Service terminates HTTP; internal calls use private hostnames and ports to Private Services or internal URLs.
2. **Worker to database** — Background Worker (no internal hostname) connects **outbound** to Postgres or Key Value **internal URLs**.
3. **Microservices** — Private Services (and eligible Web Services) call each other by **internal hostname:port** on the private network.

## References

| Document | Purpose |
|----------|---------|
| `references/private-networking.md` | Current private-network capabilities, addressing, discovery, ports, isolation, and troubleshooting |
| `references/communication-patterns.md` | Gateway, worker→DB, mesh, URL construction, and Blueprint `fromService` patterns |

## Related Skills

- **bex-web-services** — Public web services, `PORT`, and HTTP behavior
- **bex-private-services** — Private Service–specific setup and scaling
- **bex-blueprints** — `render.yaml`, `fromService`, and multi-service wiring

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
