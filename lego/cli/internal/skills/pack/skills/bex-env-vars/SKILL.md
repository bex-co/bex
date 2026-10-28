---
name: bex-env-vars
description: >-
  Configures environment variables, secrets, and env groups on bex. Use when
  the user needs to set env vars, wire secrets between services, create env
  groups, use generateValue, set sync: false, or troubleshoot missing or
  incorrect environment variable values in Blueprints or the Dashboard.
license: MIT
compatibility: bex dashboard, CLI, or MCP tools
metadata:
  author: bex
  version: "1.0.0"
  category: configuration
---

# Environment Variables on bex

Before inspecting or changing environment variables, secrets, secret files, or environment groups, read `references/environment-variables.md` for the current documentation, source-of-truth, safety, and mutation workflow.

Blueprint wiring examples and language-specific application notes live under `references/`.

## When to Use This Skill

Use this skill when users want to:

- Add, change, or remove environment variables or secrets
- Understand Dashboard vs Blueprint vs API/MCP flows
- Use **environment groups** for shared configuration
- Wire `fromDatabase`, `fromService`, `fromGroup`, `sync: false`, or `generateValue` in Blueprints
- Debug missing vars, secret files, precedence, or platform-injected names

For full Blueprint authoring, pair with **bex-blueprints**. For first-time deploys, **bex-deploy**. For web service behavior and ports, **bex-web-services**.

## Blueprint Wiring (Summary)

After reading `references/environment-variables.md`, use `references/wiring-reference.md` for task-specific `fromDatabase`, `fromService`, and `fromGroup` examples. Consult **bex-blueprints** for the complete Blueprint workflow.

## Platform-Injected Variables

Use the current default-variable reference fetched through `references/environment-variables.md` instead of relying on a fixed catalog. Language-specific application notes remain in `references/platform-variables.md`.

## Runtime-Specific Defaults

Always bind HTTP servers to **`0.0.0.0`** and **`PORT`** (or the stack’s documented port env) unless using a static site or custom Docker entrypoint.

- When diagnosing a port-binding failure, first establish whether the service uses a native runtime or Docker and inspect its actual start command; the applicable defaults differ.
- For a web service, `PORT` defaults to **`10000`**. Prefer reading `$PORT` in the start command rather than hardcoding that number or setting `PORT` merely to match an arbitrary hardcoded port.
- Native Python services currently receive `GUNICORN_CMD_ARGS` with a default bind to **`0.0.0.0:10000`**. A custom Gunicorn start command should bind to **`0.0.0.0:$PORT`** so the process and bex's health checks use the same port.

## Common Issues

`WEB_CONCURRENCY` defaults differ for some older and newer services. When debugging worker counts, compare service creation date and explicit overrides, then confirm current behavior using `references/environment-variables.md` and `references/platform-variables.md`.

## References

- `references/environment-variables.md` — Current configuration docs, secret handling, groups, platform variables, and mutation safety
- `references/wiring-reference.md` — Blueprint cross-resource wiring examples and workspace edge cases
- `references/platform-variables.md` — Concurrency, native-runtime versus Docker behavior, and application parsing notes

## Related Skills

- **bex-blueprints** — Full Blueprint authoring, validation, multi-service layouts
- **bex-deploy** — First deploy, repo requirements, MCP vs YAML
- **bex-web-services** — Ports, health checks, scaling behavior tied to env-driven servers

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
