# w2 · m317 — Manual Postgres storage increases take Render's 5 GB steps and wait 6 hours between them

**Worker:** worker2 **Goal:** every surface that sets a Postgres database's storage (REST, GraphQL, MCP, Blueprint and the dashboard) accepts only Render's sizes. At create that means 1 GB or a multiple of 5 GB; an increase means a multiple of 5 GB above the current size, at most once every 6 hours. Each refusal is a named error saying when the next increase is allowed. **Status:** done

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | One storage-increase rule: the 5 GB step at create and on increase, refused by name on REST/GraphQL/MCP — **DONE** | 50m | — |
| t002 | The 6-hour wait: accepted increases are stamped, and a second within 6 hours (manual or autoscaled) is refused naming when it opens — **DONE** | 45m | t001 |
| t003 | Blueprint create and sync apply the same rule as field conflicts — **DONE** | 40m | t001, t002 |
| t004 | MCP tool text, ADR009 and ADR018 describe the rule — **DONE** | 20m | t003 |
| t005 | dev-2 API check: a 5 GB step is accepted, an off-step size and a second increase within 6 hours are refused — **DONE** | 20m | t003 |
| t006 | Render parity — **DONE** | 20m | t004, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Definition of done

- **Create** (REST, GraphQL, MCP, Blueprint):
  - `diskSizeGB` of 1 or any multiple of 5 is accepted; anything else is refused with a named 400.
  - An omitted size keeps the plan default.
  - A restore keeps its source's size, whatever it is.
- **Increase** (any surface):
  - The requested size must be a multiple of 5 above the current high-water mark; otherwise a named 400. Shrinking stays the existing grow-only 400, and an unchanged size is a no-op.
  - A second increase within 6 hours of the last one, manual or autoscaled, is refused with a named 409 whose params say when the next one is allowed. Blueprint reports it as a field conflict.
- The dashboard Storage edit shows the server's message, as it already does.
- Tests cover each rule on REST, GraphQL and MCP and on the Blueprint path. A dev-2 check confirms the 409 on a real second increase.

## Source + Goal linkage

- **Source:** promoted from [w2/080](../done/080.md) (w2/m307's Render parity task, 2026-11-10).
- **Render** ([creating & connecting § storage](https://render.com/docs/postgresql-creating-connecting), fetched 2026-11-10):
  - Create: "You can specify 1 GB or any multiple of 5 GB."
  - API increase: "Provide any multiple of 5 GB greater than the current storage capacity."
  - "After you increase a database's storage, you can't increase it again for 6 hours."
  - The CLI reference (`--disk-size-gb`): "Must be 1 or a multiple of 5."
- **bex today:**
  - `validateDatabaseStorageResize` (`lego/backend/internal/postgres/service.go`) checks only grow-only. Blueprint's apply (`apps/blueprint_plan.go`) checks shrink and admission.
  - Disk autoscaling stamps `app.bex.co/disk-autoscale-at` on the Database (`lego/operator/internal/controller/database_autoscale.go`) for its own 12-hour cooldown.
  - The dashboard enforces the 5 GB step client-side only (w2/m307).
- **Design:**
  - The rule lives in one exported `postgres` helper, used by bex-api's update path and Blueprint's apply.
  - The step applies to the size a caller asks for: an unchanged legacy size keeps syncing, and a restore keeps its source's size.
  - Accepted manual increases stamp `app.bex.co/storage-increased-at`. Both annotation names move to `lego/types` so the operator and bex-api share them.
- **Goal linkage:** Render parity first (ADR008), V0 roadmap item 4 (PostgreSQL), ADR018 Postgres storage row.
- **Expected outcome:** a Render user's storage request behaves on bex exactly as on Render: odd sizes are refused, and rapid back-to-back increases are refused with the time they open.
- **Why now:** w2/m307 put storage editing in every paid database's dashboard and on GraphQL. The API is now the only surface that still accepts off-step sizes and unlimited back-to-back volume expansions.
- **Render parity included:** REST, GraphQL, MCP and Blueprint behavior changes.
