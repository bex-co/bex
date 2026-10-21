# w6/m149 live acceptance — production, 2026-10-21

- **Deploys:**
  - run 39577962500 (a3f66521f) shipped `render-v1`, the bex seccomp profile and the user-namespace switch (off);
  - run 39590658584 (69f81c234) widened `BEX_IMAGE_COMPATIBILITY_WORKSPACES` to `*`.
- **QA user:** sessions were revoked after use.

## t009 — canary in `bex` (`tea-d98210cbbpdc73dcrkvg`)

- **Probe:** the multi-port probe pod (`srv-dbc9ottee68s73aifemg`, render-v1) reads `CapBnd 00000000000400cb` (Render's six), `NoNewPrivs 1` and `Seccomp 2`.
  - `io_uring_setup: ok` and non-root `unshare -Ur: ok`. RuntimeDefault blocks both, so the bex Localhost profile is in effect.
  - `uid_map` is the host map (userns switch off).
  - Files: `canary-probe-report.json`.
- **ClickHouse:** unchanged `render-examples/clickhouse` (`srv-dbc9ousc7gps73f83dn0`) is Live, with `openPorts` 8123/9000/9009. A peer reads `PING=Ok.` and `SELECT1=1`. Files: `canary-clickhouse-*`.

## t010 — customer workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`, never on the old allowlist)

- **DoD 2, unchanged ClickHouse:**
  - `srv-dbcat41akels738t2kr0` (slug `clickhouse-wfi3`; the global slug was taken) went Live as `render-v1`, with `openPorts` 8123/9000/9009.
  - Its first deploy failed with the w6/080 diagnosis, "no node currently has room… not enough cpu, memory", while the bex canary held the capacity. It went Live on the redeploy after that capacity was freed.
  - A native-protocol peer (`clickhouse-driver`, port 9000) reads `SELECT 1 = 1`, writes the sentinel `m149-t010`, and reads it back after a Restart and after a manual redeploy. Files: `t010-sentinel-*.log`.
- **DoD 3, running pod:**
  - The render-v1 probe (`srv-dbcat49akels738t2kv0`) reads `CapBnd`/`CapEff 00000000000400cb`, `NoNewPrivs 1` and `Seccomp 2`.
  - `uid_map` is `0 0 4294967295`: the userns switch covers only `bex` (w6/m151).
  - Files: `t010-probe.log`.
- **DoD 4, unchanged `nginx:1.27`** on port 80 (root master, `setuid` workers): `curl -sSI https://qa-20261021-nginx.onbex.co/` → `HTTP/2 200`. Files: `t010-nginx-head.txt`.
- **DoD 5, legacy:** pre-existing services' last deploys predate the widen (`t010-legacy-deploys.txt`), so nothing rolled. **Not run live:** the operator-run migration (`/api migrate-container-policy` inside the bex-api pod), which needs production cluster access (w6/088). It is covered by `TestMigrateContainerPolicy*`.
- **DoD 6, isolation:** from the render-v1 probe, another workspace's pod (`multi-port.tea-d98210…:8123`), `bex-api.bex-system:8090`, `169.254.169.254:80` and the node gateway's kubelet `:10250` are all unreachable (`t010-probe.log`). **Not run live:** admission dry-runs on the production API server (w6/088); the envtest `Application container policy admission` covers them on a real API server.

## Cleanup

Every fixture was deleted by exact ID, each returning REST 404, and the ClickHouse Blueprint `blp-dbcat41akels738t2kt0` was deleted.
