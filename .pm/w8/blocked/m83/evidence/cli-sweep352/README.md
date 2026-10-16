# Ordinary web responses lack automatic compression — sweep352

[Results](results.json) · [complete native POSTs, source/config readbacks, HTTP bodies/headers, stream frames/timing and cleanup](native-wire-API-HTTP-stream-cleanup.json) · [fresh main comparison](fresh-main-comparison.json) · [source reference](primary-source-reference.json) · [real route probe](actual-route-probe.go.txt) / [events](actual-route-probe-events.jsonl) · [maintained routing controls](maintained-routing-neighbor-events.jsonl) · [final inventory](final-inventory.json) · [dedupe](dedupe.json).

Research/fetched source bd0ffac525f2ff1dfb4d7c7add9f81000fed8036; serving revision unobserved. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 use api.bex.co and the continuing isolated human QA identity in bex-canary. Installed command help/pin were checked earlier in this continuing hunt; no executable is overwritten.

## Two native first-deployment fixtures

Each client creates one explicit Free Node24 web service, Frankfurt, autoDeploy off, public expressjs/serve-static/master, no PATH override and no dependency installation. Actual unchanged native create POST201/native0 and exact workspace/name/Free/pre-inventory absence prove ownership. The authored build verifies package.json SHA2564f2884fa761dee155f31cb960d792c2900dbaa6aecbe6ebcc3bcfd7e754706b8 and bakes its nonce/version/hash snapshot. Both first deployments become Live/HTTP200 at independently checked Git commit109a299c26d2d532027bb6e66e50e365eeb8fcdd, with exact build/runtime markers and Node24. There are no later update/redeploy children.

- bex: srv-db8rv6i9cl7s73ae4au0 / dep-db8rv6i9cl7s73ae4aug; 280000 plain bytes; 920 authored gzip bytes.
- render: srv-db8rvdndbg8c73dghcfg / dep-db8rvdndbg8c73dghcg0; 295000 plain bytes; 966 authored gzip bytes.

Exact original commands: [Bex](bex-initial-command.sh.txt), [Render](render-initial-command.sh.txt). The same harmless Node stdlib program serves all variants without any configuration update. Six API-only REST/GraphQL/MCP reads agree on exact repo/master/runtime/Free/autoDeploy/build/start; MCP explicitly names the QA workspace. This is a native create/deploy journey followed by HTTP serving diagnostics, not a CLI compression flag.

## Compression failure with positive controls

Each fixture serves a large deterministic UTF-8 text body: its own nonce plus repeated plain text, accents and Japanese. Twelve HTTP/1.1 GETs include identity, three normal encoding offers (br, gzip, br+gzip), no-transform, and authored gzip for each service. Four further curl --http2 GETs offer br and gzip; actual response status lines confirm HTTP/2, not a fallback claim. Every one of the16 response bodies decodes to its exact independently computed payload byte count/hash.

All10 eligible automatic-encoding offers return no Content-Encoding and their full280000/295000bytes. The2 authored-gzip controls return Content-Encoding:gzip and920/966bytes, decoding exactly. Both no-transform controls remain identity as expected. Identity responses remain valid HTTP; the failure is the documented hosting compression capability, not an Accept-Encoding conformance violation, damaged content, inability to carry compressed bytes or missing app deployment.

The [Render load-balancer description](https://render.com/docs/render-vs-heroku-comparison) explicitly places automatic Brotli/gzip response compression at the hosting edge; the [web-service feature list](https://render.com/docs/web-services) also includes Brotli. No authenticated Render fixture or provider-exact threshold/encoded size is measured. The current test checks one eligible content type/size per fixture; broader negotiation and HTTP neighbors belong to implementation acceptance.

Full raw responses are retained in the main JSON: exact headers plus complete body bytes as gzip+base64 wrappers, original raw byte length/SHA256 and independent decoded length/SHA256. The filing verifies decoding each wrapper back to the recorded raw body. These wrappers contain only this run's synthetic fixture body, not an unrelated encoded board blob. Raw curl files are removed after each request. [HTTP1 driver](HTTP1-diagnostic-driver.py.txt), [HTTP2 driver](HTTP2-diagnostic-driver.py.txt).

## Streaming behavior passes

Four separate finite SSE connections (identity and br+gzip for both services) each receive three exact nonce/sequence/Unicode events. Their first arrivals are619/627/668/808ms; later events arrive separately around1.7–1.9s and2.9–3.0s, following the application's1.1-second emit interval. Full frames, emitted timestamps, arrival times, headers/chunk boundaries, exits and elapsed times are retained. No SSE buffering issue is found.

The [same stream client's loopback preflight](local-stream-preflight.json) receives events at8/1109/2211ms and ends3313ms using the exact server program on host Node. Its owned local child is terminated in finally; this is observer validation, not Node24 image acceptance. [Stream client](finite-stream-client.js.txt), [server template](harmless-web-server.js.txt), [preflight driver](local-stream-preflight-driver.py.txt). Future compression must preserve this passing behavior, pre-encoded data and no-transform.

## Real source projection and limits

The private overlay calls unchanged reconcileIngressWithMiddlewares with native Node, native Python, Dockerfile and prebuilt-image web inputs on a fake Kubernetes client. In every case the actual Ingress backend and WebSocket-meter reference controls pass; the created/referenced middlewares contain no compress. Four explicit expected-compression assertions fail, plus their parent (5 named failing events, no setup failure/skip). Four actual projected Ingress/middleware exports are retained. This is the shared route-projector seam, not four full controller lifecycles or four live runtime classes. No repository Go file is added.

Base/prod checked-in Traefik values contain no global compression setup. The existing WebSocket-meter plugin bypasses ordinary HTTP; the static-server Handler's Brotli implementation serves a different backend. [Traefik's own Compress documentation](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/compress/) identifies the available edge mechanism. A fix must verify the deployed pin's behavior and neighboring HTTP rules before enabling it broadly; no counterfactual proxy configuration is executed here. Three maintained wake/maintenance routing tests pass (7 named events, no skips) independently of the expected failing probe. No full operator suite, proxy-image execution or production deployment is claimed.

## Cleanup

Both native service deletes exit0; both parent details/former URLs and their2 first-deployment children are404/absent. Original6 service IDs/original1 Blueprint remain; services used6/terminating0, PostgreSQL/KeyValue0, groups/projects empty. No paid fixture, cron, custom domain or remote source edit. Per-call request/config/HTTP files and local processes are cleaned; the isolated login remains for the authorized continuing loop. Credentials are masked and baseline configurations projected before filing. Uncommitted.
