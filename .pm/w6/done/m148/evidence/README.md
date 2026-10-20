# w6/m148 live acceptance — production, 2026-10-20

These checks ran in production (`api.bex.co`, `dashboard.bex.co`) as the QA user. The m148 platform code is commit `94533d795`, live since 05:43Z. The fixtures were built from `bex-co/bex@45fd398e4`, `examples/multi-port`.

- **Customer workspace:** `bex-canary` (`tea-daif693dqjvc73e7as3g`). It is not in `BEX_IMAGE_COMPATIBILITY_WORKSPACES` (`lego/operator/config/prod/kustomization.yaml`).
- **Allowlisted workspace:** `bex` (`tea-d98210cbbpdc73dcrkvg`).

The `dev-6` pass the task names first was skipped: production had already been running m148 since 05:43Z, and the local CAPD refresh failed on a Keychain-backed Helm OCI fetch. Production is the stronger proof.

## DoD 1 — customer workspace, strict policy

- The Blueprint was applied at 06:16:37Z (`multi-port-blueprint-deploy.json`).
  - Private service `multi-port` `srv-dbbgehr581vs73bjhi4g`, no port configured. Its first deploy `dep-dbbgehr581vs73bjhi50` was live at 06:19:15Z.
  - Consumer `multi-port-consumer` `srv-dbbgehr581vs73bjhi60`.
- **The running pod drops `ALL` and adds nothing.** The server's own `/proc/self/status` was read from inside the pod and relayed by the consumer (`multi-port-consumer-report.json`):
  - `CapEff: 0000000000000000`, `CapBnd: 0000000000000000`;
  - `NoNewPrivs: 1`, `Seccomp: 2`;
  - `uid_map: 0 0 4294967295`.
- **The same-workspace web consumer reaches both ports through the stable hostname.** It gets `http` = `multi-port http 8123` and `tcp` = `multi-port tcp 9000`.

## DoD 2 — read surfaces agree

REST (`multi-port-rest.json`), GraphQL `server(id){openPorts port internalAddress}` (`multi-port-graphql.json`) and MCP `get_service` (`multi-port-mcp.json`) all return:

- `openPorts == [{"port":8123,"protocol":"TCP"},{"port":9000,"protocol":"TCP"}]`
- `port == 8123`
- `internalAddress == "multi-port:8123"`

The dashboard header shows `服务地址 multi-port:8123` and `开放端口 8123 主端口 · 9000` (`.playwright-mcp/m148-multi-port-header.png`, local only).

## DoD 3 — references follow the real port

- **Image-port target.**
  - First apply: the target's port was not yet known. As ADR089 port decision 4 specifies, `API_HOSTPORT` and `API_PORT` were omitted (empty in the first revision), never `3000`.
  - At 06:19:04Z, bex-api's propagator wrote `multi-port:8123` and `8123` and opened `config_change` deploy `dep-dbbgfm3581vs73bjhi80`, which was live at 06:33:59Z.
  - The running consumer then reports `API_HOSTPORT=multi-port:8123` and `API_PORT=8123` (`multi-port-consumer-report.json`).
- **Configured-port target.** The second Blueprint (`port-change-blueprint-deploy.json`) has two services:
  - web target `qa-20261020-m148-target` `srv-dbbgng19a20c7385rar0`, port 3000;
  - worker `qa-20261020-m148-probe` `srv-dbbgng19a20c7385rasg`, with `fromService` `hostport` and `port`.

  On apply, the probe held `qa-20261020-m148-target:3000` and `3000`.
  - At 07:00:10Z, `setPort(8080)` (`port-change-setport.json`) was called.
  - By 07:00:21Z the probe held `qa-20261020-m148-target:8080` and `8080` (`port-change-probe-env.json`), and `config_change` deploy `dep-dbbh2vucbnls73997bog` had opened (`port-change-deploys.txt`).
  - On `main` before m148, this case resolved to 3000 forever.

## DoD 4 — isolation and legacy

