# w5 · m120 — Env-group links store service ids and are removed when the service is deleted

**Worker:** worker5 **Goal:** Deleting a service unlinks it from every env group, and a later service with the same name is never treated as linked. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce a same-name service after delete | 30m | — |
| t002 | Service delete unlinks its env groups | 45m | t001 |
| t003 | Store service ids in link sets and migrate existing names | 1h | t002 |
| t004 | Delete the deleted-service unlink fallback | 20m | t003 |
| t005 | Render parity | 15m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- After deleting "web" and creating a new "web", the group lists no linked service, and Unlink doesn't restart the new service.
- Deleting a linked service removes it from every group, leaving no dangling entries.
- Existing name-based link entries are migrated to ids; dangling ones are dropped and logged.
- `unlinkDeletedService` and its cluster-wide List are gone.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `ee1341133` (w4/183). Scenario: a Blueprint links group G to "web"; "web" is deleted and an unrelated "web" is created. G now shows the new "web" as linked, and Unlink restarts it through `rollLinked`. w4/183's fix covers only the not-found case.
- **Goal linkage:** ADR006/ADR018 env group parity; ADR008 reliability.
- **Expected outcome:** Env-group links can't leak to unrelated services.
- **Why now:** w4/183 patched one symptom on 2026-10-04, and the same-name restart is a cross-service side effect.
- **Render parity included:** env-group link lists change on every surface.
