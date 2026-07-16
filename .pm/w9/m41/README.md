# w9 · m41 — Datastore Render-metadata consistency: audit + hygiene

**Worker:** worker9 **Goal:** Every datastore/service REST read path emits Render's `owner`/`region`/`dashboardUrl`/`updatedAt` identically (postgres, apps, keyvalue), and the one stale comment that misdescribes the shipped wire shape is corrected. **Status:** todo

## Tasks (in order)

| id   | title                                                                        | est | depends_on   |
| ---- | ---------------------------------------------------------------------------- | --- | ------------ |
| t001 | Fix stale `renderKeyValue` comment + retire resolved `w6/016`                 | 20m | —            |
| t002 | Field-by-field cross-sibling metadata parity audit (every read path)         | 45m | t001         |
| t003 | Render parity                                                                | 30m | t002         |
| t004 | Simplify                                                                     | 20m | t003         |
| t005 | Test coverage                                                               | 30m | t003         |
| t006 | Closeout                                                                     | 15m | t005         |

## Definition of done

The `renderKeyValue` doc comment (`lego/backend/internal/keyvalue/rest.go`) matches its code — it no longer claims `Region`/`dashboardUrl` are omitted, since `renderKeyValues` sets both; `version` remains genuinely omitted. `.pm/w6/016.md` sits in `.pm/w6/done/016.md`. Every datastore/service REST read path — list, single-item get, and create/update/suspend/resume responses — emits `owner` (nested `{id,name,email,type}`), `region`, `dashboardUrl`, and `updatedAt` identically across postgres, apps, and keyvalue where the source data exists; any path found dropping a field is fixed, and paths verified clean are recorded. A cross-sibling parity test asserts the four fields are present and identically-shaped on each read path (not merely id/name presence).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 19, 2026-07-15 — shipped-diff mine. `w6/016` (Postgres/Service/KeyValue miss nested `owner`/`region`/`dashboardUrl`/`updatedAt`) verified **resolved** by the `resourcemeta` package's adoption across `postgres/rest.go`, `apps/render.go`, `keyvalue/rest.go`; the mine's one escapee is the stale `renderKeyValue` comment (`rest.go:52-54`) that still claims `Region`/`dashboardUrl` are "deliberately omitted rather than faked" while the code emits both.
- **Goal linkage:** Render API compatibility (`docs/ADR006-bex-api.md` "one core, thin adapters"; `docs/ADR018-render-parity.md`) — `owner`/`region`/`dashboardUrl`/`updatedAt` are part of the datastore/service read contract the official CLI's text renderer reads (`workspaceLine`/`Region:` lines silently vanish when `owner`/`region` are empty).
- **Expected outcome:** the metadata wire fields are provably identical across the three sibling resource types on every read path, and the code's own comments stop misdescribing the shipped shape — closing the class of "already fixed, but only spot-checked by id/name" drift that produced the original KeyValue nested-`owner` and Postgres flat-`ownerId` bugs.
- **Why now:** the `resourcemeta` adoption just landed across three packages — re-verify field-by-field while the surface is warm, before the next feature batch buries it. The stale comment actively misleads the next reader/mine.
- **Render parity:** included — this touches the REST read-shape surface (`owner`/`region`/`dashboardUrl`/`updatedAt`); t003 checks the emitted shape against Render's schema and confirms GraphQL/MCP consistency (or records why those surfaces carry the fields differently).
