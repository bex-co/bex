# Delayed stdout prefix loss supplements m174's existing residual

Functional QA a321, 2026-10-12 local / 2026-10-13 UTC, user-directed w4, workspace tea-d98210cbbpdc73dcrkvg. **Existing owner: m174/t007; no new duplicate milestone.** The existing major log-loss finding remains open. This schedules a distinct delayed-stdout verification task; no product, shipper or node change was made.

## Observed case and independent control

Owned Free image cron **qa-20261012-a321-cron**, service **srv-db6qfqgvrh0s73bstcu0**, docker.io/library/busybox:1.37, schedule **0 0 1 1 \***, unchanged command:

```sh
sh -c 'printf "qa-a321-begin\n"; sleep 90; printf "qa-a321-end\n"'
```

The create request and response below verify the exact command, type cron_job and plan free. Its first deploy was Live before the manual runs. No automatic January run occurred during this October trial.

| Run | API timing/status | Stored app messages |
| --- | --- | --- |
| crr-u6kh4nc0lol1kg76v8jt | started 03:43:02Z; successful, finished 03:44:37Z | Only **qa-a321-end**, timestamp **03:44:34.849475863Z**, instance **srv-db6qfqgvrh0s73bstcu0-jnos5sam4d8mc9ekavp6** |
| crr-7p6b7olln6gmonoa3s4r, triggered after a fresh page load | started 03:59:19Z; successful, finished 04:00:55Z | **qa-a321-begin** at **03:59:22.939172102Z** and **qa-a321-end** at **04:00:53.025441233Z**, instance **srv-db6qfqgvrh0s73bstcu0-e93qeeo08f0fmtifiqde** |

**Expected:** both newline-terminated logical messages from each successful execution are available in the stored history. **Actual:** three messages across the two successful executions; the first run lacks its prefix, and the independently fresh second run is complete. This is **one observed loss in two runs**, not a claim that the fresh control failed or an estimated production loss rate.

The first GraphQL read at 03:44:36.614Z already returned only the end. A read at **03:53:36.677Z**, long after that run settled, still returned only the end. At **04:02:54.691Z**, the query spanning **03:39–04:05Z** returned all three messages and **hasMore false**. The start predates service creation, and both runs lie within the explicit bounds; this is neither the new relative-clock issue m203 nor a page cap.

At **04:06:36.151Z**, native browser REST GET /v1/logs with the same resource/type and a containing window returned the same three messages, hasMore false. It also exposes both instances' container **app** and level **unknown**. The two markers are 13 and 11 message bytes; no long-line cap or structured-level filter was involved. The same page's fixed Custom Application history, **Live off**, shows those three rows. A fresh 390px replay includes the later cancellation-control rows while the original prefix remains absent. Saved command was not edited between these runs.

## Cancellation control and limits

A third manual run, **crr-7348vmkuthllmsagf16l**, was accepted at **04:09:16.954Z** and canceled by the ordinary confirmation. CancelCronJobRun returned **canceled**, finished **04:10:26.675687756Z**; a fresh read at **04:13:13.190Z**, after its intended 90-second end, still reports canceled. Last successful run remains **04:00:55Z**, and next scheduled run remains **2027-01-01T00:00:00Z**.

The process emitted qa-a321-end at **04:10:50**, after cancellation acceptance. Immediate physical termination/grace timing was not established; this is not filed as a separate cancellation defect, and the canceled run is not counted in the two-successful-run completeness assertion.

Two programmatic stream attempts returned 200 with no parsed events. Source review at `lego/backend/internal/logs/rest.go:430–469` then identified a capture issue: Fetch did not request `Accept: text/event-stream`, so the endpoint used its default JSON-lines format while the observer parsed SSE packets. This capture cannot establish SSE health, raw producer bytes or the live UI's behavior. **No separate SSE defect or raw-pod completeness claim is made.** The follow-up task must collect actual pod/CRI and correctly negotiated live wire records.

Screenshot **.playwright-mcp/qa-cron-prefix-a321-mobile.png** (67489 bytes) was viewed. It shows the original end-only instance, the fresh complete instance and the cancellation-control instance. The durable evidence is the complete requests/responses in [a321-evidence.json](a321-evidence.json); the ignored local ledger is supplementary.

## Placement and researched boundary

The existing m174 goal is every App line reaching historical REST/GraphQL/MCP/UI. Its open t007 already owns residual one-line loss after the CRI switch. Original loop63/64 losses were stderr/JSON lines in fast runs; this plain-stdout prefix loss during a **90-second** run adds a delayed control, rather than creating another owner.

Current declared producer path is **deploy/gitops/base/log-shipper.yaml:235–298**: App-pod relabeling creates namespace/app/pod/container labels and a UID/container CRI-file path; local.file_match app_pods feeds loki.source.file app_pods, then loki.process app_logs with stage.cri. Source comments at **58–72** explain the former API-tail timestamp drop and the intended file-offset replacement. The observed history transports agree after query/adapter handling, so a UI list/reader fix cannot restore this prefix.

**Cause remains unverified:** raw CRI/pod contents, node placement, live Alloy version/config/positions, delivery timing and Loki intake were not inspected. Do not assert that file_match, initial seeking or any one stage dropped the line. t007 remains responsible for pinning and fixing the cause; t008 verifies the delayed corpus and its boundaries after that work. Read the actual deployed/pinned dependency paths before choosing a fix; no blind config change is proposed here.

The app_pods pipeline is shared by runtime web/worker/private/cron App containers; datastores, build pods and request logs have separate producer paths. This run live-probed one image cron only. Verification must preserve the other shared consumers and both earlier fast-run corpora. No API field, transport alias, auth/error policy or resource family is changed by this filing.

## Original definition-of-done placement

- Original five-message/six-fast-run 30/30 matrix: **not replayed**; remains required.
- Original order/write timestamps: this trial retains exact returned timestamps; **no raw-write comparison**, so not satisfied.
- Original 3-second sleep/stderr control, long-running web live tail, level parsing and request pipeline: **not replayed**. This is an added 90-second stdout control, not a replacement.
- Original restart/once-per-lifetime/no re-ship checks: **not replayed**.
- Cross-surface set: REST/GraphQL/UI agree on this loss/control; **MCP unprobed**.
- Existing m96/m200 full scheduled/manual/byte-integrity guarantees: **not walked**, no full regression claim.

Targeted all-board/open/done searches place intermittent stored-line loss in m174, and m200 concerns larger mixed stdout/stderr framing at a different boundary. Current shipper source and targeted history retain m174's unresolved task. DO_NOT_DO has no conflict. Keep status **blocked** and preserve the existing fast-run controls. [Render logging documentation](https://render.com/docs/logging) describes runtime log history; exact authenticated Render startup timing was not tested.

## Cleanup

DeleteService returned 200/true at **04:19:25.896Z**. At **04:19:35.844Z**, service REST lookup returned **404** and all seven inventory sets exactly matched baseline. The cron has no public URL; public-file 404 is inapplicable. Only the owned fixture and its run were changed. Console capture had zero errors.

Own-session logout returned **ok logged-out**, exit 0. At **04:22:40.196Z**, owners returned 401, browser had one about:blank page, zero cookies and 1440×1000 viewport. Permissions cleared, the read-only stream controller removed, own jar/handle removed and shared jar removed only after its recorded stat matched.
