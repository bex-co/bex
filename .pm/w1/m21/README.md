# w1 · m21 — Static sites (Render `static_site` type)

**Worker:** worker1 **Goal:** Ship Render-parity static-site hosting — build a repo and serve the output from an object-store/CDN origin (not a Deployment), with a `publishPath`, redirects/rewrites (`/routes`), and custom response headers (`/headers`), plus the REST/GraphQL/MCP + dashboard surfaces. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on                          |
| ---- | ---------------------------------------------------------------------------------------- | --- | ----------------------------------- |
| t001 | `static_site` service type: CRD/spec + build → object-store/CDN origin path              | 90m | —                                   |
| t002 | Serving: `publishPath` from the CDN origin + redirects/rewrites + custom headers          | 90m | w1/m21/t001                         |
| t003 | REST/GraphQL/MCP static-site surface (create/read/update, routes, headers)                | 60m | w1/m21/t002                         |
| t004 | Dashboard static-site UI (create, publishPath, redirects/rewrites, headers)               | 60m | w1/m21/t003                         |
| t005 | Render parity — verify static-site surfaces + edge rules against Render                    | 30m | w1/m21/t002,w1/m21/t003,w1/m21/t004  |
| t006 | Simplify — `/simplify` over what this milestone changed                                    | 20m | w1/m21/t005                         |
| t007 | Test coverage — build→publish path + edge rules + surface tests                            | 40m | w1/m21/t005                         |
| t008 | Closeout — verify DoD holds, then move the milestone to `done/`                            | 10m | w1/m21/t007                         |

## Definition of done

- A repo with a static build (e.g. a Vite/CRA `dist/`) deploys as a `static_site`: bex builds it and serves `publishPath` from an object-store/CDN origin — **no Deployment/Service** for the served content.
- `/routes` redirects/rewrites and `/headers` custom response headers apply on the served responses (verifiable with `curl -I` and a redirect check).
- REST/GraphQL/MCP create/read/update the static site + its routes/headers with Render-identical shape; the dashboard exposes create + publishPath + routes + headers.
- `docs/render-parity.md` static-site rows (build→CDN, redirects/rewrites, headers) move gap → ✅ with evidence.

## Source + Goal linkage

- **Source:** inbox note `w1/012` (split from m15 in the 2026-07-08 reorg; originally m13 audit note `w1/009`); moved to `w1/done/012.md` on promotion.
- **Goal linkage:** pillar 1 (Render parity — service-type breadth).
- **Expected outcome:** static front-ends host on bex without a running container, matching Render's `static_site` type and its edge rules.
- **Why now:** the user prioritized promoting the parity backlog. Promoting this note is also what unblocks the static-site CDN edge rules that `.pm/DO_NOT_DO.md` (line 23) parks as non-goals "unless `w1/012` is promoted" — so the redirect/rewrite/header rows are now in-scope. Sequenced **after** the compute-service parity (m20) since bex is compute-first; this is the larger build→CDN effort.
- **Render parity task included:** adds REST/GraphQL/MCP/UI + edge-rule surfaces, so cross-surface parity must be verified.
