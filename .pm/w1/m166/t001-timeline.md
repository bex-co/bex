# w1 · m166 · t001 — measured readiness timeline (2026-09-28, production)

Fixture `qa-20260928-m166` (`red-dat25gq1pbgc73a24llg`), `starter`, public so an
external `rediss://` endpoint exists. Deleted after the run (`204`, then `404`).

Two concurrent one-second clocks: an external TLS connect + `AUTH` + `PING` on stdlib
`ssl`, and `GET /v1/key-value/{id}.status`. Only state *changes* are logged, so a line
means a transition. Offsets are seconds from the first sample.

```text
     0.0s  [setup ] store red-dat25gq1pbgc73a24llg, external TLS endpoint resolved
     0.3s  [api   ] status=available
     0.8s  [client] SERVING
     8.0s  [action] PATCH maxmemory -> allkeys_lfu
     8.3s  [action]   -> 200
     9.6s  [api   ] status=config_restart
    14.1s  [client] DOWN(timeout)
    21.7s  [client] DOWN(tls:SSLEOFError)
    28.6s  [api   ] status=available
    30.5s  [client] DOWN(timeout)
    32.3s  [client] SERVING
    33.6s  [client] DOWN(tls:SSLEOFError)
    36.8s  [client] SERVING
    38.1s  [client] DOWN(tls:SSLEOFError)
    41.2s  [client] SERVING
    46.1s  [client] DOWN(tls:SSLEOFError)
    53.2s  [client] SERVING
   128.3s  [action] PATCH persistence -> snapshot
   128.6s  [action]   -> 200
   129.1s  [api   ] status=config_restart
   134.7s  [client] DOWN(timeout)
   136.1s  [client] DOWN(tls:SSLEOFError)
   140.5s  [api   ] status=available
   144.9s  [client] DOWN(timeout)
   152.8s  [client] SERVING
   154.2s  [client] DOWN(tls:SSLEOFError)
   157.3s  [client] SERVING
   160.4s  [client] DOWN(tls:SSLEOFError)
   163.7s  [client] SERVING
   165.0s  [client] DOWN(tls:SSLEOFError)
   166.8s  [client] SERVING
   248.6s  [action] POST suspend
   249.0s  [action]   -> 202
   250.1s  [api   ] status=suspended
   254.3s  [client] DOWN(timeout)
   268.2s  [client] DOWN(tls:SSLEOFError)
   275.8s  [client] DOWN(timeout)
   277.2s  [client] DOWN(tls:SSLEOFError)
   309.0s  [action] POST resume
   309.3s  [action]   -> 202
   309.7s  [api   ] status=config_restart
   336.3s  [api   ] status=available
   340.4s  [client] SERVING
   345.4s  [client] DOWN(tls:SSLEOFError)
   351.2s  [client] SERVING
   359.7s  [client] DOWN(tls:SSLEOFError)
   362.8s  [client] SERVING
```

Read the three `status=available` lines against the `SERVING` line that follows and
*stays* — 24.6s, 26.3s and 26.5s. The intermediate `SERVING` lines are the flap, and
they are why a single pre-publish probe cannot close this.
