# w4 · m109 — Projects are the one resource family the Render-metadata adoption skipped: `updatedAt` is frozen and `owner` is born blank

**Worker:** worker4 **Goal:** a project's REST object reports when it was actually last modified and who actually owns it, the way every other bex resource already does. **Status:** done — code+tests green (`go test ./internal/projects/ ./internal/api/ ./internal/resourcemeta/`); live production probe awaits deploy (no-ship constraint)

## Tasks (in order)

| id   | title                                                                          | est | depends_on                 |
| ---- | ------------------------------------------------------------------------------ | --- | -------------------------- |
| t001 | `updatedAt` reports the project's real last-modified time — **DONE**            | 50m | —                          |
| t002 | The project's embedded `owner` carries the owner's name and email — **DONE**    | 50m | —                          |
| t003 | Project conformance asserts field _values_, not just key sets — **DONE**        | 40m | w4/m109/t001, w4/m109/t002 |
| t004 | Render parity: projects across REST, GraphQL, MCP and the dashboard — **DONE**  | 25m | w4/m109/t003               |
| t005 | Simplify the code this milestone touched — **DONE**                             | 20m | w4/m109/t004               |
| t006 | Test coverage for the shipped behavior — **DONE**                               | 40m | w4/m109/t004               |
| t007 | Closeout — **DONE**                                                             | 15m | w4/m109/t005, w4/m109/t006 |

## Definition of done

Each bullet is a probe the next person can re-run against production.

- **A rename moves `updatedAt`.** `PATCH /v1/projects/{id}` with a new name, then `GET /v1/projects/{id}`: `updatedAt` is later than `createdAt` and later than the pre-rename `updatedAt`. Today all three timestamps are byte-identical — the 2026-09-16 capture below shows a successful rename leaving `updatedAt` unchanged.
- **Every project-mutating verb moves it.** The same holds after each of the three `PUT .../{service,database,keyvalue}-links` calls, which already write `updated_at = now()` in the store (`internal/store/projects.go:194,203`).
- **The env-group control still behaves.** `PATCH`ing an environment group's name still advances its `updatedAt` — verified working on 2026-09-16 (`07:30:06Z` → `07:30:36Z`), and it is the proof that this is a projects defect, not a platform convention.
- **`owner` is populated.** `GET /v1/projects/{id}` and `GET /v1/projects?ownerId=…` return `owner.name` and `owner.email` matching what `GET /v1/owners` reports for the same `owner.id` — the same values `GET /v1/services`, `/v1/postgres` and `/v1/key-value` already return today.
- **An unresolvable owner is omitted, not blanked.** When the owner cannot be resolved or is not visible to the caller, the response follows `resourcemeta`'s stated contract (`internal/resourcemeta/metadata.go:53-58`: "must be omitted, never replaced with an id-as-name placeholder") — decide and implement whether that means omitting the sub-fields or the whole `owner` object, and make the Render schema accept it.
- **A blank value fails CI.** Reverting either fix turns `cd lego/backend && go test ./internal/api/...` red. Demonstrate it, the way `w6/m126` t003 demonstrated its own drift guard — that milestone's key-set-only conformance check is exactly why these two wrong values shipped green.
- **The other three families are unchanged.** `GET /v1/services`, `/v1/postgres`, `/v1/key-value` return byte-identical `owner` blocks before and after.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-16 (w4-targeted run, pass 2). Journeys exercised: project + environment create/rename/delete, environment-group create/edit/rename/delete, resource-family API sweep. Created and deleted `qa-20260916-p1` (`prj-dal49f3kmutc73d7qaig`) and `qa-20260916-grp` (`evg-dal4aqu7hpms73fqh0ng`).

  The rename probe, run from inside the authenticated page:

  ```
  GET   /v1/projects/prj-dal49f3kmutc73d7qaig
    createdAt 2026-09-16T07:26:52.362014Z   updatedAt 2026-09-16T07:26:52.362014Z

  PATCH /v1/projects/prj-dal49f3kmutc73d7qaig  {"name":"qa-20260916-p1-renamed"}  -> 200
    {"id":"prj-dal49f3kmutc73d7qaig","name":"qa-20260916-p1-renamed",
     "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"","email":"","type":"team"},
     "environmentIds":["env-dal49jrkmutc73d7qak0"],
     "createdAt":"2026-09-16T07:26:52.362014Z","updatedAt":"2026-09-16T07:26:52.362014Z"}

  GET   /v1/projects/prj-dal49f3kmutc73d7qaig  -> name changed, updatedAt STILL 07:26:52.362014Z
  ```

  The owner comparison across the four families, same workspace, same session:

  ```
  /v1/projects?ownerId=…  "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"","email":"","type":"team"}
  /v1/services?limit=1    "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"bex","email":"puncsky@gmail.com","type":"team"}
  /v1/postgres?limit=1    "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"bex","email":"puncsky@gmail.com","type":"team"}
  /v1/key-value?limit=1   "owner":{"id":"tea-d98210cbbpdc73dcrkvg","name":"bex","email":"puncsky@gmail.com","type":"team"}
  ```

  Root cause, both in ten lines: `lego/backend/internal/projects/rest.go:56-66` `toRenderProject` sets `Owner: renderOwner{ID: p.OwnerID, Type: "team"}` — no name/email lookup — and `UpdatedAt: p.CreatedAt`, a hardcoded copy. `ProjectView` (`internal/projects/service.go:156-164`) has no `UpdatedAt` field at all, so the value the store already maintains (`internal/store/projects.go:59` `UPDATE projects SET name = $2, updated_at = now()`) is written and then never read back.

- **Precedent, and why this is not a duplicate:** `.pm/w6/done/016.md` described exactly this defect — "Postgres/KeyValue/Service REST objects never populate Render's `owner` … or `updatedAt`" — and was retired as "resolved by resourcemeta adoption (`w9/m41`)". `w9/m41`'s goal names its scope as "postgres, apps, keyvalue"; **projects was never in it**, and `internal/projects/` imports `resourcemeta` nowhere. This is the fourth family, still carrying the original defect. Separately, `w6/m126` reworked this exact file so every project verb funnels through `toRenderProject` — its own source capture quotes both the blank `owner` and `createdAt == updatedAt`, but its DoD only asserted **key sets**, so both wrong values were normalized into every handler and shipped green. That is the specific gap `t003` closes.
- **Goal linkage:** Render API compatibility ([docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md), [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)). `updatedAt` is what a CLI, a sync client or a cache uses to decide whether it has stale data; a frozen one is worse than a missing one because it reads as authoritative.
- **Expected outcome:** the four resource families answer the same question the same way, and `render-oss/cli` (the pinned compatibility client, `docs/cli-compatibility-checklist.md`) shows a real owner and a real modification time for a project.
- **Why now:** the mechanism is a five-line omission, the correct seam (`resourcemeta.OwnerResolver`) already exists and is already wired into the three sibling families, and CI is currently structurally unable to catch either bug — so the same class will keep shipping until `t003` lands.
- **Render parity task included** because projects are a user-facing REST surface with GraphQL and MCP representations and a dashboard consumer.
