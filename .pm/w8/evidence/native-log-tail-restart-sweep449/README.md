# Native live log tails across a restart

Sweep 449 found no new issue. The published Bex 0.3.2 launcher and unchanged Render 2.27.0 pin each followed the same owned Free Node 24 service through one native Bex restart. Both clients observed the warmup before measurement, then received all eight measured origin events exactly once: four from the old boot and four from the replacement boot. Independent HTTP responses supplied the payload oracle, including Unicode, an embedded newline and the literal text `Render CLI`. Both later historical queries recovered the same eight payloads.

Both followers remained alive until the harness sent its own SIGINT; their raw exit status was -2. The production WebSocket handshake and frames were not intercepted. Actual incremental native stdout, arrival records, HTTP origin payloads and the pinned stream consumer are retained. Local prerequisites used the real app and real native binaries against owned loopback WebSocket servers with fake credentials, including the exact capture helper used live.

The native create returned HTTP 201 and exit 0, the first deploy became Live, and the single native restart returned exit 0 with one replacement deploy becoming Live. Six pre/post REST, GraphQL and MCP reads preserved the source, commands and runtime; two API environment reads preserved explicit Node 24. Source and program proofs matched across both boots. The actual serving platform SHA and OCI digest remain unknown.

Native deletion returned exit 0. The parent, both recorded deploy children and former URL are absent. The six baseline services and unrelated resources were preserved, and current quota and termination checks passed. All owned follower processes, temporary client configurations, local servers, containers and contexts were removed. The isolated human login remains for the continuing QA loop.

No product implementation or new issue was filed. These evidence files remain uncommitted.

- [Measured results](results.json)
- [Complete native lifecycle and correlation](complete-native-live-historical-logs-and-cleanup.json)
- [Final inventory](final-inventory.json)
- [Pinned native stream consumer](pinned-native-stream-consumer.json)
- [Incremental capture prerequisite](actual-local-incremental-capture-results.json)
