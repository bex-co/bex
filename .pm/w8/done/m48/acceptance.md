# m48 acceptance — 2026-10-02

## Automated verification

- Final full backend suite (after the severity follow-up): `source /tmp/bex-w8-m48-test.env && GOWORK=off go test -p 1 ./...` passed against dedicated local Postgres, OpenFGA, and OpenBao services. The private environment file contains credentials and is not an artifact for publication. Serial package execution keeps database-reset tests isolated.
- Final focused Blueprint apps/API regressions passed after simplify: `go test -p 1 ./internal/apps ./internal/api -run 'Test(ValidateBlueprint|BlueprintValidation|RESTBodyLimit)'`.
- `make lint` passed across all four modules, including whole-program dead-code checks. All-module lint and dead-code checks passed again after the live-discovered PostgreSQL severity-filter fix; its focused logs/datastorelogs regressions also passed.
- All five `scripts/test_log_shipper.py` cases passed: App missing-container flood, structured severity, CNPG normalization, platform collection, and Zot GC metrics. Production Helm chart **1.3.1**, Alloy **v1.11.2**, image digest `sha256:6ab34b8201f0e8b0c4346be4934c9965723af3f7f21dd9a65fd73f270f69b451`. The three tenant cases ran in 104.932 seconds; the two unrelated pipeline controls ran in 80.256 seconds.
- The two new log regressions failed against the pre-fix configuration, confirming they detect the reported behavior rather than merely accepting the new implementation.
- `bash scripts/gitops-validate.sh` passed. Its optional OpenFGA CLI drift check was skipped because the `fga` executable was unavailable; this change does not alter the authorization model.
- Markdown formatted with repository-pinned Prettier 3.4.2; `git diff --check` passed.

## Boundaries and deliberate differences

- The decoded Blueprint manifest remains limited to **512 KiB**, before YAML parsing. REST validation uses the same named 413 for an oversized manifest or its separate **4 MiB** request envelope. Tests cover exact-boundary and boundary-plus-one inputs, 600 KiB and 3 MiB, six-byte JSON escaping, multipart overhead, trailing data, malformed content, authentication, and custom/disabled global body limits.
- GraphQL and MCP adapters share the compiler diagnostic, while their ordinary HTTP body bounds still apply first. Direct-adapter tests of larger manifests are not claims that 3 MiB HTTP requests bypass those bounds. Render documents a [10 MB total multipart request limit](https://api-docs.render.com/reference/validate-blueprint); bex intentionally retains its smaller compiler guard.
- The Alloy Kubernetes source removes the kubelet timestamp before processing. A genuine tenant message byte-identical to the exact missing-container placeholder is therefore also dropped. Structured, quoted, prefixed, suffixed, and application-timestamped lookalikes survive. Existing Loki history is not rewritten.

## Infrastructure interruption

The first final-check runs failed after host disk exhaustion: compilers reported `no space left on device`, and the backend suite lost its dedicated database connection when OrbStack stopped. The engine restarted without a reset or data pruning. Dedicated test services were restored and authenticated; focused tests, backend lint, and the full backend suite subsequently passed in sequence. The CLI-started engine later shut down gracefully again. Starting the normal OrbStack desktop app remained stable. Cluster recovery preserved its CA and data: the missing API serving certificate was regenerated with existing/local service SANs, the missing local kubeadm config was restored, and stale numeric load-balancer endpoints in HAProxy, kubelets, and kube-proxy were repaired using stable local names. All three existing nodes and dev-8 pods recovered. No production or shared-monitoring resources were changed. These interrupted runs are not represented as successful tests; the exact repair is retained in the evidence.

## Live acceptance

All **50 live checks passed** in local dev-8. The [sanitized summary](evidence/summary.json), [binary evidence](evidence/binary-evidence.json), and [hash index](evidence/SHA256SUMS.json) preserve the exact results and provenance.

- Current Bex and unmodified Render used the same upstream pin `a764810a7682` (2.27.0); the installed Bex using 2.26.0 provided an additional compatibility control.
- Blueprint REST/CLI matrix: **26/26**; authenticated HTTP GraphQL/MCP 600 KiB checks: **2/2**. Exact-boundary decoded input returns 200; boundary-plus-one, 600 KiB, and 3 MiB REST inputs receive the named 413. Escaped JSON and multipart epilogue/envelope bounds are covered.
- Log CLI matrix plus raw CNPG control: **16/16**. Each CLI returned 36 normalized unfiltered Postgres records without CNPG noise and 13 error-filtered records including four real bad-password FATAL entries. Raw-current-container and retained-Loki counts span different restart/history windows, so they are not represented as a one-to-one count comparison.
- Live testing exposed a remaining rejection of Postgres `level` filters. The shared query now accepts severity aliases and OR/`*` matching through Loki and decoded CNPG pod buffers, with authorization first and unsupported service/request filters still rejected. Durable severity discovery passed **2/2**. REST, GraphQL, and MCP regression coverage uses the same core; the dashboard's existing time/text/instance view receives the normalized records.
- A fixture pod with a 2 MiB ephemeral-storage limit was actually **Evicted** after a bounded 4 MiB write (**1/1**). All three CLIs could still read its two own stdout records (**3/3**), without placeholder or platform-unavailable records.
- That eviction produced a terminated-container HTTP error, not the exact missing-log body. Separately, 100 exact placeholder lines were replayed through owned pod stdout, the real kubelet API source, unmodified production Alloy stages, isolated Loki, and authenticated API/CLI. All were excluded; twelve structured/prefixed/suffixed/short-ID controls survived. The isolated drop counter was 102, including startup replay bodies. This distinguishes a real lifecycle check from an exact-body replay; no node-wide disk pressure was induced.
- The Blueprint/eviction-history API binary hash is `01efa6687602c4da6f38676ae4f92a64a3e22ba7defe0e50ea12e96eea273b79`. The final log/discovery binary hash is `bc5e6c8a0f975cf177725d576f9064ecc8d31031706fa9e0595882ccd3167319`; its additional code is the severity-filter fix. Blueprint and Alloy stages were unchanged between them.

## Cleanup and retained infrastructure

The [cleanup ledger](evidence/cleanup-evidence.jsonl) records DELETE 204 then GET 404 for the App and Database, absence of the tenant namespace and PVs, absent replay/eviction pods, and 404 for the disposable identity/OAuth client. Isolated Alloy/Loki configuration, RBAC, services, deployments, pods, and ReplicaSets are absent; the Loki port-forward is stopped. An initial asynchronous cleanup poll still saw two terminating pods; the final independent poll saw zero.

The base dev-8 harness and four explicitly named local test/dev dependency containers remain for subsequent w8 work, with root responsible for their eventual cleanup. These are retained test infrastructure, not tenant acceptance fixtures. No credentials, private environment files, or kubeconfigs were copied into the evidence.

## Simplify

All three whole-milestone reviews completed, and all four original findings were applied. The later severity-filter follow-up received separate reuse, quality, and efficiency reviews with no actionable findings. Matching is compiled once per query; the pod fallback remains bounded and filters in place; Loki applies its selector server-side.
