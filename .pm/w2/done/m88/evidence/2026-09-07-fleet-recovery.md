# m88 fleet recovery + pool green-run evidence — 2026-09-07

Why: the whole runner fleet was found offline; no job on either pool could run, so
no green-run evidence (t004) — or any CI at all — was possible until recovery.

## Found state

- All org runners `offline` on GitHub: 7 `bex-ci`, 3 `bex-production`, plus 13
  stale registrations carrying no pool label.
- Docker Desktop (the engine hosting the repaired, labeled fleet from the
  2026-09-05 recovery) was not running.
- OrbStack had auto-restarted the stale pre-repair fleet (16
  `github-runner-runner-bex-co-*` containers created 2026-08-25, `restart:
unless-stopped`) in a crash loop: some with broken runner binaries
  (`./bin/Runner.Listener: No such file or directory`, exit 127), others failing
  registration with "A runner exists with the same name" against their own stale
  offline registrations.
- Today's pushes were fully queued: platform deploy `34160723350` pending with
  every test gate queued.

## Recovery applied

- Started Docker Desktop; the labeled fleet came back healthy by itself: 7×
  `runner-bex-ci` + 3× `runner-bex-production` containers (compose checkout
  `external/block-eden-mono` at `1bc8ff92f`, clean and pushed).
- Stopped all 16 stale OrbStack `github-runner-*` containers (the crash-looping
  2026-08-25 fleet; `unless-stopped` keeps them down across engine restarts).
- Deleted the 13 offline, pool-label-less GitHub runner registrations they had
  leaked (ids 2381, 2462, 2498, 2523, 2552, 2553, 2555, 2571, 2583, 2585, 2605,
  2619, 2621). Every remaining registration carries exactly one pool label.
- Verified via `gh api orgs/bex-co/actions/runners`: `bex-ci` 7/7 online (all
  busy draining the queue), `bex-production` 3/3 online.

## Green-run evidence (shared-host topology)

- CI pool: `scripts (test)` run `34160723241` → job `test` concluded `success`
  on runner `c57df3d66c97`, labels `self-hosted,Linux,ARM64,bex-ci` (plus
  today's backend/dashboard/operator/controller/lint/govulncheck gates, all
  green on `bex-ci` runners).
- Production pool: deploy run `33994179845` ("fix(services): validate
  repository access before source updates") concluded `success`; its
  `build-and-deploy` job succeeded on runner `57789c8a59a8`, labels
  `self-hosted,Linux,ARM64,bex-production` — a real post-re-label production
  deploy. (The 2026-09-05 note recorded it mid-build; confirmed complete
  today.) A further deploy `34160723350` (remote HEAD `0276d0ab1e`) was
  mid-build on the same production runner at closeout time.

## Host separation + credential rotation: rejected by user decision (2026-09-07)

Asked directly this session, the user ruled: the single-Mac runner fleet is a
**hard constraint** ("使用 single mac runner 是硬性规定, accept the risk") —
do not provision separate production hosts, and do not perform the
shared-runner credential rotation. Recorded as `.pm/DO_NOT_DO.md`
`#RUNNER-HOSTS`; ADR083 finding 3/4 dispositions and operator obligations 1/2,
ADR019 §Decision 5, and `docs/runbooks/runner-pool-relabel.md` updated to
match. m88 therefore closes on the label-level pool split + fail-closed
validation + the green-run evidence above.
