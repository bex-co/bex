# Native Git builds omit required submodule files

Why: a valid repository's pinned dependency files never reach the build, so a normal native deployment fails even though both the parent and child repositories are publicly readable.

**Severity:** major. **Status:** researched, todo. **Ownership:** operator source-fetch phase. Functional CLI QA sweep393,2026-10-16 local/17 UTC. Published Bex0.3.2/a6350da0d6e4 and unchanged Render2.27.0/a764810a7682 against api.bex.co/v1, isolated human QA in bex-canary/tea-daif693dqjvc73e7as3g. Source009a9409a48bcc5d241d87b0460ba5b0390698a4; actual serving revision unknown. This schedules a fix; QA made no product implementation.

## Reproduction and behavior

[Complete evidence](evidence/cli-sweep393/README.md). Each actual native client creates its own explicit Free Node24 web service from public git-fixtures/submodule/master, auto-deploy off. Both unchanged create POSTs return201/native0 and record the exact returned ID/name/workspace/plan against absent-before inventories. Both first deployment records identify parent b685400c1f9316f350965a5993d350bc746b0bf4 and become build_failed. Creation is asynchronous: the defect is the failed deployment, not a claimed nonzero create exit.

The parent has an ordinary top-level HTTPS Gitlink `basic`, mode160000, pin6ecf0ef2c2dffb796033e5a02219af86ec6584e5 at https://github.com/git-fixtures/basic.git. Independent anonymous Git initialization retrieves basic/LICENSE1072B, SHA25620b064910b32bce1bc04595a5b20477a28c503995ef8528929a311d6cb7a3b09. The authored no-dependency Node build fixture checks original parent .gitmodules/README hashes, then reads that harmless child file. Both actual native build logs prove exact parent hashes, the own nonce and child_file_exists=false before the check exits1. Neither original first child becomes Live. The parent also declares an older itself Gitlink; only the top-level basic file is consumed. No recursive/nested claim is made.

The exact commands, native wire bodies and full authored program are retained. The command shape is:

```sh
bex services create --name OWN_NONCE_NAME --type web_service --runtime node \
  --repo https://github.com/git-fixtures/submodule --branch master \
  --env-var NODE_VERSION=24 --build-command '<complete original check in evidence>' \
  --start-command 'node qa393-app.cjs runtime' --plan free --region frankfurt \
  --health-check-path / --auto-deploy=false --confirm -o json
```

The unchanged Render binary uses the same command/flags in its separate config against Bex. Twelve actual helps pass. The QA app is authored; no existing application from the fixture repository is claimed to run, and no repository is edited remotely.

[Render's Git contract](https://render.com/docs/git-provider#git-submodules) automatically reads a root .gitmodules and clones its specified submodules before building. [Git's update contract](https://git-scm.com/docs/git-submodule) checks out the child commit recorded by the parent and resolves relative URLs through the parent's remote. Merely checking out the parent leaves Gitlink directories unpopulated.

## Controlled restoration

Each actual native client changes only buildCommand to prepend an explicit anonymous fetch of the exact parent-pinned child commit. The original source/ref/check/app/start/Node setting and Free plan remain. Both native updates exit0 and produce exactly one new child each. Both second children become Live on the same parent commit; three real HTTPS reads per service return200 with the original parent hashes, exact child file bytes/hash, own marker and stable observed boot. Native build logs independently show the restored child. Twelve REST/GraphQL/MCP command/source reads plus four environment reads agree across the original and control states.

This explicit child fetch is a diagnostic workaround. It is not the platform repair and must not be required in the fixed first-deployment acceptance. No first failure is hidden by a branch correction, remote source change or replacement application.

## Root cause and concrete repair

The actual exported BuildJob clone initContainer, executed locally with its source-pinned alpine/git2.43 image, exits0 with the exact parent but no child file. Adding standard `git submodule update --init -- basic` to that actual command retrieves the exact expected file while preserving the parent. Both owned clone containers/source directories and the temporary Go export package are removed. A separate pinned Node24 prerequisite proves missing-file build1, exact explicit-fetch build0 and matching real runtime HTTP200; its three containers/context are removed. These are actual source-generator and runtime controls, not a product patch or full Kubernetes execution.

- `lego/operator/internal/build/build.go:1401–1457` generates the clone. Lines1435–1448 initialize/fetch/check out only the parent and verify EXPECTED_COMMIT. There is no submodule update. Source checks1454–1456 then run, and1457 removes .git. A later ordinary `git submodule update` inside the authored build cannot recover the now-removed parent metadata.
- `BuildJob` at926/1034 adds that clone before nativeBuildPreparer at1132 and the BuildKit phase. `native.go:348` materializes the generated native recipe; COPY uses the incomplete source. The correct boundary is the shared fetch step, before source-path checks and metadata removal.
- `EnsureBuild` at601–619 sends native and Dockerfile builds through BuildJob. Native languages and native static artifacts share this source producer; only Free Node24 is hosted here. BuilderBuildpack branches to ensureBuildpack at607/kpack.go:142 and needs a separate boundary review; no buildpack failure or unconditional kpack rewrite is claimed. Prebuilt images bypass source building.

Initialize supported top-level Gitlinks at their parent-pinned commits after verifying the parent and before source-path checks/.git removal. Use the existing Git credential transport and failure classification; retain normal source/ref/cache/metadata behavior. Do not use --remote to replace pinned commits, add a CLI/API field, fork request construction, require an authored child fetch, or silently ignore an unavailable child. No tool/image bump is needed for the demonstrated case: the existing immutable Git image passes the local initializer control. Nested recursion is a separate, unproven requirement.

## Acceptance and limits

Meaningful regressions must execute real clone behavior: ordinary HTTPS and relative URLs, a child pin differing from its branch tip, root/context/Dockerfile paths inside a module, no-module controls, unavailable child and wrong expected-parent failures. Prove correct file/commit contents and visible classified failures, not just a command-string snapshot. Preserve shared native/Dockerfile behavior and audit the separate buildpack path without assuming it fails.

Fresh published Bex and unchanged Render-pin first deployments must use the original file check and reach Live/HTTP200 with the exact child, without the diagnostic fetch prefix. Verify shared configuration and source/log/runtime outcomes, original baselines and exact cleanup. Source suites/lint and broader hosted variants are not executed by this QA and remain implementation acceptance work. Actual serving OCI and physical teardown are unobserved.

Dedupe searched all open/blocked/done workstreams and docs for Gitlinks, submodules and recursive clone terms; the sole match is an unrelated worker5 plain-source-copy note. This is separate from Poetry/uv tool availability, Blueprint branch selection and runtime environment defaults.

## Cleanup

Both native deletes exit0. Two services, all four recorded children and former URLs are404/listabsent; baseline six services/one Blueprint, empty groups/projects/managed data and zero termination are preserved. Relays/copied configs/request files, local containers/source contexts and temporary Go export package are removed. The isolated login remains for the continuing authorized loop. The evidence/filing is uncommitted; no shared rollout, paid operation or existing-resource mutation occurred.
