# w5 · m285 — A webhook's signing secret is readable after create, as on Render

**Worker:** worker5 **Goal:** `GET /webhooks/{id}` (and GraphQL, MCP, dashboard) returns the webhook's signing secret, as Render's webhook object does, to the roles Render lets read it, with each read audited. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's webhook secret reads and amend ADR052/ADR018 | 30m | — |
| t002 | REST webhook reads return the secret | 1h | t001 |
| t003 | GraphQL and MCP parity; audit every read | 1h | t002 |
| t004 | Dashboard: reveal and copy the signing secret | 1h | t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

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
