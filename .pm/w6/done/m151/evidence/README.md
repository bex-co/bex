# w6/m151 user-namespace canary — production, 2026-10-21

Deploy run for 4b6687b5b set `BEX_RENDER_V1_USER_NAMESPACES=tea-d98210cbbpdc73dcrkvg` (the `bex` workspace).

- **Probe pod:** the multi-port probe (`srv-dbcbqup59hnc73e5hn80`, render-v1) reads:
  - `uid_map 0 2104885248 65536`, a pod user namespace rather than the host map;
  - still `CapBnd 00000000000400cb`, `NoNewPrivs 1` and `Seccomp 2`;
  - `io_uring_setup: ok` and non-root `unshare -Ur: ok`.

  Files: `userns-probe-report.json`.
- **Disk-backed ClickHouse:** unchanged ClickHouse (`srv-dbcbqvcekfmc73cr36a0`) with its 10 GB `hcloud-volumes` disk at `/var/lib/clickhouse` went Live under `hostUsers: false`. A non-idmappable volume fails such a pod closed at container start, so this proves Hetzner volumes accept idmapped mounts.
- **Data across a restart:** a native-protocol peer wrote the sentinel `m151-t003` and read it back after a Restart (`userns-sentinel-*.log`).
- **Not run:** snapshot/restore. The API has no on-demand snapshot, so it needs the first nightly `dskbak-` run on a userns disk. The local spike already proved a non-userns snapshot Job reads and restores a userns-written disk (unshifted UIDs; `docs/render-artifacts/container-privileges.md` § bex user namespaces). Tracked as w6/089.
- **Cleanup:** every fixture was deleted (REST 404).

## Final check — customer workspace `bex-canary`, after the `*` widen (f64d4c85f, run 39609797497)

- **Probe:** the render-v1 probe (`srv-dbcd1ssiimkc73f6l0hg`) reads `uid_map 0 2016935936 65536`, `CapBnd 00000000000400cb`, `NoNewPrivs 1` and `Seccomp 2`, with `io_uring_setup: ok` and non-root `unshare -Ur: ok` (`customer/probe-report.json`).
- **ClickHouse:** unchanged ClickHouse (`srv-dbcd1t4iimkc73f6l0m0`, 10 GB disk) is Live in a pod user namespace. A native peer reads `SELECT 1 = 1` and the sentinel `m151-customer` (`customer/sentinel.log`).
- **Kept for w6/089:** ClickHouse and its sentinel stay overnight so the first nightly snapshot can be restored and checked. The probe, consumer and peer were deleted.
