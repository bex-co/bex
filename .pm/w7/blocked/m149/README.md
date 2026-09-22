# w7 · m149 — Correct cold deep-link initialization

**Worker:** worker7 **Goal:** Direct service and agents URLs settle from initial workspace and capability results instead of waiting for the periodic poll. **Status:** blocked (2026-09-21: dev-7 live measurement needs shared VM memory recovery; no tasks complete)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce and measure cold-link initialization | 50m | w7/m148/t007 |
| t002 | Repair the measured workspace and capability startup failure | 50m | w7/m149/t001 |
| t003 | Wait for workspace resolution before agents route eligibility | 35m | w7/m149/t001 |
| t004 | Preserve and verify cold-link destinations end to end | 45m | w7/m149/t002, w7/m149/t003 |
| t005 | Render parity | 30m | w7/m149/t001, w7/m149/t002, w7/m149/t003, w7/m149/t004 |
| t006 | Simplify | 25m | w7/m149/t005 |
| t007 | Test coverage | 45m | w7/m149/t005 |
| t008 | Closeout | 15m | w7/m149/t006, w7/m149/t007 |

## Definition of done

- [ ] Record a reproducible failing sequence with timestamps and a passing navigation control.
- [ ] Distinguish delayed workspace resolution from pre-dispatch rejection or cancellation; do not infer the cause from timing alone.
- [ ] No persistent token or secret logging is introduced.
- [ ] Healthy initial results settle capability state before the periodic poll is needed.
- [ ] A stale response from a prior workspace or generation cannot enable actions.
- [ ] Failed refresh and confirmed denial remain distinct; no permission gate is weakened.
- [ ] An eligible cold agents URL does not redirect home solely because workspace selection is pending.
- [ ] Confirmed ineligibility still follows the existing refusal/navigation policy.
- [ ] Pending geometry matches the destination at desktop and narrow-mobile widths.
- [ ] Service env/Settings and agents cold links retain their intended destination.
- [ ] Authorized, denied, unavailable and recovered states are recorded at desktop and narrow-mobile widths.
- [ ] No fixed delay or faster polling masks the startup failure.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 2 via `$pm all for w7`; promoted `w7/049`, archived at `.pm/w7/done/049.md`.
- **Goal linkage:** ADR008 dependable hosting controls and agent-session access; ADR018 Viewer capability projection.
- **Expected outcome:** Cold direct links preserve their destination and produce truthful allowed, denied, and unavailable states without waiting for the 30-second poll.
- **Why now:** The recorded user-facing failure is parked on executable diagnosis, not unavailable credentials. w6/m144 shipped the refresh lifecycle; this fixes its initialization residual and coordinates with w6/074 without duplicating that broader walkthrough.
- **Render parity:** Included: tenant-facing changes; compare the applicable ADR018 coverage and all exposed surfaces.
- **Sizing:** 180m implementation; 295m including standing closing tasks, 8 tasks.

- **Dependency:** t001 follows w7/m148/t007 so the live diagnosis starts on a verified local substrate. Coordinate evidence with w6/074; do not claim its whole walkthrough complete.

## UNBLOCKED 2026-09-17 — the upstream gate cleared the same day

`w7/m148/t007` is **done**: the rotted local CAPD cluster was reprovisioned and a healthy bring-up recorded, so t001's dependency is satisfied and this milestone is workable.

What has **not** changed is t001's nature: it is still a measurement task, and the DoD still forbids substituting reasoning for it — _"do not infer the cause from timing alone."_ The source findings below remain the aim for that measurement, not a replacement for it.

### Original block (kept for the diagnosis it carries)

Nothing here is implementable yet, and that is the milestone's own design rather than a judgement call:

- **t001 `depends_on: w7/m148/t007`**, which is held: m148 is parked because the local CAPD cluster has rotted (machines 20d old, backing containers gone). Every other task in this milestone chains off t001, so the whole queue is behind it.
- **t001 is a measurement task, and the DoD forbids substituting reasoning for it** — _"Distinguish delayed workspace resolution from pre-dispatch rejection or cancellation; do not infer the cause from timing alone."_ That is the same gate `049` was parked on, now written into the acceptance criteria.

A source pass during the `049` triage already advanced the diagnosis and is worth reading before t001 runs, so the measurement is aimed rather than exploratory (`.pm/w7/done/049.md`):

- **Candidate (b) is falsified.** `routes/__root.tsx` `beforeLoad` awaits the session and returns it in context, so `useIsAuthenticated()` is true on first render and the workspaces query is never `skip`ped during hydration.
- **The contradiction to resolve is sharp.** `capabilities.grantUnavailable` has exactly two sources (`capability-policy.ts:166`, `:179`), both requiring a *resolved* state — yet the page showed it while no `ViewerCapabilities` request reached the wire for 30s. A query rejecting before dispatch fits; `refresh()` passes `context.fetchOptions.signal`, and an already-aborted signal rejects with no request.
- **One fix step is already satisfied and should be dropped.** Step 1 of `049`'s fix ("the same copy covers 'never asked' and 'asked and failed'") is not true in current code: a `checking` state returns `undefined`, so pending and failed are already distinct.

