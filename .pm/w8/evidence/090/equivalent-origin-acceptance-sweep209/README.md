# Equivalent API base dashboard actions — current-build acceptance — sweep209

A private checkout binary at cb16919ab75d1a72663109b06707fb3663fc33cb (qa209-cb16919ab75d, pinned Render 2.27/a764810a7682) now passes all nine native read-only service-list Dashboard cases. Normal API base, explicit :443, hostname case, saved :443, slashless BEX_HOST, slashless explicit RENDER_HOST overriding BEX_HOST, missing config, missing-config service alias and explicit-origin control all pass https://dashboard.bex.co/cron/<baseline-id> to the captured opener. The selected baseline cron is inspected only for metadata and never mutated.

Published Bex 0.3.2 and unmodified pinned Render 2.27 controls still open /cron/<baseline-id> with the :443 base and a fresh version-only config. This is existing w8/090's release gap, not a new implementation finding. Both source guards now use isDefaultHost; actual HEAD before/after the native checks is unchanged. The 15 existing helper cases plus parent pass, including preserved origins, custom hosts, malformed config and non-consumer controls. No full CLI suite is claimed.

All 11 PTYs exit0 with no timeout/panic and are reaped. No environment grant persists in any config. Each case's owned config directory and the opener stub are removed. The isolated grant remains for the ongoing authorized loop. Real browser arrival, deploy-menu navigation and all 12 shared link shapes remain unexercised by this receipt. No hosted resource is created or changed; no product fix, commit, push or release is performed by this QA process.

The native driver captures the OS open child locally; no browser is launched. Production metadata uses the documented environment-grant override with explicit own workspace tea-daif693dqjvc73e7as3g and private config. Full terminal output and sanitized base64 bytes, commands, timings, opener argv and persistence checks are retained. The standalone driver expects its private opener stub and current binary; recreate those harmless prerequisites when replaying.

Evidence:

- [checkout-build.json](checkout-build.json)
- [helper-results.json](helper-results.json)
- [native-driver.py.txt](native-driver.py.txt)
- [native-results.json](native-results.json)
- [source.go.txt](source.go.txt)
