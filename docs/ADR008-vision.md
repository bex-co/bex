# Vision — the open-source Render alternative, AI-native

## Mission

**bex gives you Render's developer experience — `git push` becomes a running HTTPS service — as open source, on infrastructure you control, with AI agents as first-class users.**

Render, Heroku, and Railway proved the product: developers don't want to operate Kubernetes, they want a URL. But that experience is only rented — closed platforms, someone else's cloud, someone else's pricing. bex is the same product as software you can run: Apache-2.0, one Go operator, one API, deployable on a €10 Hetzner box or a hundred of them.

## The AI-native thesis

The next wave of deployments won't be typed by humans. Agents already scaffold apps end-to-end; the missing piece is a platform they can _operate_ — deploy, check status, roll back, suspend — without screen-scraping a dashboard built for people.

A PaaS built for agents must be:

- **API-first** — every action a human can take is an API call. No dashboard-only features, ever.
- **Deterministic** — declarative intent (`App` CRs, `render.yaml`) in, converged state out. An agent can retry, diff, and reason about it.
- **Machine-readable** — state is structured (`phase` / `revision` / `url`), not prose in a web page.

Render compatibility is part of the same thesis: agents (and their toolchains) already know Render's API shapes. bex speaks them, so existing tooling and habits transfer instead of restarting from zero.

## Pillars

| # | Pillar | Status |
| --- | --- | --- |
| 1 | **Render-compatible REST + GraphQL** — `bex-api` serves Render's `/v1/services` shapes (verified against Render's OpenAPI spec) and its dashboard GraphQL ([ADR006-bex-api.md](ADR006-bex-api.md)) | ✅ shipped |
| 2 | **Agent-readable state** — `App` CR `status.phase` / `status.revision` / `status.url`; `kubectl get apps.app.bex.co` is the dashboard. Treated as a stable contract | ✅ shipped |
| 3 | **MCP server** — the bex-api verbs (list / get / restart / suspend / resume / plan-change / logs / metrics / env-vars / api-keys) exposed over MCP (`/mcp` + a stdio mode); by design just another thin adapter over the same core ([ADR006-bex-api.md](ADR006-bex-api.md)) | ✅ shipped |
| 4 | **Deploy-from-chat** — one API call takes a repo + `render.yaml` to a live URL, so "deploy this" is a single agent action ([ADR017-deploy-from-chat.md](ADR017-deploy-from-chat.md); MCP `deploy` / `create_web_service` over `Core.Create`, HMAC push-to-deploy; in-cluster builds via [ADR034](ADR034-scalable-build-pipeline.md) / [ADR060](ADR060-build-worker-reliability-and-performance.md)) | ✅ shipped |
| 5 | **E2B-compatible sandboxes** — the opensandbox runtime's real pause/resume as hosted execution environments for agents, with idle sandboxes hibernated ("sleep = free") ([ADR014](ADR014-sandboxes.md), [ADR042](ADR042-sandbox-cluster-substrate.md), [ADR047](ADR047-cloud-coding-agent-sessions.md), [ADR059](ADR059-agent-sandbox-hibernation.md); `render ea sandbox` + the agent-session stack) | ✅ shipped |

All five pillars ship today: an agent can create, operate, and sandbox-execute on bex via MCP, `curl`, or `kubectl`.

## Roadmap

### Shipped foundation (was the original de-risk list)

1. **Postgres control plane** — ✅ production default ([ADR003](ADR003-control-plane.md), [ADR043](ADR043-tenant-namespace-isolation.md); `NamespaceReconciler` requires `BEX_CP_DB_URI`; datastores cut over in `w7/m77`).
2. **Wake activator + HMAC webhook** — ✅ `lego/operator/cmd/activator` and git HMAC push-to-deploy ([ADR017](ADR017-deploy-from-chat.md)).
3. **Cluster Autoscaler wiring** — ✅ `deploy/gitops/base/autoscaler.yaml` (`w1/m19`).
4. **In-cluster builds** — ✅ BuildKit workers ([ADR034](ADR034-scalable-build-pipeline.md), [ADR060](ADR060-build-worker-reliability-and-performance.md); `w1/m5`).
5. **MCP server** — ✅ shipped (pillar 3).

### What's next

- **Local agent-session draft-PR proof** — finish `w3/m79` once a developer-supplied GitHub App installation exists (repo-less sessions already green on `dev-N`).
- **Mobile mission control** — `w11` (push hygiene, live attach / needs-decision steering, tier-2 quick actions).
- **Pillar-5 follow-ons** — sandbox metering, deeper hibernation/continuity, and the remaining ADR047 phase-2 attach surface.
- **Ops hardening** — authorize the production canary fixture (`BEX_CANARY_*`) so m83's scheduled synthetics stop soft-skipping; keep origin-vs-edge SLIs honest as traffic grows ([ADR088](ADR088-platform-observability-ui.md)).

## Non-goals

- **Multi-cloud abstraction layers** — bex targets Cluster API providers (Hetzner first, Docker for local dev), not a lowest-common-denominator cloud API.
- **A closed SaaS** — the hosted offering, when it exists, runs the same code in this repo.

(Managed databases used to be a non-goal; that changed — bex now ships Render-compatible managed Postgres, [ADR009-postgresql-management.md](ADR009-postgresql-management.md).)