- **Cross-workspace probe is denied.** Probe: `qa-20261020-m148-xweb` `srv-dbbhb29q56vc73bn8bbg`, in the `bex` workspace (`cross-workspace-probe.log`).
  - `multi-port.tea-daif693dqjvc73e7as3g.svc.cluster.local` resolves to ClusterIP `10.107.181.59`.
  - HTTP 8123 times out (`wget: download timed out`) and TCP 9000 hangs until `timeout` kills `nc` (`punt!`), on every 20s cycle. The connection is dropped by policy, not refused.
- **No public Ingress.** REST `url` is empty. `https://multi-port.onbex.co/` and `https://srv-dbbgehr581vs73bjhi4g.onbex.co/` both return 404.
- **Legacy is unchanged, but this rests on code and tests, not a live pre-upgrade private service.** None of the QA user's three workspaces holds a private service created before 05:43Z, so no live case was available.
  - m148 changed no operator projection for `portMode ''`. The operator diff is `rolloutStallDiagnosis` (status text only) and the explicit-primary `< 1` guard (image modes only).
  - bex-api's propagator touches only Apps carrying `app.bex.co/has-service-refs`, a label stamped by a post-m148 Blueprint apply, so pre-upgrade consumers are never patched.
  - Pinned by `TestLegacyPrivatePortKeepsConfiguredContract`, `TestBlueprintSyncPreservesPortMode` and the operator image-network tests.

## DoD 5 — unchanged ClickHouse in `bex`

- `render-examples/clickhouse@355817b1d95c1a9f2b088474437593234f4027f4`, byte-for-byte.
  - Blueprint `blp-dbbg0k99a20c7385r9pg`, service `clickhouse` `srv-dbbg0k99a20c7385r9o0`.
  - Deploy `dep-dbbg0k99a20c7385r9og` live at 05:49Z (`clickhouse-deploys.json`).
- `openPorts` = 8123/9000/9009, `port` 8123, `internalAddress` `clickhouse:8123`, disk `/var/lib/clickhouse` 10 GB (`clickhouse-rest.json`).
- Peer worker `srv-dbbg1qh9a20c7385r9rg` logs `PING=Ok.` and `SELECT1=1` every 20s, from 05:5x until cleanup (`clickhouse-peer.log`).
- **Image digest not recorded.** The Dockerfile is `FROM yandex/clickhouse-server:21`. Neither REST nor GraphQL exposes the built image digest, and the build log was not retrievable through `/v1/logs`.

## Observations filed separately

- From about 06:35Z, three new **starter-plan** worker pods never produced a log line in 15 minutes: busybox in both workspaces, then alpine. One failed with only `rollout did not become healthy within the progress deadline`. Free-plan web pods started within seconds over the same window, and existing starter pods kept running.
  - The operator's stall diagnosis has no case for a Pending (unschedulable) or `ContainerCreating` pod, so the user gets no cause. Filed as w6/080.
- The multi-port Blueprint deploy upserted the pre-existing `bex-canary` Blueprint `blp-db136288mmqc73d4hpug` (created 2026-10-04, repo `bex-co/bex`, branch `main`) and replaced its manifest. The Blueprint was left in place; its `path` `render.yaml` does not exist in the repo, so auto-sync cannot re-apply it.

## Cleanup — 2026-10-20 07:3xZ

Every fixture was deleted by exact ID, each `DELETE` returned 204, and a follow-up `GET` returned 404 for all of them:

- `bex-canary`: `srv-dbbgehr581vs73bjhi4g`, `srv-dbbgehr581vs73bjhi60`, `srv-dbbgng19a20c7385rar0`, `srv-dbbgng19a20c7385rasg`;
- `bex`: `srv-dbbg0k99a20c7385r9o0` (its disk no longer listed), `srv-dbbg1qh9a20c7385r9rg`, `srv-dbbh3qmcbnls73997bs0`, `srv-dbbhb29q56vc73bn8bbg`;
- Blueprint `blp-dbbg0k99a20c7385r9pg`.

App CRs were not checked directly, because this pass had no cluster access. Both QA sessions (API jar and browser) were revoked with `qa-login.sh --logout`.
