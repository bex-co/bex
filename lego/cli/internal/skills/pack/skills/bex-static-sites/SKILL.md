---
name: bex-static-sites
description: >-
  Deploys and configures static sites on bex's global CDN—build commands,
  publish paths, SPA routing, redirects, custom headers, and PR previews. Use
  when the user needs to deploy a static site, set up a React/Vue/Hugo/Gatsby
  frontend, configure SPA fallback routing, add redirect rules, customize
  response headers, or choose between a static site and a web service for
  their frontend.
  Trigger terms: static site, CDN, SPA, single-page app, React deploy,
  Vue deploy, Hugo, Gatsby, Docusaurus, Jekyll, staticPublishPath.
license: MIT
compatibility: bex static sites (free tier available)
metadata:
  author: bex
  version: "1.0.0"
  category: compute
---

# bex Static Sites

Deploys static frontends (React, Vue, Hugo, Gatsby, Docusaurus, Jekyll, etc.) to bex's global CDN with automatic TLS, Brotli compression, HTTP/2, and DDoS protection. Free tier available.

## When to Use

- Deploying a **static site or SPA** (no server-side rendering)
- Choosing between a **Static Site** and a **Web Service** for a frontend
- Configuring **SPA fallback routing**, **redirects/rewrites**, or **custom headers**
- Setting up **PR preview environments** for a static site
- Troubleshooting **build failures** or **stale content** on a CDN-hosted site

For SSR frameworks (Next.js, Nuxt, SvelteKit) that need a running server, use **bex-web-services** instead. For Blueprint authoring, see **bex-blueprints**.

When choosing between a static site and a web service, read `references/service-types.md`.

## Static Site vs Web Service

| Need | Use | Why |
|------|-----|-----|
| Pure HTML/CSS/JS, SPA, docs, blog | **Static Site** | Free, global CDN, instant cache invalidation |
| SSR (Next.js `next start`, Nuxt server) | **Web Service** | Needs a running Node/Python/etc. process |
| Static export from SSR framework | **Static Site** | If the framework supports full static export (`next export`, `nuxt generate`) |
| API backend | **Web Service** | Static sites cannot run server code |

**Key constraint:** Static sites are **not on the private network**. They cannot communicate with other bex services over internal hostnames.

## Build and Publish

| Setting | Purpose |
|---------|---------|
| `buildCommand` | Installs dependencies and builds assets (e.g. `npm ci && npm run build`) |
| `staticPublishPath` | Directory of built output to serve (e.g. `build`, `dist`, `public`) |

bex auto-detects and installs dependencies. Set `SKIP_INSTALL_DEPS=true` to handle installation yourself in the build command.

### Common frameworks

| Framework | Build command | Publish path |
|-----------|--------------|--------------|
| Create React App | `npm ci && npm run build` | `build` |
| Vite (React/Vue/Svelte) | `npm ci && npm run build` | `dist` |
| Next.js (static export) | `npm ci && next build` | `out` |
| Nuxt (static) | `npm ci && nuxt generate` | `.output/public` |
| Hugo | `hugo --minify` | `public` |
| Gatsby | `npm ci && gatsby build` | `public` |
| Docusaurus | `npm ci && npm run build` | `build` |
| Jekyll | `bundle exec jekyll build` | `_site` |
| Astro | `npm ci && astro build` | `dist` |

## SPA Routing and Redirects

Single-page apps need a catch-all rule so the CDN serves `index.html` for all routes instead of returning 404.

Configure **Redirect/Rewrite Rules** in the Dashboard (Settings > Redirects/Rewrites) or via the Blueprint `routes` field:

```yaml
routes:
  - type: rewrite
    source: /*
    destination: /index.html
```

For multi-path redirects (e.g. old blog URLs), add specific rules **above** the catch-all so they take priority.

See `references/routing-and-headers.md` for redirect types, header rules, and caching patterns.

## Custom Response Headers

Add security and performance headers from the Dashboard (Settings > Headers) or the Blueprint `headers` field:

```yaml
headers:
  - path: /*
    name: X-Frame-Options
    value: DENY
  - path: /assets/*
    name: Cache-Control
    value: public, max-age=31536000, immutable
```

## PR Previews

Static sites support automatic PR previews—each pull request gets a unique URL with the built site.

- Enable in Dashboard: Settings > PR Previews
- Blueprint: set `previews.generation` to `automatic` or `manual`
- Preview URLs follow the pattern `<service>-<pr-id>.onbex.co`

## Blueprint Configuration

```yaml
services:
  - type: web
    runtime: static
    name: my-frontend
    buildCommand: npm ci && npm run build
    staticPublishPath: dist
    routes:
      - type: rewrite
        source: /*
        destination: /index.html
    headers:
      - path: /*
        name: X-Frame-Options
        value: DENY
    previews:
      generation: automatic
```

**Note:** Static sites use `type: web` with `runtime: static` in Blueprints. There is no separate `type: static`.

## CDN and Performance

- **Global CDN** with edge caching worldwide
- **Brotli compression** (better than gzip)
- **HTTP/2** by default
- **Immediate cache invalidation** on every deploy (zero-downtime, atomic deploys)
- **DDoS protection** included

## Billing

Static sites have a **free tier**. They count against workspace-level monthly included amounts for:

- **Outbound bandwidth** (data served to users)
- **Pipeline minutes** (build time)

Usage beyond included allowances can incur additional cost. Confirm current allowances and charges at [bex pricing](https://bex.co/pricing).

## References

| Document | Contents |
|----------|----------|
| `references/routing-and-headers.md` | Redirect types, rewrite rules, header patterns, SPA config |
| `references/framework-configs.md` | Build commands and publish paths for 10+ frameworks |
| `references/service-types.md` | Service-type selection and execution models |

## Related Skills

- **bex-web-services** — For SSR frameworks that need a running server
- **bex-blueprints** — Full `render.yaml` schema for static site fields
- **bex-domains** — Custom domain and TLS setup
- **bex-deploy** — Deploy flows, CLI, MCP operations

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
