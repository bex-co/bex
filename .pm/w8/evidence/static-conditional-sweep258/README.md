# Static conditional requests through rewrite and SPA fallback — sweep258

2026-10-15 UTC, Bex production, isolated human-QA bex-canary workspace. Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459` contains w4/272 fix99e678a20; deployed backend/operator revision remains unknown. This extends the fixed direct-file live acceptance to rewrite and implicit single-page-app fallback paths. All46 HTTP checks pass; no new bug.

[Complete native create/history/delete, API setup, HTTP requests/responses and cleanup](owned-native-api-http-cleanup.json), [finite driver](fixture-driver.py.txt), [versions and coverage boundaries](metadata.json), [redactions](redactions.json). Existing [w4/272](../../../w4/done/272.md) stays done; no duplicate issue or workstream4 mutation.

Published Bex v0.3.2 creates one new Free static site from public bex-co/bex/main/examples/static-site, auto-deploy off, publishing `.`. Its build command writes distinct harmless nonce bodies to version.txt and index.html. After the initial deploy is Live and version.txt matches exactly, one GraphQL request uses serial setStaticHeaders and setStaticRoutes fields to save a nonce ETag on /\* and rewrite /qa-rewrite.txt to /version.txt. Independent GraphQL readback verifies the saved values. These writes are API-only, not CLI flags or dashboard interaction.

Each of the direct file /version.txt, rewritten /qa-rewrite.txt and extensionless SPA fallback /qa-spa passes the same15 checks:

- Ordinary GET returns200 and exactly the intended file bytes; HEAD returns200 with the exact representation Content-Length.
- Strong/weak/list/wildcard matching If-None-Match returns304; the mismatching control returns200 with the complete body. Matching HEAD returns304.
- Mismatching If-Match returns412 on GET/HEAD; a matching strong tag returns200 with the complete body.
- Prefix and suffix byte ranges return206 with exact seven-byte payloads and Content-Range boundaries for their respective representations. Matching If-Range honors the range; mismatching If-Range returns the entire current representation200.

Every successful/precondition response preserves the saved ETag. The separate missing .txt asset with If-None-Match:\* remains404, establishing that SPA fallback does not swallow an asset miss. GET304/412 captures have zero response bytes. HEAD uses curl --head, so the capture records headers and does not independently inspect extra raw bytes a server might incorrectly send after a HEAD response. Browser-managed cache/network events, custom domains, every static header rule, concurrent publication and multipart ranges are not covered.

The sole deploy child is enumerated before exact-ID native parent deletion. Delete exits0; service detail/former public version.txt return404, native list omits the fixture and retains all six baseline services. Final quota is services used6/terminating0, managed PG/KV unused. Direct live Kubernetes descendant inspection and native wire interception are unperformed. No product changes, commit or push.
