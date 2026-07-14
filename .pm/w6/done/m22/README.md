# w6 · m22 — CNB buildpack builds via kpack

**Worker:** worker6 **Goal:** implement in-cluster Cloud Native Buildpack builds (`spec.builder: buildpack`), replacing today's explicit "not yet supported" error **Status:** done — kpack v0.17.2, a Paketo ClusterBuilder, authenticated Zot publishing, digest-based operator reconciliation, `BP_*`/`BPE_*` build env, least-privilege RBAC, and API extension parity are implemented. A follow-up compatibility pass restored Render-native runtime/build/start fields across REST/GraphQL/MCP/dashboard while retaining explicit buildpacks as a bex extension. A real Dockerfile-less Paketo Go sample built in the CAPD mock cluster, published to Zot, reached `Running`, and returned `Powered By Paketo Buildpacks` from its Ready pod.

## Tasks (in order)

| id   | title                                                                                                                       | est  | depends_on | status |
| ---- | --------------------------------------------------------------------------------------------------------------------------- | ---- | ---------- | ------ |
| t001 | infra: add kpack controller + `ClusterBuilder`/`ClusterStack` CRs to `deploy/gitops/` (builder image from `BEX_CNB_BUILDER`) | 2h   | —          | — **DONE** (controller, CRDs, Stack/Store/Builder, Zot alias/credentials, and node runtime mapping verified live) |
| t002 | operator: replace `build.go`'s buildpack error branch with a real kpack `Image` CR dispatch                                 | 3h   | t001       | — **DONE** (dispatch, polling, digest canonicalization, failures, cancellation, and concurrency accounting) |
| t003 | Thread build-time env vars (buildpacks' `BP_*` mechanism) into the kpack `Image` spec                                      | 1h   | t002       | — **DONE** (literal `BP_*`/`BPE_*` values only; `BP_GO_TARGETS` and `BP_LOG_LEVEL` observed in the real build) |
| t004 | RBAC: manager permissions to create/watch kpack `Image`/`Build` CRs                                                        | 30m  | t001       | — **DONE** (namespace-scoped credentials plus minimal Image/Build verbs; uncached build-plane client) |
| t005 | Render parity: verify `spec.builder: buildpack` round-trips consistently across REST/GraphQL/MCP/UI                         | 45m  | t002, t003 | — **DONE** (buildpack extension round-trip tests; Render-native runtime/build/start UI and API compatibility) |
| t006 | Simplify                                                                                                                    | 30m  | t005       | — **DONE** (`/simplify` unavailable; manual behavior-preserving review consolidated the build-plane client path) |
| t007 | Replace `build_test.go`'s "buildpack rejected" assertion with real dispatch test coverage                                   | 1.5h | t005       | — **DONE** (CR shape/env/success/failure/credential tests; operator `make test` green) |
| t008 | Closeout                                                                                                                    | 15m  | t007       | — **DONE** (real Dockerfile-less E2E and all board status synchronized) |

## Definition of done

Deploying an App with `spec.builder: buildpack` against a Dockerfile-less, buildpack-detectable repo (e.g. a plain `go.mod` or `package.json` app) produces a running, healthy revision end-to-end — verified against a real repo, not a mock. ✅ Verified with `paketo-buildpacks/samples` (`go/mod`) in the CAPD mock cluster: kpack built and pushed an immutable digest, the operator reconciled it into a Ready Deployment, and the service returned `Powered By Paketo Buildpacks`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `lego/operator/internal/build/build.go:24-27,164` and `docs/ADR004-deployment.md` both explicitly document this as a deferred "not yet, wait for kpack" contract; `BEX_CNB_BUILDER` (see root `CLAUDE.md` env table) and the `spec.builder` enum have been sitting ready since `w1/m5` (`.pm/w1/done/m5/README.md:38` records `CNBBuilder` was kept on purpose, "reserved for kpack (a 'not yet', not dead code)"). Materialized under `w6` **per user direction**, same as `w6/m19`-`m21`.
- **Goal linkage:** Render parity — Render's core value prop is buildpack auto-detect with zero Dockerfile; bex is Dockerfile-only today.
- **Expected outcome:** repos without a Dockerfile become deployable, matching Render's headline onboarding flow.
- **Why now:** the contract (CRD enum, env var, error message) has been ready since `w1/m5`; this is the largest of the four `/pm-brainstorm more milestones` (2026-07-13) proposals — a new cluster-wide infra dependency (kpack) — so it's sequenced last among them, but it's real, substantial, unbuilt Render-parity work, not a non-goal. Render parity included — the new `spec.builder` value must round-trip consistently across REST/GraphQL/MCP/UI.
