# w5 · m33 — Env Groups create completeness: initial contents + metadata

**Worker:** worker5 **Goal:** one `POST /env-groups` (and one dashboard create flow) mints a populated, service-linked group — Render's initial `envVars`/`secretFiles`/`serviceIds` — and the Env Groups pages surface `ownerId`/timestamps. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's `POST /env-groups` initial-contents contract — **DONE** | 30m | — |
| t002 | REST create accepts `envVars`/`secretFiles`/`serviceIds` atomically — **DONE** | 45m | t001 |
| t003 | GraphQL/MCP symmetry — **DONE** | 30m | t002 |
| t004 | Dashboard: create dialog initial contents + metadata display — **DONE** | 45m | t003 |
| t005 | ADR018 env-groups row residual update — **DONE** | 15m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 15m | t008 |

## Definition of done

A single Render-shaped POST creates an env group already containing vars/secret files and linked to services (all-or-nothing — a validation failure leaves no orphan group); the dashboard create dialog offers the same in one step; list/detail show `ownerId` + created/updated timestamps; the residual sentences at `docs/ADR018-render-parity.md:62` are deleted with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — docs miner (ADR018:62: "bex POST still creates empty contents and service links, which clients add afterward… Dashboard list/detail views don't yet surface ownerId/timestamps"). The authenticated 2026-07-15 [dashboard walk](../../../../docs/render-artifacts/dashboard-walk/workspace.md#page-by-page-verdicts) independently confirmed Render's one-step create controls; no duplicate gap was filed from the walk.
- **Goal linkage:** env-groups Render parity (polishes w1/m16 + w5/m26).
- **Expected outcome:** Blueprint- and CLI-driven env-group creation stops needing N follow-up calls; UI metadata matches Render's.
- **Why now:** w6/m24 just reworked env-group attribution — the code is warm and this is its unfinished create-side half. Render parity closing task included — REST/GraphQL/MCP/UI change.

## Resolution

Shipped one shared atomic create verb across REST, GraphQL, and MCP. It validates and prepares initial values, files, Environment membership, and service links before minting state; late write failures compensate OpenBao paths, projection Secrets, and App patches. The dashboard now sends the same initial contents in one mutation and displays exact `ownerId`, `createdAt`, and `updatedAt` values on list and detail views. The pinned Render contract and the closed parity evidence are recorded under `docs/render-artifacts/` and ADR018.

Final verification passed: full backend `go test ./...`; dashboard lint and all 1,168 tests; dashboard production build; Markdown Prettier; and `git diff --check`. The isolated `dev-5` live proof returned REST 201 for a Render-shaped body containing two vars (literal + generated), one secret file, and one linked service, then headless Chrome created a second populated linked group through the real dialog in one submit. Its detail page showed the var, file, service, owner, and both timestamps; sensitive value/file read-back and cleanup also passed.
