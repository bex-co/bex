# w1 · m166 t007 — production verification timeline on the fixed build (2026-10-02, production, READ-ONLY cluster)

Operator `ghcr.io/bex-co/bex-operator@sha256:e396cf35…` (deploy pin `20fb64867` → `1263d12ae`, which contains `2e35b42a1`). Fixture `qa-20261002-3b7f-m166` (`red-db09sr2tm2ss7389qmqg`), `starter`, **public**, with an allowlist of the prober's own `/32`, in the QA workspace `tea-d98210cbbpdc73dcrkvg`. It was deleted afterwards: `DELETE` → `204`, `GET` → `404`, and the KeyValue CR, StatefulSet, Service and PVC were gone about two minutes later. Clock times are UTC on 2026-10-03.

The method matches [t003-prod-rootcause-timeline.md](t003-prod-rootcause-timeline.md). Five concurrent samplers about one second apart log only *changes*; proxy lines are every routed failure for this resource, and runs are elided.

- `[client]`: external TLS connect + `AUTH` + `PING` to `<id>.kv.bex.co:6379` over IPv4, written on Python stdlib `ssl`. The password is read from `connection-info` inside the process and never printed.
- `[api   ]`: `GET /v1/key-value/{id}` `.status`.
- `[pods  ]`: the pods and their Ready state.
- `[endpts]`: Endpoints.
- `[proxy ]`: `kubectl logs -f` on all three `bex-kv-sni-proxy` pods.

The Service was `10.106.180.168/ClusterIP`. The two pre-existing KeyValue Services in production (`red-d9p49kdrtmes73c34ovg`, `red-da4086iii7bs73drbqh0`) also show a cluster IP, not `None`.

