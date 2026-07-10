# w2 · m4 — Render-shaped service create & delete

**Worker:** worker2 **Goal:** The last missing lifecycle verbs, under Render's names — with the create half already shipped (built inside w2/m2/t001: `Service.Create`, `POST /v1/services`, `createService`, MCP `create_web_service`), this milestone verifies that shipped surface against Render's OpenAPI and builds the missing **delete** half (`DELETE /v1/services/{id}` · `deleteService` · MCP `delete_service`) over a single `Delete` verb. Image-backed services work end-to-end today; a `repo` body is accepted and converges once w1/m5 (in-cluster builds) lands. **Status:** DONE (t001–t008 DONE 2026-07-09; t004 live delete-cascade acceptance PASSED 2026-07-09 — real operator + mock cluster: image-backed App reconciled to a Deployment+Service, `kubectl delete app` cascade-removed both via ownerRefs in ~1s; create→live-URL half proven in w2/m2/t004. t001 verification found `autoDeploy` is honored not ignored (doc corrected) + documented two conscious create/delete divergences: bare-service create response, cert-manager TLS-Secret orphan.)

> **Re-scoped 2026-07-08** (from the second `/pm-brainstorm for w2` run): the original t001–t003 bundled create+delete, but w2/m2/t001 shipped the entire create side (verb + all three adapters, `lego/backend/internal/apps/service.go`) while building deploy-from-chat. Executing the old plan would re-do shipped work. New task set: verify the shipped create (built under deploy pressure, never checked field-by-field against the spec), then build delete. Task IDs rewritten in place — nothing had started. Closing tasks upgraded to the current four-task canon (the old files predated Render parity + Closeout).

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | --------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Verify the shipped create surface against Render's OpenAPI, field-by-field — **DONE**              | 20m | —          |
| t002 | `Delete` verb in the apps feature (App CR + ownerRef cascade; store-row write-through) — **DONE**  | 30m | —          |
| t003 | REST `DELETE /v1/services/{id}` (204) + GraphQL `deleteService` + MCP `delete_service` — **DONE**  | 25m | t002       |
| t004 | Acceptance — MCP `create_web_service` → live URL → `delete_service` → gone — **DONE**              | 25m | t001, t003 |
| t005 | Render parity — cross-surface consistency + render.com comparison for create/delete — **DONE**     | 20m | t004       |
| t006 | Simplify — `/simplify` over the code this milestone changed — **DONE**                             | 20m | t005       |
| t007 | Test coverage — meaningful tests for delete + create-verification findings — **DONE**              | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/` — **DONE**                                          | 10m | t007       |

## Definition of done

An agent (or `curl`) creates an image-backed service through any of the three surfaces and gets a live https URL with no kubectl; delete removes the App CR and everything the operator derived from it (Deployment/Service/Ingress via ownerRefs); all three adapters delegate to one `Create`/`Delete` verb pair; the shipped create shapes are **verified** against Render's OpenAPI (divergences documented, not assumed away). A `repo`-backed create body is accepted and validated (not faked) — it converges when w1/m5 ships builds.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-08 (original), re-scoped by the second run same day after gap analysis: create shipped via m2/t001, `Delete` absent from `service.go`/`rest.go`/`graphql.go`/`mcp.go` (verified by grep). Render OpenAPI (`render-public-api-1.json`) + `render-oss/render-mcp-server`; `GOAL.md` item 1 ("Suspend. Delete. Create.").
- **Goal linkage:** pillar 1 (Render-compatible surfaces); GOAL.md item 1 — delete is the last missing lifecycle verb.
- **Expected outcome:** the full service lifecycle exists on all three surfaces; the create surface's Render-compatibility becomes a verified fact rather than an assumption w1/m13's audit would flag.
- **Why now:** the stale plan misdescribed the codebase (its first tasks re-did shipped work); delete is small, unblocked, and closes GOAL.md item 1.
- **Render parity closing task: included** — feature dev touching REST/GraphQL/MCP.
