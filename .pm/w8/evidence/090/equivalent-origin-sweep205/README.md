# Equivalent production API origin forms — sweep205

Current private CLI build c5a0eb9dd15f reproduces w8/090 with explicit :443 and hostname case, as well as the saved-host veto. These are valid native API inputs: the actual services list and selected metadata row load, then Dashboard passes a relative /cron/srv-dap3v694dm7c7390q930 to the OS opener.

| Current-build input | Actual opener | Seconds |
| --- | --- | --- |
| BEX_HOST https://api.bex.co:443/v1/ | relative cron path | 1.892 |
| BEX_HOST https://API.BEX.CO/v1/ | relative cron path | 1.502 |
| stored api.host https://api.bex.co:443/v1/, effective API normal default via environment grant | relative cron path | 1.079 |
| explicit :443 plus saved dashboard origin control | absolute Bex dashboard URL | 1.441 |
| normal production default control | absolute Bex dashboard URL | 1.441 |

The stored-host case isolates the second guard; it does not claim the stored host won API precedence. Unmodified Render2.27/a764810a7682 with explicit :443 also opens the relative path (1.550s), the known empty-origin neighbor. All six native PTYs exit0 after normal Ctrl+C, no timeout/panic. The local stub captures the actual OS child and launches no browser. Only immutable baseline metadata is read; no hosting action is selected. HTTP request bytes, real browser arrival and deploy-menu behavior are not captured.

Three current-helper virtual diagnostics reproduce the absent origin with port/case/stored-port forms; all eight existing helper subtests still pass. They expect the current guard bug, not a corrected regression. No implementation is changed. Both raw comparisons in bridge/dashboard.go:22/26 need a narrow comparison of known equivalent production API bases, beyond only appending the missing slash. Keep scheme/production hostname/default port and /v1 path checks explicit; preserve custom host/port/path, saved origins, malformed input and non-consuming commands. Do not rewrite caller config hosts or persist the environment grant.

All six temporary configs and PTYs and the opener stub are removed; inspection shows no environment grant persisted. Actual QA values are absent from decoded terminal bytes. Current source c5a0eb9dd15f6b417bf586fdf26510706497b032; live server revision unknown. Evidence paths are sanitized. No new issue is filed: these cases extend w8/090, now 35m remaining implementation, followed by its existing release/acceptance obligations.

Evidence:

- [browser-stub.py.txt](browser-stub.py.txt)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-driver.py.txt](native-driver.py.txt)
- [native.json](native.json)
- [origin-events.jsonl](origin-events.jsonl)
- [origin-overlay.json](origin-overlay.json)
- [origin-probe_test.go.txt](origin-probe_test.go.txt)
- [origin-results.json](origin-results.json)
- [source.txt](source.txt)
