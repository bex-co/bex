# w4 · m145 — Report ordinary Save-only configuration as pending

**Worker:** worker4 **Goal:** Make the saved-versus-running indicator reflect ordinary Save-only changes without deploying them. **Status:** done 2026-10-02 — t001–t006 complete; production replay passed

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Track effective Save-only divergence without a rollout — **DONE** | 55m | — |
| t002 | Audit shared writers, runtime types and passing controls — **DONE** | 40m | t001 |
| t003 | Render parity and API/dashboard verification — **DONE** | 25m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003, t004 |
| t006 | Closeout with live replay and cleanup — **DONE** | 20m | t005 |

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

## Implementation outcome — 2026-10-02

[Implementation and local verification](implementation.md) complete t001–t005. **Gate:** platform release owner deploys backend/operator/dashboard; QA owner replays every live DoD and records fixture/session cleanup. t006 remains open. No production fixture/session was created in this implementation run.

## Live acceptance — 2026-10-02

Production carried fix `905ceb299` (ancestor of deployed `18958459c`). Fixture `qa-20261002-w4x-saveonly` (`srv-db0a308ehcmc739j12vg`), free Go web, `bex-co/bex@main` root `examples/hello-go`, build `go build -o app .`, start `export MESSAGE="$(cat /etc/secrets/qa-w4x-message)"; exec ./app`, port 3000, auto-deploy off, secret file `qa-w4x-message`. Save-only writes used the dashboard's own `patchServiceEnvironment(saveMode:"save_only")` GraphQL mutation; reads used GraphQL, REST `GET /v1/services/{id}`, MCP `get_service`, the dashboard, and external `curl`. Times UTC 2026-10-03.

| Step | Deploy | Saved file | External HTTP | `undeployedChanges` |
| --- | --- | --- | --- | --- |
| v1 initial | `dep-db0a308ehcmc739j1300` live 06:36:25 | v1 | v1 | — |
| Save only v2 (`rolledOut:false`, no new deploy) | — | v2 (dashboard Reveal after reload) | v1 | true on GraphQL, REST and MCP; both dashboard notices "Saved changes aren't live yet. Use a standard deploy to apply them." |
| Cancel v2 build | `dep-db0a63oehcmc739j1390` canceled | v2 | v1 | true |
| Standard deploy | `dep-db0a6a8ehcmc739j13ag` live 06:41:25 | v2 | v2 | false |
| Roll back to v1 | `dep-db0a7coehcmc739j13bg` live (trigger rollback) | v2 | v1 | true |
| Restart | `dep-db0a7v2tm2ss7389qnqg` live | v2 | v1 | true |
| Deploy hook | `dep-db0a8c8ehcmc739j13cg` live 06:45:36 (trigger deploy_hook) | v2 | v2 | false |
| Save only v3 | — | v3 | v2 | true (GraphQL, REST; fresh dashboard load shows both notices) |

Dashboard Deploys list and Events agree on every terminal outcome (Canceled, Live, Rollback to `dep-db0a308ehcmc739j1300`, Deploy Hook, Deactivated). Minor copy observation, not in scope: Restart appears as "Manual Deploy" in the list/Events. Screenshot `.playwright-mcp/qa-w4x-m145-saveonly-v2.png` (gitignored).

**Cleanup:** fixture deleted; REST GET → 404; read-only kubectl finds no `w4x` App, Deployment, Service, Ingress, Secret or Pod. QA session revoked at the end of the run.
