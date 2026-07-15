# w2 · m44 — Image-origin override: manual-deploy `imageUrl` + deploy-hook `imgURL`

**Worker:** worker2 **Goal:** An image-backed service can deploy a specific image via Render's `POST /v1/services/{id}/deploys {imageUrl}` on all three surfaces and via `?imgURL=` on its deploy hook — gated by one written origin-safety rule, with unsafe origins rejected by a Render-shaped 400 that names the rule. **Status:** todo

## Tasks (in order)

| id   | title                                                                                       | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Design the image-origin safety rule; write the decision down                                 | 45m | —                |
| t002 | Core verb: validated image-origin override on deploy creation                                | 45m | t001             |
| t003 | REST `imageUrl` on the deploy body + GraphQL/MCP counterparts                                | 45m | t002             |
| t004 | Deploy-hook `imgURL` param through the same validator                                        | 30m | t002             |
| t005 | Render parity                                                                                 | 25m | t003, t004       |
| t006 | Simplify                                                                                      | 20m | t005             |
| t007 | Test coverage                                                                                 | 35m | t005             |
| t008 | Closeout                                                                                      | 15m | t007             |

## Definition of done

An image-backed service deploys a new tag via `POST …/deploys {imageUrl}` from REST, GraphQL, and MCP, and via `?imgURL=` on its deploy hook; an origin outside the allowed rule returns a Render-shaped 400 naming the rule; the origin-safety design decision is written down; the two named-rejection paths (`deployhook.go`'s 400 and m30's deliberate `imageUrl` omission) are gone; source note `w2/012` closed.

## Source + Goal linkage

- **Source:** promotes `.pm/w2/012.md` (deploy-hook `imgURL` design gate, filed by round 12's code miner — `lego/backend/internal/deploys/deployhook.go:256` rejects with 400 at `:292`) + the unowned closeout residual `.pm/w2/done/m30/t005.md:41` ("`imageUrl` … not part of this milestone, file separately if wanted" — never filed; found by round 14's residual miner).
- **Goal linkage:** Render deploy-API parity (docs/ADR006-bex-api.md) + deploy-from-chat (pillar 4, docs/ADR017-deploy-from-chat.md) — redeploying a specific image is a core agent verb.
- **Expected outcome:** the last two named-rejection 400s on the deploy surface become working parity.
- **Why now:** w2/m30 shipped everything around it this week; the design gate is one task, not a milestone of its own — and the hook and API halves need the same safety rule, so building them apart would duplicate the design.
- **Render parity:** included — REST/GraphQL/MCP deploy-body surface change.
- **Trust note for t001:** the authenticated API body and the unauthenticated secret-URL hook have different trust models — the design must say whether they get the same origin rule or a stricter hook subset.
