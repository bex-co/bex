# w6 · m291 — Build toolchain digests are fresh again and #105 closes

**Worker:** worker6 **Goal:** every pinned build-toolchain image is at a reviewed current digest, apps still build on the new toolchain, and the freshness monitor's issue #105 closes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Re-resolve the 25 drifted build-toolchain entries and verify their manifests](t001.md) | 45m | — |
| t002 | [Update every pinned file and the freshness inventory](t002.md) | 45m | t001 |
| t003 | [Qualify the new toolchain: build hello-go and one Node or Python buildpack app on dev-6](t003.md) | 1h | t002 |
| t004 | [Close PR #106 with the reason and confirm the monitor closes #105](t004.md) | 20m | t002 |
| t005 | [Simplify](t005.md) | 15m | t003, t004 |
| t006 | [Test coverage](t006.md) | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- `toolchain-freshness.json` and every pinned file carry the reviewed digests; the freshness workflow's next run reports no drift and #105 is closed.
- hello-go and one Node or Python buildpack app build and serve on dev-6 with the new toolchain; after deploy, a production build succeeds.
- PR #106 is closed with an explanation.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-08, proposal 2; issue #105 (open since 2026-10-22, 25 entries on 2026-11-05).
- **Goal linkage:** ADR008 in-cluster builds — reliable and secure builds (the CNB builder drift includes CVE fixes; ADR060 D7).
- **Expected outcome:** customer builds use current, reviewed toolchain images and the monitor's review queue is empty.
- **Why now:** the queue has grown for 17 days, partial refreshes (`93bc3ef2e`, `ee7e3eaa7`) left it open, and the only fix attempt (PR #106) is unmergeable.
- **Render parity:** omitted — build toolchain pins only, no REST/GraphQL/MCP/UI change.
