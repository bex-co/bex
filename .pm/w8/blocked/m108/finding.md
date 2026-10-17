# Ordinary file uploads are cut off by a 60-second incoming-body deadline

Why: a one-MiB upload that progresses continuously and completes in77s against the same app is truncated before completion on the hosted service.

**Severity:** major. **Attribution:** ingress request-body timeout policy. Functional CLI sweep441,2026-10-17, isolated human QA in bex-canary/tea-daif693dqjvc73e7as3g, api.bex.co/v1. Published Bex0.3.2/a6350da0 and unmodified Render2.27.0/a764810a7682 each create an equivalent Free Node24 service through unchanged captured POST201/native0. Source43203045ecf4458ae1b2f64f1af21adc020be926; actual deployed edge/serving revision is unobserved. [Complete evidence](evidence/cli-sweep441/README.md).

## Reproduce and isolate

Both first deployments become Live on git-fixtures/basic/master6ecf0ef2c2dffb796033e5a02219af86ec6584e5, with exact LICENSE/CHANGELOG and authored program/build identity. The explicit environment is only NODE_VERSION24. The authored standard Node HTTP app reads PUT bodies, hashes actual received bytes, and returns count/hash on end. Its request timeout is explicitly180s and is verified in hosted health responses. The carrier repository is not claimed to implement this upload app.

Generate1,048,576 bytes with `byte[i]=(i*17+31)&255`; full SHA256 is `bd8a5183fdafb052402b7f52480145004a781b927f0d376e4fbd922da44f6926`. Send the same fixed-length body to a fresh own `/upload/<case>` path using the retained Node HTTPS client. It writes4096-byte chunks, records each write and any early HTTP response, and has an independent120s overall client bound. Run one fast case, then the same body with300ms between chunks. No client deadline is reached.

| Created through | Fast upload | Paced upload | Actually received before abort |
| --- | --- | --- | --- |
| Published Bex | 200, full count/hash,2.571s | 499 at60.608s | 819,200 bytes; app60.002s |
| Pinned Render | 200, full count/hash,2.279s | 499 at60.574s | 819,200 bytes; app60.003s |

Native application logs prove each declared length is1,048,576, each fast body completes exactly, and each paced request aborts after consuming only819,200 bytes. The independently regenerated prefix hash matches both aborted-body hashes. Four native request-log rows independently correlate paths/status/bytes: the failed edge durations are60.001109404s and60.000934958s, with499. Both apps remain healthy on their original boots and exact source/program after the failures. Six REST/GraphQL/MCP source/plan/commands and two explicit-env readbacks agree. Wire capture proves CLI create201; runtime HTTP requests/results are separately recorded.

The identical direct local Node24 app accepts the full paced body at77.325s. A second local experiment uses the repository-described Traefik3.7.5 image, resolved immutably to `traefik@sha256:e4d98158c01ad752fc1071d4e9573788747230d902cdde00a772516e692d07c9`, the same app/container/boot, payload and pacing. Its default case returns499 at60.013s after815,104 submitted bytes. Changing only `--entryPoints.fixture.transport.respondingTimeouts.readTimeout=100s` makes the same full upload return200 at77.222s with the exact hash. The diagnostic100s value is not the proposed product budget. Local containers/network/contexts/payloads are removed.

