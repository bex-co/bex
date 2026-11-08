# w3 · m282 — Webhook secret is readable on list, get and patch, as on Render

**Worker:** worker3 **Goal:** the webhook object carries its `secret` on every read, as Render's schema requires, on REST, GraphQL, MCP and the dashboard **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | REST list, get and patch return the webhook secret — **DONE** | 45m | — |
| t002 | GraphQL and MCP expose the secret to callers who may manage webhooks — **DONE** | 45m | t001 |
| t003 | Dashboard webhook detail reveals and copies the signing secret — **DONE** | 45m | t002 |
| t004 | Amend webhooks-api.md and ADR018 row 300 — **DONE** | 30m | t001 |
| t005 | Render parity — **DONE** | 30m | t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- `GET /v1/webhooks`, `GET /v1/webhooks/{id}` and `PATCH /v1/webhooks/{id}` return the same `whsec_…` that create returned. The m272 pinned-schema validator passes with no `secret` exemption.
- GraphQL and MCP return the secret to a caller who may manage webhooks, and never to one who cannot.
- The dashboard webhook page reveals and copies the secret for managers.
- `webhooks-api.md` and ADR018 row 300 no longer describe the secret as mint-once.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-07, item 2.
  - Render's webhook object requires `secret` on create, list, get and patch (`docs/render-artifacts/webhooks-api.md:18`).
  - bex returns it only on create (`:20`; `lego/backend/internal/webhooks/rest.go:32-41`). It is already stored in plain text for signing (`internal/store/webhooks.go:201`).
  - No user sign-off records the divergence in ADR018.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render API compatibility ([ADR006](../../../docs/ADR006-bex-api.md)): Render CLI and SDK clients can read the secret back.
- **Expected outcome:** clients that read a webhook's secret from Render's API work against bex unchanged.
- **Why now:** m272's schema validator is live, and this is one of its remaining exemptions. Under `.pm/DO_NOT_DO.md`'s first anti-goal, an unsigned hardening divergence is amended, not kept.
  - If the user instead wants mint-once kept, that needs their sign-off recorded in ADR018 before t001 starts.
- **Render parity:** included. The change touches REST, GraphQL, MCP and the dashboard.

## Closed — 2026-11-08

The webhook object carries its `secret` on list, get and patch for a workspace manager on REST, GraphQL and MCP, as on Render, validated against the pinned schema; members get neither the secret nor the exact URL. Webhook Settings reveals and copies it. Follow-up `w3/058` (post-create landing).
