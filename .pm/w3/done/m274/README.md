# w3 · m274 — Legacy /redis routes alias Key Value, as on Render

**Worker:** worker3 **Goal:** Render clients still using the deprecated `/redis` API family work against bex **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Mount /v1/redis aliases over the Key Value handlers with Render's redis shapes — **DONE** | 1.5h | — |
| t002 | Conformance coverage and remove the /redis omissions — **DONE** | 1h | t001 |
| t003 | Update ADR018 and the CLI compatibility checklist — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Every pinned-OpenAPI `/redis*` operation (CRUD, connection-info, suspend, resume) answers in Render's shape over a real Key Value
- Conformance covers them and `renderOperationOmissions` no longer lists them
- ADR018 "Legacy Redis spelling" row reads ✅

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-05 (Render-parity-filtered round, item 5); code-verified at `158cb45d0`; ADR018 omissions row "Legacy Redis spelling" (unsigned)
- **Goal linkage:** Render parity first (ADR008); bex-api is Render-API-compatible (ADR006)
- **Expected outcome:** scripts and older SDKs calling Render's `/redis` API work on bex instead of hitting 404
- **Why now:** small, certain parity gain; the pinned spec still carries the routes, and the omission has no sign-off
- **Render parity:** included — REST surface change; the parity task checks the alias against the Key Value REST/GraphQL/MCP twins
