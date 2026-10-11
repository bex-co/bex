# m199 verification — first-chunk CRI timestamp

All times UTC, 2026-10-11.

## Collector artifact

- **Upstream:** `github.com/grafana/alloy` v1.11.2 = `4a8f93cd1755dd8205c3a9d6c35b136c476cff92` (the chart 1.3.1 default; chart identity and `deploy/helm-artifacts.lock` unchanged). Upstream is still unfixed in v1.19.2 (`cri.go` keeps the current entry's stamp on both merges), so no release could be pinned instead.
- **Patch:** `deploy/log-shipper/patches/cri-first-chunk-timestamp.patch`. On both merge branches (repeated `P` and completing `F`) `keepFirstChunkTime` copies the buffered entry's `Timestamp` and extracted `time`. Concatenation, fingerprint keys, `max_partial_lines` and the size/truncate bounds are untouched. It adds `TestCRI_partialKeepsFirstChunkTime`. Locally this test **fails on unpatched v1.11.2** (got `10:35:01.220000002Z`, want `10:35:00.100000001Z`) and passes patched. The whole `stages` package passes too.
- **Recipe:** `deploy/log-shipper/Dockerfile`, which mirrors upstream (alloy-build-image v0.1.21 digest-pinned, UI build, `netgo builtinassets promtail_journal_enabled`, `RELEASE_BUILD=1`). The patched stage tests run inside the build, and only `/bin/alloy` is swapped into `grafana/alloy:v1.11.2@sha256:6ab34b82…`. Version: `v1.11.2-bex-cri-first-ts`.
- **Publish:** `.github/workflows/log-shipper-image.yml` (path-triggered, signed + SBOM). The first two runs lost the runner mid-build with no log. Fix commit 0acf2187e serialized BuildKit (`max-parallelism = 1`), and run 38143764855 then succeeded. Local amd64 and arm64 builds also succeeded (≈5 min each).
- **Pinned:** `ghcr.io/bex-co/bex-alloy@sha256:d448ad121e332519f3427b4bc0e61241a1c03edd41d60218acb6935ef08b7a0a` (public, linux/amd64 `0fd0a1a4…`, linux/arm64 `1acb5867…`), in `deploy/gitops/base/log-shipper.yaml`. Shipped in d6ebd8d5a. deploy run 38146956613 and gitops run 38146956563 passed; gitops runs the harness against the pinned digest in CI.

## Real-image replay (t001/t002/t005)

The exact finding fixture went through the rendered production `app_logs` in the published arm64 image:

```json
{
  "qa-a177-local-control": {
    "__ts": "2026-10-11T10:35:00.300000003Z",
    "app": "web",
    "container": "app",
    "level": "unknown",
    "namespace": "tenant",
    "pod": "web-1",
    "type": "app"
  },
  "qa-a177-local-first-last": {
    "__ts": "2026-10-11T10:35:00.100000001Z",
    "app": "web",
    "container": "app",
    "level": "unknown",
    "namespace": "tenant",
    "pod": "web-1",
    "type": "app"
  }
}
```

The stock v1.11.2 image stamped `first-last` at 10:35:00.220000002Z, as filed.

`scripts/test_log_shipper.py` → `CriPartialTimestampTest` runs against the real pipelines:

| Pipeline | Cases | Stock v1.11.2 | Pinned |
| --- | --- | --- | --- |
| app_logs, build_logs, keyvalue_logs | P→F (120 ms); single-F control; P,P,F across a second boundary with an interleaved stderr P→F (separate buffer key) | FAIL (final stamps) | pass |
| database_logs | CNPG `msg=record` JSON split across P→F, unwrapped to the PostgreSQL line, `level=info` | FAIL | pass |
| app_logs | 2×120000-byte P + F, still truncated to exactly 200000 bytes, first stamp | FAIL | pass |

Full suite on the pinned image: `test_log_shipper.py` 10/10 OK. That includes the existing out-of-order write-timestamp test, severity/classification, Postgres/KV tables, file-only sources and the platform pipelines. `test_log_delivery.py` 1/1 OK, with `alloy_build_info` now asserting `v1.11.2-bex-cri-first-ts`. `gitops-validate.sh` and `github-actions-validate.sh` PASS. The request/platform pipelines have no `stage.cri` and are unchanged.

Not exercised: the restart-position case (the hostPath positions code is unchanged) and the max-partial-**count** overflow. That overflow branch flushes the buffered entries, which now carry their first stamps, and it is covered by the upstream `TestCRI_tags` cases that passed in the build.

## Shared readers and aliases (t002/t003)

There are no backend or frontend code changes. Every historical alias reads the same Loki records through `parseLokiStreams`:

- generic `GET /v1/logs`, GraphQL `logs` and MCP `list_logs`
- Postgres `GET /v1/postgres/{id}/logs`, `databaseLogs` and `get_postgres_logs`
- Key Value `GET /v1/key-value/{id}/logs`, `keyValueLogs` and `get_key_value_logs`

So they get the corrected stamp from the collector. Live evidence covers REST, GraphQL, SSE and the dashboard for an App web service and a cron job. MCP and the datastore aliases have local pipeline evidence only (Postgres/KV pipelines replayed above), and their live behavior is **unverified**. Render documents generated timestamps and timestamp paging but not its partial-line policy, and that was not probed (ADR018 row note).

The frontend key (`map.ts`: millisecond + instance + exact message) was not changed or coarsened. No mapper regression was added because the fix makes both inputs identical.

## Live DoD (t006)

Session: `qa-login.sh --serve`. Fixtures were owned Free services in workspace `tea-d98210cbbpdc73dcrkvg`.

**Web** `qa-20261011-m199-log-time` / `srv-db5q8pkhj6ks739l0lbg`: `python:3.12-slim` running the a177 quiet command (markers renamed `qa-m199-v1`), created 14:59:50. Ready returned `qa-m199-v1-ready 0`. One `/emit` at 15:00:17.256Z returned `qa-m199-v1-ready 1`.

| Record | Bytes | SSE stamp | REST stamp | GraphQL stamp | SHA-256 prefix (all three) |
| --- | --- | --- | --- | --- | --- |
| short | 47 | 15:00:17.372589379Z | same | same | 0a48bea42715d02f |
| small, delayed | 8,071 | 15:00:17.49283594Z | same | same | 10be6cd39104c38d |
| CRI-partial, delayed | 50,075 | **15:00:17.493331055Z** | **same** | **same** | 21b2852cf4427889 |

All three sources reported the same instance (`srv-db5q8pkhj6ks739l0lbg-8504ahvgq1up63fan8to`). History returned `hasMore:false` and SSE had 0 errors. The partial record's stamp is its first chunk's: it was written right after the small record finished, while its own final chunk came about 120 ms later. In a177 history carried the final chunk's stamp.

Dashboard:

- **1440×1000, `?text=qa-m199-v1-partial-1&live=0`:** 1 row, with the start and end markers in the same row.
- **Live enabled:** 1 row.
- **390×844, freshly reloaded Live:** partial 1 row, small control 1 row, short control 1 row, all `qa-m199-v1` records 3 rows. Document width stayed 390.

**m96 controls**, cron `qa-20261011-m199-cron` / `srv-db5qa54hj6ks739l0lgg`, busybox 1.37, `*/2`:

- **Runs:** scheduled 15:04 and 15:06, manual 15:07:37 and 15:09:28 (UI Trigger Run), and a deferred scheduled run at 15:08:21. All were `successful`.
- **History:** GraphQL `qa-` returned 30 records (`hasMore:false`), with 6 `qa-cron-success` records on 6 distinct instances.
- **SSE against history:** every SSE frame for a still-present run pod (5 frames) matched history on timestamp, instance and message.
- **Framing controls:** the JSON `{"m":"qa-m199-cron-json\nsecond"}` kept its escaped newline. `qa-m199-cron-trail   ` kept its trailing spaces (21 bytes).
- **Logs page row counts:** `qa-cron-success` showed 6 rows with Live off, Live on, after a Live reload, after toggling off, and after toggling on again.
- **Cancel:**
  - After a PATCH to a `sleep 300` command, a UI-triggered manual run (15:12:29) was canceled in the UI at 15:12:54. It became `canceled`, and logs show `qa-m199-long-start` with no `qa-m199-long-finish`.
  - An earlier UI cancel of the deferred `sleep 40` run (15:10:48) also became `canceled`, but its finish marker appeared at 15:10:54. busybox `sh` as PID 1 ignores SIGTERM, and the sleep ended inside the 30 s grace period. This is a fixture artifact, not a regression, and is recorded as observed.

**Cleanup:**

- Both services were deleted (DELETE 204). REST lookups returned 404, the public URL returned 404, and the inventory has no `m199` service.
- `qa-login.sh --logout` revoked only this session (`ok logged-out`). API afterwards returned 401, and the browser held 0 cookies.
- The local jar, state and probe scripts were removed.

## Limits

- Records stored before the pin keep their final-chunk stamps and age out with retention. The collector can't rewrite them.
- w4/256's over-cap message mismatch remains independent.
- w4/blocked/m174's fast-exit loss residual was not exercised and is not closed by this milestone.
- The production kubelet version was not queried.
