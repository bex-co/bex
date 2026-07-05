# w2 · m1 — MCP server over bex-api verbs

**Worker:** worker2 **Goal:** Expose the bex-api lifecycle verbs over MCP as "just another thin adapter over the same `Core`" — so an agent operates bex natively (list/get/deploy/restart/suspend/resume/logs) instead of screen-scraping a dashboard. Delivers pillar 3. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | MCP adapter over `Core` (list/get/restart/suspend/resume) | 30m | — |
| t002 | Add a `Logs` verb to `Core` + expose over MCP | 30m | t001 |
| t003 | Auth + transport: reuse `bex-api-token`, stdio + streamable-http | 25m | t001 |
| t004 | Manifests + deploy + end-to-end acceptance | 30m | t001,t003 |

## Definition of done

An MCP client (e.g. Claude) can list apps, get one, restart/suspend/resume, and tail logs — every verb delegating to the same `Core` (`operator/internal/api/core.go`) as REST/GraphQL, authed by the `bex-api-token` Secret. No verb has a second implementation.

## Source

`docs/vision.md` pillar 3 (MCP server); `docs/bex-api.md` "one Core, thin adapters".
