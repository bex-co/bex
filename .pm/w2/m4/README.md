# w2 · m4 — Render-shaped service create & delete

**Worker:** worker2 **Goal:** The last missing lifecycle verbs, under Render's names: `POST /v1/services` / `DELETE /v1/services/{id}` / MCP `create_web_service` (+ bex `delete_service` extension) over a single `Core.Create`/`Core.Delete` pair — image-backed services work end-to-end today; a `repo` body is accepted and converges once w1/m5 (in-cluster builds) lands. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | `Create` + `Delete` verbs in the apps feature: validate name/plan (tiers catalog)/image/env; write the App CR — via the store row when `BEX_CP_DB_URI` | 30m | —            |
| t002 | REST: `POST /v1/services` (201, shapes verified against Render's OpenAPI) + `DELETE /v1/services/{id}` (204)                                           | 25m | t001         |
| t003 | GraphQL (`createService`/`deleteService`) + MCP (`create_web_service`, `delete_service`) fragments over the same verbs                                 | 25m | t002         |
| t004 | Acceptance: MCP `create_web_service` with an image → live `*.onbex.co` URL → `delete_service` → gone; repo-body pending path exercised                 | 25m | t003         |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                                            | 20m | t004         |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                                               | 30m | t004         |

## Definition of done

An agent (or `curl`) creates an image-backed service through any of the three surfaces and gets a live https URL with no kubectl; delete removes the App CR and everything the operator derived from it (Deployment/Service/Ingress via ownerRefs); all three adapters delegate to one `Create`/`Delete` verb pair. A `repo`-backed create body is accepted and validated (not faked) — it converges when w1/m5 ships builds.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-08; `docs/bex-api.md` scope gap ("Not yet: … service creation"); Render OpenAPI + `render-oss/render-mcp-server` (`create_web_service`); `GOAL.md` item 1 ("Suspend. Delete. Create.").
- **Goal linkage:** pillar 1 (Render-compatible surfaces) + pillar 4 (deploy-from-chat needs a create verb).
- **Expected outcome:** the create/delete half of the service lifecycle exists on all three surfaces; w2/m2 (deploy-from-chat) rides on `Core.Create` instead of inventing a bespoke `POST /v1/deploy`.
- **Why now:** V0 roadmap item 1 is unfinished without it, and w2/m2 is queued behind a bespoke endpoint this replaces with the Render-parity one; plan validation (w1/m8 tiers) and the store write path (w1/m2) it needs both just shipped.