t003 (the `/agents` guard) looks separable, and its first two criteria are, but its third — _"Pending geometry matches the destination at desktop and narrow-mobile widths"_ — is a rendering requirement, and it still declares `depends_on t001`. Implementing it ahead of the measurement would contradict the sequencing this milestone chose and risk the exact error its DoD warns about.


## Blocked 2026-09-21 — live measurement requires shared VM recovery

The m148 dependency remains complete, but current dev-7 is not healthy. The local
CAPD substrate was brought up again, CRDs and the operator installed, and all
three database pods reached Ready. Auth migration jobs initially could not reach
their database across CAPD nodes; pinning dev-7 auth workloads to the control-plane
node completed the migrations. Subsequent kernel diagnostics confirmed global
out-of-memory kills in the shared OrbStack VM (16 GiB), repeatedly dropping
Kratos/CNPG readiness and preventing bex-api database initialization. Per-dev-7
memory limits, scheduling pins and scaling down unused auth components did not
stabilize it. No other workstream or runner was stopped.

**Gate:** the user/operator must schedule a shared OrbStack restart with enough
memory (24 GiB was the proposed setting), or otherwise free sufficient shared VM
memory without disrupting other workloads; then rerun dev-7 bring-up and t001's
instrumented browser measurement. `orb config set memory_mib` reported that a VM
stop was required, so its pending change was reverted to 16384 and no shared
restart occurred. This is an operational availability gate, not missing access.

No task is marked complete: t001 requires a live measured journey and the
remaining tasks explicitly depend on it. Temporary diagnostic source edits were
removed. The actual providers/transport and router were exercised offline, but
those results are not claimed as desktop/mobile or authenticated dev-7 acceptance.

### Reproduced diagnosis to resume from

- Real `WorkspaceProvider` + `CapabilitiesProvider` with the production Apollo
  client transport: two failing regressions (cold missing-cookie fallback and
  workspace switch before batch dispatch), two passing controls (retained
  selection and failure/denial/recovery). A healthy Workspaces reply selects the
  fallback at 10 ms; generation 0 capability request aborts; generation 1 starts
  at 10 ms but rejects `AbortError` around 940 ms while its own signal is still
  active. This is measured pre-dispatch cancellation, not a 30-second delay in
  workspace readiness. Test fetch honors AbortSignal; auth/network boundaries
  are test doubles, with fixed retry jitter for deterministic batching.
- Installed Apollo deduplicates identical document/variables without including
  AbortSignal. Disabling dedup alone still fails because BatchHttpLink groups
  signals by serialized fetch options and sends the batch with its first signal.
  Candidate repair: disable query deduplication only for capability checks and
  send explicitly signalled operations through HttpLink, preserving batching for
  other reads. Verify against the browser trace before implementing t002.
- Real TanStack memory routers using the actual list/detail `beforeLoad`
  handlers redirect cold stale-cookie agents links to Home with zero workspace
  requests. Fourteen guard regressions fail on baseline. Candidate repair:
  await membership in the guard, preserve a valid preferred member or select the
  first member, return the selected workspace to the loader, and redirect only
  after confirmed ineligibility. Null/error membership is unavailable, not denial;
  an invalid cached null must be evicted so retry can recover.
- One further regression proves the existing ErrorPage's Try again only resets
  its React boundary and never reruns a failed `beforeLoad`. A bounded companion
  fix would invalidate the router before resetting the boundary.
- ADR018 lines 261 and 263 classify agent sessions and Viewer capabilities as
  bex extensions. Proposed changes alter no REST/GraphQL/MCP fields, permission
  verbs or beta workspace targeting.

The three proposed regression files are preserved in
[`evidence/startup-regressions.patch`](evidence/startup-regressions.patch). Apply it
from the repository root to restore the test work. They intentionally fail on the
unfixed implementation, so they are diagnostic artifacts rather than part of the
shipped runnable suite. The capability test's TypeScript and ESLint checks passed;
no fix or full dashboard-suite pass is claimed.


### Local recovery state left behind

The installed CRDs/operator and dev-7 database/auth resources remain in the local
cluster. Dev-7 Hydra, Mailpit and courier were scaled to zero to reduce pressure;
auth pods have local control-plane pins, memory limits and Go caps that may need
reapplication after `scripts/dev-env.sh 7 up`. All API, dashboard and port-forward
processes started by this run were stopped, and temporary auth artifacts were
removed. No usable authenticated browser state was produced.
