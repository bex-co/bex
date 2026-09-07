# m88 CI recovery — 2026-09-05

Why: restore label-addressed CI without mistaking a runner group for a label or declaring shared-host runners isolated.

## Verified starting state

- GitHub listed ten organization runners, all missing `bex-ci` / `bex-production` labels.
- Seven CI and three production containers were already running. Sampled `.runner` metadata confirmed existing org groups 3 (`bex-ci`) and 4 (`bex-production`) with ephemeral registration.
- Both groups used the same Docker Desktop engine and npm/yarn/pnpm cache volumes. The earlier documentation's assertion that groups were unavailable was incorrect.

## Applied recovery

- Cloned `bex-co/block-eden-mono` into gitignored `external/block-eden-mono/` and excluded it from bex Markdown formatting.
- Updated `projects/github-runner/docker-compose.yml` with durable matching pool labels and separate cache volumes; recreated the seven CI and three production runners from this checkout using the existing private deployment env file without printing it.
- GitHub now lists matching labels and assigns jobs to both pools. Ephemeral re-registration preserves the labels.
- Found production liveness job `101251607854` failed after its successful request-log check because `gh` was missing. Added checksum-pinned GitHub CLI 2.100.0 and Docker Compose 5.5.1 to the runner Dockerfile, built the image, and verified required tools as user `docker` with networking disabled. Installed the same verified binaries into active runner containers without interrupting their jobs.
- Added an external runner configuration/build/tool-smoke CI gate. Local configuration tests pass on both the host and the built runner image; mutation spot checks reject the original missing-label config, both labels on CI, and shared npm cache sources.
- Simplify review: three reviewers inspected reuse, quality and efficiency. Removed conflicting runner scaling/cache/resource instructions; no helper extraction or runtime optimization was warranted.

## GitHub evidence so far

- [Docs](https://github.com/bex-co/bex/actions/runs/33995451065): success.
- [Scripts, including runner workflow policy tests](https://github.com/bex-co/bex/actions/runs/33994179727): success.
- [Go vulnerability scan](https://github.com/bex-co/bex/actions/runs/33994179721): success.
- [Operator tests](https://github.com/bex-co/bex/actions/runs/33992408708): success.
- [Platform deploy](https://github.com/bex-co/bex/actions/runs/33994179845): test gates still running/queued at recording time; no successful production deploy claimed.
- [Go lint](https://github.com/bex-co/bex/actions/runs/33994179738): success.
- Deploy controller job `101385575766`: success.
- Backend/operator/CLI/mobile/dashboard checks remain under observation. Mobile cache restoration failed after ten minutes with an Azure authentication/signature error; its setup-go action remained active afterward. Operator and CLI test steps passed but their remote cache-save post steps were still active.
- Prepared `cache: false` for setup-go in backend/mobile/operator/CLI-test/edge-liveness workflows, following the existing go-lint policy. Runner-local Go caches remain enabled; all test commands are unchanged. These workflow changes still require shipping.

## Dashboard test failure

The dashboard run `33994179717` finished with 2,993 passing tests and one
failure: the first workspace-settings test timed out on an empty DOM during
cold route code-splitting. Its second case passed after the component was warm.
The test helper now awaits `router.load()` before rendering/asserting; no sleep,
retry, assertion removal, or timeout increase was added. Both focused cases pass
locally. Full local dashboard lint/typecheck and all 2,994 tests (397 files) passed.

## Cache recovery without changing active job code

Removed only cache ID `7270276348` (4,449,563,889 bytes), the exact backend
cache key observed in failed mobile/backend restorations. Requested cancellation
of standalone backend `33994179697`, mobile `33993444890`, and deploy
`33994179845` before rerunning their affected jobs. The deploy's dashboard,
operator, controller, secret-scan, and supersession gates had all passed; only
backend remained in its failed cache restore. Reruns must wait for authoritative
terminal status. CLI workflow `33992408677` is now green too.

## Still open

_(Superseded 2026-09-07: host separation + credential rotation were rejected by user decision — `.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`; see `2026-09-07-fleet-recovery.md`.)_

- Production and CI still share a privileged Docker host. Separate host placement and the documented credential rotation remain prerequisites for m88 closure.
- Confirm latest CI and a real production deploy green; record job-to-runner/host evidence.
- Source changes are not yet committed/pushed: awaiting explicit `$ship` authorization under the root AGENTS.md rule.
- m89's secretless-build split is unchanged and remains open.

## Live checkpoint after cache recovery

- Backend `33994179697`, mobile `33993444890`, operator `33992408708`, CLI
  `33992408677`, scripts `33994179727`, lint `33994179738`, vulnerability scan
  `33994179721`, and docs `33995451065` are green.
- Edge-liveness `33990860338` is green after restarting its public-edge job,
  which still held the removed cache URL; request-log success was preserved.
- Deploy `33994179845` has all test gates green and production job
  `101389576990` running on `57789c8a59a8` (`bex-production`). TLS validation,
  kubectl/buildx setup, and GHCR login passed; image building is underway.
- The standalone dashboard run retains its original intermittent failure. Its
  fix passes the full local suite, and the deploy's dashboard gate passed on the
  same original source SHA. Ship the fix before claiming durable green CI.
- No successful rollout or m88 host-isolation completion is claimed yet.
