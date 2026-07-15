# w2 · m38 — Full deploy status lifecycle + transition timestamps

**Worker:** worker2 **Goal:** bex deploy rows expose the evidence-backed lifecycle states and transition timestamps Render-trained clients expect, so REST/GraphQL/MCP and the dashboard can explain where a deploy failed. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store schema: updated-at + transition facts | 40m | — |
| t002 | Define the evidence-backed deploy transition model | 40m | t001 |
| t003 | Persist build-phase progress and failure | 50m | t002 |
| t004 | Persist pre-deploy · rollout · deactivation transitions | 60m | t002 |
| t005 | REST · GraphQL · MCP status and filter parity | 45m | t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 30m | t006 |
| t008 | Transition and dashboard test coverage | 45m | t006 |
| t009 | Closeout | 15m | t008 |

## Definition of done

Deploy rows move through the truthful subset of Render's eleven-state vocabulary as build, pre-deploy, rollout, cancel, and deactivation work occurs, with `updatedAt` advancing on each real transition. Filters accept every stored status and the dashboard timeline renders those facts. Surface-parity and failure-mode tests pass, and no adapter invents data absent from the store.

## Source + Goal linkage

- **Source:** `w5/m29` Render-parity audit, 2026-07-14; replaces the lifecycle portion of the referenced but never materialized `w2/m32` scope. w9/001 independently shipped commit id/message provenance before this was filed.
- **Goal linkage:** Render API/dashboard parity and trustworthy deploy debugging for human and agent clients.
- **Expected outcome:** a failed deploy identifies its failing phase without inference from logs alone; existing commit id/message provenance stays intact.
- **Why now:** `w5/m29` shipped the consuming detail-page shape and recorded the missing lifecycle facts explicitly; delaying the store contract would make more consumers depend on today's coarse four-state projection. Render parity is included because REST, GraphQL, MCP, and dashboard surfaces all change.
