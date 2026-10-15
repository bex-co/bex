# Native Dashboard menu follow-up — sweep267

2026-10-15 UTC, production Bex, unchanged source6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459/deployed revision unknown. Existing [w8/089](../../../blocked/089.md) gains native menu and cron redirect acceptance; it remains blocked for fresh login, authenticated page arrival and the remaining full acceptance scope. No new finding.

[Four complete native terminal/action results](native-menus.json), [five complete public HTTP receipts](public-HTTP.json), [final native inventory](final-service-inventory.json), [versions/boundaries](metadata.json), [redactions](redactions.json).

Published Bex v0.3.2 and unchanged Render2.27.0/a764810a7682 each read the existing Services list and the settled baseline cron deploy list in interactive mode. Four newly copied owner-only configs retain the actual prior-login dashboard_url=https://oauth.bex.co; it is not corrected to the dashboard origin. Each process selects only Dashboard: the service palette's Dashboard action, or explicitly the deploy palette's second Dashboard entry after Logs. The OS opener is replaced per process by a [local argument recorder](browser-opener.py.txt); no personal browser is opened.

All four commands exit0 after normal Ctrl+C dismissal, without timeout or panic, in1.439–1.786 seconds. Both clients capture exactly https://oauth.bex.co/cron/srv-dap3v694dm7c7390q930 and its /deploys/dep-dap3v694dm7c7390q93g child. No cancellation/trigger/restart action is selected. Baseline service/deploy metadata is read-only.

Unauthenticated, non-following GETs to those exact native arguments return302 to dashboard.bex.co with the entire path retained. Each dashboard alias returns307 to its corresponding /services path. Public issuer discovery returns200 and the expected OAuth issuer. Thus the former wrong-origin404 is no longer reproduced by these actual native menu outputs; this supplements sweep262's public web/query checks. Redirects are not followed, and authenticated page rendering, fresh device login and other native resource kinds remain unverified.

All four copied config directories are removed, PTYs reaped, and the exact six baseline service IDs remain in final native inventory. No workload, project, environment, group, datastore or Blueprint is created or modified. The continuing hunt's isolated grant stays active. No product edit, external message, commit or push. [Services driver](sweep267-service-dashboard.py.txt), [deploy driver](sweep267-deploy-dashboard.py.txt), [HTTP driver](sweep267-http.py.txt).
