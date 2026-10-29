# w8/m125 evidence

## t001/t002 — one minted id on every hop (2026-10-20, local docker)

`docker.io/traefik:v3.7.5` with the `clientipheaders` plugin on the entrypoint (`abortOnPluginFailure=true`), access log JSON with `fields.headers.names.Rndr-Id=keep`, backend `docker.io/ealen/echo-server:0.9.2`. A request sending `Rndr-Id: forged` and `True_Client_IP: 1.2.3.4`:

```
response header   Rndr-Id: cf4fae52-9cc1-43ff-89d5-55c8cb8001c1
app received      rndr-id: cf4fae52-9cc1-43ff-89d5-55c8cb8001c1   true-client-ip: 172.29.0.1
access log        "request_Rndr-Id":"cf4fae52-9cc1-43ff-89d5-55c8cb8001c1"
```

The caller's values never reached the app or the log.
