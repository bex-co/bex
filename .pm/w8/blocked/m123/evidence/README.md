# w8/m123 evidence

## t006 — the pinned Traefik loads the plugin and sanitizes (2026-10-20, local docker)

`docker.io/traefik:v3.7.5` (the chart appVersion), `--experimental.abortOnPluginFailure=true`, the plugin mounted as its ConfigMap ships it (`.traefik.yml`, `go.mod`, `clientipheaders.go`), `--entryPoints.web.http.middlewares=sanitize@file`, backend `docker.io/ealen/echo-server:0.9.2`.

```
2026-10-20T08:37:52Z INF Loading plugins... plugins=["bexClientIPHeaders"]
2026-10-20T08:37:52Z INF Plugins loaded. plugins=["bexClientIPHeaders"]
$ curl localhost:48123/cf -H 'CF-Connecting-IP: 203.0.113.66' -H 'True-Client-IP: 203.0.113.77' -H 'X-Client-IP: 203.0.113.88' -H 'CF-IPCountry: KP' -H 'X-Forwarded-For: 1.2.3.4'
echoed: {'true-client-ip': '172.29.0.1', 'x-forwarded-for': '172.29.0.1', 'x-real-ip': '172.29.0.1'}  (no cf-connecting-ip, cf-ipcountry, x-client-ip)
```

After the `/simplify` pass (underscore spellings stripped, multi-value `CF-Connecting-IP` collapsed, range scan only when `CF-Connecting-IP` is present), the same pinned Traefik with `abortOnPluginFailure=true` loaded the revised plugin. A request carrying forged `CF-Connecting-IP`, `True-Client-IP`, `True_Client_IP`, `X-Client-IP` and `CF-IPCountry` reached the echo app as `{'true-client-ip': '172.29.0.1', 'x-real-ip': '172.29.0.1'}`, with no other client-identity header.

