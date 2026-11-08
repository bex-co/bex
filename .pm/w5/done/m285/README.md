# w5 · m285 — A webhook's signing secret is readable after create, as on Render

**Worker:** worker5 **Goal:** `GET /webhooks/{id}` (and GraphQL, MCP, dashboard) returns the webhook's signing secret, as Render's webhook object does, to the roles Render lets read it, with each read audited. **Status:** done — already met by w3/m282

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's webhook secret reads and amend ADR052/ADR018 — **DONE** | 30m | — |
| t002 | REST webhook reads return the secret — **DONE** | 1h | t001 |
| t003 | GraphQL and MCP parity; audit every read — **DONE** | 1h | t002 |
| t004 | Dashboard: reveal and copy the signing secret — **DONE** | 1h | t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 5h15m**.

## Definition of done

- After create, `GET /v1/webhooks/{id}` returns the same `secret` that signs deliveries; GraphQL, MCP and the dashboard reveal show the same value.
- Roles Render does not let read webhooks cannot read the secret on any surface; each secret read writes an audit-log event.
- ADR052 and ADR018:300 no longer list mint-once as a divergence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 3 (2026-11-07); ADR018:300 ◐ "mint-once secret versus Render's re-readable schema".
- **Goal linkage:** Render parity first (ADR008); outbound event webhooks.
- **Expected outcome:** losing a webhook secret no longer means deleting and recreating the webhook; Render CLI/API reads get a real value.
- **Why now:** the divergence was never signed off, and the parity rule says to amend the ADR rather than keep it.
- **Decision (2026-11-07):** the user approved the round-7 proposal ("them all to w5"). The secret is returned as Render returns it, to the roles Render lets read webhooks (t001 verifies), with an audit event per read. A stricter admin-only variant would be a divergence needing sign-off.
- **Render parity:** included.

## Overlap record (2026-11-08)

- **Affected:** w5/m285 (all tasks todo) and `w3/done/m282`.
- **Source:** verified evidence at `c258f408f`. w3/m282 shipped the same row-300 change: `ade2f62aa` "the signing secret is readable on list, get and patch, as on Render" and `b8f3d90cb` (store suite). Found by `/pm-brainstorm for w6` 2026-11-08.
- **Next step for worker5:** before t002, diff w3/m282's shipped behavior against this milestone's DoD. Keep only what m282 did not deliver (t001's Render role check, the per-read audit event, any surface m282 skipped). If nothing is left, archive m285 to `w5/done/` as superseded by w3/m282, with that diff as evidence.
- This record does not mark any m285 task done.

## Closed as already met — 2026-11-08 (loopx w5)

`w3/m282` (`ade2f62aa`, done 2026-11-08) shipped this milestone's whole Definition of done while it waited in the queue:

- REST list, get and patch return the `secret` create returned, validated against the pinned schema. A manager gets it on GraphQL and MCP; a member gets neither the secret nor the exact URL (`lego/backend/internal/webhooks/service.go` `toView`, `rest.go` `endpointWire`).
- Webhook Settings reveals and copies the secret.
- ADR018 row 300 and `webhooks-api.md` no longer describe the secret as mint-once.

Render's rule ([render.com/docs/webhooks](https://render.com/docs/webhooks): "Only workspace admins can create and modify webhooks"; the secret is shown on the webhook's Settings page) is the same manager gate `w3/m282` applies. The one item here that `w3/m282` lacks is an audit-log event per secret read, which is not Render behavior, so it is dropped rather than filed. No code change in this milestone.
