# w6 · m158 — Platform build in minutes on the existing runners: native compile, cached and parallel image builds

**Worker:** worker6 **Goal:** the `build` job of `deploy.yml` finishes in ≤ 30 min (from 60–187 min) on the existing self-hosted runners, so a green main reaches production in about an hour. **Status:** todo (t001, t002, t004 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Cross-compile the Go/Node build stages from `$BUILDPLATFORM`](done/t001.md) — **DONE** | 45m | — |
| t002 | [Registry layer cache and cache mounts for every platform image](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Build the five images, and sign + SBOM them, in parallel](t003.md) | 90m | t002 |
| t004 | [Start the build with the tests; gate only the pin on them](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Measure five main deploys against the baseline](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- No compiler (Go, Node/TypeScript) runs under amd64 emulation in `deploy.yml`'s `build` job; only target-platform runtime stages (`apt-get`, copying binaries) still do.
- Over 5 consecutive successful main deploys, `build` takes ≤ 30 min each (median and max reported from `gh run view --json jobs`), against the 2026-11-03/04 baseline of 61.6, 136.0 and 186.9 min (runs 41705171927, 41870722101, 41816119204).
- A deploy whose image inputs did not change rebuilds those images from cache (cache hits visible in the build log); only main can write the cache; `scripts/github-actions-validate.sh` passes.

## Source + Goal linkage

- **Source:** user discussion 2026-11-04 ("讨论一下怎么给 bex platform build 提速"). Baseline step timings: operator image 13–16 min, agent sandbox image 9–93 min, sign + SBOM 12–25 min, all serial, on ARM64 runners (this Mac Studio's Docker Desktop) emulating `linux/amd64`, with no `cache-from`/`cache-to` and a fresh buildx builder per run.
- **Decision 2026-11-04 (user):** keep all CI on the local Mac Studio runners — cheaper than moving to Hetzner. A native amd64 Hetzner runner (`bex-build-amd64-1`, cx43) was provisioned and deleted the same day, unused. Moving CI to the cloud (~€150/month for 2× cax41 + cx43/cx53 + cax31) is not planned; reopen only on a user decision.
- **Goal linkage:** ADR008 reliable delivery — finished work reaches production promptly, which every live-verification DoD depends on.
- **Expected outcome:** a deploy run takes about an hour instead of 1.5–4 h, and fewer runs are superseded while they build (w6/m156).
- **Why now:** w6/m156's replay showed six fully built runs discarded in four 11–25 h gaps because a run takes 1–3 h; shortening the run shrinks that window directly, alongside m156's allowlist (D8 = (a)).
- **Render parity:** omitted. CI/deploy infrastructure, no REST/GraphQL/MCP/UI surface.

## Measurements and follow-ups — 2026-11-05

- **Run 41993520985** (`c8bbc07cb`, after Docker Desktop went to 24 CPU / 72 GB and the two fixes below): tests 6–14 min in parallel, `build` **38.0 min** (images 10.5 / 3.2 / 1.2 / 4.4 / 7.2 min, sign + SBOM 7.0, CVE scans ~1), deploy 8.4 min; push → production ~60 min against the 1.5–4 h baseline. `build` still ran on `bex-production` with a 4-CPU / 512-PID runner.
- **Fixes shipped on the way:** `f258c8a49` (CVE scans reuse the installed Trivy: ~8 min and ten flaky downloads per run), `c8bbc07cb` (`GOMAXPROCS=4` in the Go builder stages: with 24 visible CPUs `go build` exhausted the runner's 512-PID cap), `7d5b65710` (the Helm chart mirror skips upstream downloads for charts already published at their reviewed digest).
- **Follow-ups for this milestone** (from the reviews of those fixes):
  - Move `deploy.yml`'s `build` job to the `bex-build` pool and raise that pool's runner limits (e.g. 12 CPU / 24 GB) in `block-eden-mono/projects/github-runner/docker-compose.yml`; raise every pool's `pids: 512` (e.g. 4096): DinD's dockerd, buildkitd and the build share it, and `bex-ci`'s `make test` / `make lint` and `bex-build`'s log-shipper / hcloud-csi builds run uncapped Go parallelism under the same limit.
  - Persist the Trivy DB and binary across runs (per-pool cache volume), or start the DB download in the background at job start: 2–6 min plus the last flaky download per run. Keep the CVE scans in one job if t003 parallelises the image builds.
  - Merge `lego/Dockerfile`'s 16 sequential `go build` steps into ~3 invocations (operator plain flags with explicit packages, the `-trimpath` pair, backend) so links run in parallel; do not use `./cmd/...` (`registry-migrate` is not shipped).
  - `scripts/helm-artifact.sh mirror`: make the skip check anonymous so it also proves public pullability, and run the full anonymous pull + sha256 only after a fresh push; then parallelise the 16 charts (~1.6 min → seconds).

## Runner resize and CI sweep — 2026-11-05

- **Runners (block-eden-mono `3e43b7c68d`, `7d3a2c28ef`, `20569fcad8`):** `bex-production` and `bex-build` runners now have 12 CPU / 24 GB, every runner 4096 PIDs (was 512), and the runner image ships PyYAML. All 12 were replaced idle-only. `deploy.yml`'s `build` stays on `bex-production` (ADR019 §Decision 5: a deploy never queues behind `bex-build`'s image builds), so it now gets 12 CPUs without changing pools. The "move `build` to `bex-build`" follow-up above is superseded by this.
- **bex fixes shipped:** `fcd2330f7` (gVisor fixture seds), `3505a0a39` (kubelet back on Go 1.25.11 + `.go-version` assertion), `c4306d521` (Syft installer with retries), `eb25fe5f3` + `00170e192` + `d2b00c61e` (isolation matrix: paid probe targets, sandbox leg finally runs with `setup-go`, stale session-egress fixture), `acde09993` (operand extension order; PG 16–18 crash filed as w8/234), `f87e8184d` (main checks no longer cancelled by the next push; docs/govulncheck/go-lint get concurrency groups; repo-artifact-guard keeps every pending run), `1e55daf76` / `ea495e230` / `03e850853` / `405cbefa6` (retries for upstream 5xx, `gh` calls in every scheduled monitor, Go module downloads).
- **State after the sweep:** every main workflow's latest completed run is green except `pg-operand-image` (w8/m92 upstream gate for PG 13–15; w8/234 for 16–18) and `render-schema-drift` (fix on main, next scheduled run 11-09).
- **Remaining speed levers for t001–t004:** pass `GOMAXPROCS` as a build-arg (an `ARG` defaulting to 4 in `lego/Dockerfile` and the controller Dockerfile) set to 12 in `deploy.yml`; `FROM --platform=$BUILDPLATFORM` for the Go builder stages (fix `govulncheck.yml`'s `awk '$2 ~ /^golang:/'` image derivation in the same change); registry layer cache; parallel image builds.
- **Optional follow-up:** one `scripts/lib/tracking-issue.sh open|close <label>` helper to replace the ~20 copies of the label/list/edit-or-create/close sequence, re-listing before retrying `gh issue create` so a lost response cannot open a duplicate.

## Network-bound CI and runner caches — 2026-11-06

- **Finding:** at ~08:20Z the host's link delivered ~1 MB/s in total (GitHub release download 68 KB/s, Cloudflare speed test 160 KB/s), shared by every runner. `bex-ci` sat 7/7 busy at ~0 % CPU on `git clone` (32 min), `docker pull valkey` and a GitHub release `curl` (10 min); the deploy slot holder's 1-minute `check-supersession` waited 49 min for a runner (the fixed watchdog reported it). Build optimisations cannot show while this holds; t005's measurement waits for a normal link.
- **block-eden-mono `47827d2027`:** each pool gets a Go module cache volume (`/home/docker/go/pkg/mod`) and its own `registry:3` pull-through Docker Hub cache (`registry-mirror-<pool>`, push refused); the nested Docker Engine starts with `--registry-mirror`. Verified on a replaced `bex-build` runner: `alpine:3.20` was served and cached by `registry-mirror-build`. All runners are being replaced idle-only.
- **Shipped in this run:** t003 part 1 `056dc0796` (sign/SBOM/attest concurrently; also fixes an SBOM failure the old pool could lose). Part 2, building the five images concurrently, waits for t005's numbers.
