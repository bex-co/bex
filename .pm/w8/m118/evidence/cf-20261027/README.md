# m118 Cloudflare custom-domain check — 2026-10-27 UTC

User-authorized test using the already authenticated `cf` CLI. One temporary DNS-only CNAME at `qa-20261027-m118-19862e.10x.pub`, TTL 60, targeted an owned Free Python echo service `srv-dbg41fkd3d3c73caqitg` in the existing Scale workspace `bex`. No plan change or existing service mutation. [Request/response receipts and cleanup](receipts.json).

## Results

- With the platform subdomain enabled (the default), HTTP redirects to HTTPS, and HTTP-01 issuance for the new custom hostname succeeds. The custom Certificate becomes Ready at 06:12:57Z. The solver's HTTP 200 responses are preserved in [Traefik excerpts](traefik-excerpts.txt).
- HTTPS to the custom hostname still fails normal verification (`curl` exit 60): the edge presents the unrelated `*.onbex.co` fallback certificate. The API nevertheless reports `certificateStatus: verified`, `serverStatus: active`.
- The [serving Ingress](own-ingresses.json) names the platform-host TLS Secret first, then the custom-host Secret. The [platform Certificate](own-certificates.json) cannot issue: Let's Encrypt returned its 50-per-168-hour `onbex.co` limit, with retry-after 2026-10-27 08:51:06 UTC. Traefik v3.7.5 logs that the first Secret does not exist.
- On the **same owned fixture**, clearing the IP rule and setting `serviceDetails.renderSubdomainPolicy: disabled` removes the platform entry. After fresh HTTP-01 issuance under the remaining primary Secret, custom HTTPS passes normal certificate verification and serves the exact fixture marker. This is a diagnostic control, not acceptance of the default configuration. The issued certificate's SAN is exactly the custom hostname, issuer Let's Encrypt YR2.
- In that custom-only configuration, GET HTTP returns 301; HEAD and POST return 308; each Location preserves `/x?y=1`. Following HTTP produces exactly one redirect and HTTPS 200; the app sees `X-Forwarded-Proto: https` and the original path/query.
- Setting the fixture's inbound rule to `203.0.113.0/24` produces HTTPS 403; following HTTP gives one redirect then 403. This also passed on the platform host before the custom-only control.
- Focused existing operator tests pass: `GOWORK=off go test ./internal/controller -run 'TestWebServiceIngressRedirectsHTTPToHTTPS|TestEveryAppIngressRedirectsAndDropsTheRedirectWithItsHosts' -count=1`. They cover the off switch and redirect removal but do not exercise real certificate loading.

## Root cause and required work

`lego/operator/internal/controller/app_controller.go` `applyAppIngress` / `appendIngressHostRoute` emit every host's TLS reference into one Ingress, platform first. The deployed provider's [getCertificates](https://github.com/traefik/traefik/blob/v3.7.5/pkg/provider/kubernetes/ingress/kubernetes.go#L741) returns at the first missing Secret, before reading later entries. Thus a pending platform certificate prevents the valid custom certificate from being loaded. Default wildcard TLS does not satisfy that missing per-App reference.

[t006](../../done/t006.md) owns removing this dependency while preserving both public hostnames, HTTP-01, router attribution, and per-host certificate lifecycle. Dedupe checked open/done PM records for the exact TLS error, missing primary Secret, and rate-limit/custom-domain combinations: w2/done/m85 supplies the global wildcard fallback but does not fix this mixed-host loading failure; m118 already owns the end-to-end custom-domain closeout, so no separate milestone was filed.

## Cleanup

The Cloudflare DNS record `78d6b1387d0b565acefc1b4b8fe7665c` was deleted and exact-name DNS inventory is empty. The custom-domain claim was removed; the service was deleted and GET by ID returns 404. [Cluster verification](cluster-cleanup.json) found zero remaining owned Apps, Deployments, ReplicaSets, Pods, Services, Ingresses, Certificates, CertificateRequests, Orders, Challenges, Middlewares, PVCs, Secrets, ConfigMaps or Jobs. This test's Kratos session was revoked, its cookie/state files were deleted, and both temporary kubeconfig files were removed. No application fix, deployment or commit was made.
