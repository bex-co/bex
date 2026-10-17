# Native chunked and gzip request uploads — sweep400

[Results](results.json) · [complete native creates, first builds, source proofs, public uploads, native app logs and cleanup](complete-native-uploads-logs-and-cleanup.json) · [source/contract/limits](source-scope-contract-and-limits.json) · [final inventory](final-inventory.json).

Both published Bex0.3.2 and unchanged Render2.27.0/a764810a7682 create separate explicit Free Node24 apps, actual unchanged POST201/native0. Their original first children become Live on actual Node24.22.0 with exact public carrier commit/file hashes and authored qa400 build markers. Two native build-log checks and six REST/GraphQL/MCP source/command plus two API-only explicit-env reads agree. The anonymous git-fixtures/basic source is only the carrier; the complete observer/server is authored in the recorded build command. No remote repository edits or claim to execute an upstream app.

Five uploads per fixture all return HTTP200 and exact plain/raw byte counts and independent Python/Node SHA256 values:

- HTTP1 chunked65659B, no Content-Length.
- HTTP1 chunked65659B with16 client writes over at least3.5s.
- HTTP1 fixed-length gzip262367B, explicitly decoded by the application to262267B.
- HTTP1 chunked gzip with the same exact raw and decoded values.
- Public TLS HTTP2 gzip without Content-Length; client ALPN is h2. The app observes normal HTTP1.1/chunked translation.

All ten exact replies correlate with a fresh native app-log read (five each). Both paced requests span multiple seconds in the app,3466/6221ms; this is an observation on these requests, not a buffering/timing guarantee. Public-client HTTP2 is distinct from upstream HTTP1.1; gzip decoding is explicitly implemented by the application, not a platform automatic-decompression claim. Node22.17.1 is the external public client, while the hosted native app is Node24.22.0. No malformed or ambiguous framing is sent.

[Observer](authored-upload-observer.cjs.txt), [bounded direct-write H1/H2 client](bounded-public-H1-H2-upload-client.cjs.txt), [five-case driver](finite-five-upload-cases.py.txt), [live driver](finite-live-upload-driver.py.txt). The client writes directly to Node request streams and waits for backpressure, so the paced case does not rely on a buffering curl stdin wrapper. The plain data is deterministic authored binary material, with compressed/plain hashes independently checked. Private upload scratch is removed.

[Five real pinned-Node24 local controls](five-pinned-Node24-local-controls.json), [driver](finite-pinned-Node24-local-controls.py.txt), pass over loopback HTTP1; the fifth is explicitly a local H1 stand-in. A [separate actual HTTP2 client prerequisite](loopback-H2-client-prerequisite.json), [driver](finite-loopback-H2-client-prerequisite.py.txt), passes on an own plaintext loopback server with Host Node22; it does not certify hosted TLS/ALPN. Both actual hosted fifth cases prove ALPN separately. [Initial local harness failure](excluded-initial-client-path-local.json) used an absent guessed Node path before uploading; its own container/context were cleaned, then the existing executable was resolved and controls passed.

[Actual helps/source](actual-native-helps-and-source.json), [fresh reference](fresh-public-source-reference.json), [isolated clients/create relay](isolated-native-clients-and-create-capture.py.txt), [native lifecycle/log checks](finite-native-lifecycle-source-and-log-checks.py.txt), [shared readers](shared-source-command-and-env-readbacks.py.txt). The first four-read readiness window ended queued and an early HTTP assertion failed; later unchanged first children progressed through build/update to Live, with no create/update retry. [First acceptance](finite-first-deployment-acceptance.py.txt), [bounded observer](bounded-first-child-observer.py.txt). A premature cleanup command was refused by the writer lock before checkpoint access/delete while the last log reader was running; [qualification](excluded-cleanup-preflight-lock-refusal.json). After log-reader exit0, exact cleanup ran normally.

[Cleanup](finite-exact-cleanup.py.txt), [final checker](final-inventory-check.py.txt): both native deletes0, two parents/two children/former URLs404 and listabsent; Bex needed an ordinary second convergence read. Original six services/one Blueprint remain; groups/projects/managed data empty, termination0. All own local servers/containers/contexts/payloads/relays/copied configs removed; isolated human login retained for the authorized loop. Physical teardown and serving revision unknown.

This closes the small chunked/gzip/paced coverage gaps left by sweep396. Its larger-upload throughput candidate remains inconclusive and was not replayed or reclassified. No new product bug, fix, shared configuration change or full platform-suite claim. Evidence remains uncommitted.
