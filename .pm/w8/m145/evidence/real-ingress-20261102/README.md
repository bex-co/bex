# m145 real-Ingress comparison, Traefik v3.7.5 → v3.7.17 — 2026-11-02

A throwaway kind cluster (`w8-m145`, deleted afterwards) ran the locked chart 41.0.0 with the unchanged base values (hostNetwork edge, `bexClientIPHeaders` on both HTTP entrypoints, the production access-log allowlist and Prometheus settings) and the `traefik-plugins` ConfigMaps rendered by `kubectl kustomize`. [Tenant](tenant.yaml): a `traefik/whoami` App behind a routed Ingress carrying the `bexWebsocketEgress` Middleware, and a second Ingress behind an IPAllowList that refuses the probe. [Probe](probe.sh) ran from the node; the release was installed at `image.tag=v3.7.5`, probed, then `helm upgrade`d to the pinned `v3.7.17@sha256:30ff6249…` and probed again.

| check | v3.7.5 | v3.7.17 |
| --- | --- | --- |
| plugins load under `abortOnPluginFailure` | both | both |
| forged `True-Client-IP` / `CF-Connecting-IP` / `X-Client-IP` / `Rndr-Id` | replaced/stripped; fresh `Rndr-Id` | identical |
| IPAllowList refusal / unknown host | 403 / 404 | identical |
| WebSocket upgrade + `bexWebsocketEgress` counters | 101, metered | identical |
| router names + `traefik_router_*`/`traefik_service_*` series | [probe](probe-v3.7.5.txt) | [identical](probe-v3.7.17.txt) |
| access-log JSON (routed 200, refused 403, unknown 404) after [normalizing](normalize.py) times/ids | [lines](access-log-v3.7.5.jsonl) | [byte-identical](access-log-v3.7.17.jsonl) |

The attribution fields (`ServiceName`, `KubernetesIngressNamespace`, `KubernetesIngressName`) and the `Rndr-Id` header keep their values on routed and refused requests; the new per-request `KubernetesService*` fields are dropped by the allowlist. Router names match `core.TraefikRouterNamesForIngress` (upstream `makeRouterKeyWithHash`/`provider.Normalize` unchanged; default `strictTLSOptions=false` keeps the TLS-sibling naming). PROXY protocol: same `go-proxyproto` v0.12.0 and listener code. Not covered here: the Hetzner LB/PROXY-protocol path and cert-manager TLS, which the production roll exercises.
