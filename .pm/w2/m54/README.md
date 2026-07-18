# w2 · m54 — Dashboard document-head parity: Render-shaped titles + global social metadata

**Worker:** worker2 **Goal:** Give every bex dashboard page a deterministic, SSR-correct, localized document title following Render's current page/resource hierarchy, while adding a bex-branded global description/Open Graph/Twitter shell that never exposes private resource names. **Status:** planned

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Shared head contract + global bex metadata shell | 45m | — |
| t002 | Static, auth, collection, create, and settings route titles | 45m | t001 |
| t003 | Project/workspace titles from loaded names | 40m | t001 |
| t004 | Service/deploy titles: SSR name + type, identical across tabs | 50m | t001 |
| t005 | Datastore, environment-group, and Blueprint dynamic titles | 50m | t001 |
| t006 | Redirect, API, error, loading, and not-found head policy + guard | 35m | t002–t005 |
| t007 | Render parity: live matrix + evidence close | 30m | t006 |
| t008 | Simplify: one title/data/metadata path | 30m | t007 |
| t009 | Test coverage: SSR, navigation, privacy, i18n, and route inventory | 45m | t007 |
| t010 | Closeout | 15m | t008, t009 |

## Definition of done

Every user-facing HTML route in `dashboard/src/routes/` is classified in one checked-in matrix as content, inherited-layout, redirect-only, non-HTML API, or error/fallback. Content routes render the correct localized `<title>` in the initial authenticated SSR response and after client navigation. Static pages follow `<page> ・ bex Dashboard`; resource pages follow `<human name> ・ <resource type> ・ bex Dashboard`; a service and all of its tabs/deploy pages share `<service name> ・ <service type> ・ bex Dashboard`; project/environment Settings preserves Render's name-first hierarchy. Opaque `srv-`/`dpg-`/`red-`/project/env-group/Blueprint ids are loading/not-found fallbacks only, never the settled title.

The root head carries one bex-branded description, Open Graph, Twitter-card, locale/site-name, viewport, and favicon contract derived from the request/configured dashboard origin so a self-hosted installation never points at `dashboard.bex.co`. Like Render, route-specific names change only `<title>`; description/Open Graph/Twitter content stays generic and cannot disclose private resource names. No canonical/robots policy or route-specific social image is invented without evidence. Redirect/API routes emit no competing title, error/not-found states cannot retain the previous route's title, and loading behavior is deterministic. `yarn typecheck && yarn lint && yarn test && yarn build` pass; a live SSR/client-navigation matrix is recorded in the evidence artifact.

## Source + Goal linkage

- **Source:** user request 2026-07-17 — research all Render dashboard pages' SEO/document head, achieve the same behavior for the bex dashboard, and assign the work to w2. Research captured the deployed Render shell and current route bundles, cross-checked against the official [Render Dashboard](https://render.com/docs/render-dashboard), [Deploys](https://render.com/docs/deploys), [Logs](https://render.com/docs/logging), and [Service Metrics](https://render.com/docs/service-metrics) documentation. Findings and the 58-route bex inventory are in [the audit](evidence/2026-07-17-render-dashboard-head-audit.md).
- **Goal linkage:** `docs/ADR008-vision.md` promises Render's developer experience and transfer of existing Render habits; `docs/ADR006-bex-api.md` names the GraphQL/dashboard surface as Render-dashboard-compatible. This milestone is pure `dashboard/` behavior with no REST/GraphQL/MCP contract change expected. The standing Render-parity task is included because the change is user-facing.
- **Expected outcome:** browser tabs, history, bookmarks, screen readers, SSR HTML, and shared dashboard links identify the current bex page consistently without leaking tenant resource names into global social metadata.
- **Why now:** the dashboard has grown to 58 route definitions, but only 30 declare a head; the remainder mixes legitimate inheritance/redirect/API cases with implicit behavior. Existing titles use three competing suffix/separator styles, project/service names are patched through client-only `document.title` effects, and Database/Key Value/env-group/Blueprint pages settle on opaque ids. w5/m42 solved only the post-load service-tab title and explicitly left the SSR id fallback, making the missing shared contract visible rather than complete.
- **Explicitly excluded:** public marketing-site SEO, analytics changes, sitemap generation, indexing private dashboard content, route redesign, API field additions, and copying Render-owned marketing text/images. Bex metadata is branded from the existing mission and assets.
