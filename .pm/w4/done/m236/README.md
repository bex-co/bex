# w4 · m236 — The Metrics page shows one working Application Metrics card, not two

**Worker:** worker4 **Goal:** a service's Metrics tab renders exactly one Application Metrics card and it carries data; tenant deploy/event surfaces stop printing the cluster-internal build registry host. **Status:** done 2026-10-29 — live DoD replayed on pin `d66f96c06991`: one Application Metrics card on a fresh git-built service and on hello-go at 1440 and 390, and no internal registry host on deploy/event surfaces (loopx w4).

## Tasks (in order)

| id             | title                                                                    | est | depends_on                    |
| -------------- | ------------------------------------------------------------------------ | --- | ----------------------------- |
| [t001](done/t001.md) | Root-cause and remove the duplicate dead Application Metrics card — **DONE**      | 45m | —                             |
| [t002](done/t002.md) | Keep cluster-internal build image refs off tenant deploy/event surfaces — **DONE** | 45m | —                             |
| [t003](done/t003.md) | Render parity — **DONE**                                                            | 20m | w4/m236/t001, w4/m236/t002    |
| [t004](done/t004.md) | Simplify — **DONE**                                                                 | 15m | w4/m236/t003                  |
| [t005](done/t005.md) | Test coverage — **DONE**                                                            | 30m | w4/m236/t003                  |
| [t007](done/t007.md) | The dead card survives the pending-frame fix — remove the duplicate sibling key — **DONE** | 45m | w4/m236/t005 |
| [t006](done/t006.md) | Closeout                                                                 | 15m | w4/m236/t004, w4/m236/t005, w4/m236/t007 | — **DONE**

Seven tasks, 215m (3h35m). One **major** finding and one **minor** finding. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) on production (`https://dashboard.bex.co`) with the recorded probes, on a fresh owned Free Docker web service and on the pre-existing `hello-go` (`srv-daif6dsmg29s73d1umvg`) as the read-only control.

- **One card.** After a fresh document load of `/services/<id>/metrics` settles (≥ 9 s), the DOM probe in finding.md counts exactly **1** `Application Metrics` heading and **2** `[data-slot=card]` elements (Application + Network) — not today's 2 headings and 3 cards. The one card shows resolved values (an Instances selector and numeric latest values), never a permanent `…`.
- **No dead twin on the control either.** The same probe on `hello-go` also counts 1 heading, at desktop and 390px widths, after the range is changed once.
- **No internal registry host.** `GET /v1/services/<id>/deploys/<deployId>` and `GET /v1/services/<id>/events` for a git-built service contain no `zot.bex-registry.svc` (or other `*.svc` / `*.cluster.local`) substring anywhere in the body. The Events tab shows no such string. A user-supplied image (image-backed service) still reads back byte-identical on every surface.
- **Checks pass.** Dashboard `yarn lint` and `yarn test`; backend `go test ./...` (real-DB gate) for the packages t002 touches.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, loop run 1, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md). Owned fixtures `qa-20261019-loop1-web` / `-kv` / `-broken` were created, exercised and deleted in the same run; research is on main `f4e383710`.
- **Goal linkage:** [ADR010](../../../docs/ADR010-observability.md) (Metrics is the tenant's resource-usage truth) and [ADR008](../../../docs/ADR008-vision.md) truthful product state. A permanently-loading duplicate card is the page's first content on every service, so the tab reads as broken before it reads as useful.
- **Expected outcome:**
  - The Metrics tab's first card is the working one; nothing on the page loads forever.
  - Tenants no longer see cluster-internal registry DNS (`zot.bex-registry.svc:5000`) in their deploy/event data.
- **Why now:**
  - It reproduces on every service's Metrics tab today (verified on two services, fresh loads).
  - The dead card is a second mount of the same query fan-out — it doubles the card's metrics requests until its queries stall.
- **Render parity:** included (t003) — the milestone touches the dashboard Metrics surface and the REST/GraphQL deploy + event shapes.