[Render's web-request contract](https://render.com/docs/render-vs-vercel-comparison) allows responses up to100 minutes; its [migration guide](https://render.com/articles/migrating-from-supabase) explicitly relates that request budget to large file uploads. This bounded77s/one-MiB case exercises a normal continuously progressing upload. No100-minute hosted run or Render-hosted comparison is claimed.

## Producer, consumer and repair

`deploy/gitops/base/traefik.yaml:38–43` pins the vendored chart and selects repository values. Base `deploy/gitops/base/values/traefik.values.yaml:47–52` defines public web/websecure entrypoints; production `deploy/gitops/overlays/prod/values/traefik.values.yaml:42–67` overrides ports/NodePorts and inherits their transport policy. Neither supplies an incoming-body read budget.

The repository describes chart41.0.0/Traefik3.7.5. [Pinned Traefik source](https://github.com/traefik/traefik/blob/v3.7.5/pkg/config/static/static_config.go#L50) defines DefaultReadTimeout60s and applies it in RespondingTimeouts.SetDefaults at207–208. ReadTimeout covers the complete incoming request including its body. [Chart41 values](https://github.com/traefik/traefik-helm-chart/blob/v41.0.0/traefik/values.yaml) expose `ports.<name>.transport.respondingTimeouts.readTimeout`; its [pod template](https://github.com/traefik/traefik-helm-chart/blob/v41.0.0/traefik/templates/_podtemplate.tpl) turns transport values into entrypoint CLI arguments. The actual local default/override experiment establishes this mechanism. Live app/request logs and timings are consistent with that default; deployed arguments/revision were not read, so exact live-config identity remains unverified.

Configure a reviewed finite body-read budget that permits the documented web-service request duration, targeting100m for web/websecure through the existing Helm transport values. Confirm base/prod rendered arguments and inheritance. Keep the100s control diagnostic. Preserve explicit application/API limits, normal header handling, HTTP routing/TLS/PROXY settings, existing redirects/maintenance/static responses, and TCP entrypoints. Do not change tenant source/start commands, introduce a CLI workaround or new API selector, or treat the failure as a maximum one-MiB payload size. Source/native creation and exact shared readbacks work; the observed cutoff tracks body duration.

Audit consumers of the shared public entrypoints in a separate task: native/Docker/image web apps, static routes and platform hosts, both public ports and HTTP versions. Preserve each backend's own request limits and document the supported ingress budget in ADR004/ADR060 as appropriate. Actual chart rendering and pinned-proxy regressions must verify the intended argument, real fail-before/pass-after body completion, below/above-budget behavior and cancellation. Include normal fixed-length, chunked/Expect forms and a fast control without introducing load testing.

After rollout, both unchanged native clients' fresh original services must accept the same paced77s upload with full count/hash and native complete200 events/request logs, preserve source/metadata, and pass exact cleanup/baseline/current-quota checks. Record the executed duration/protocol limits; a values-string assertion, authored app workaround or local-only control does not meet live acceptance.

## Dedupe and limits

[Prior sweep396](../../evidence/native-file-transfers-sweep396/README.md) retained a5MiB upload cutoff as inconclusive because size, variable bandwidth and client bounds were not isolated. This sweep adds controlled one-MiB pacing, successful same-body fast controls, independently verified actual consumed prefixes, both-client reproduction, a direct77s control, primary duration contract and a one-setting proxy correction. It schedules that previously unresolved concern; no existing open/blocked/done fix or source repair owns this request-body deadline. Static-server/activator application timeouts are different consumers and are preserved. No roadmap non-goal applies.

Only Free Node24/HTTP1 fixed-length PUT at the named pacing is hosted. HTTP2/chunked/Expect, other runtimes/plans, large payloads, full production topology/local chart-generated deployment,100-minute boundary, browser and actual serving/edge revision remain unverified. Both exact parents/two children/former URLs and all owned local resources are removed; baseline/current quotas agree with zero termination. Product code and live ingress configuration are unchanged; this schedules the repair.

## HTTP/2 upload follow-up — sweep442

[Complete HTTP/2 follow-up evidence](evidence/cli-sweep442/README.md) confirms the same60s incoming-body deadline on two fresh native-created Free Node24 services. Both fast one-MiB PUTs200/full hash; both paced PUTs502 after60.606/60.570s,819200 submitted and consumed bytes. Native app aborts, independently verified prefix hashes and HTTP/2.0 edge502 records agree, while original boots stay healthy. The original HTTP/1 response was499. Direct local TLS/HTTP2 succeeds at77.620s. Pinned local Traefik3.7.5 reproduces502 at60.024s; changing only readTimeout to diagnostic100s allows the full paced200/hash. Exact local/hosted cleanup is verified. Preserve these protocol-specific statuses in regression evidence; do not require every failed protocol to return499. The milestone remains todo and its live post-fix acceptance is outstanding.
