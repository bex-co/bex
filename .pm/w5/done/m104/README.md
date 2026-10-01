# w5 · m104 — Correct cold-link initialization

**Worker:** worker5 **Goal:** Cold service and agent links retain their destination and settle truthful permissions without waiting for a poll. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render cold-link and workspace behavior for parity  — **DONE** | 30m | — |
| t002 | Reproduce and measure cold-link initialization on dev-5  — **DONE** | 45m | t001 |
| t003 | Repair measured capability-request cancellation — **DONE** | 50m | t002 |
| t004 | Resolve workspace before agent eligibility and repair retry — **DONE** | 45m | t002 |
| t005 | Verify destinations and pending geometry end to end — **DONE** | 40m | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout — **DONE** | 15m | t007, t008 |

## Definition of done

- [x] Dated parity matrix distinguishes observed behavior, documentation, inference and bex extensions.
- [x] The service/agent cold-link acceptance is reconciled with findings before t002.
- [x] A reproducible failing sequence and passing control are timestamped.
- [x] Workspace delay and pre-dispatch cancellation are distinguished.
- [x] Current shared capacity is verified; no dev-7 resources are mutated.
- [x] Initial healthy capability reads settle without periodic polling.
- [x] Canceled prior-generation work cannot cancel or authorize the current generation.
- [x] Failure and confirmed denial remain distinct.
- [x] Eligible cold links are not redirected solely because workspace resolution is pending.
- [x] Confirmed denial still follows existing policy.
- [x] Unavailable membership can recover through a real loader retry.
- [x] Intended destinations survive cold navigation and workspace selection.
- [x] Pending and ready geometry match at both breakpoints.
- [x] No faster poll or fixed delay masks initialization failure.

## Source + Goal linkage

- **Source:** [w7/m149 retained diagnosis](../../../w7/blocked/m149/README.md) and its preserved regression patch; approved by the user on 2026-09-30 following the w5 brainstorm and parity correction.
- **Goal linkage:** ADR008 dependable, deterministic hosting and agent operation.
- **Expected outcome:** Cold service and agent links retain their destination and settle truthful permissions without waiting for a poll.
- **Why now:** Permission refresh shipped in w6/m144; measured startup residuals remain and w5 has capacity.
- **Render parity:** Included because tenant-facing behavior changes. t001 researches Render first; the closing parity task verifies the finished behavior. Label unmatched surfaces as bex extensions, not proven Render parity.
- **Sizing:** 210m research/implementation; 320m including closing tasks; 9 tasks.

## Transfer and execution constraints

Execution ownership transfers from w7/m149 to w5/m104. The original evidence and task history stay in w7; its tasks wait on this milestone and are not a second implementation queue. Read the original regression patch in place; do not apply it blindly. Historical shared-VM OOM remains a capacity risk, not proof dev-5 is currently blocked. Local harness recovery is pre-approved under root instructions, within isolation boundaries; do not restart the shared VM or alter another workstream as an incidental recovery. Research is immediately actionable. Coordinate with w6/074 without claiming its broader walkthrough complete.

## Drain evidence — 2026-09-30

### t001 Render research

Official [dashboard documentation](https://render.com/docs/render-dashboard) documents resource navigation and workspace switching; [workspace roles](https://render.com/docs/team-members) documents membership-scoped permissions. Neither establishes timing or fallback behavior for a cold URL with a stale/missing selection. No authenticated Render cold-link capture was made. The repo ADR018 classifies Viewer capabilities and agent sessions as bex extensions. Preserve valid navigation and fail-closed permissions as bex correctness; do not claim matching internal GraphQL or agent routes. Acceptance needs no widening or authentication change.

### t002 diagnosis

The isolated dev-5 stack successfully started on current main (API health 200, Kratos/Hydra/Postgres Ready); the historical shared-VM OOM did not recur. A disposable verified Kratos identity authenticated through the actual dashboard. A locally created service Settings cold load with selection cookie removed sent Workspaces at 1956ms and resource reads at 2402ms, but no ViewerCapabilities in the following three seconds. Retained-cookie reload sent Workspaces at 2017ms and ViewerCapabilities at 2033ms. These are measured browser request timestamps, not inferred workspace latency. Preserved real-provider/router regressions reproduce 17 failures with 2 passing controls: generation cancellation/batching, premature agent redirect, and retry that never reruns beforeLoad. Diagnostic test changes came only from the preserved patch.

The base dev harness uses insecure local authz, so this run does not claim a real OpenFGA deny-path proof. Permission denial/unavailable/recovery must remain covered with explicit controlled UI responses plus the real provider tests.

An isolated beta-target workspace fixture was added only to dev-5, ordered first in this disposable identity's actual membership list. Without a selection cookie, authenticated `/agents` redirected to `/`; with the same identity and an explicit beta selection cookie it retained `/agents` and displayed the truthful platform-not-configured state. This confirms premature eligibility, not a denied membership. No production beta policy was changed.

### Implementation and closeout

Abortable reads now use HttpLink independently while ordinary reads stay batched; capability refresh opts out of Apollo deduplication so generations cannot share an aborted operation. Canceled requests no longer retry with an unusable signal. Agent guards resolve membership before feature eligibility and return selected workspace context; missing/null membership is unavailable and evicted for a real retry. Error-page retry invalidates routing before resetting the boundary.

Live dev-5 after correction: missing-cookie `/agents` retained its destination; first capability checks began at 1894ms/1909ms during page hydration, instead of waiting for polling. Service Settings and Environment links, archived/all agent links, and confirmed non-beta redirects were exercised at 1440×900 and 390×900 with no horizontal overflow. Controlled browser responses (not real OpenFGA role changes) proved denied = loaded + cannot create, unavailable = not loaded + cannot create, and recovered = loaded + can create at both widths. Provider state was inspected read-only through React's live context; no diagnostic code was added to the application.

The actual AgentsPageSkeleton component was rendered temporarily in a separate browser-only probe at the real content width, then removed. Composer bounds match the ready form: desktop 640×152, mobile 358×188. Screenshots are local `.playwright-mcp/w5-m104-component-pending-{1440,390}.png` and `w5-m104-agents-{1440,390}-ready.png`. Automatic route-pending capture attempts did not catch the skeleton, so these are explicit component comparisons, not claimed timed route captures. The unconfigured local gateway adds its existing ready-state warning above the composer; the comparison verifies the unchanged primary composer region, not a newly configured agent backend. No agent was executed.

Parity: REST/GraphQL/MCP fields and server permission policy are unchanged. Shared cached membership and bex-only agent/capability handling remain labeled extensions; no exact Render cold-link timing is claimed. Simplify's three reviews found one worthwhile efficiency correction (abort-aware retry), applied with a behavioral regression. The proposed selector extraction was not adopted: the tiny selection expressions consume different raw/provider types and a new cross-layer helper would add indirection for no material gain.

Validation: `yarn lint` passed (includes production/test typecheck, ESLint, knip). Final full `yarn test`: 456 files / 3785 tests passed. Preserved baseline: 17 failures / 2 passing controls; focused corrected startup/guard/retry tests passed. Disposable service, both fixture workspaces and both Kratos identities were deleted via local APIs. No production mutations or secret values were recorded. The historical w7/m149 evidence stays unchanged; this is its w5 execution result, not completion of another workstream's board.
