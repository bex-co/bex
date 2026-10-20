# w4 · m238 — A moved service's out-of-scope environment group is flagged, not silent

**Worker:** worker4 **Goal:** a service that leaves its environment while still linked to that environment's group is moved knowingly, and every page that shows the link says it is out of scope. Today the move is silent, the service page lists the link as ordinary, and the group page calls it "Legacy". **Status:** blocked (t001–t006 done 2026-10-20, loopx w4; t007 closeout gated on the dashboard deploy + live DoD replay)

## Tasks (in order)

| id              | title                                                                                 | est | depends_on                 |
| --------------- | ------------------------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Confirm before an environment move strands a scoped group link — **DONE** | 45m | —                          |
| [t002](done/t002.md) | Same confirmation on Move to project and Remove from project — **DONE** | 25m | w4/m238/t001               |
| [t003](done/t003.md) | Flag out-of-scope linked groups on the service Environment page; drop "Legacy" wording — **DONE** | 30m | —                          |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | w4/m238/t002, w4/m238/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | w4/m238/t004               |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | w4/m238/t004               |
| [t007](t007.md) | Closeout                                                                              | 20m | w4/m238/t005, w4/m238/t006 |

Seven tasks, 185m (3h05m). One **major** finding, recorded with complete probes in [finding.md](finding.md). This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) on production (`https://dashboard.bex.co`) with fresh owned fixtures: a project with Production and Staging, a Free web service in Production, and a Production-scoped group with a synthetic `MESSAGE` linked to it.

- **The move asks first.** Project page → tick the service → **Move selected resources to environment** → Staging → **Move** opens a confirmation. It names the group with `<project> / Production` and says its values keep applying until it is unlinked or moved. **Cancel**: `GET /v1/services/<id>` still reads `environmentId` = Production and no write was sent. **Confirm**: the service reads Staging, `GET /v1/env-groups/<id>` still lists the service in `serviceLinks`, and the public URL keeps answering.
- **No dialog without a stranded link.** The same Move for a second service with no linked group happens immediately, exactly as today.
- **The service page says it.** A fresh `/services/<id>/env` after the move shows, on the linked group's row, a notice naming Production and Staging next to **Unlink**. A Staging-scoped group linked to the same service shows no notice. While the scope index is pending, the row shows no notice.
- **The group page says it without "Legacy".** A fresh `/env-groups/<id>` shows the new out-of-scope wording on that row; the string "Legacy" appears nowhere on the page.
- **Guards unchanged.** `POST /v1/env-groups/<prod-group>/services/<staging-service>` still answers `409 ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH` with the same params. Moving the group to an environment its linked service is not in still answers `ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES` and changes nothing.
- **Checks pass:** dashboard `yarn lint` and `yarn test`. Fixtures deleted (by-id reads 404, public URL 404), and only this run's session is revoked.

The Manage resources, Move to project and Remove from project paths were not exercised live by the hunt. t001/t002 verify them as work; they are not asserted here.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run c7a1, 2026-10-20 UTC, `muse.env`, workspace `bex-canary`. Probes, captures and root cause are in [finding.md](finding.md). Local evidence (gitignored): `.playwright-mcp/qa-envgroup-c7a1-1.png` (group page "Legacy link") and `.playwright-mcp/qa-envgroup-c7a1-2.png` (service page without a notice).
- **Goal linkage:**
  - ADR008: trustworthy managed configuration.
  - ADR032: environment-scoped groups, environment membership, the `w6/m24` group-scope extension.
  - ADR018: environment-group parity row.
  - Render's [scoped configuration](https://render.com/docs/projects#scoped-configuration): an environment-scoped group is meant for that environment's services.
- **Expected outcome:** no service ends up running another environment's group without a confirmation at the move and a visible notice afterwards. The group-side guards and the readable API state stay exactly as they are.
- **Why now:** this is the case environment scoping exists to prevent: a staging copy running on production credentials. It is reachable with one click on the project page, and live it served the Production group's value from a Staging service. The two neighbouring fixes (`w4/217` service-panel candidates, `w4/257` pending-state warning) both left "genuine out-of-scope link" unverified; this pass produced one through an ordinary move.
- **Dedupe:**
  - `w4/217`: Available-to-link filter (fixed; its candidate list was correct here).
  - `w4/257`: the warning flashing while the scope index loads; a different state.
  - `w4/m187`: project moves clear environment placement. It deliberately preserves links, and this milestone keeps that.
  - `w4/183`: unlinking a deleted service's link.
  - `.pm/DO_NOT_DO.md`: no matching anti-goal. No open item covers the move-time or service-page presentation.
- **Render parity included:** the dashboard surfaces change; REST, GraphQL and MCP stay unchanged and must be confirmed unchanged (t004).
