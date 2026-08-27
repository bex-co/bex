# w6 · m111 — The Logs tab's range is a lie in both directions: the live tail ignores it, and the empty state denies logs it excluded

**Worker:** worker6 **Goal:** the Logs tab's range selector means what its own helper text says — the pane shows lines from the selected window and nothing else, and when that window is empty the page says so instead of asserting the service has never logged anything. **Status:** todo

## Background (found live, 2026-08-27, 22nd `/qa-find-bugs` run)

Opened `https://dashboard.bex.co/services/srv-da7o6ovvqdcc73bpn9hg/logs` with the tab's defaults — range **"Last hour"**, Live **on** — at `2026-08-27T02:23Z`. The pane rendered exactly one line:

```
05:33:06 PM  [tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-6d6cfb74ddh5dr]
             hello-go listening on 3000: "OK"
```

`05:33:06 PM` local is `00:33:06Z` — **1h50m outside** the selected one-hour window. Directly above it the page states its own contract:

> The range limits history. Live mode appends new lines as they arrive.

Two separate defects fall out of that one screen, and the second one is the more damaging.

### Defect A — the live tail never learns which window the user picked

The backend honours a window on the subscribe endpoint. Probed in-page (`fetch('https://api.bex.co/graphql', {credentials:'include'})`, per this hunt's Phase-3 trap about bare-UA clients getting Cloudflare `1010`), the history query is correct:

```
query L($resource: String!, $startTime: String, $endTime: String, $limit: Int) {
  logs(resource: $resource, startTime: $startTime, endTime: $endTime, limit: $limit) {
    timestamp type message } }

now = 2026-08-27T02:23:41Z
  startTime = now-1h  → 0 rows
  startTime = now-6h  → 9 rows, all type "app", timestamps 00:25:40Z … 00:33:12Z
```

So the range-scoped history read returns nothing for the last hour — correctly. The line on screen did not come from it. **The discriminator:** flipping the Live switch off (`?live=0`, range unchanged at "Last hour") empties the pane entirely. The line was delivered by the SSE stream.

`dashboard/src/features/logs/hooks/use-live-logs.ts:45-56` builds the subscribe URL:

```ts
function subscribeUrl(resource, type, text, instance): string {
  const params = new URLSearchParams({ resource });
  if (type !== LOG_TYPE_ALL) params.set("type", type);
  if (text) params.set("text", text);
  if (instance) params.set("instance", instance);
  return `${config.apiBaseUrl}/v1/logs/subscribe?${params.toString()}`;
}
```

No `startTime`. And `dashboard/src/features/logs/components/log-viewer.tsx:134-143` threads the window to the history hook only:

```ts
const historyWindow = useLiveRange(range);
const history = useLogHistory(resource, queryFilters, historyWindow);
const stream  = useLiveLogs({ resource, enabled: live && liveSupported,
                              type: queryFilters.type, text: debouncedText,
                              instance: queryFilters.instance });   // ← no window
```

Server-side, `lego/backend/internal/logs/rest.go:152-156` then leaves `q.Since` zero (its only fallback is `Last-Event-ID`, which a browser `EventSource` sends on **re**connect, never on the first one), so `NewPodLogStream` (`lego/backend/internal/logs/podlogs.go:56-66`) omits `SinceTime` and kubelet replays the pod's log from offset 0. The pod had produced exactly one line, which is exactly what appeared — the counts match, which is the check that this is the mechanism and not a coincidence.

**This is the cost `w6/m93` was filed to remove, still live on the first connect of every page load.** That milestone's own outcome scopes its fix to reconnects — _"`Last-Event-ID` resumes the window — so the browser EventSource's own invisible reconnect stops paying for a replay"_ — and `podlogs.go:48-54` states the general problem verbatim: _"Without it kubelet replays the pod's ENTIRE log from offset 0 on every subscribe."_ Not a regression of m93; the half m93 did not claim. On a chatty pod every tab open re-ships the whole history from kubelet.

### Defect B — the empty state denies logs the range excluded

With Live off and range "Last hour", the pane reads:

> **No logs yet** — This service hasn't produced any logs yet.

The service produced 9 log lines two hours earlier (captured above). The claim is false, and it is the sentence a user debugging a silent service will act on.

`dashboard/src/features/logs/components/log-viewer.tsx:151,180-183` chooses the copy:

```ts
const filtered = hasActiveLogFilters(queryFilters);
…
title={filtered ? t("logs.emptyFilteredTitle") : t("logs.emptyTitle")}
description={filtered ? t("logs.emptyFilteredBody") : t("logs.emptyBody")}
```

and `dashboard/src/features/logs/types.ts:125-131` is:

```ts
export function hasActiveLogFilters(f: LogFilters): boolean {
  return f.type !== LOG_TYPE_ALL || f.text !== "" ||
         STRUCTURED_FILTER_KEYS.some((key) => f[key] !== "");
}
```

**The range is not in `LogFilters` at all** — it is a separate `RangeSelection` handed to `useLiveRange`/`useLogHistory` (`log-viewer.tsx:134-135`), so the empty-state chooser structurally cannot see the one input that actually excluded the rows.

**And the range is always bounded.** `RANGE_PRESETS` (`dashboard/src/features/metrics/lib/range.ts:16-26`) is nine spans — `30m · 1h · 4h · 12h · 24h · 2d · 7d · 14d · 30d` — plus custom absolute windows; there is no "all time". The Logs tab defaults to the second-narrowest (`DEFAULT_LOG_RANGE = parseRangePreset("1h")`, `dashboard/src/features/logs/lib/log-search.ts:134`), while Metrics defaults to `12h`. So `logs.emptyBody`'s "This service hasn't produced any logs yet" (`locales/en.ts:134-136`; zh `该服务尚未产生任何日志。`, `locales/zh.ts:132-134`) is a claim this page is **never** in a position to make.

**This predicate has been widened twice already and missed the range both times** — `w6/m47/t003` (added `logs.emptyFilteredTitle` and branched the title, having found the title/body pair contradicting each other) and commit `717143ae` _"fix(logs): treat instance filter as active for empty-state copy"_. Each widened the set of things counted as "a filter"; neither could reach the range, because the range is not a filter in this type. A third widening of the same shape would miss it again — the type is the bug, not the predicate's contents.

## Target behavior (named — not "make them consistent")

- **A:** the pane's contents are exactly the selected window ∪ lines that arrived after the page opened. The stream is told the window; the correct source of truth is the user's range selector, not kubelet's offset 0.
- **B:** three distinct states, not two — *no logs in this window* · *no logs matching these filters* · *this service has never logged*. The third may only be asserted when it is actually known; if no bounded query can establish it, the string goes away rather than being shown on a guess.

## Blast radius

- `subscribeUrl` has exactly **1** caller (`use-live-logs.ts`, used only by `log-viewer.tsx`), and `useLiveLogs` has **2** consumers — the service Logs tab and `dashboard/src/features/deploys/components/deploy-log-panel.tsx`. The deploy log panel renders `logs.emptyTitle`/`logs.emptyBody` **directly** (`deploy-log-panel.tsx:178-179`) with no `filtered` branch at all, so whatever B settles must be applied there too or the same false claim survives on the deploy tab. It behaves acceptably today only because a deploy's log window is implicitly the deploy — it is correct by accident, and needs a regression test, not just the broken case.
- `hasActiveLogFilters` — grep for callers and give the count in t002; it is a shared predicate and both prior widenings touched it.

## Confirm the layer can express the fix

`/v1/logs/subscribe` already parses `startTime`/`endTime` (`logs/rest.go:442`, `core.ParseTimeWindow`) and documents that _"An explicit startTime always wins"_ (`rest.go:151`). But `rest.go:61` runs `core.CheckQueryWindow(s.MaxQueryHours, …)`, and `BEX_MAX_QUERY_HOURS` defaults to **720** (`lego/backend/cmd/api/main.go:523`) — exactly the widest preset, `30d`. `CheckQueryWindow` rejects `end.Sub(start) > 720h` (`core/pagination.go:82`), so a 30d preset sits precisely on the boundary and any custom range past it returns `400`. The fix must clamp what it sends rather than forwarding the raw range and discovering this in production.

## Adjacent classes

Not an authorization boundary. The class distinction that matters is inside the empty state: *never logged* · *nothing in this window* · *nothing matching these filters* · *the store is unavailable* (already its own state, `log-viewer.tsx:156-165`). B must place all four, not just split one in two.

## Look-alike symptoms traced separately (not folded in)

Six of the nine captured `type: "app"` lines are Kubernetes' own error text — `unable to retrieve container logs for containerd://d3abf88ca…` — served as if it were the application's own stdout. It is genuinely what kubelet returned on that read, so it is a pipeline-classification question with its own cause, not this milestone's. Recorded, not filed, cause unverified.

## Unverified (carried forward)

- Whether `deploy-log-panel.tsx`'s window is genuinely always the deploy's own span, or merely usually — asserted from reading, not probed live this run.
- What `logs.emptyBody`'s replacement should be when a service truly has never logged: whether any bounded query can establish that, or the state should simply not exist.
- Whether the same range-blind empty state affects the Postgres/Key Value log tabs (`datastore-log-range.ts` exists, suggesting a sibling window path) — not opened this run.

## Tasks (in order)

| id   | title                                                                                   | est | depends_on |
| ---- | --------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Send the selected window to the live tail, clamped to `BEX_MAX_QUERY_HOURS`                | 45m | —          |
| t002 | Give the empty state a range-aware state machine, and put the range where the chooser can see it | 45m | —          |
| t003 | Apply t002's outcome to `deploy-log-panel`, and count `hasActiveLogFilters`' callers        | 30m | t002       |
| t004 | Render parity — the log-subscribe contract across REST/GraphQL/MCP and the viewer           | 30m | t001, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed                                 | 20m | t004       |
| t006 | Test coverage                                                                               | 40m | t004       |
| t007 | Closeout                                                                                    | 15m | t006       |

## Definition of done

Each bullet is a click or a command the next person can repeat and watch pass or fail, on `dashboard.bex.co`.

1. On a service whose only log lines are older than an hour, open the Logs tab with range **Last hour** and Live **on**: the pane shows no historical lines. Today it shows them. Reproduction fixture: `srv-da7o6ovvqdcc73bpn9hg`, whose pod logged once at `2026-08-27T00:33:06Z`.
2. Widen the range to **6h** on the same service with Live on: the same nine lines the history query returns (`logs(resource:…, startTime: now-6h)` → 9 rows) appear, with no duplicates against the live-appended ones.
3. `/v1/logs/subscribe` is requested with an explicit `startTime` matching the selected range — visible in the Network panel — and a **30d** preset does not return `400` (`BEX_MAX_QUERY_HOURS` = 720 boundary).
4. With Live off and range **Last hour** on the same service, the empty state says the window is empty — not "This service hasn't produced any logs yet." Widening to 6h then shows the rows, proving the copy referred to the window.
5. A service that genuinely has never logged either shows the never-logged copy or shows the window copy — whichever t002 chose — and the choice is recorded in this README.
6. The deploy log panel's empty state was checked against the same three-way distinction and either branches correctly or is documented as not needing to.
7. Both `en` and `zh` catalogs carry every string the new state machine introduces.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 22nd run, 2026-08-27, journey 7 (Logs). Probes and their complete responses are pasted inline above — the durable artifact for an API contract is the request and the response, not a screenshot; `.playwright-mcp/` captures are gitignored and session-local and nothing here rests on them.
- **Goal linkage:** `docs/ADR010-observability.md` owns the logs surface and the live-tail/store split this milestone corrects; ADR008's hosting-primitive pillar — a log viewer that hides lines inside the window and denies lines outside it is not a debuggable product.
- **Expected outcome:** the range selector becomes trustworthy in both directions, and the first connect of every Logs tab stops re-shipping a pod's entire history from kubelet.
- **Why now:** Defect B tells a user debugging a silent service that it never logged, which is the single most misleading thing this page can say, and its predicate has already survived two widenings aimed at exactly this class. Defect A lives in the same component and the same user gesture, so fixing them separately means touching `log-viewer.tsx`'s window plumbing twice.
- **Render parity task included:** yes — the change alters the `/v1/logs/subscribe` request contract, which is a REST surface with GraphQL and MCP siblings (`lego/backend/internal/logs/{rest,graphql,mcp}.go`), and changes UI copy.
