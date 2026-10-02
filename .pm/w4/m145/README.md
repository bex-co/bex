# w4 · m145 — Report ordinary Save-only configuration as pending

**Worker:** worker4 **Goal:** Make the saved-versus-running indicator reflect ordinary Save-only changes without deploying them. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Track effective Save-only divergence without a rollout | 55m | — |
| t002 | Audit shared writers, runtime types and passing controls | 40m | t001 |
| t003 | Render parity and API/dashboard verification | 25m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 30m | t003, t004 |
| t006 | Closeout with live replay and cleanup | 20m | t005 |

## Definition of done

- On a disposable free Go web service using the [recorded file-to-MESSAGE recipe](finding.md#reproduce), deploy v1, edit the existing file to v2 with **Save only**, reload and Reveal. Saved content is v2, external HTTP still serves v1, no deploy is opened, GraphQL reports `undeployedChanges: true`, REST/MCP service reads carry true, and both dashboard notices explain that saved changes are not live.
- A standard deploy applies v2 and clears the difference once Live. Save only v3 repeats the expected state: saved v3, HTTP v2, true pending indicator after a fresh load. This works for existing references, not only the first file.
- Repeat the observed controls: cancel a v2 build while v1 serves; deploy v2; roll back to v1; Restart; deploy by hook. Cancel and historical selection retain saved v2 and a true flag while v1 serves; Restart preserves that runtime; successful standard/hook deployment applies v2 and clears the flag. List, detail and Events agree on terminal outcomes.
- Delete the fixture, verify by-id 404 and no owned runtime residue, revoke its QA session, and record live evidence. Unexercised adjacent cases remain explicit in t002/t005, rather than inheriting a claimed live pass.

## Source + Goal linkage

- **Source:** Continuous `qa-find-bugs` sweep 5 on 2026-10-02, using `muse.env`, user-selected w4. [Finding, complete probes and cleanup](finding.md). Screenshots: `.playwright-mcp/qa-env-r5-save-only-pending.png`, `.playwright-mcp/qa-env-r5-save-only-repeat.png`; durable probes are in the finding.
- **Goal linkage:** ADR008 dependable hosting, ADR004 saved-versus-runtime semantics, ADR006 consistent APIs.
- **Expected outcome:** Users can distinguish saved configuration from what is running before they deploy. Save only continues to defer application.
- **Why now:** Two fresh production saves reproduce false reporting, including after a standard deploy correctly cleared the same flag. Cancellation and rollback fixes leave this ordinary path uncovered.
- **Render parity included:** REST/GraphQL/MCP/UI consume this status. Render documents deferred application; this field/banner is a Bex extension.
- **Scope:** One major reporting bug, not lost data or broken deploy dispatch. About 3h 10m across six tasks, including a dedicated shared-code audit. This filing implements no product fix.
