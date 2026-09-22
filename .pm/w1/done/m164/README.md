# w1 · m164 — A cancel over a never-served release must not stamp a `releaseGeneration` that never succeeded

**Worker:** worker1 **Goal:** a first release that never served (a crash loop) followed by a cancel closes its deploy row truthfully, with no generation recorded as successful, while Apps that predate `status.activeRevision` keep reporting exactly what they do today. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                      | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend evidence: what the deploy reconciler does with `releaseGeneration` 0 and with legacy Apps lacking `activeRevision` — **DONE** | 45m | —          |
| t002 | Operator: a never-served release reports no successful generation; the legacy fallback becomes an explicit branch — **DONE** | 40m | t001       |
| t003 | Reconciler test for both shapes plus an envtest for crash-loop-then-cancel — **DONE** | 40m | t002       |
| t004 | Render parity — **DONE** | 20m | t003       |
| t005 | Simplify — **DONE** | 15m | t004       |
| t006 | Test coverage — **DONE** | 40m | t004       |
| t007 | Closeout — **DONE** | 10m | t006       |

## Definition of done

- A canceled deploy over a never-served first release closes as `canceled` and `status.releaseGeneration` records no successful generation (0, or the value t001's evidence recommends, stated in this README).
- A legacy App without `status.activeRevision` reports the same `releaseGeneration` before and after the change (pinned by t003).
- Both shapes are covered by a store-reconciler test and an operator envtest that fail on the pre-fix code.

## Root cause

- `lego/operator/internal/controller/release_identity.go` `successfulReleaseGeneration` falls back to `status.image` + `observedGeneration` when `status.activeRevision` is empty, so a first release that never served can be recorded as the last successful one. The same fallback is the legacy path for Apps that predate `activeRevision`, which is why `w1/m160` t003 left it out.

## Source + Goal linkage

- **Source:** `w1/106` (found in `w1/m160` t003's blast-radius pass, 2026-09-15; parked 2026-09-16 pending store-reconciler evidence), promoted 2026-09-21 via `/pm-brainstorm for w1` item 4.
- **Goal linkage:** release-identity truth (`docs/ADR004-app-deployment.md`; `w1/m160`) — the deploy row bex-api closes from this field is what REST, GraphQL, MCP and the dashboard show as the deploy's outcome.
- **Expected outcome:** the last known path by which a never-served release can be reported as successful is closed; legacy Apps unaffected.
- **Why now:** `m160` shipped the sibling fix for the failure and cancel phases; this fallback is the remaining discrepancy, and the note itself says it needs a milestone's worth of evidence rather than a line change.
- **Render parity:** included, narrowly — deploy status on REST/GraphQL/MCP and the dashboard deploy page; Render closes a canceled deploy as `canceled` without advancing the live deploy.

## Evidence and implementation (2026-09-21)

The recommended canceled, never-served value is **0**. The changed branch is only
`successfulReleaseGeneration` (`lego/operator/internal/controller/release_identity.go:301`):
valid `rev-N` remains authoritative; absent active revision plus any release/artifact
fingerprint proves a modern attempt, so returns zero. Unfingerprinted legacy
image/observed-generation status and noncanonical older active revision names
retain the previous fallback. Fingerprints are recorded before image readiness.
A first crash loop can initially have observed generation zero; cancellation
stamps the observed generation, and the **next** reconcile formerly promoted it
to a successful generation despite no active revision.

Backend evidence: `lego/backend/internal/deploys/service.go:798` patches the cancel
annotation, then line 827 closes the row as canceled with a terminal CAS.
`lego/backend/internal/store/reconciler.go:1330` falls back from zero release
generation to metadata generation; `releaseIsActive` at line 1337 preserves
legacy observed-generation matching. Positive release generation additionally
requires the matching active revision. `PhaseCanceled` has no live transition.
The SQL terminal guard prevents a stale reconciler snapshot (including an expired
gate timeout) from overwriting Cancel's closed row or adding a rollback image.
A preexisting partial Cancel failure between annotation patch and row closure can
still time out as failed; this change does not introduce or widen that behavior.

Read-only production inventory at **2026-09-22T06:14:28Z**: 14 Apps total,
one without active revision (phase Failed), zero without active revision with an
image, and zero unfingerprinted legacy Apps without active revision.
No production writes. Local CAPD API health remains the m163 live-drill blocker;
the production inventory satisfies this task's population check.

Surface audit: REST `internal/deploys/rest.go:270`, GraphQL
`internal/deploys/graphql.go:235`, and MCP `internal/deploys/mcp.go:161`
all call the same Cancel service and expose its persisted deploy status.
Dashboard `src/features/deploys/components/deploy-actions.tsx:137` calls that
GraphQL mutation; `lib/deploy-timeline.ts:81` renders canceled as terminal.
No field names, response/error shapes, or UI behavior change.
[Render's cancel endpoint](https://api-docs.render.com/reference/cancel-deploy)
and [deploy lifecycle](https://render.com/docs/deploys#canceling-a-deploy)
document cancellation and readiness-gated activation. Zero release generation is
an internal Bex representation with no claimed Render wire equivalent.

Validation distinguishes **regressions** from **compatibility**: the three modern
fingerprint unit cases and repeated crash-loop/cancel envtest must fail pre-fix;
legacy unit/envtest and real-PG compatibility cases intentionally pass before
and after. The original task wording requiring unchanged legacy/backend cases
to fail pre-fix is incorrect. Operator envtest checks CRD identity; backend
real-Postgres tests check row state without violating the module import boundary.

Validation completed: full operator `GOWORK=off make test` passed (controller
74.171s); full backend `GOWORK=off go test -p 1 ./...` passed against isolated
real Postgres and OpenFGA (store 31.483s). Operator and backend golangci-lint each
reported zero issues. Three simplify reviews (reuse, quality, efficiency) were
clean; skill layout validation and markdown formatting passed. Baseline archive
of c29c8fe70 fails all three modern fingerprint unit cases and the repeated
crash-loop cancellation assertion (reports 1 instead of 0); its legacy envtest
passes. No live-cluster execution is claimed or required for this milestone.
