# w1 · m166 — production root-cause timeline (2026-10-02, production, READ-ONLY cluster)

Fixture `qa-20261002-m166` (`red-davkbd38dvus73e89p30`), `starter`, **public**, in the QA
workspace `tea-d98210cbbpdc73dcrkvg`. Deleted after the run (`204`, then `GET` → `404`).
Old build: headless `<id>` Service, TCP-socket readiness on 6379.

Five concurrent samplers, one second apart (only *changes* are logged, except proxy lines,
which are every routed failure for this resource — runs of identical lines are elided):

- `[client]` external TLS connect + `AUTH` + `PING` to `<id>.kv.bex.co:6379` over IPv4 on
  `node:tls` (password read from `connection-info` inside the process; never printed).
  The failure stage is recorded: `tcp`, `tls` (handshake), `resp`.
- `[api   ]` `GET /v1/key-value/{id}` `.status`, from the signed-in QA browser session.
- `[pods  ]` `kubectl get pods -l app.bex.co/keyvalue=<id>`: name/uid, IP, Ready, start,
  deletionTimestamp, controller revision.
- `[endpts]` `kubectl get endpoints <id>` (the token cannot read `discovery.k8s.io`
  EndpointSlices; the Endpoints mirror carries the same ready/notReady split).
- `[proxy ]` `kubectl logs -f` on all three `bex-kv-sni-proxy` pods, filtered to this
  resource's `dial backend` errors. `ts=` is the proxy's own clock (the dial *ended* then;
  a `i/o timeout` dial started 10s earlier, `dialTimeout`).

Cluster facts read the same session: CoreDNS `kube-system/coredns` Corefile has
`kubernetes cluster.local … { ttl 30 }` and `cache 30`, **two** CoreDNS replicas; the
StatefulSet is `RollingUpdate`/`OrderedReady`, one replica, `terminationGracePeriodSeconds:
30`, readiness `tcpSocket: 6379` every 10s; the Service is `clusterIP: None`, no
`publishNotReadyAddresses`.

