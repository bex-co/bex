# HTTP/2 paced upload follow-up — sweep442

[Results](results.json) · [complete native creates, transfers, app/edge logs and cleanup](complete-native-HTTP2-uploads-logs-and-cleanup.json) · [final inventory](final-inventory.json).

This extends [the original HTTP/1 finding](../cli-sweep441/README.md) and [m108](../../README.md). No duplicate issue, product implementation, live ingress change, commit or push is made. Source HEAD is e27576d8a05acdc0447dfe094da042de9fedd406; serving edge revision/configuration remains unobserved.

Published Bex0.3.2/a6350da0 and unmodified Render2.27.0/a764810a7682 each create a fresh explicit Free Node24 service through the unchanged actual native POST201/exit0. Both first children become Live. The authored upload application is byte-identical to sweep441, including its180s request timeout; fresh harmless markers distinguish the fixtures. Immutable git-fixtures/basic source, actual Node24.22.0, program/build hashes and original boot identities are checked. Six REST/GraphQL/MCP source/command/plan readbacks and two NODE_VERSION24-only environment reads pass.

The new independent Node HTTP/2 client verifies negotiated ALPN h2. It sends the same fixed-length1,048,576-byte body as sweep441, in4096-byte writes, with either0ms or300ms pacing. Full SHA256 is bd8a5183fdafb052402b7f52480145004a781b927f0d376e4fbd922da44f6926. Each case is one bounded attempt; no reconnect or retry. The120s client deadline is never reached.

| Native creator | Transfer | Client HTTP | Seconds   | Submitted bytes |
| -------------- | -------- | ----------- | --------- | --------------- |
| Bex            | fast     | 200         | 2.709094  | 1048576         |
| Bex            | paced    | 502         | 60.605780 | 819200          |
| Render         | fast     | 200         | 8.075980  | 1048576         |
| Render         | paced    | 502         | 60.569569 | 819200          |

Both fast responses prove full consumed bytes/hash. Both paced apps log an abort after60003/60001ms,819200 consumed bytes and partial SHA1e759378ea22d5e194eabeadb368600e118ae4a67610e39100aa6244dfc8af98. Independent Python prefix regeneration verifies both hashes. Their original processes remain healthy on the same boots. Native request logs identify HTTP/2.0, status502,819200 request bytes and60002211019/60000768100ns durations. Fast app-complete and edge200 records also match their exact unique paths. The HTTP/2 result is502; the original HTTP/1 result was499. Client, app and edge byte counts are retained separately.

The [direct local TLS/HTTP2 prerequisite](actual-local-TLS-HTTP2-results.json), using pinned Node24 with the same application callback and a local HTTP2 TLS wrapper, accepts the complete paced body/hash after77.619932s. This is a client/control prerequisite, not a hosted HTTP2 backend or full generated-image claim. The local certificate is accepted only for127.0.0.1; production certificate verification remains enabled.

The [pinned local TLS/HTTP2 proxy comparison](actual-pinned-TLS-HTTP2-proxy-results.json) uses the same immutable Traefik3.7.5 image as441. Default behavior returns502 at60.023751s after815104 submitted bytes. Changing only its entrypoint body-read timeout to diagnostic100s allows the complete77s upload with200 and the full hash. Actual extended duration is retained in the JSON. This independently reproduces the protocol-specific502 without claiming to inspect the deployed edge. The100s diagnostic is not the planned product duration budget.

Both exact owned parents, both children and former URLs are absent after native delete0. Baseline identities and current quota/termination reconcile. Own local Node/proxy containers, network, source contexts, payloads, local certificate/key and copied native configs are removed. Image caches remain reusable. Request-log client address/port/username fields are omitted from filed evidence; timing, protocol, status, count and unique paths remain. Raw captures stay private.

[Current Render web-service contract](https://render.com/docs/web-services) lists HTTP/2, and its [duration comparison](https://render.com/docs/render-vs-vercel-comparison) allows100-minute web responses. The existing m108 repair/consumer-audit/test/rollout gates remain open. HTTP2 coverage here is limited to Free Node24 fixed-length PUTs; chunked HTTP1, multipart/Expect, other plans and the full duration boundary remain unrun.
