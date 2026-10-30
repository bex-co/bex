# Native supplier consumer audit — 2026-10-29

This is a source-path audit of the combined m91 supplier mechanism, checked against main `902630131` plus this session's pending native changes. It is not hosted, browser, phase, or architecture execution evidence.

## Authoring paths

REST `apps/rest.go:513` normalizes Render's nested build/start/runtime fields into the neutral CreateRequest; GraphQL `apps/graphql.go:725` and MCP `apps/mcp.go:266` use the same request. `apps/service.go:3091` persists the authored commands in AppSpec. Blueprint compilation uses the same spec conversion (`apps/blueprint.go:677`) and retains authored build commands on export (`apps/blueprint_generate.go:402`). The dashboard new-service form passes build/start commands through its ordinary GraphQL inputs (`dashboard/src/features/services/hooks/use-new-service-form.ts:139`). No authoring schema, workspace condition, installer prefix, command rewrite, or CLI fork is introduced by the supplier.

Paths abbreviated as `apps/` above are under `lego/backend/internal/apps/`.

## Image assembly and retention

The controller passes AppSpec runtime/build/start/root/env/files directly to build.Options (`lego/operator/internal/controller/app_controller.go:1131`). BuildJob appends the preparer and resolver only for BuilderNative (`lego/operator/internal/build/build.go:1219`). The preparer writes nativeDockerfile; ResolveNativeBuildExact resolves primary/companions first, then applies the supplied base. The 27 official primary pins remain independent of the derivative catalog, so exact lookup and companion copies retain official provenance.

The replacement precedes source COPY and authored RUN. Exact primaries replace the complete `/usr/local` tree, restore verified image ENV, and restore primary-Python packaging offline before companion installation. An exact-source filesystem guard refuses incompatible Debian suites. Utilities and their ordinary system library/resource trees remain outside `/usr/local`; JavaScript utilities and their private interpreter are under `/opt/bex`. A supplied base neither adds a tenant apt step nor overrides LD_LIBRARY_PATH.

The same built image reaches runtime and the pre-deploy Job (`app_controller.go:6538` passes the resolved image). A native static build uses the same assembly before publish extraction, with no workload CMD. BuilderDockerfile, prebuilt image, BuilderBuildpack, and buildless static branches do not invoke the native supplier. Existing source-root, env loader/revision, secret-file mounts, cache arguments and start-command generation remain in their original path.

## Verification boundary

Mechanism regressions cover every reviewed primary, exact primary/companion ordering, invalid supplier pins, unavailable metadata, suite selection, and untouched unadopted behavior. Real generated exact OTP 28.0 and primary Python restoration checks are recorded separately. The factory checks every published architecture, but publication and an empty production catalog do not establish adoption. Actual adopted-image digests, hosted two-client archive builds, original source/data/log/HTTP proofs, phase/static/companion/env/cache execution and exclusion checks must be attached before the milestone closes.
