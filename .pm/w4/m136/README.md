# w4 · m136 — Datastore and deploy log viewers stop at the newest 100 lines with no warning

**Worker:** worker4 **Goal:** the Key Value and Postgres Logs tabs keep the range control's promise the way the service Logs tab has since `w4/m107`: reaching the top of the pane loads older entries, and a capped view says it is capped **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on                  |
| ---- | ----------------------------------------------------------------------------------------- | --- | --------------------------- |
| t001 | Key Value and Postgres Logs tabs page backward through `useLogHistory` and state truncation | 50m | —                           |
| t002 | Verify the deploy-log panel against a >100-line build, and page or warn if it truncates    | 40m | —                           |
| t003 | Blast radius: every `LogsDocument` consumer either pages or states why it cannot need to   | 20m | t001, t002                  |
| t004 | Render parity across REST / GraphQL / MCP / UI                                            | 20m | t003                        |
| t005 | Simplify                                                                                  | 20m | t004                        |
| t006 | Test coverage                                                                             | 30m | t004                        |
| t007 | Closeout                                                                                  | 10m | t006                        |

## Definition of done

Each bullet is a probe the next person can repeat against production from an authenticated `https://dashboard.bex.co` page.

- **Postgres Logs reaches the whole selected range.** On `https://dashboard.bex.co/databases/dpg-d9nqg95cavls73fp8m20?tab=logs&range=24h`, scrolling the log pane (`main div.overflow-auto.font-mono`) to its top loads entries older than the first page's oldest line, and keeps doing so until the API answers `hasMore:false`. At filing (2026-09-25) the oldest reachable line was `06:05:38 PM` local (`01:05:38Z`), about 4 of the 24 selected hours, and the pane stopped there.
- **Key Value Logs reaches the whole selected range.** Create a throwaway `qa-<date>-kv`, run `BGSAVE` about 14 times over its external URL so "Last hour" holds more than 100 lines, then open `?tab=logs`. Scrolling to the top reaches the store's own `Valkey is starting` line from creation. At filing, the oldest reachable line was the first restart's (`04:55:43Z`), and creation at `04:49:29Z` could not be reached.
- **A capped view says so, and a complete view does not.** When the first page's `hasMore` is `true`, both tabs show the same truncation notice the service Logs tab shows (the `w4/m107/t003` strings: "newest 100 of more" is distinct from "everything in range"). A quiet store whose range fits in one page (for example `beancount-forum-db` with "Last hour": 24 lines, `hasMore:false`) shows no notice. At filing, neither tab had a notice or a load-older control; the only button in the log region was "Jump to latest".
- **The deploy-log panel reaches a long build's first line.** Verified truncated live in pass 163 (2026-09-25). On a finished Docker build of `examples/hello-go` (`dep-darlpfq9slkc73beqtqg`, window `05:48:47.669Z`–`05:51:12.921Z`), `logs(type:"build", startTime, endTime, limit:100)` returned `{hasMore:true, n:100, first:"05:49:56.897Z"}`, and the window `05:48:47.669Z`–`05:49:56.897Z` held 9 more lines starting `==> Build queued`. After `scrollTop = 0`, the panel's oldest visible line was `10:49:56 PM` local (= `05:49:56Z`): the build's first 9 lines, including `==> Build queued`, were unreachable, with no notice (only "Jump to latest"). Done means scrolling to the top reaches `==> Build queued`, or the panel states the view is capped.
- **The 100-row server cap is unchanged** (`maxLogLimit = 100`, Render parity, same constraint as `w4/m107`).

Probe used at filing (run inside the page with `fetch(..., {credentials:'include'})`, never from a bare script; Cloudflare returns 1010 for those):

```graphql
query ($resource: String!, $startTime: String, $endTime: String, $limit: Int) {
  logs(resource: $resource, startTime: $startTime, endTime: $endTime, limit: $limit) {
    hasMore
    nextEndTime
    logs { timestamp }
  }
}
```

- `dpg-d9nqg95cavls73fp8m20`, last 24h, `limit:100`: `{hasMore:true, n:100, first:"2026-09-26T01:05:38.419Z", last:"2026-09-26T05:00:07.946Z"}`
- `dpg-d9nqg95cavls73fp8m20`, last 1h: `{hasMore:false, n:24}` (the control case: a complete view)
- `red-darktcjbdpcs73f5eha0` (deleted), last 1h after BGSAVEs: `{hasMore:true, n:100, first:"2026-09-26T04:55:43.488Z", nextEndTime:"2026-09-26T04:55:43.488Z"}`, while the same store's 04:45–04:57 window held 41 lines starting `04:49:29.663Z`

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 159, 2026-09-25 (w4-targeted, `muse.env` credentials), journeys 12 (Key Value) and 7 (Logs). Evidence (local, gitignored): `.playwright-mcp/qa-kv-logs-1.png`, `.playwright-mcp/qa-pg-logs-1.png`. The Postgres probe was read-only on an existing database. The Key Value fixture `qa-20260925-kv` was created, exercised, and deleted in the same pass.
- **Root cause:** `w4/m107` built paging (`dashboard/src/features/logs/hooks/use-log-history.ts:55`, used by `features/logs/components/log-viewer.tsx:135`) and a truncation notice for the **service** Logs tab only. Its DoD names only `services/<id>/logs`. The three other `LogsDocument` consumers issue one `limit: 100` query and never read `hasMore`:
  - `features/keyvalue/hooks/use-key-value-logs.ts:34-42`
  - `features/databases/hooks/use-postgres-logs.ts:41`
  - `features/deploys/hooks/use-deploy-logs.ts:127` (`LOGS_LIMIT`)

  m107 touched those three files only for the GraphQL field-shape change (`git show 8667bab77 --stat`). The backend already returns the envelope and cursors for `red-`/`dpg-` resources (probe above), so this is a UI-only gap. It is not a regression of m107, which never covered these tabs.
- **Not a false positive from virtualization:** the pane is virtualized (`scrollHeight` 4358 vs `clientHeight` 518). Every "oldest visible" figure above was taken after setting `scrollTop = 0`, the same method m107/t003 used.
- **Goal linkage:** ADR010 (observability), `docs/render-artifacts/keyvalue-logs.md` and `postgres-logs.md` ("range … controls"), ADR018 logs row. The Render dashboard's datastore log view scrolls back through history. So does the Render CLI (`render-oss/cli` `pkg/tui/views/logview.go`, cited by m107).
- **Expected outcome:** an operator investigating last night's Postgres or Valkey restart can reach it from the dashboard. Today, selecting "Last 24 hours" silently shows the newest few hours.
- **Why now:** the fix is almost entirely reuse. `useLogHistory` and the m107 notice strings already exist, so the datastore viewers can adopt them now, before a third, divergent paging implementation grows.
- **Render parity task included:** the UI behavior changes on two tabs. REST/GraphQL/MCP already carry the envelope and should not move, and t004 confirms that for `red-`/`dpg-` resources specifically.

## Unverified

- ~~Deploy-log truncation was not observed live.~~ **Observed in pass 163** (see the DoD bullet). Evidence (local): `.playwright-mcp/qa-deploy-logs-1.png`.
- Key Value was only probed with the durable (Loki) source. The pod-buffer fallback in `keyvalue-logs.md` § "Durability" was not exercised.
