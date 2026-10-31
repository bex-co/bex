# Web responses transfer their full size despite automatic-compression offers

**Minor · hosting-edge Render compatibility/performance.** Two native clients create healthy Free Node24 web services. All16 HTTP/1.1/negotiated-HTTP2 response comparisons preserve exact content, but all10 eligible Brotli/gzip offers return identity at280000/295000bytes. Their app-authored gzip controls transfer920/966bytes and decode exactly; no-transform and all4 streamed-event connections pass. [Full native/wire/runtime/source/cleanup evidence](evidence/cli-sweep352/README.md).

Identity is protocol-valid. The expected capability is [Render's automatic load-balancer Brotli/gzip compression](https://render.com/docs/render-vs-heroku-comparison), also listed for [web services](https://render.com/docs/web-services). This is not a claim of content corruption, a particular vendor threshold or encoded size, or an authenticated Render comparison.

## Producer and consumer

Both captured pinned-native create builders send unchanged source/master/Node24/Free and exact commands; POST201/native0 and first Live/artifact proof pass. REST/GraphQL/MCP settings agree in6 API-only reads. The program deliberately emits plain eligible text at /text and a separate valid app-gzip representation at /origin-gzip, so absent transfer encoding is not a failed app build or an inability to carry compression.

The public HTTP path is checked-in Traefik plus an App-owned Ingress. `lego/operator/internal/controller/app_controller.go:2904–2922` assembles only the existing filtering layers and WebSocket meter before calling reconcileIngress; `:3505–3521` creates the meter; `:3533–3566` and `applyIngressRouting:483–499` reference exactly that chain. There is no compression producer. `deploy/gitops/base/values/traefik.values.yaml` and the prod values add no global compressor. `deploy/gitops/charts/traefik-plugins/websocketegress/websocketegress.go:106–110` directly delegates normal HTTP.

The actual shared route projector reproduces absent compression for four source-class inputs while backend/meter controls pass. It is an explicitly failing private source experiment (4 subcases plus parent), not an implementation or a claim that a fake client executed Traefik. Three maintained wake/maintenance tests and7 named events pass. Serving SHA is unobserved; fetched main is identical to research bd0ffac525f2ff1dfb4d7c7add9f81000fed8036 and has no newer repair.

## Concrete correction and shared scope

Wire an App-owned response-compression middleware into the ordinary public web route chain, using the deployed/reviewed Traefik Compress facility where it satisfies the contract. Its [official configuration](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/compress/) supports Brotli/gzip selection; verify the actual pinned proxy before choosing configuration. Preserve existing references/order and owned-resource convergence/deletion. Use a bounded eligible-response policy rather than adding compression code to every tenant app or rewriting the CLI.

Audit content-type/size/quality negotiation, absent/empty/identity/q=0 requests, already encoded responses, no-transform, HEAD/framing, strong ETags/304, Range/If-Range, SSE flush timing, upgrades and gRPC. A bare enable is not accepted if it changes these neighbors; add only the necessary compatible guard/composition after exercising the actual proxy. Preserve existing static-server behavior and avoid double encoding, cross-publication caching or broad API changes. Verify compressed-byte accounting and bounded runtime costs without making a new observability product.

Census all reconcileIngress/applyIngressRouting callers: normal web serving, auto-hibernate activator routing, host redirects and the static-server backend use related paths but need distinct dispositions. Only warm public web responses are live exercised here. The shared web projector is source-observed for Node/Python/Dockerfile/image; other native runtimes follow the same web-routing branch, without a live claim. Private/worker/cron have no ordinary public web ingress. REST/GraphQL/MCP/dashboard/Blueprint source settings retain their current schema, with no new customer switch or client fork. This is existing hosting behavior below those adapters.

## Dedupe and acceptance

Open/blocked/done PM titles/content, DO_NOT_DO, ADR018, source/history and fresh origin searches find no dynamic-web compressor owner. [m79](../done/m79/README.md) and its five tasks/whole DoD are explicitly static-site Handler serving, and were accepted with static text/JS/CSS, representation neighbors and cleanup. Its original diagnosis noted no global edge compressor but did not promise to repair dynamic web serving. The new web route bypasses that Handler, so keep m79 done within its accepted scope. No CDN/purge work or unrelated API compression is proposed.

Six tasks total160m. Required acceptance is fresh native first-deployment fixtures serving negotiated, smaller Brotli/gzip that decode exactly across HTTP1/2 while preserving the current identity/gzip/no-transform/SSE controls. Add maintained projection plus actual-proxy fail-before/pass-after checks, audit shared routing callers and cleanup, and close only with deployed evidence. All fixtures from this hunt are deleted; no product fix is implemented.
