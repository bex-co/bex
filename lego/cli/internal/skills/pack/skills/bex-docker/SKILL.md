---
name: bex-docker
description: >-
  Builds and deploys Docker containers on bex—Dockerfiles, multi-stage
  builds, Blueprint Docker fields, private registries, layer caching, and
  platform constraints. Use when the user mentions Docker,
  Dockerfile, container images, multi-stage builds, container registry,
  GHCR, ECR, BuildKit, dockerContext, runtime docker or image, or
  optimizing Docker builds on bex.
license: MIT
compatibility: >-
  Any bex compute service (web, private, worker, cron) with runtime: docker
  or runtime: image.
metadata:
  author: bex
  version: "1.0.0"
  category: deployment
---

# bex Docker Deployments

bex uses **BuildKit** for Docker builds. All compute service types that support custom runtimes can use **`runtime: docker`** (build from a Dockerfile in the repo) or **`runtime: image`** (pull a prebuilt image; no Dockerfile build on bex). Deeper patterns and copy-paste templates live under `references/`.

## When to Use

- Authoring or debugging a **Dockerfile** for a bex service
- Choosing **`runtime: docker`** vs **`runtime: image`** in a Blueprint
- Wiring **private base images** or **prebuilt images** with registry credentials
- **Multi-stage builds**, **build args**, **secrets**, and **layer caching**
- **Performance** and **security** hardening of container images on bex

For full Blueprint authoring, see **bex-blueprints**. For end-to-end deploy flows, see **bex-deploy**.

Before deploying a built or prebuilt image, read `references/deployments.md` for current trigger, lifecycle, health-check, and signal behavior.

## bex Docker Builds

- **BuildKit** is used for Docker builds on bex.
- **`runtime: docker`**: bex builds an image from your repo using `dockerfilePath`, `dockerContext`, and optional `dockerCommand` (overrides image `CMD`).
- **`runtime: image`**: bex pulls **`image.url`**; no repo-based image build. Set **`image.creds.fromRegistryCreds.name`** when the registry is private.

## Blueprint Configuration

| Field | Role |
|-------|------|
| `dockerfilePath` | Path to the Dockerfile (default `./Dockerfile`) |
| `dockerContext` | Build context directory (what is sent to the daemon) |
| `dockerCommand` | Overrides the container `CMD` after the image is built |
| `image.url` | Image reference for `runtime: image` (registry/repo:tag or digest) |
| `image.creds` | Dashboard-stored credential reference for a private prebuilt image |
| `registryCredential` | Dashboard-stored credential reference for private base images used by `runtime: docker` |

Example sketch (values illustrative):

```yaml
services:
  - type: web
    name: api
    runtime: docker
    region: oregon
    plan: starter
    dockerfilePath: ./Dockerfile
    dockerContext: .
    dockerCommand: node server.js
    envVars:
      - key: PORT
        value: 10000
```

For `runtime: image`, set `image.url` and, if needed, `image.creds` per **Registry Configuration** below.

## Multi-Stage Builds

**Recommended for production.** Use a **builder** stage for compilation and dependency installation, and a minimal **runner** stage that only copies artifacts and runtime files. Benefits:

- Smaller images and faster pulls
- Fewer tools and secrets in the final image (smaller attack surface)
- Clear separation between build-time and run-time dependencies

See `references/dockerfile-patterns.md` for language-specific templates.

## Build Args vs Secrets

**Critical:** **Never pass secrets via `ARG`.** Build arguments are stored in image **layers** and can be recovered from the image history or intermediate layers.

- Prefer **runtime environment variables** (bex **env vars** / secret files) for application secrets.
- For **build-time** secrets (e.g. private package feeds), use **Docker BuildKit secret mounts** (`RUN --mount=type=secret,...`) rather than `ARG`.

Treat anything sensitive as **runtime** or **BuildKit secret mount**, not as a build arg.

## Registry Configuration

Private **base images** (for `runtime: docker`) or **prebuilt images** (`runtime: image`) need authentication:

- Store credentials in the bex dashboard under **Registry Credentials**.
- For `runtime: docker`, reference private base-image credentials with **`registryCredential.fromRegistryCreds.name`**.
- For `runtime: image`, reference private prebuilt-image credentials with **`image.creds.fromRegistryCreds.name`**.

Supports common registries (Docker Hub, GHCR, ECR, Google Artifact Registry, and others). Step-by-step per provider: `references/registry-setup.md`.

Prebuilt-image services do not automatically deploy when a mutable registry tag moves. After publishing a new image, trigger a manual deploy through the Dashboard or a supported API or deploy-hook flow. Prefer immutable tags or digests so each deploy identifies an explicit artifact; follow `references/deployments.md` for the complete trigger and verification workflow.

## Layer Caching

- bex **caches Docker layers** between builds; **order Dockerfile instructions** so that frequently unchanged layers stay early (see `references/optimization-guide.md`).
- **Tags and caching:** mutable tags like **`latest`** can resolve to **stale cached** images. Prefer **immutable** references: **digest** (`repo/image@sha256:...`) or **version pins** (`v1.2.3`).

## Platform Specifics

- bex builds **linux/amd64**. Avoid assumptions about other architectures in production images.
- **Port binding** matches native services: bind HTTP to **`0.0.0.0:$PORT`** (bex sets `PORT`).
- **Health checks** use the same platform behavior described in `references/deployments.md`.
- **Secret files** from bex appear under **`/etc/secrets/`** — do not rely on repo-root secret paths inside the container unless you copy or mount them explicitly in the image.

## `.dockerignore` and Start Commands

- Always maintain a **`.dockerignore`** that excludes **`node_modules`**, **`.git`**, **`.env`**, build artifacts, logs, and OS junk. This shrinks context upload time and avoids leaking local files into layers. Lists and rationale: `references/optimization-guide.md`.
- **Custom start command:** if you need multiple shell steps, use a single shell form, e.g. **`/bin/sh -c 'set -e; ./migrate && exec node server.js'`** (prefer **`exec`** so your app receives signals for graceful shutdown).

## References

| Document | Contents |
|----------|----------|
| `references/dockerfile-patterns.md` | Multi-stage templates (Node, Python, Go, Ruby, Rust, static sites) |
| `references/registry-setup.md` | Docker Hub, GHCR, ECR, Artifact Registry + Blueprint wiring |
| `references/optimization-guide.md` | Layer order, `.dockerignore`, BuildKit cache mounts, debugging |
| `references/deployments.md` | Deploy triggers, lifecycle, health checks, shutdown, and verification |

## Related Skills

- **bex-deploy** — Deploy flows, Blueprint vs Dashboard, operational steps
- **bex-blueprints** — Full `render.yaml` schema, wiring, and validation
- **bex-web-services** — Web service behavior, health checks, and HTTP edge cases

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
