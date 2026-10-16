# Hosted automatic Brotli acceptance — sweep340

[Results](results.json) · [native requests, actual HTTP representations and exact cleanup](native-wire-HTTP-cleanup.json) · [maintained source regressions](source-regressions.jsonl) · [final baseline](final-inventory.json).

Source 7ee5a95fd1d88dd8766072f497e8e191d51a06cb and the observed desired platform pin5a39b80ff05a via45d809f99 contain m79 implementation5a39b80ff. Serving SHA is unobserved. QA supplies no implementation, release or platform deployment. All resource operations use the existing isolated human session in bex-canary.

## Native creation and real compressed content

Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 each create a fresh Free static site with explicit master. The actual native POST bytes are forwarded unchanged through a finite relay; both201/native0 responses prove same-workspace ownership and Free plan. Both sites reach Live/HTTP200 with their exact markers. The retained sweep326 asset generator uses reviewed Node24 and adds harmless authored gzip/no-transform control assets; a local execution checks it before live submission.

Each client produces text, JavaScript and CSS assets. Identity, br, gzip-only and weighted br/gzip offers yield24 successful decoded-content/hash comparisons across the two sites. All12 br-accepting responses are actually br-coded, smaller than their originals and independently decoded to the exact authored bytes. Identity and gzip-only offers return exact identity representations; automatic gzip is not claimed.

## Negative and neighboring behavior

Fourteen additional bounded HTTP checks per site pass: independently checked generated control manifest; explicit br;q=0, wildcard and malformed-q negotiation; authored Vary:Origin merged with Accept-Encoding; encoded GET length/bytes; HEAD representation length; byte ranges against the selected encoded and identity representations;416 without content coding; bodyless304 for the encoded validator; unchanged tiny and binary assets; authored gzip bytes/coding; and no-transform CSS. These are28 HTTP checks, not native CLI commands.

The harmless response-header rules per site are configured/read back through the supported own-service REST headers endpoint because the pinned CLI has no header flag. That API-only setup is explicit: three initial rules preserve gzip/no-transform/Vary, then a fourth adds a harmless authored ETag for validator coverage. The two initial complete br GETs passed content/framing but failed the helper's assumption of an automatic ETag; both are retained as setup exclusions. Four earlier valid checks per site remain retained, and ten further reads per site resume after authored-validator setup. The br representation weakens that authored strong ETag as expected. The gzip control's build-generated wire hash/length and decoded identity hash/length agree with HTTP; its compressed file is below the automatic-compression size threshold, so a separately hosted large precompressed case is not claimed. No-transform uses a large otherwise eligible CSS body. No package installation or client request repair is added.

A local Python helper keyword syntax error is caught by compilation before the neighbor driver makes any HTTP request; the corrected driver runs once per site. A later304 response produced no curl output file, so the initial conditional capture stopped before saving its response; that intent is excluded. The capture initializes an empty private file and resumes the final five requests per site, retaining the nine earlier passing checks. All actual runtime receipts are retained. No source-only assertion is used as proof of compressed hosted content.

## Maintained checks and limits

Seventeen maintained tests across internal/staticserver and cmd/staticserver pass, with zero failures/skips. They execute every direct/rewrite/SPA serving caller, actual Brotli decode, MIME/size eligibility, authored headers, no-transform, cached/incompressible variants, shared budgets/concurrency/fairness, revision isolation, conditions/HEAD/ranges/framing and bounded encoder memory, plus startup-server integration. The worker's completed implementation tasks retain broader make test/lint/race/simplify records; QA reruns these focused checks without claiming a new full-suite run. Other public configuration paths remain the common producer/consumer audit in t002; no browser interaction, independent Render hosting, production concurrent-load bound or new public field is claimed.

## Cleanup

Both exact owned service parents and all2 recorded deployment children are absent: native delete0, parent/detail404, native list absence, former URLs404 and every child404. The original six service IDs remain; service quota is used6/terminating0, PostgreSQL/Key Value usage0, groups/projects empty and the original Blueprint is the sole connection. Baseline Blueprint configuration is omitted. Relay/config/request/HTTP capture files are removed and finite processes terminate; isolated login remains for the continuing authorized hunt. No paid resource, cron, database, project, group or Blueprint is created.

Helpers: [native fixture and base HTTP](finite-native-static-driver.py.txt), [API-only header controls](owned-header-configuration.py.txt), [bounded HTTP neighbors](bounded-HTTP-controls.py.txt). The original326 failure evidence is preserved in this milestone.
