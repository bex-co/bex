# w2 · m56 — Separate deploy revisions from operational App reconciliation

**Worker:** worker2 **Goal:** Make operational App changes—starting with manual scale—converge against the active artifact without starting a build or pre-deploy run, while real deploy changes still produce a fresh artifact and revision. **Status:** done

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Define artifact and release identity independently of App generation | 45m | — | — **DONE** |
| t002 | Reconcile operational changes against the active artifact | 75m | t001 | — **DONE** |
| t003 | Backfill existing Apps and preserve deploy-history semantics | 45m | t002 | — **DONE** |
| t004 | Make dashboard scale acknowledgement asynchronous and honest | 30m | t002 | — **DONE** |
| t005 | Render parity: revalidate manual scale on every surface | 30m | t003, t004 | — **DONE** |
| t006 | Simplify: one deploy-decision path | 20m | t005 | — **DONE** |
| t007 | Test coverage: stale credential, legacy status, and true deploys | 45m | t005 | — **DONE** |
| t008 | Closeout | 10m | t006, t007 | — **DONE** |

## Definition of done

A running source-built service with an active image/revision and a deliberately stale or unusable clone credential scales from one to two replicas through REST, GraphQL, MCP, and the dashboard without creating a build Job or pre-deploy Job. The existing Deployment keeps the same image, active revision, and pod-template hash; its original pod remains eligible to serve while a second pod becomes ready. The operational change creates no deploy-history row and cannot set `BuildFailed`.

A genuine deploy still refreshes the clone credential through the backend, builds exactly once, runs pre-deploy exactly once when configured, and advances the artifact/revision. Existing Apps that predate the new status fields—including the production incident shape of generation 2, observed generation 1, active revision 1, a reusable generation-1 image, and a failed generation-2 build—backfill and converge without an unnecessary build or pod replacement. The dashboard reports an accepted scaling request without claiming that convergence has already completed. Relevant Go and dashboard suites pass, and live evidence records the scale and true-deploy paths.

## Source + Goal linkage

- **Source:** user-requested production investigation, 2026-07-18. Scaling `srv-d9dd16roviqs738quds0` from 1→2 correctly updated `App.spec.replicas`, but the operator treated the new Kubernetes generation as a deploy, launched `bld-tea-d98210cbbpdc73dcrkvg-bex-gen-2`, and failed it with `BackoffLimitExceeded`. The source-backed service's copied GitHub installation credential was more than three hours past its documented one-hour lifetime—a strong explanation for the fast clone/build failure, but not the reason scale should have needed source access. The generation-1 Deployment remained healthy at one replica while the App reported `BuildFailed`.
- **Goal linkage:** `docs/ADR008-vision.md` pillars 1–3 require declarative intent to converge deterministically and machine surfaces to report honest state. `docs/ADR004-deployment.md` currently makes every spec generation a release generation, which violates that contract for operational fields such as replicas. `docs/ADR018-render-parity.md` marks manual scale complete on all four surfaces, so the implementation and parity evidence must be corrected together.
- **Expected outcome:** release identity is explicit rather than inferred from arbitrary metadata-generation churn. Scaling and other classified operational changes remain independent of source-host credentials, build capacity, and pre-deploy side effects.
- **Why now:** the production incident is reproducible from the current controller gate in `app_controller.go`, and the existing w2/m12 scale milestone covered API behavior without exercising a source-backed service. Refreshing the clone token on scale would conceal the symptom while retaining an unnecessary build, pod rollout, and migration risk.
- **Explicitly excluded:** replacing BuildKit or GitHub App authentication, changing the public scale request shape, redesigning all deploy history, introducing protected-environment policy, or silently changing free-plan replica policy without separate parity evidence.

## Implementation summary (2026-07-18)

- Added exhaustive, versioned artifact/release fingerprints plus a separately persisted candidate artifact image and release generation; operational fields reconcile against the active release.
- Added legacy adoption for the exact production failure shape, release-generation-aware build/pre-deploy/history identity, and a backend annotation that closes deploy-trigger/operational-update races.
- Made REST, GraphQL, MCP, and dashboard scale fixtures source-backed with an unusable clone credential; scale opens no deploy row and the dashboard reports accepted asynchronous intent in English and Chinese.
- `$simplify` was not installed in this Codex session, so the required pass was performed manually: one `prepareAppReleaseDecision` path owns classification/backfill, stale generation gates/comments were removed, and lint reports zero issues.
- Full operator/backend/dashboard suites passed. Sanitized live CAPD evidence is in [`evidence/2026-07-18-live-acceptance.md`](evidence/2026-07-18-live-acceptance.md).
