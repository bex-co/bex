# w3 · m475 — The official Render CLI's log viewer can scroll back on bex: `GET /v1/logs` accepts its `limit=1000`

**Worker:** worker3 **Goal:** scrolling up in `render logs`' interactive viewer loads older lines on bex, as it does on Render, instead of failing with `400 limit must be an integer between 1 and 100`. **Status:** todo (t001–t004, t006–t008 done; t005 waits for the production pin)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The Render contract gate accepts `limit` up to 1000 on `GET /v1/logs` — **DONE** | 45m | — |
| [t002](done/t002.md) | A page asked for more than 100 lines serves 100 with live cursors — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Contract test: the pinned Render CLI's paginated log request succeeds — **DONE** | 40m | t002 |
| [t004](done/t004.md) | Docs: CLI checklist, ADR010 and ADR018 — **DONE** | 15m | t003 |
| [t005](t005.md) | Production check with the unmodified Render CLI binary | 30m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t004 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](t009.md) | Closeout | 10m | t005, t008 |

## Definition of done

- On production, `GET /v1/logs?…&limit=1000` for a service the caller can read answers 200. The body follows Render's log list shape, with at most 100 lines (t002's recommendation), `hasMore` and following `nextStartTime`/`nextEndTime`.
- `limit=0`, `limit=1001` and a non-integer still answer 400. The message names the new bound.
- A backend test sends the pinned `render-oss/cli` client's request with `limit=logs.PaginationLogLimit` through the real handler stack, Render contract gate included, and gets 200.
- The unmodified Render CLI release binary pointed at `api.bex.co` runs `render logs -r <qa service> --limit 1000 -o json` with exit 0 and lines, as the QA user in `bex-canary`. Scrolling up in the interactive viewer loads an older page with no error (t005).

## Source + Goal linkage

- **Renumbered:** filed as `w3/m473` on 2026-11-18 (`3103fa87f`), renumbered to m475 the same day. w9 had filed m472–m474 four minutes earlier (`054f29128`), and milestone numbers are global. `w3/m472` keeps its number: it shipped first, and every reference to it is workstream-qualified.
- **Source:** the `/loopx w3` drained report of 2026-11-17, finding 5. That finding came from the deleted first draft of ADR113, which traced the CLI's log path. `/pmx 1,2,3,5 to w3` filed it on 2026-11-18. Re-checked against `origin/main` `d41bbb9c2`.
- **Evidence:**
  - The pinned CLI (`render-oss/cli@v1.1.3-0.20261106232710-4fd34231b637`, `pkg/tui/views/logview.go:196-203`) sets `paginatedInput.Limit = logs.PaginationLogLimit` for every scroll-to-load request, and `pkg/logs/constants.go:7` sets `PaginationLogLimit = 1000`.
  - Render's own CLI does this against Render's own API, so Render's server accepts the request.
  - The pinned Render OpenAPI declares `limit` `maximum: 100` on `/logs` (`api/openapi/render-public-api-1.json`). bex's contract gate enforces declared numeric bounds (`render_openapi.go` `parameterConstraintMessage`).
  - Live on production, 2026-11-18, as the QA user: `GET https://api.bex.co/v1/logs?ownerId=…&resource=srv-…&limit=100` gave 404 (the resource does not exist, so it got past the gate). The same request with `limit=1000` gave `400 {"id":"bad_request","message":"limit must be an integer between 1 and 100"}`.
  - The logs service itself would clamp to 100 (`logs/service.go:128-134`, `normalized` at `:394-405`). Only the gate refuses.
  - No other pinned-CLI request exceeds a declared bound: every other `Limit:` in `pkg/` is 100 or less.
- **Prior work:** w4/m107 (2026-09-14) fixed the cursors this same scroll path feeds back (`.pm/w4/done/m107/README.md`). It did not touch `limit`, so the scroll still fails one step later.
- **Open decision, recorded as a recommendation, not a user decision:** the page size served for `limit` 101–1000.
  - **Recommendation:** serve at most 100, Render's documented maximum and bex's existing clamp, with `hasMore` and cursors, so the CLI keeps paging.
  - **Basis:** ADR008 Render parity first. The official CLI must work, and parity work is never parked on an optional decision.
  - **Still open:** whether Render's server returns up to 1000 lines for such a request. It is unverified here, with no Render account in reach. If it does, record the page-size difference in ADR018 or file a follow-up that raises the clamp.
- **Goal linkage:** Render CLI compatibility ([ADR018](../../../docs/ADR018-render-parity.md), `docs/cli-compatibility-checklist.md`, `DO_NOT_DO.md`'s "point Render's own CLI at bex-api"). `render logs` is one of the most-used CLI commands on the `bex-cli-usage` board (1.34K commands in 30 days, 11.5% failing).
- **Expected outcome:** a customer reading logs in the official CLI's viewer can scroll back through history, as on Render.
- **Why now:** it is a live, reproducible failure of the official CLI's default interactive log view. The fix is small and local to the gate.
- **Render parity included:** the change alters a REST validation boundary that the CLI, the API docs and the MCP `list_logs` tool all describe.

## Render parity (t006, 2026-11-18)

| Surface | `limit` accepted | Page served | `hasMore` and cursors | Out of range |
| --- | --- | --- | --- | --- |
| REST `GET /v1/logs` | 1 to 1000 (this milestone) | at most 100 | yes, `queryLogPage` | 400 `limit must be an integer between 1 and 1000`; a non-integer gets the gate's generic 400 `invalid query parameter "limit"` |
| REST `/v1/logs/subscribe`, `/v1/logs/values` | 1 to 100 (pinned spec) | a tail; a values list cut at `limit` | — | 400 `limit must be an integer between 1 and 100` |
| GraphQL `logs(limit:)` | any Int, 0 or less means 20 | at most 100 | yes, the same `queryLogPage` | clamped, no error (a bex-native surface, no Render counterpart) |
| MCP `list_logs` | any number ("max 100", as Render's `list_logs` says; Render's pinned tool types it `number` with no bound) | at most 100 | yes, the same envelope | clamped, no error |
| Dashboard | sends at most 100 through GraphQL | at most 100 | follows the cursors | — |
| Render API docs | 1 to 100 | — | yes | — |
| Render CLI (pinned) | sends 100 for the first page and 1000 (`logs.PaginationLogLimit`) for every older page; `--limit` passes through to a list and to a tail | — | follows the cursors | — |
| Render's server for `limit=1000` | admits it, since the CLI's scroll works on Render | **unverified**, no Render account in reach | — | — |

Differences and where each one stands:

- **The page size Render serves for `limit=1000` is unverified.** It is recorded in ADR018's Application logs row as a recommendation without the user's sign-off. It was not filed, because it was not found to exceed 100.
- **A user's own `render logs --tail --limit` above 100** still gets the pinned 400 from `/logs/subscribe`. Whether Render's subscribe admits it is unverified too. The CLI's own code never sends more than 100 there unless asked to, so it stays as pinned (t001 step 3).
- **GraphQL and MCP clamp an out-of-range limit instead of refusing it.** This predates the milestone and is unchanged (t002). Render has no GraphQL, and its MCP tool takes an unbounded `number`.

