# Vision — the open-source Render alternative, AI-native

## Mission

**bex gives you Render's developer experience — `git push` becomes a running HTTPS service — as open source, on infrastructure you control, with AI agents as first-class users.**

Render, Heroku, and Railway proved the product: developers don't want to operate Kubernetes, they want a URL. But that experience is only rented — closed platforms, someone else's cloud, someone else's pricing. bex is the same product as software you can run: Apache-2.0, one Go operator, one API, deployable on a €10 Hetzner box or a hundred of them.

## The AI-native thesis

The next wave of deployments won't be typed by humans. Agents already scaffold apps end-to-end; the missing piece is a platform they can _operate_ — deploy, check status, roll back, suspend — without screen-scraping a dashboard built for people.

A PaaS built for agents must be:

- **API-first** — every action a human can take is an API call. No dashboard-only features, ever.
- **Deterministic** — declarative intent (`App` CRs, `bex.yml`) in, converged state out. An agent can retry, diff, and reason about it.
- **Machine-readable** — state is structured (`phase` / `revision` / `url`), not prose in a web page.

Render compatibility is part of the same thesis: agents (and their toolchains) already know Render's API shapes. bex speaks them, so existing tooling and habits transfer instead of restarting from zero.

## Pillars

| # | Pillar | Status |
| --- | --- | --- |
| 1 | **Render-compatible REST + GraphQL** — `bex-api` serves Render's `/v1/services` shapes (verified against Render's OpenAPI spec) and its dashboard GraphQL ([ADR006-bex-api.md](ADR006-bex-api.md)) | ✅ shipped |
| 2 | **Agent-readable state** — `App` CR `status.phase` / `status.revision` / `status.url`; `kubectl get apps.app.bex.co` is the dashboard. Treated as a stable contract | ✅ shipped |
| 3 | **MCP server** — the bex-api verbs (list / get / restart / suspend / resume / plan-change / logs / metrics / env-vars / api-keys) exposed over MCP (`/mcp` + a stdio mode); by design just another thin adapter over the same core ([ADR006-bex-api.md](ADR006-bex-api.md)) | ✅ shipped |
| 4 | **Deploy-from-chat** — one API call takes a repo + `bex.yml` to a live URL, so "deploy this" is a single agent action (needs the control plane, [ADR003-control-plane.md](ADR003-control-plane.md)) | 🔜 planned |
| 5 | **E2B-compatible sandboxes** — the opensandbox runtime's real pause/resume as hosted execution environments for agents, with idle sandboxes hibernated ("sleep = free") | 🔜 planned |

Pillars 1–3 mean an agent can already operate bex today natively — MCP, `curl`, or `kubectl`. Pillars 4–5 close the loop from "operate" to "create".

## Roadmap

Roughly ordered — de-risk the live system, then the source-of-truth control plane, then the elastic/cost machinery:

1. **Postgres control plane** — ✅ built (opt-in via `BEX_CP_DB_URI`, not yet the prod default; [ADR003-control-plane.md](ADR003-control-plane.md)). Remaining: flip it on in prod, tenant onboarding.
2. **Wake activator + HMAC webhook** — push-to-deploy from Git hosting, and wake-on-request for hibernated apps.
3. **Cluster Autoscaler wiring** — add/remove machines reactively instead of manually.
4. **In-cluster builds** — BuildKit/kpack Jobs so build-from-git images are pullable by cluster nodes.
5. **MCP server** — ✅ shipped (pillar 3).

## Non-goals

- **Multi-cloud abstraction layers** — bex targets Cluster API providers (Hetzner first, Docker for local dev), not a lowest-common-denominator cloud API.
- **A closed SaaS** — the hosted offering, when it exists, runs the same code in this repo.

(Managed databases used to be a non-goal; that changed — bex now ships Render-compatible managed Postgres, [ADR009-postgresql-management.md](ADR009-postgresql-management.md).)