```text
05:48:36.599Z     2.0s  [setup ] create 201 red-davkbd38dvus73e89p30
05:49:37.193Z    62.6s  [setup ] available; external host red-davkbd38dvus73e89p30.kv.bex.co; namespace tea-d98210cbbpdc73dcrkvg
05:49:37.493Z    62.9s  [api   ] status=available
05:49:37.822Z    63.2s  [client] SERVING
05:49:37.977Z    63.3s  [endpts] ready=[10.244.34.249] notReady=[] publishNotReady=-
05:49:37.991Z    63.4s  [pods  ] red-davkbd38dvus73e89p30-0/b661ec ip=10.244.34.249 phase=Running ready=True@05:49:30 start=05:48:40 rev=b49bd7
05:49:46.103Z    71.5s  [action] PATCH maxmemoryPolicy -> allkeys_lfu
05:49:46.581Z    71.9s  [action]   -> 200
05:49:46.773Z    72.1s  [client] DOWN(resp:EOF)
05:49:46.800Z    72.2s  [api   ] status=config_restart
05:49:46.815Z    72.2s  [endpts] ready=[] notReady=[] publishNotReady=-
05:49:46.970Z    72.3s  [pods  ] red-davkbd38dvus73e89p30-0/b661ec ip=10.244.34.249 phase=Running ready=True@05:49:30 start=05:48:40 DELETING@05:50:16 rev=b49bd7
05:49:47.786Z    73.1s  [pods  ] red-davkbd38dvus73e89p30-0/8083e1 ip=- phase=Pending ready=-@- start=- rev=9684b7
05:49:48.849Z    74.2s  [pods  ] red-davkbd38dvus73e89p30-0/8083e1 ip=- phase=Pending ready=False@05:49:47 start=05:49:47 rev=9684b7
05:49:50.376Z    75.7s  [client] DOWN(timeout@tls)
05:49:57.553Z    82.9s  [proxy ] xtz4w ts=2026-10-02T05:49:57Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
    ... 4 more identical proxy dial failures (all 3 pods) ...
05:50:07.082Z    92.4s  [proxy ] w5xmh ts=2026-10-02T05:50:07Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
05:50:08.804Z    94.2s  [endpts] ready=[] notReady=[10.244.34.120] publishNotReady=-
05:50:08.960Z    94.3s  [pods  ] red-davkbd38dvus73e89p30-0/8083e1 ip=10.244.34.120 phase=Running ready=False@05:49:47 start=05:49:47 rev=9684b7
05:50:09.951Z    95.3s  [pods  ] red-davkbd38dvus73e89p30-0/8083e1 ip=10.244.34.120 phase=Running ready=True@05:50:09 start=05:49:47 rev=9684b7
05:50:10.256Z    95.6s  [proxy ] xtz4w ts=2026-10-02T05:50:10Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
05:50:10.519Z    95.9s  [api   ] status=available
05:50:10.783Z    96.1s  [endpts] ready=[10.244.34.120] notReady=[] publishNotReady=-
05:50:11.055Z    96.4s  [proxy ] gpcn2 ts=2026-10-02T05:50:11Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
05:50:13.133Z    98.5s  [client] SERVING
05:50:13.426Z    98.8s  [proxy ] w5xmh ts=2026-10-02T05:50:13Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
    ... 2 more identical proxy dial failures (all 3 pods) ...
05:50:19.770Z   105.1s  [proxy ] w5xmh ts=2026-10-02T05:50:19Z dial backend err=dial tcp 10.244.34.249:6380: i/o timeout
05:50:39.268Z   124.6s  [action] POST suspend
05:50:39.862Z   125.2s  [action]   -> 202
05:50:39.922Z   125.3s  [client] DOWN(tls:EOF)
05:50:39.922Z   125.3s  [proxy ] xtz4w ts=2026-10-02T05:50:39Z dial backend err=dial tcp 10.244.34.120:6380: connect: connection refused
05:50:40.503Z   125.9s  [api   ] status=suspended
05:50:40.906Z   126.3s  [endpts] ready=[] notReady=[] publishNotReady=-
05:50:41.017Z   126.4s  [proxy ] gpcn2 ts=2026-10-02T05:50:40Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:50:41.087Z   126.5s  [pods  ] red-davkbd38dvus73e89p30-0/8083e1 ip=10.244.34.120 phase=Succeeded ready=False@05:50:40 start=05:49:47 DELETING@05:51:09 rev=9684b7
05:50:41.969Z   127.3s  [pods  ] none
05:50:44.784Z   130.1s  [client] DOWN(timeout@tls)
05:50:45.145Z   130.5s  [client] DOWN(tls:EOF)
05:50:45.145Z   130.5s  [proxy ] xtz4w ts=2026-10-02T05:50:45Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 5 more identical proxy dial failures (all 3 pods) ...
05:50:51.138Z   136.5s  [proxy ] xtz4w ts=2026-10-02T05:50:51Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:50:51.956Z   137.3s  [proxy ] w5xmh ts=2026-10-02T05:50:51Z dial backend err=dial tcp 10.244.34.120:6380: i/o timeout
05:50:52.148Z   137.5s  [proxy ] gpcn2 ts=2026-10-02T05:50:52Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 16 more identical proxy dial failures (all 3 pods) ...
05:51:09.169Z   154.5s  [proxy ] xtz4w ts=2026-10-02T05:51:09Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:09.919Z   155.3s  [action] POST resume
05:51:10.152Z   155.5s  [proxy ] gpcn2 ts=2026-10-02T05:51:10Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:10.368Z   155.7s  [action]   -> 202
05:51:10.636Z   156.0s  [api   ] status=config_restart
05:51:11.056Z   156.4s  [pods  ] red-davkbd38dvus73e89p30-0/ab17d6 ip=- phase=Pending ready=False@05:51:10 start=05:51:10 rev=9684b7
05:51:11.154Z   156.5s  [proxy ] w5xmh ts=2026-10-02T05:51:11Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 47 more identical proxy dial failures (all 3 pods) ...
05:51:42.180Z   187.5s  [proxy ] w5xmh ts=2026-10-02T05:51:42Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:42.943Z   188.3s  [endpts] ready=[10.244.34.140] notReady=[] publishNotReady=-
05:51:43.110Z   188.5s  [pods  ] red-davkbd38dvus73e89p30-0/ab17d6 ip=10.244.34.140 phase=Running ready=True@05:51:42 start=05:51:10 rev=9684b7
05:51:43.193Z   188.6s  [proxy ] xtz4w ts=2026-10-02T05:51:43Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:43.576Z   188.9s  [api   ] status=available
05:51:43.723Z   189.1s  [proxy ] gpcn2 ts=2026-10-02T05:51:43Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 4 more identical proxy dial failures (all 3 pods) ...
05:51:46.866Z   192.2s  [proxy ] xtz4w ts=2026-10-02T05:51:46Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:47.373Z   192.7s  [client] SERVING
05:51:49.182Z   194.5s  [client] DOWN(tls:EOF)
05:51:49.184Z   194.5s  [proxy ] gpcn2 ts=2026-10-02T05:51:49Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 1 more identical proxy dial failures (all 3 pods) ...
05:51:51.186Z   196.5s  [proxy ] gpcn2 ts=2026-10-02T05:51:51Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:52.357Z   197.7s  [client] SERVING
05:51:54.183Z   199.5s  [client] DOWN(tls:EOF)
05:51:54.185Z   199.5s  [proxy ] xtz4w ts=2026-10-02T05:51:54Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 6 more identical proxy dial failures (all 3 pods) ...
05:51:58.807Z   204.2s  [proxy ] gpcn2 ts=2026-10-02T05:51:58Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:59.374Z   204.7s  [client] SERVING
05:52:00.187Z   205.6s  [client] DOWN(tls:EOF)
05:52:00.187Z   205.6s  [proxy ] xtz4w ts=2026-10-02T05:52:00Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
    ... 1 more identical proxy dial failures (all 3 pods) ...
05:52:02.216Z   207.6s  [proxy ] gpcn2 ts=2026-10-02T05:52:02Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:02.375Z   207.7s  [client] SERVING
05:52:03.179Z   208.5s  [client] DOWN(tls:EOF)
05:52:03.179Z   208.5s  [proxy ] w5xmh ts=2026-10-02T05:52:03Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:04.371Z   209.7s  [client] SERVING
05:52:05.563Z   210.9s  [proxy ] xtz4w ts=2026-10-02T05:52:05Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:06.201Z   211.6s  [client] DOWN(tls:EOF)
05:52:06.202Z   211.6s  [proxy ] gpcn2 ts=2026-10-02T05:52:06Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:07.186Z   212.5s  [proxy ] w5xmh ts=2026-10-02T05:52:07Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:07.392Z   212.8s  [client] SERVING
05:52:08.765Z   214.1s  [proxy ] w5xmh ts=2026-10-02T05:52:08Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:10.203Z   215.6s  [client] DOWN(tls:EOF)
05:52:10.203Z   215.6s  [proxy ] gpcn2 ts=2026-10-02T05:52:10Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:10.357Z   215.7s  [proxy ] w5xmh ts=2026-10-02T05:52:10Z dial backend err=dial tcp: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:52:11.405Z   216.8s  [client] SERVING
05:52:42.983Z   248.3s  [setup ] delete 204 then GET 404
```
