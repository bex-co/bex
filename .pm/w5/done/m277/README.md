# w5 · m277 — One `usr-` user id on every surface, as on Render

**Worker:** worker5 **Goal:** a person has one Render-shaped user id (`usr-…`) wherever bex reports one — the members list, `GET /v1/users`, audit-log actors — and the member mutations accept it. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A `usr-` public alias for user ids — **DONE** | 45m | — |
| t002 | Members answer `usr-` and the member mutations accept it — **DONE** | 1h | t001 |
| t003 | `GET /v1/users` answers the caller's `usr-` id — **DONE** | 45m | t001 |
| t004 | Audit-log user actors carry the `usr-` id — **DONE** | 1h | t001 |
| t005 | Render parity — **DONE** | 30m | t002, t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 5h15m**.

## Definition of done

- `GET /v1/owners/{ownerId}/members` `userId`, `GET /v1/users` `id` and a user actor's audit-log `id` are the same `usr-[0-9a-v]{20}` for the same person, on REST, GraphQL and MCP where those surfaces carry it.
- `PATCH`/`DELETE /v1/owners/{ownerId}/members/{userId}` accept that `usr-` id and the `own-` ids handed out before.
- Storage keeps `own-`; ADR020 and ADR018 record the alias.

## Source + Goal linkage

- **Source:** promoted from [w5/179](done/179.md) (found by w5/m267's parity pass, 2026-11-05) — over an hour across four surfaces, so a milestone.
- **Goal linkage:** ADR008 Render parity first; ADR020 identifiers; ADR018 REST compatibility.
- **Expected outcome:** Render clients and scripts that read or validate user ids round-trip on bex; one person stops having two different ids (`GET /v1/users` returns the raw subject today, the members list `own-…`).
- **Why now:** w5/m267 just made member mutations reachable by `userId`, and Render's spec pins that id to `usr-`; no ADR018 row signs the divergence.
- **Render parity:** included — REST, GraphQL, MCP and the dashboard show user ids.

## Closeout (2026-11-05)

- **Alias:** `id.UserPublicID` / `id.UserStorageID` respell the durable `own-` key as Render's `usr-` (the `respell` the Blueprint and Environment aliases use).
- **Members:** the Render members list and bex's native members views (REST/GraphQL/MCP `userId`) answer `usr-`. `PATCH`/`DELETE /v1/owners/{ownerId}/members/{userId}` resolve both spellings.
- **`GET /v1/users`:** answers the caller's `usr-` id (minted on first sight, as the members list does) instead of the raw Kratos subject. A store failure degrades to no id rather than failing whoami.
- **Audit log and events:** Render REST `user` actors, and the events feed's triggering/suspending/resuming users, answer `usr-` through one non-minting lookup (`store.LookupOwnerIDs` → `core.PublicUserIDs`). API-key clients keep their subject and never acquire user ids.
- **Render parity (t005):** one person now has one `usr-` id across whoami, members, audit logs and events. One gap is recorded rather than fixed: a user-consented OAuth token (for example a CLI device login) is classified `rest_api` on the audit log and keeps its raw subject (ADR020).
- **Simplify (t006):** the review is applied:
  - one shared resolution helper;
  - the non-minting lookup for read surfaces;
  - whoami degradation;
  - the ADR018 table row repaired;
  - stale `own-` wording fixed (including an MCP tool description).
- **Tests (t007):**
  - id round-trip;
  - members, owners REST and whoami assertions;
  - both mutation spellings;
  - audit and events resolution, including that API-key clients are left alone;
  - a real-DB store test proving `LookupOwnerIDs` mints nothing.
- **Docs:** ADR020 "User public aliases", plus the ADR018 Workspace members and Audit logs rows.
