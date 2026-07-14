# w6 · m22 — CNB buildpack builds via kpack

**Worker:** worker6 **Goal:** implement in-cluster Cloud Native Buildpack builds (`spec.builder: buildpack`), replacing today's explicit "not yet supported" error **Status:** todo

## Tasks (in order)

| id   | title                                                                                                    | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | infra: add kpack controller + `ClusterBuilder`/`ClusterStack` CRs to `deploy/gitops/` (builder image from `BEX_CNB_BUILDER`) | 2h   | —          |
| t002 | operator: replace `build.go`'s buildpack error branch with a real kpack `Image` CR dispatch                | 3h   | t001       |
| t003 | Thread build-time env vars (buildpacks' `BP_*` mechanism) into the kpack `Image` spec                       | 1h   | t002       |
| t004 | RBAC: manager permissions to create/watch kpack `Image`/`Build` CRs                                         | 30m  | t001       |
| t005 | Render parity: verify `spec.builder: buildpack` round-trips consistently across REST/GraphQL/MCP/UI          | 45m  | t002, t003 |
| t006 | Simplify                                                                                                     | 30m  | t005       |
| t007 | Replace `build_test.go`'s "buildpack rejected" assertion with real dispatch test coverage                    | 1.5h | t005       |
| t008 | Closeout                                                                                                     | 15m  | t007       |

## Definition of done

Deploying an App with `spec.builder: buildpack` against a Dockerfile-less, buildpack-detectable repo (e.g. a plain `go.mod` or `package.json` app) produces a running, healthy revision end-to-end — verified against a real repo, not a mock.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `lego/operator/internal/build/build.go:24-27,164` and `docs/ADR004-deployment.md` both explicitly document this as a deferred "not yet, wait for kpack" contract; `BEX_CNB_BUILDER` (see root `CLAUDE.md` env table) and the `spec.builder` enum have been sitting ready since `w1/m5` (`.pm/w1/done/m5/README.md:38` records `CNBBuilder` was kept on purpose, "reserved for kpack (a 'not yet', not dead code)"). Materialized under `w6` **per user direction**, same as `w6/m19`-`m21`.
- **Goal linkage:** Render parity — Render's core value prop is buildpack auto-detect with zero Dockerfile; bex is Dockerfile-only today.
- **Expected outcome:** repos without a Dockerfile become deployable, matching Render's headline onboarding flow.
- **Why now:** the contract (CRD enum, env var, error message) has been ready since `w1/m5`; this is the largest of the four `/pm-brainstorm more milestones` (2026-07-13) proposals — a new cluster-wide infra dependency (kpack) — so it's sequenced last among them, but it's real, substantial, unbuilt Render-parity work, not a non-goal. Render parity included — the new `spec.builder` value must round-trip consistently across REST/GraphQL/MCP/UI.
