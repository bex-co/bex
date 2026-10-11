# m200 verification: mixed stdout/stderr pod-log records

All times are UTC, 2026-10-11. **Status: implementation complete and proven on a real kubelet. Production is unchanged until a reviewed snapshot bake and node roll.**

## Producer remedy (t001)

The producer is Kubernetes **v1.34.9** (`release.conf`, `bex-worker.pkr.hcl`, `cluster.yaml` pins), commit `ad7c7374b74c04d07ea041d367ecb1a526bdf758`, `staging/src/k8s.io/cri-client/pkg/logs/logs.go` `ReadLogs`. Upstream `master` (2026-10-11) still writes each partial chunk straight through, so no newer release fixes this.

**Patch:** `infra/packer/log-record-framing/kubelet-cri-records.patch`. It adds a `recordAssembler` that keeps one pending record per stream:

- It holds the first chunk's time and payload, then writes the whole record with that timestamp when the stream's `F` chunk arrives.
- It holds at most 1 MiB per stream. Past that, the record streams on as before, but is still since-filtered by its first chunk.
- It flushes an unterminated record when a finite read stops, when the tail limit is reached, or when an exited container's follow ends.
- It skips a record whose first chunk is before `sinceTime`.
- `LimitBytes` truncation is unchanged.

**Unit tests** (in the patch; the Docker build runs them before building the binary):

- the interleaved replay, both finite and follow
- split streams
- literal timestamp-looking content
- an empty `F` line
- an empty first `P` chunk
- since-by-first-chunk
- the pending bound
- unterminated flush
- over-budget plus since

All upstream `pkg/logs` tests also pass. Run against the unpatched reader, the interleaved test fails: it shows the joined prefix and the orphan suffix.

**Real pod API (kind, isolated `m200-cri` cluster, deleted afterwards).** The fixture pod `python:3.13-alpine` wrote:

- short stdout and short stderr lines
- a 48,059-byte stdout-only JSON line, written in two parts 180 ms apart
- a 48,061-byte stdout JSON line whose two parts surround a 92-byte stderr JSON line

The CRI file held `P,P,F` for the solo line and `P,P,stderr F,F` for the mixed one. The same file was read with `kubectl logs --timestamps`:

| kubelet | message bytes | solo intact | mixed intact |
| --- | --- | --- | --- |
| stock v1.34.0 | 17, 17, 48059, **32860, 15293**, 12 | yes | **no** (32,768 + 92 joined, then the orphan suffix) |
| patched v1.34.9 (arm64 `42d34fb18a02…`) | 17, 17, 48059, **92, 48061**, 12 | yes | **yes** |
| patched, `-f` follow of a fresh pod | 17, 17, 48059, 92, 48061, 12 | yes | yes |

Other checks on the patched kubelet:

- Without `--timestamps`, the same complete records come back.
- `--limit-bytes=100` returns 100 bytes.
- `--tail=2` starts at the physical line holding the mixed record's last chunk, so it returns that 15,293-byte suffix. This is the existing physical-line tail behavior, documented in ADR010, and it is unchanged by the patch.
- `--since-time` is second-granular in the API, so the nanosecond-boundary case is covered by unit tests only.

## Artifact and node-image integration (t002, partial)

**Build recipe:** `infra/packer/log-record-framing/Dockerfile`.

- Pinned `golang:1.25.11-bookworm` digest, cross gcc, upstream commit plus the patch, and tests run before the build.
- It keeps `GitVersion` at `v1.34.9` with `GitTreeState:"dirty"`, so bootstrap's `kubelet --version` pin still matches.
- The build is reproducible. Two clean amd64 builds both gave sha256 `0857047b00faf0c9c56d19450f181c5d387d2a6e02f12d852a50fa56ced13809`.

**Publishing:** `.github/workflows/kubelet-artifact.yml`.

