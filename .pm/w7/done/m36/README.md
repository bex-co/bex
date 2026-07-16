# w7 · m36 — Per-App registry pull credentials

**Worker:** worker7 **Goal:** each tenant App pulls its images with its own registry credential, so a credential leaked from one tenant's pod cannot read any other tenant's images — closing ADR022's recorded shared-credential blast radius before real tenants exist. **Status:** done

**Resolution:** shipped 2026-07-15 in `0a782eff` (feat(operator): per-App Zot pull credentials) — implementation recorded in `docs/ADR022-tenant-isolation.md` § Per-App pull credentials (`BEX_REGISTRY_NS` + `reg-pull-<name>` Secrets + htpasswd/ACL management). _Task-file statuses were synced after the fact by `w10/m3/t008` — the work landed with this milestone's ship, but the board files were never flipped._

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Design the per-App credential scheme (htpasswd + Zot ACLs)   | 45m | —          | — **DONE** |
| t002 | Operator mints per-App pull credential + Secret              | 45m | t001       | — **DONE** |
| t003 | Zot htpasswd/ACL generation + revocation on App delete       | 45m | t002       | — **DONE** |
| t004 | Compose with w6/m29's imagePullSecret attach                 | 30m | t003       | — **DONE** |
| t005 | Live cross-tenant pull-denial verification + ADR022 update   | 45m | t004       | — **DONE** |
| t006 | Simplify                                                     | 30m | t005       | — **DONE** |
| t007 | Test coverage                                                | 45m | t005       | — **DONE** |
| t008 | Closeout                                                     | 15m | t007       | — **DONE** |

## Definition of done

Tenant A's pod-mounted pull credential cannot pull tenant B's image from Zot (proven live with a real denied pull); deleting an App revokes its credential; `docs/ADR022-tenant-isolation.md:204`'s "shared pull credential" residual is recorded closed with the verification evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — docs miner (ADR022:204: "closing it needs per-App pull creds… deferred for the same sequencing reason as the rest of w7 (before real tenants)").
- **Goal linkage:** tenant isolation (w7 charter; GOAL.md #7); registry authn/z (extends w7/m8).
- **Expected outcome:** registry credential compromise is contained to one App's images.
- **Why now:** the "before real tenants" sequencing clock is the whole point, and w7 is the lightest-loaded workstream. **Sequencing: after `w6/m29`** — the imagePullSecret-attach mechanism this composes with. **Render parity closing task omitted** — registry-internal mechanism; no REST/GraphQL/MCP/UI surface changes.
