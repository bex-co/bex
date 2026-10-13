# Live severity-label acceptance — sweep192

On 2026-10-13 UTC, a fresh owned Free Node service reaches Live and HTTP200 with its exact marker. The App collector now labels all six previously misplaced alert/emergency inputs as error. The native reproduction of w8/097 is corrected in production; exact deployed API/operator/collector revisions remain unknown.

Published Bex v0.3.2 and the unmodified pinned Render2.27/a764810a7682 client against Bex agree on all 16 IDs/message bytes. Each client's ten level filters passes with exact IDs/bytes, no duplicates or unexpected records:

| Filter | Rows in each client |
| --- | --- |
| debug | 1 |
| info / notice | 2 |
| warning / duplicate warning | 3 |
| error / critical / alert / emergency | 9 |
| debug,warning,emergency union | 13 |

The original 13 inputs are joined by literal fatal, uppercase severity WARNING and literal warn controls. Nine error rows include the original eight error inputs plus fatal; three warning rows include warning, WARNING and warn. The plain control remains unknown. API-only diagnostics return that single unknown row and the same three warning IDs for the legacy `level=warn` query spelling. These are direct GET diagnostics, not intercepted native traffic. Three existing query-construction subtests pass, preserving both warning/warn stored labels and alias unions; actual retained old warn-labelled history is not live-tested.

The SourceBuild spends bounded observations queued/building before reaching Live. No mutation is retried, and intermediate HTTP404 is not filed as a bug. Native delete exits0 in 1.828s after enumerating its one own deployment. Detail/list/public404, all six baseline services present, quota6/terminating0, and the comparison config removed. No local container or Kubernetes descendant inspection is claimed.

Live platform streams and GraphQL/MCP/dashboard level queries remain unexercised. The existing source-fix outcome documents its rendered App/platform pipeline tests; this sweep does not rerun or claim those tests. Current App producer/native acceptance has passed, so w8/097 returns to the active queue for the remaining acceptance review rather than staying blocked on these already-completed native receipts. No implementation, commit or push is performed by this sweep. Known credential/email values and private paths are redacted; non-secret records remain exact.

Evidence:

- [cleanup-driver.py.txt](cleanup-driver.py.txt)
- [existing-legacy-query-events.jsonl](existing-legacy-query-events.jsonl)
- [existing-legacy-query-test.json](existing-legacy-query-test.json)
- [filter-driver.py.txt](filter-driver.py.txt)
- [hosted.json](hosted.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-driver.py.txt](native-driver.py.txt)
- [unknown-API-diagnostic.json](unknown-API-diagnostic.json)
- [warn-API-diagnostic.json](warn-API-diagnostic.json)