- It refuses any bytes other than the Packer pin, treats an existing tag as immutable, and passes `--latest=false` so `bex-cli/v0.3.2` stays the latest release.
- Run 38170126323 succeeded and created release `bex-kubelet/v1.34.9-cri-records.1` with `kubelet-linux-amd64` and its `.sha256`.

**Packer recipe:** `infra/packer/bex-worker.pkr.hcl`.

- It downloads the asset, checks it against `kubelet_amd64_sha256`, and installs it over the held package binary. It then re-checks `kubelet --version`.
- The snapshot name and the `bex.co/kubelet` label carry the sha256 prefix.
- `packer fmt -check` and `validate -syntax-only` pass.

**Guard:** `scripts/clusterapi-validate.sh` now fails if the pinned install step, the patch file or the version alignment drifts. A negative test (Dockerfile at v1.34.8) failed as expected, and the validator passes on the current tree.

**Not done, gated:**

- step 3: verify the baked and non-baked install paths and a node rollover
- step 4: canary worker provisioning and fixture service
- bootstrap sha256 assertion and rollback record

These steps need a `snapshot.yml` bake, which waits on the `production-snapshot` environment approval, and then a production node roll.

## Consumer audit (t003)

- **Code paths:**
  - Both pod-log factories are `logs.NewPodLogSource` and `logs.NewPodLogStream` (`internal/logs/podlogs.go`, wired once in `cmd/api/main.go`).
  - The finite reader serves datastore collection, the App wrapper and pre-deploy reads.
  - The follower serves App and build tails.
  - The Postgres and Key Value fallback shims reuse the same finite source.
- **No bex code change is needed:**
  - Every reader parses one `Timestamps:true` line per record.
  - The finite read sorts by timestamp, so a stderr record that completed first, with a later stamp, is ordered correctly.
  - The follow's held-unstamped-first-line rule now only meets the kubelet's own answers.
- **Test:** the backend test `TestPodReadersKeepInterleavedRecordsWhole` passes the patched producer's exact output shape (stderr first with the later stamp, then the 48 KB stdout record) through `Service.Logs` and `streamContainerLogs`, and both keep each record byte-exact.
- **Unchanged:** authorization, instance selection, quotas and the refusal classes. No product code changed.
- **Not exercised:**
  - CRLF and identical repeated payloads at the producer
  - non-web resource kinds
  - WebSocket, CLI and MCP
  - the `Stream` split option in a cluster (alpha, off), which is covered only by unit tests

## Simplify and tests (t006, t007)

**Simplify** ran three reviews. Fixes applied:

- since-filtering of over-budget continuations
- first-chunk time when the first `P` is empty
- the workflow reuses the validator's `pk_var` parser, takes version notes from the tag, triggers on the pin file and keeps tags immutable
- derived version parts, and shorter comments

**Suites:**

- patched `pkg/logs` passes, in the build and locally
- `lego/backend` `internal/logs` passes, plus `ship-gates` (real-dependency backend suite)
- `make lint`: 0 issues
- `clusterapi-validate.sh` and `github-actions-validate.sh` pass
- the Packer syntax check passes

**Follow-up:** `scripts (test)` turned red on 1f3d18dd3 because the image-pin guard misread `FROM --platform=` lines from m199's and m200's recipes. Fix 7a93270cf strips the flag, adds a self-test case that fails on the old guard, and inlines the Alloy build image digest.

**Not done:** the zero-Loki live-path check through a real local stack (t007 step 3). The live path's code is unchanged.

## Remaining (gated)

1. Approve the `production-snapshot` environment for a `snapshot.yml` bake.
2. Canary one worker, then roll workers and the control plane per `infra/packer/README.md`, and add the bootstrap sha256 assertion.
3. Run the live DoD: the a179 fixture over SSE, REST and GraphQL, desktop and 390×844, and the m96 cron controls (t004/t005/t008).

m199's first-chunk history stamps are deployed and verified (w4/done/m199), which satisfies the combined-identity dependency.