```text
06:19:23.656Z      0.1s  [setup ] create qa-20261002-3b7f-m166
06:19:24.138Z      0.6s  [setup ] create 201 red-db09sr2tm2ss7389qmqg
06:20:23.607Z     60.0s  [setup ] available; external host red-db09sr2tm2ss7389qmqg.kv.bex.co
06:20:24.385Z     60.8s  [setup ] service clusterIP/type = 10.106.180.168/ClusterIP
06:20:25.857Z     62.3s  [api   ] status=available
06:20:26.208Z     62.6s  [endpts] ready=['10.244.34.51'] notReady=[]
06:20:26.385Z     62.8s  [pods  ] red-db09sr2tm2ss7389qmqg-0/e7bd42 ip=10.244.34.51 phase=Running ready=True rev=7795b5
06:20:26.517Z     62.9s  [client] SERVING
06:20:35.423Z     71.8s  [action] PATCH maxmemoryPolicy -> allkeys_lfu
06:20:35.738Z     72.2s  [action]   -> 200
06:20:35.831Z     72.2s  [endpts] ready=[] notReady=[]
06:20:36.004Z     72.4s  [pods  ] red-db09sr2tm2ss7389qmqg-0/e7bd42 ip=10.244.34.51 phase=Running ready=True DELETING rev=7795b5
06:20:36.019Z     72.4s  [proxy ] 2hg4b ts=2026-10-03T06:20:35Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:20:36.019Z     72.4s  [client] DOWN(tls:SSLEOFError)
06:20:36.442Z     72.9s  [api   ] status=config_restart
06:20:37.407Z     73.8s  [proxy ] zgnr7 ts=2026-10-03T06:20:37Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:20:38.286Z     74.7s  [pods  ] red-db09sr2tm2ss7389qmqg-0/29a995 ip=- phase=Pending ready=False rev=8f7d6f
06:20:40.219Z     76.6s  [proxy ] 2hg4b ts=2026-10-03T06:20:40Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 4 more identical proxy dial failures (ClusterIP, connection refused) ...
06:20:49.948Z     86.4s  [proxy ] 2hg4b ts=2026-10-03T06:20:49Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:20:50.734Z     87.1s  [endpts] ready=[] notReady=['10.244.34.10']
06:20:51.300Z     87.7s  [proxy ] zgnr7 ts=2026-10-03T06:20:51Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:20:52.140Z     88.6s  [pods  ] red-db09sr2tm2ss7389qmqg-0/29a995 ip=10.244.34.10 phase=Pending ready=False rev=8f7d6f
06:20:54.071Z     90.5s  [proxy ] 2hg4b ts=2026-10-03T06:20:54Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 3 more identical proxy dial failures (ClusterIP, connection refused) ...
06:21:02.243Z     98.7s  [proxy ] 2hg4b ts=2026-10-03T06:21:02Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:21:02.294Z     98.7s  [api   ] status=available
06:21:02.414Z     98.8s  [pods  ] red-db09sr2tm2ss7389qmqg-0/29a995 ip=10.244.34.10 phase=Running ready=True rev=8f7d6f
06:21:03.088Z     99.5s  [endpts] ready=['10.244.34.10'] notReady=[]
06:21:03.951Z    100.4s  [client] SERVING
06:21:50.739Z    147.2s  [action] PATCH persistenceMode -> snapshot
06:21:51.140Z    147.6s  [action]   -> 200
06:21:51.403Z    147.8s  [endpts] ready=[] notReady=[]
06:21:51.663Z    148.1s  [api   ] status=config_restart
06:21:52.404Z    148.8s  [pods  ] red-db09sr2tm2ss7389qmqg-0/29a995 ip=10.244.34.10 phase=Succeeded ready=False DELETING rev=8f7d6f
06:21:52.567Z    149.0s  [client] DOWN(tls:SSLEOFError)
06:21:53.958Z    150.4s  [proxy ] 2hg4b ts=2026-10-03T06:21:53Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:21:54.655Z    151.1s  [pods  ] red-db09sr2tm2ss7389qmqg-0/ddfdd9 ip=- phase=Pending ready=False rev=b549b5
06:21:55.353Z    151.8s  [proxy ] zgnr7 ts=2026-10-03T06:21:55Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:21:58.169Z    154.6s  [proxy ] 2hg4b ts=2026-10-03T06:21:58Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:21:59.560Z    156.0s  [proxy ] zgnr7 ts=2026-10-03T06:21:59Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:22:01.467Z    157.9s  [pods  ] red-db09sr2tm2ss7389qmqg-0/ddfdd9 ip=10.244.34.86 phase=Pending ready=False rev=b549b5
06:22:01.820Z    158.2s  [endpts] ready=[] notReady=['10.244.34.86']
06:22:02.344Z    158.8s  [proxy ] 2hg4b ts=2026-10-03T06:22:02Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 6 more identical proxy dial failures (ClusterIP, connection refused) ...
06:22:16.327Z    172.7s  [proxy ] zgnr7 ts=2026-10-03T06:22:16Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:22:17.369Z    173.8s  [pods  ] red-db09sr2tm2ss7389qmqg-0/ddfdd9 ip=10.244.34.86 phase=Running ready=True rev=b549b5
06:22:17.876Z    174.3s  [api   ] status=available
06:22:18.084Z    174.5s  [client] SERVING
06:22:18.601Z    175.0s  [endpts] ready=['10.244.34.86'] notReady=[]
06:23:06.143Z    222.6s  [action] POST suspend
06:23:06.496Z    222.9s  [action]   -> 202
06:23:06.916Z    223.3s  [endpts] ready=[] notReady=[]
06:23:07.282Z    223.7s  [pods  ] red-db09sr2tm2ss7389qmqg-0/ddfdd9 ip=10.244.34.86 phase=Succeeded ready=False DELETING rev=b549b5
06:23:07.311Z    223.7s  [client] DOWN(tls:SSLEOFError)
06:23:07.311Z    223.7s  [proxy ] 2hg4b ts=2026-10-03T06:23:07Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:23:07.545Z    224.0s  [api   ] status=suspended
06:23:08.681Z    225.1s  [proxy ] zgnr7 ts=2026-10-03T06:23:08Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:23:09.347Z    225.8s  [pods  ] none
06:23:11.409Z    227.8s  [proxy ] 2hg4b ts=2026-10-03T06:23:11Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 32 more identical proxy dial failures (ClusterIP, connection refused) ...
06:24:19.182Z    295.6s  [proxy ] zgnr7 ts=2026-10-03T06:24:19Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:24:21.500Z    297.9s  [action] POST resume
06:24:21.906Z    298.3s  [action]   -> 202
06:24:21.989Z    298.4s  [proxy ] 2hg4b ts=2026-10-03T06:24:21Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:24:22.656Z    299.1s  [api   ] status=config_restart
06:24:22.854Z    299.3s  [pods  ] red-db09sr2tm2ss7389qmqg-0/eb2f31 ip=- phase=Pending ready=False rev=b549b5
06:24:23.376Z    299.8s  [proxy ] zgnr7 ts=2026-10-03T06:24:23Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 8 more identical proxy dial failures (ClusterIP, connection refused) ...
06:24:42.966Z    319.4s  [proxy ] 2hg4b ts=2026-10-03T06:24:42Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:24:43.307Z    319.7s  [pods  ] red-db09sr2tm2ss7389qmqg-0/eb2f31 ip=10.244.34.174 phase=Pending ready=False rev=b549b5
06:24:43.655Z    320.1s  [endpts] ready=[] notReady=['10.244.34.174']
06:24:44.349Z    320.8s  [proxy ] zgnr7 ts=2026-10-03T06:24:44Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
    ... 3 more identical proxy dial failures (ClusterIP, connection refused) ...
06:24:52.739Z    329.2s  [proxy ] zgnr7 ts=2026-10-03T06:24:52Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:24:55.191Z    331.6s  [pods  ] red-db09sr2tm2ss7389qmqg-0/eb2f31 ip=10.244.34.174 phase=Running ready=False rev=b549b5
06:24:55.546Z    332.0s  [proxy ] 2hg4b ts=2026-10-03T06:24:55Z dial backend err=dial tcp 10.106.180.168:6380: connect: connection refused
06:24:56.780Z    333.2s  [endpts] ready=['10.244.34.174'] notReady=[]
06:24:56.984Z    333.4s  [api   ] status=available
06:24:57.460Z    333.9s  [client] SERVING
06:24:57.636Z    334.1s  [pods  ] red-db09sr2tm2ss7389qmqg-0/eb2f31 ip=10.244.34.174 phase=Running ready=True rev=b549b5
06:25:38.911Z    375.3s  [setup ] delete red-db09sr2tm2ss7389qmqg
06:25:39.234Z    375.6s  [setup ] delete -> 204
06:25:44.489Z    380.9s  [setup ] GET after delete -> 404
```
