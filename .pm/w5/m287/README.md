# w5 · m287 — Sandboxes have names (`--name`), as in Render CLI v2.36

**Worker:** worker5 **Goal:** `render sandboxes create --name`, the NAME column and `list --name` work against bex, with Render's `invalid_sandbox_name` error. **Status:** in progress — t001, t002 and t004–t006 done (shipped); t003's checklist update is done, and its Render CLI v2.36 walk runs on production once deployed, then t007

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store and validate a sandbox name — **DONE** | 1h | — |
| t002 | REST, GraphQL and MCP return the name and filter by it — **DONE** | 1h | t001 |
| t003 | CLI checklist, OpenAPI pin and CLI v2.36 walk | 45m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

Total: **7 tasks, about 4h30m**.

## Definition of done

- With Render CLI v2.36 against bex, `sandboxes create --name foo` succeeds, `sandboxes list` shows a NAME column, and `list --name foo` returns only that sandbox.
- An invalid name answers `invalid_sandbox_name`; REST, GraphQL and MCP all show the name (bex has no Render-sandbox dashboard list; t003 checks whether Render has one).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 5 (2026-11-07); Render CLI v2.36.0 (2026-11-06).
- **Goal linkage:** Render parity first (ADR008); the Render CLI is bex's CLI (DO_NOT_DO: no first-party CLI); pillar 5 reopened 2026-07-27.
- **Expected outcome:** users who upgrade the Render CLI can name sandboxes on bex.
- **Why now:** CLI v2.36 shipped yesterday, and `--name` fails today with `unknown field "name"`.
- **Render parity:** included. This surface is Render's `/sandboxes` API driven by Render's CLI, so render.com is the reference; agent-session sandboxes (Devin reference) are not touched.

## Findings (2026-11-08, loopx w5)

- **Render's contract** (live OpenAPI, `api-docs.render.com`, 2026-11-08):
  - `POST /sandboxes` takes `name`, "a label for the sandbox, like `my-agent`. It isn't unique, so the sandbox ID stays the handle. It can't start with `sbx-`", with `maxLength` 64. A name longer than 64 or starting with `sbx-` answers 400 `invalid_sandbox_name`.
  - `GET /sandboxes?name=` returns "only sandboxes with exactly this name".
  - The sandbox object's `name` is nullable.
  - [render.com/docs/sandboxes](https://render.com/docs/sandboxes) has no Dashboard sandbox page (sandboxes are managed by CLI and SDK), so no dashboard work follows.
- **Storage (t001):** OpenSandbox persists bex's metadata as Kubernetes labels (`ownerID`'s comment in `service.go`), which can't hold 64 arbitrary characters. Names therefore live in bex-api's Postgres (`sandbox_names`, migration 0214), keyed by workspace and the public `sbx-` id.
  - The row is written before the sandbox is created, so a listed sandbox never appears unnamed, and removed if the create fails.
  - The idle sweep prunes rows of vanished sandboxes once they are older than 10 minutes, so a create still in flight keeps its row.
  - With the store off, a named create is refused as unavailable.
- **Surfaces (t002):** REST create, get and list (`?name=` exact match) replace w32/m284's `SANDBOX_NAMES_UNSUPPORTED` refusal. GraphQL `createSandbox(name)`, `sandboxes(name)` and `Sandbox.name`, plus MCP `spawn_sandbox` and `list_sandboxes` `name`, go through the same `Create` and `listFiltered`. Every read answers `"name": null` when unnamed, as Render does. The refusal code is Render's literal `invalid_sandbox_name`.
- **t003:** `docs/cli-compatibility-checklist.md` is updated (the SDK row and the w32/m284 note). The live walk with Render CLI v2.36 (`ea sandboxes create --name`, `list --name`) is pending the production deploy.
- **t004 Render parity:** checked against the live OpenAPI as above; the dashboard has no sandbox list to change, on Render or on bex.
- **t006 tests:** `TestSandboxNames` covers both refusals before any OpenSandbox call, a non-ASCII name, `null` when unnamed, duplicate names, exact list filtering, get, and a failed create leaving no name. `TestSandboxNamesPG` (real Postgres) covers per-workspace isolation, the 64-character check, young rows surviving a prune, prune and delete.
