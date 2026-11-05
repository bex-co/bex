# w2 · m269 — Blueprint ids answer as `exs-`, Render's prefix, so the v2.35 CLI can address them

**Worker:** worker2 **Goal:** every surface shows a Blueprint as `exs-<xid>`, Render's form, and accepts both `exs-` and the durable `blp-`. The pinned Render CLI v2.35.0 then works on a bex Blueprint: `ea blueprints approve|plan|update`, and the `blp-…` ids it shows. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | `id.BlueprintPublicID` / canonical parse, mirroring `EnvironmentPublicID`              | 30m | —                      |
| t002 | REST, GraphQL and MCP emit `exs-` and accept both prefixes on every Blueprint id input | 1.5h | t001                   |
| t003 | Dashboard routes, links and `dashboardUrl` use the public id; `blp-` URLs still resolve | 45m | t002                   |
| t004 | Events, webhooks, audit metadata and resource labels a client reads carry the public id | 45m | t002                   |
| t005 | Amend ADR020 (the 2026-08-26 "prefix is not changing" line) and the OpenAPI path gate   | 20m | t002                   |
| t006 | Render parity                                                                          | 15m | t003, t004, t005       |
| t007 | Simplify                                                                               | 15m | t006                   |
| t008 | Test coverage                                                                          | 30m | t006                   |
| t009 | Closeout                                                                               | 10m | t007, t008             |

## Definition of done

- `GET /v1/blueprints`, the create response, GraphQL and MCP return `exs-…` ids. Every Blueprint id input accepts `exs-…` and the legacy `blp-…` and resolves both to the same Blueprint. Stored rows, labels and foreign keys stay `blp-…`; nothing is migrated or recreated.
- With bex CLI v2.35.0, `bex ea blueprints approve <id>`, `plan <id>` and `update <id> --branch` pass the CLI's `exs-[a-z0-9]{20}` check and reach bex-api.
- The dashboard's Blueprint pages and links use the `exs-` id, and old `blp-` links still open.
- ADR020 records the public/durable split, citing the environments precedent (`evm-` public, `env-` durable).

## Source + Goal linkage

- **Source:** the w2/m256 t007 production walk, 2026-11-05.
  - Pinned v2.35.0 CLI, QA workspace `tea-daif693dqjvc73e7as3g`.
  - `bex ea blueprints create --repo https://github.com/bex-co/bex --path examples/static-site/render.yaml --existing-resources adopt` exited 0 with `planningOutcome: planned` and created nothing.
  - The next step, `bex ea blueprints approve blp-db136288mmqc73d4hpug`, exited 1 before any request: `invalid Blueprint ID "blp-db136288mmqc73d4hpug": Blueprint IDs start with exs- followed by 20 lowercase letters or digits`.
  - Upstream added `validate.IsBlueprintID` (`pkg/validate/id.go`) in v2.35.0 and calls it in `pkg/blueprint/service.go` and `pkg/blueprint/manualenvvars.go`.
  - The test Blueprint was deleted afterwards (REST DELETE 204, GET 404).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). ADR020 already lists `blp-` vs `exs-` as an unrecorded gap that "had a live cost". Under the parity rule its "prefix is not changing" line gets amended rather than preserved.
- **Expected outcome:** the Render CLI's Blueprint onboarding commands (w2/m256) work end to end against bex, and a Render user's tooling that matches `exs-` ids works unchanged.
- **Why now:** it blocks m256 t007, so the shipped v2.35 bex CLI cannot approve a Blueprint it just created.
- **Render parity included:** REST/GraphQL/MCP/dashboard id surface.
