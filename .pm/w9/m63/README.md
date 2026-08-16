# w9 · m63 — Log-list virtualization + honest loading states on the detail tabs

**Worker:** worker9 **Goal:** kill the dominant render-time costs the w9/m69 list-route skeletons don't cover — virtualize the unbounded log line list (the live-tail jank source) and fix the detail-tab loading states that today read as empty data ("No data in range" flash on Metrics, `null`/bare spinners on Logs, layout pop-in on Scaling) **Status:** todo

## Tasks (in order)

| id   | title                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------- | --- | ---------- |
| t001 | Virtualize the log line list + parse ANSI once at ingest             | 1h  | —          |
| t002 | Metrics tab: loading shimmer instead of the "No data in range" flash | 45m | —          |
| t003 | Logs/Scaling tabs: skeletons instead of null/spinner + reserved slot | 45m | —          |
| t004 | Render parity                                                        | 20m | t001, t002, t003 |
| t005 | Simplify                                                             | 20m | t004       |
| t006 | Test coverage                                                        | 40m | t004       |
| t007 | Closeout                                                             | 10m | t006       |

## Definition of done

- The log viewer renders only visible rows (virtualized) and a busy live tail no longer re-parses/reconciles all ~1,100 retained lines per SSE frame (ANSI parsing happens once per line at ingest, not per render); scrolling a full buffer stays smooth (spot-check via the React profiler or frame timing on a busy build tail).
- Metrics charts show a chart-shaped loading shimmer during fetch — the literal "No data in range" empty state appears **only** when the fetch resolved empty; the metrics event timeline shows skeleton rows instead of a centered spinner.
- The Logs tab never renders a blank body (`null`) or bare spinner while the service type / first page resolves — a log-panel skeleton holds the space; the Scaling tab reserves the manual-scaling card slot while autoscaling state loads (no pop-in shift).
- Autoscroll/follow behavior, line selection, and instance-slug affordances in the log viewer are unchanged.
- All dashboard tests green.

## Source + Goal linkage

- **Source:** perf sweep 2026-08-16 (rendering leg of the w5/m67–m69 follow-on, handed to w9 by user direction). Evidence: `dashboard/src/features/logs/components/log-line-list.tsx:113` maps the full retained buffer (up to `DEFAULT_MAX_LINES = 1000` live + 100 history, `use-live-logs.ts:82`) into DOM with **no** virtualization anywhere in the app, and the `parsed` ANSI `useMemo` at `log-line-list.tsx:73` re-runs over **all** lines on every SSE frame append; `metric-section.tsx:28-33` ignores `result.loading` so `SvgLineChart` falls through to `EmptyChart` ("No data in range") during every fetch on the highest-traffic detail tab; `event-timeline.tsx:48-52` bare spinner; `non-static-route.tsx:34-36` renders `null` while `useServer` resolves (Logs tab blank), `log-viewer.tsx:161-165` bare spinner; `services.$serviceId.scaling.tsx:45` withholds the manual card until autoscaling resolves (layout shift).
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; the live log tail is the platform's most-watched screen during a deploy, and misleading "no data" flashes on Metrics read as product breakage, not latency. Completes the m69 skeleton story on the detail tabs m69 explicitly scoped out.
- **Expected outcome:** smooth live tails on busy builds (bounded per-frame work), and every detail-tab loading phase reads as *loading* — never as empty data or a blank region.
- **Why now:** w9/m62's deploy-detail waterfall fix mounts the log panels earlier, making their loading states more visible — the two land coherently; virtualization is also the prerequisite for ever raising the retained-line cap.
- **Render parity task:** **included** — this changes visible UI loading/streaming states on the service detail tabs; check against render.com's log tail + metrics loading UX and keep consistent (no REST/GraphQL/MCP surface is touched, so the parity check is UI-only — the w9/m69 precedent).
