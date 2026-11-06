# w9 · m173 — CI stability: deploys stop stalling on an overloaded self-hosted runner host

**Worker:** worker9 **Goal:** main deploys land within one run of a push again; image builds, SBOM and CVE steps finish inside their budgets on the existing runner host. **Status:** in progress — t001–t006, t008, t009 done; t007 closes after 3 consecutive main deploys pass the sign/SBOM step and the bounded buildkit pre-pull (watch #129 close)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Diagnose the 2026-11-05 deploy failures and measure the runner host — **DONE** | 45m | — |
| t002 | Bound build concurrency and thread use — **DONE** | 60m | t001 |
| t003 | SBOM and CVE steps reuse work and fit their budgets — **DONE** | 60m | t001 |
| t004 | Deploy latency watchdog reports stalls early — **DONE** | 30m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t005 |
| t008 | Bound the buildkit pre-pull — **DONE** | 15m | t006 |
| t009 | Bound every shared-runner job — **DONE** | 30m | t006 |
| t007 | Closeout | 10m | t006, t008, t009 |

## Definition of done

- No deploy fails from thread exhaustion or SBOM timeouts across 3 consecutive main pushes; an unchanged image skips SBOM regeneration; a deploy gap over the threshold raises an alert.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** platform reliability: fixes reach customers; ADR008 operations.
- **Expected outcome:** pushed fixes deploy within one run instead of waiting hours (m168 waited ~10 h).
- **Why now:** repeated stalls 2026-11-04/05 delayed production fixes; other sessions are already patching symptoms piecemeal.
- **Parity:** omitted — CI/infra only, no tenant surface.

## Findings and changes (2026-11-06, w9 loopx fork)

### t001: cause table

Deploy failures since 2026-11-04 (`gh run view --log-failed`):

| Run | Failing step | Cause |
| --- | --- | --- |
| 41966431280 | CVE scan, dashboard (gate) | Trivy re-downloaded per scan; TLS reset. Fixed by a teammate (`skip-setup-trivy`). |
| 41973364840 | every test job, no step result | Runner dropped mid-job (infra). |
| 41987595478 | docker build | `failed to create new OS thread errno=11`: Go sized threads from host CPUs past the runner's PID cap (512). Fixed in c8bbc07cb (`GOMAXPROCS=4` in `lego/Dockerfile` and the opensandbox controller). No recurrence since. |
| 42004686414, 42082648709, 42132765064 | sign images + attest SBOM | syft timed out on bex-agent-sandbox, and in 42132765064 on bex-operator too. **Root cause: 8b68cbcf1 (2026-11-04 20:12)** wrapped syft in the same 3×300 s retry as cosign, under a 30-minute step cap. In the last green run (41942865946) syft took about 5 min for the operator and about 9 min for the agent sandbox, with no timeout. Every SBOM failure since is this. |
| 42049601038 | docker build | `getaddrinfo EAI_AGAIN repo.yarnpkg.com`: transient DNS on the runner (flake). |
| 42069264215 | `make test` (operator) | Test failure, owned by other sessions. |
| 42116484997, 42120006429, 42121466087, 42123929461 | dashboard typecheck/lint/test | Code red, fixed by other sessions. |

**Runner shape.** `bex-production` is a self-hosted ARM64 runner building linux/amd64 images, with a 4-CPU and 512-PID cap per runner container (per the `lego/Dockerfile` comment). Both #CI-RUNNERS and #RUNNER-HOSTS hold: nothing moves off this host.

### t002: already met

Go build parallelism is capped by c8bbc07cb: `ENV GOMAXPROCS=4` in `lego/Dockerfile` (every operator binary) and in `deploy/opensandbox/controller.Dockerfile`. The agent-image copies a Go toolchain but runs no `go build`. No errno=11 has occurred since.

### t003: fixed in `.github/workflows/deploy.yml` "sign images + attest SBOM"

**Changes**
- `retry LIMIT ATTEMPTS`: cosign calls keep 300 s × 3. syft gets 1200 s × 2.
- A digest that already carries this workflow's spdxjson attestation is skipped (`cosign verify-attestation` with the `deploy.yml` certificate identity regex and the GitHub OIDC issuer). This covers re-runs of the same build.
- SBOMs run two at a time, so the slowest image sets the wall clock (about 9 min instead of the roughly 20 min sum), inside the 4-CPU and 512-PID caps. Attestations then run sequentially.
- The step timeout is 45 min.

The CVE gate is unchanged.

**Local validation**
- `bash -n` and shellcheck on the extracted script.
- A fake-tool harness (cosign, syft and a timeout shim):
  - 5 refs with 1 already attested give 4 SBOMs and 4 attests, exit 0;
  - a failing SBOM exits 1 with no attests;
  - 4 SBOMs of 2 s each finish in 5 s wall time, which proves 2-way parallelism.
- `scripts/github-actions-validate.sh` passes. YAML parses (actionlint is not installed locally).

### t004: already met

`deploy-latency.yml` (w6/m156) runs hourly with a 6 h threshold and opened issues #122, #123 and #125 on the gaps, closing them on recovery. #129 is open now: main is not reaching production because of the SBOM timeout this milestone fixes.

### Proven only by real deploys

- The next deploy runs must finish the SBOM step (expected about 12–15 min), and #129 must close.
- The DoD's "3 consecutive pushes" still needs observing.
- A re-run of a failed build (same digests) should log `SBOM attestation already present … skipping`.

## Simplify and tests (2026-11-05)

- **Simplify (t005).** I reviewed the `deploy.yml` diff directly. `retry` now takes the per-call limit and attempt count, so syft no longer inherits cosign's 300 s. Running the SBOMs two at a time reaps finished jobs with `wait -n` and `kill -0`, so it never waits on a pid that has already been reaped. No further findings.
- **Tests (t006).** The extracted script passes `bash -n` and shellcheck, and a simulation with fake cosign, syft and timeout covers three cases:
  - an already-attested digest is skipped;
  - one SBOM failure fails the step with no attest;
  - the SBOMs run two at a time.

  `scripts/github-actions-validate.sh` passes.

## Observation (2026-11-06 07:55 UTC): shared `bex-ci` runners starved by another repo

- The deploy of 3310f6484 (run 42176650307) sat `queued` from 06:47 because all seven online `bex-ci` org runners were busy.
- Most were held by `bex-co/beancount-cms-v2` jobs carrying the same `bex-ci` label:
  - `lighthouse` (run 42162745684), on runner 0eafd29d58ea since 05:05, so about 3 h and likely hung;
  - `build`, `submit` and `gitleaks`.
- bex's deploy-gating test jobs queue behind them.
- This worker did not cancel another project's jobs. The options are for the owner:
  - a job `timeout-minutes` in that repo;
  - a separate label for it.
- Neither is a runner host split (`DO_NOT_DO` #RUNNER-HOSTS). The t007 three-green-deploys check waits on this.
- **Resolved 2026-11-06 08:05 UTC (user decision: cancel only the hung run).**
  - beancount-cms-v2 run 42162745684 was cancelled. Its `lighthouse` job had been stuck in "Production browser smoke" since 05:47, and the job has no `timeout-minutes`, so GitHub would only kill it at 6 h.
  - That repo's other jobs were left alone. bex deploy 42176650307's test jobs started within minutes.
  - The repo's owner should give `lighthouse` (`ci.yml:327`) a `timeout-minutes`. That is not done from w9.
- **No pin from 07:15 to at least 10:35 UTC (2026-11-06).**
  - Run 42176650307 (3310f6484) finished `success` with `build`/`deploy` skipped. Its pre-build `check-supersession` saw newer manifests (056dc079), which is the designed w4/439 path: the queued run builds the newer tree.
  - The gap comes from duration. Runner contention, described above, stretched the run's tests to about 2.5 h, so any manifest commit in that window skips the build.
  - Run 42196880498 (5f5af11fc) is next.
  - For t007: if this repeats with runners free, revisit the pre-build skip on exit 4. For example, build anyway when the last pin is more than 2 h old, so a manifest burst cannot keep production unpinned.
