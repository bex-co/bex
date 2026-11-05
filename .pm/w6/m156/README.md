# w6 · m156 — Deploy latency watchdog: a green main never waits half a day to deploy

**Worker:** worker6 **Goal:** when a deployable commit on main waits more than 6 h for its production pin, an issue opens automatically, naming why, and the remaining starvation cause is fixed. **Status:** todo (t001–t002 done; t003 unblocked by D8 = (a), 2026-11-04)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure the oldest undeployed deployable commit and classify why it waits](done/t001.md) — **DONE** | 75m | — |
| t002 | [Scheduled watchdog: open an issue past 6 h, close it on recovery](done/t002.md) — **DONE** | 60m | w6/m156/t001 |
| t003 | [Replay the last week's long gaps and fix the remaining starvation cause](t003.md) | 75m | w6/m156/t001 |
| t004 | [Simplify](t004.md) | 15m | w6/m156/t002, w6/m156/t003 |
| t005 | [Test coverage](t005.md) | 20m | w6/m156/t004 |
| t006 | [Closeout](t006.md) | 10m | w6/m156/t005 |

## Definition of done

- A script computes, for `origin/main`, the oldest deployable commit (one touching deploy.yml's production-input paths) not yet contained in the current `pin platform images` commit, and its age. It classifies the wait as `main red`, `superseded`, `cancelled while pending`, `waiting for a runner`, `Argo not synced` or `in progress`, from GitHub run and job data.
- A scheduled, read-only workflow (on `bex-ci`) opens one issue when that age exceeds 6 h, naming the commit, the age and the cause. It updates the issue while the condition holds and closes it once a pin includes the commit.
- Replaying the 7 days to 2026-11-02 finds all four gaps over 10 h (10-27→28 15.7 h, 10-28→29 15.6 h, 10-29→30 11.3 h, 10-30→31 24.9 h) and names each one's cause. Any cause that is still structural (e.g. supersession or pending-slot cancellation) gets a fix shipped, or a recorded decision against it, and w6/087's record is updated.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6`, 2026-11-02, proposal 1. Evidence: 40 pins in 7 days (median gap 1.8 h, p90 9.2 h, max 24.9 h). 29 deployable commits waited through the 10-30 17:49 → 10-31 18:40 gap and 16 through 10-27→28. Since 10-30, 60 deploy runs ended 39 cancelled / 5 failed / 16 succeeded. w6/087 closed with a ">12 h ⇒ reopen" rule that nothing checks, so at least two qualifying gaps went unnoticed. `ci-red-streak.yml` only sees repeated failures.
- **Goal linkage:** ADR008 reliable delivery. Finished work must reach production promptly, which every live-verification DoD on the board depends on.
- **Expected outcome:** a stalled deploy is visible within 6 h with its cause named, instead of being discovered by a worker hours later. The remaining starvation cause is fixed.
- **Why now:** w6/m155 has just removed runner contention, one of the causes, and w6/100's interim Argo fix removed another. This is the right moment to measure what is left before the next stall.
- **Render parity:** omitted. CI/deploy infrastructure, no REST/GraphQL/MCP/UI surface.

## t001–t002 implementation — 2026-11-03

- `scripts/deploy_latency.py` finds the latest `chore(deploy): pin platform images to <sha>` commit on `origin/main` and the oldest later commit inside deploy.yml's push filter (`lego/**` minus `lego/cli/**`, `dashboard/**`, …), pin commits excluded. It classifies the wait from main-branch deploy runs that carry that commit, newest first by `createdAt` (no `--branch`, as in ci-red-streak): `deployed` / `not-triggered` / `queued` (deploy slot, or an environment gate) / `waiting-for-runner` (a job queued >5 min) / `in-progress` / `failed` (job: step) / `cancelled` / `superseded`. Exit codes: 0 within threshold, 1 late, 2 unusable. 13 tests (`scripts/test_deploy_latency.py`, wired into `scripts.yml`), including a throwaway-repo end-to-end run with the API stubbed.
- `.github/workflows/deploy-latency.yml`: hourly (`41 * * * *`) on `bex-production` with `issues: write` (ci-red-streak's pattern). Past `THRESHOLD` (6 h, or a dispatch input) it opens or updates one `deploy-latency` issue and closes it on recovery. Registered at 6 h in `scripts/ci-schedule-drift.sh`.
- First production readings (2026-11-03 06:2x UTC): `waiting-for-runner 0.6h bd33b3012c49 … check-supersession queued 32 min` while all 7 `bex-ci` runners were busy, then `in-progress 0.9h`.

## t003 replay — 2026-11-03

| gap (UTC) | deploy runs in window | deployable / manifest-changing commits | cause |
| --- | --- | --- | --- |
| 10-27 12:23 → 10-28 04:07 (15.7 h) | 13: 10 cancelled, 3 success | 15 / 3 | 2 successful runs superseded at write-back (40612899771, 40621285134), the rest cancelled pending |
| 10-28 11:23 → 10-29 02:59 (15.6 h) | 8: 3 cancelled, 2 failed, 3 success | 8 / 2 | failures 40771246884, 40790621326; 1 success superseded (40786744901) |
| 10-29 21:19 → 10-30 08:36 (11.3 h) | 23: 18 cancelled, 1 failed, 4 success | 29 / 6 | 3 successes superseded (40966856467, 40984361532, 40999184497) plus runner contention (the w6/m155 case) |
| 10-30 17:49 → 10-31 18:40 (24.9 h) | 23: 20 cancelled, 2 failed, 1 success | 21 / 4 | failures 41124028938, 41178940668; every other run cancelled pending |

**Finding:** the structural cause is still w6/087's. A run takes 1–3 h, and 2–6 manifest-changing commits landed inside each gap. A run that finishes after any of them is discarded at write-back (`deploy-superseded.sh` exit 0: superseded by newer production manifests), and the single pending slot cancels every run queued behind it. Six runs that had passed every test and build were thrown away in these four gaps. Runner contention, fixed by w6/m155, compounded the 10-29 gap.

**BLOCKED (2026-11-03): t003's structural fix needs the user's decision, as w6/087 recorded.**

- **(a) Relax the invariant (recommended):** pin a superseded run's images when the newer manifest diff touches only resources those images do not read. This needs a reviewed definition of "untouched", e.g. a path allowlist (`deploy/gitops/charts/**` other than the operator's, Helm values for third-party charts) that cannot change a CRD, RBAC or a resource the operator/dashboard/API reads.
- **(b) Buy throughput:** cut the test stage well below the manifest-change interval (more `bex-ci` runners, which were all 7 busy at 06:2x UTC today, or split the suites).
- **(c) Accept:** keep the invariant, and let t002's watchdog report each stall.

t001/t002 do not depend on this and are shipped. t004–t006 follow the decision.

## t002 live verification — 2026-11-03

- The first dispatch (41590504168) was cancelled at the 15-min timeout inside a full checkout. The checkout was fixed in `b1732715a` (blobless + sparse) and later generalised by w6/m157.
- With threshold 0, dispatch 41595534204 succeeded and opened issue #122 "main is not reaching production" at 08:04 UTC.
- With threshold 1000, dispatch 41610903992 succeeded and closed #122 with `Recovered: queued 3.8h bd33b3012c49 run=41609029897 waiting for the bex-production-deploy slot …`. That is a real reading: a deployable commit had been waiting 3.8 h behind the deploy slot.

## Unblock decision — 2026-11-04 (source: user decision D8, /unblock w6)

- **Affected:** w6/m156/t003 (and its follow-ups t004–t006); w6/087's supersession record.
- **Decision: (a) relax the invariant.** A deploy run superseded at write-back still pins its images when every manifest change between its tested revision and the newer production manifests touches only resources its images do not read. Any change outside that reviewed set (a CRD, RBAC, or a resource the operator, dashboard or API reads) still discards the run as today.
- **Rationale:** the replay above shows six fully tested and built runs discarded in four gaps of 11–25 h. A reviewed allowlist removes that waste while the dangerous changes keep the existing discard-and-rerun path. (b) throughput is partly delivered already: the 2026-11-04 history rewrite made every checkout ~6x smaller, and w6/m155 split the build pool. (c) left the stalls in place. Reference product: none applies; this is internal deploy mechanism with no customer-visible Render behavior, beyond fixes reaching production sooner.
- **Remaining gates:** none external.
- **Worker next steps (t003):**
  1. Define the allowlist in one reviewed file next to `scripts/deploy-superseded.sh` (e.g. `deploy/gitops/charts/**` other than the operator's, third-party chart Helm values), defaulting to discard for anything not listed. Exclude anything that can change a CRD, RBAC, or a resource the platform images read.
  2. Teach `scripts/deploy-superseded.sh` to diff the run's tested manifests against the newer production manifests and report "superseded but compatible" when every changed path is allowlisted; deploy.yml then pins instead of concluding neutral.
  3. Tests in `scripts/deploy-superseded.test.sh`: allowlisted-only diff pins; a CRD/RBAC/operator-config change in the same diff still discards; an empty or unparseable diff discards (fail closed).
  4. Update w6/087's record with the decision and the outcome.
- **Acceptance:** the unit tests above pass; the next live supersession whose newer diff is allowlisted-only pins instead of being discarded (cite the run); `scripts/deploy_latency.py` over the following 7 days shows no gap over 10 h caused by `superseded`.
