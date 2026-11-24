# w9 · m622 — A Contributor deploys a chosen commit or image tag and rolls back, as on Render

**Worker:** worker9 **Goal:** a Contributor's deploy rights on bex are Render's: every deploy the Manual Deploy menu offers, and rollbacks. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Amend ADR012's matrix, ADR024 § The contributor boundary (w1/m68) and ADR018's members row | 30m | — |
| [t002](t002.md) | `Rollback` and a deploy naming `commitId`, `imageUrl`, `buildId` or `clearCache` on `can_operate` | 30m | t001 |
| [t003](t003.md) | Dashboard: Manual Deploy menu and Rollback enabled for a Contributor | 45m | t002 |
| [t004](t004.md) | Role tests and a live walk on dev-9: a Contributor and a Contributor's key roll back and deploy a chosen commit | 1h | t002, t003 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

On dev-9, a Contributor in a Scale workspace rolls back a service and triggers a deploy of a chosen commit, of another tag of the configured image, and with the build cache cleared, on REST, GraphQL, MCP and the dashboard. A Contributor's API key with the Run deploy permission does the same. A different image is still refused, every configuration write stays refused (w9/m619), and a Viewer and a Billing member are refused. ADR012, ADR024 and ADR018 state the line.

## Open decision (recorded as a recommendation, not a user decision)

This milestone reverses w1/m68's hardening (codex round-5 F2: "choosing the code a workload runs" needs `can_create`). Following Render is the recommendation, on Render parity first (ADR008; AGENTS.md: amend an earlier hardening decision that conflicts with parity). Render's table gives a Contributor "Trigger service deploys" and "Trigger rollbacks", and bex already holds an `imageUrl` to the configured image. Rollback is unambiguous on Render. A chosen-commit or chosen-tag deploy is read as part of "Trigger service deploys", because Render's Manual Deploy menu offers it as a deploy. **Still open:** the user may instead sign off keeping chosen-commit and chosen-image deploys Developer-only, as an ADR018 divergence. In that case t002 moves only `Rollback` and the bare and `clearCache` deploys, and the docs record the sign-off.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w9` (2026-11-23), item 4, verified before filing. Render: role table (render.com/docs/team-members, read 2026-11-23): Contributor ✅ "Trigger service deploys", ✅ "Trigger rollbacks". render.com/docs/rollbacks (read 2026-11-23) for what a rollback restores. bex: `lego/backend/internal/deploys/service.go:649-655,869,1383-1388`, `lego/backend/internal/core/keygrant.go:312`, ADR024 § The contributor boundary.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Contributor can do the one job Render gives them, shipping and rolling back code, and the key catalog's Run level stops granting a rollback the verb refuses.
- **Why now:** Render's published table now verifies the Contributor boundary that ADR024 could only guess at, and keys follow their creator's role since w9/m481. Sequenced after w9/m619, which amends the same ADR024 section.
- **Render parity closing task:** included, because REST, GraphQL, MCP and dashboard behavior changes.
