# w8 · m61 — Honor application PATH during native builds

**Worker:** worker8 **Goal:** exact application PATH reaches native build and runtime commands independently of platform helpers and inherited image entrypoints. **Status:** todo — build-loader fixes and narrow/normal/omitted controls pass; empty-PATH Node runtime failure is confirmed; t007 and final acceptance remain open.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve application PATH across both native preparation stages — **DONE 2026-10-12** | 45m | — |
| t007 | Remove inherited relative-entrypoint dependency — **TODO: empty-PATH runtime fails** | 30m | t001 |
| t002 | Audit native consumers and verify hosted recovery | 30m | t007 |
| t003 | Render parity — **TODO: Python and Bex Node pass; complete surface/control acceptance remains** | 20m | t002 |
| t004 | Simplify — **REOPENED for runtime entrypoint correction** | 15m | t003 |
| t005 | Test coverage — **REOPENED for image startup regression** | 35m | t003 |
| t006 | Closeout — **BLOCKED** | 20m | t004, t005 |

## Definition of done

- Repeat the [finding](finding.md) with fresh owned Free Python fixtures through published/current Bex and the unmodified Render v2.27.0 pin. `PATH=/usr/local/bin` plus a second variable must reach Live and HTTP 200 with the absolute Python build/start commands unchanged.
- Repeat the [Node control](finding.md#node-hosted-control--sweep166): PATH alone and PATH plus the later marker both become Live with the original narrow PATH. The build/start commands and source commit remain unchanged.
- Build and runtime both report the exact custom PATH and retain the other environment value and file bytes. Adding another variable must not stop the platform loader. Verify normal PATH, PATH alone, omitted PATH and explicit empty PATH controls.
- Literal App/Blueprint PATH must also reach the build: preparation can start its shell and use its own codec tools while preserving the literal-over-Secret precedence and the user's bytes. Hosted Blueprint apply is currently unverified; obtain an owned acceptance route or retain a clearly stated limitation.
- Complete the native caller/runtime census, byte codec, release/cache and cross-surface checks. No global PATH rewrite, new reserved-name policy, CLI request workaround or builder-version change is introduced.
- Delete each owned fixture by its recorded ID and verify detail/list/public absence, baseline preservation and quota reconciliation. Record any unverified Kubernetes descendant teardown explicitly.
- All tasks and observable checks are complete before moving the milestone to `done/`.

## Progress / block

- **2026-10-13 UTC, sweep183:** [fresh Python live acceptance](evidence/live-python-sweep183/README.md) reproduces both initial narrow-PATH build failures through released Bex and pinned Render. Changing only PATH through owned save-only setup, then native deployment of the exact initial commit, produces Live/HTTP200 and matching build/runtime PATH plus variable/file bytes in both controls. Seven targeted local tests pass; current source has 137820b94, exact deployed SHA unknown. Both fixtures/four child records reconciled, baseline6/quota termination0, own notes/config copies removed. Existing m61 stays blocked; no duplicate issue or QA implementation/ship.

- **2026-10-12:** the native RUN loader resolves its decoder once under the image PATH, and the preparer carries literals under private `BEX_NATIVE_LITERAL_<i>` names, so a custom PATH (Secret-backed or literal) reaches the build command exactly while platform helpers keep working (`lego/operator/internal/build/native.go`). `TestNativeEnvironmentKeepsApplicationPATH` fails on the original source and passes fixed; operator `make test`/`make lint` pass.
- **Current remaining work:** t007 repairs the newly confirmed empty-PATH native image-startup boundary; t002–t006 retain complete runtime/literal/surface/caller acceptance and reopened quality/regressions. Narrow Python/Node, normal Bex and fresh-checkout omitted-PATH controls pass. Prior completion receipts remain historical; deployed SHA is unknown.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs-cli` in w8, sweep163, 2026-10-12 UTC. [Finding](finding.md) and [sanitized evidence](../evidence/m61/README.md).
- **Goal linkage:** ADR008 core hosting, ADR004 source builds and ADR018 Render compatibility. Accepted application configuration must not stop the platform before the user command runs.
- **Expected outcome:** a custom application PATH works for an absolute build command, independently of the names or order of other configured variables.
- **Why now:** two fresh native clients produce `build_failed`; changing only PATH and redeploying the same commit recovers both. Actual generated-loader and generated-preparer executions identify two affected stages.
- **Render parity included:** environment configuration is exposed through REST, GraphQL, MCP and the dashboard, although the correction belongs to the operator. The current official documentation supports build/runtime availability; this specific experiment was not run against Render production.
- **Sizing:** originally 165 minutes across six tasks; now 195 minutes across seven tasks with t007, including both preparation stages, shared consumers and closing checks. This is filing only; no product fix or ship was performed by the hunt.

## Native Python acceptance — sweep210, 2026-10-13 UTC

[Two native Python passes and complete cleanup](evidence/live-python-sweep210/README.md) now prove exact narrow PATH and marker/file bytes at build/runtime without the recovery workaround. Published Bex 0.3.2 and pinned Render both reach Live/HTTP 200, including reversed env argument order. Deployed SHA is unknown. Both recorded fixtures and their initial child deploys are reconciled; final quota is baseline 6/termination 0. The source repair is on main, and this removes the prior Python rollout blocker. Remaining Node, neighbor-control, literal-Blueprint, current-build and surface/caller checks still prevent closeout. No duplicate finding or QA implementation/ship.

## Native Node acceptance — sweep211, 2026-10-13 UTC

[Node PATH-alone and additive-marker passes](evidence/live-node-sweep211/README.md): published Bex reaches Live/HTTP 200 in both phases with unchanged absolute Node commands, exact /usr/local/bin and matching file/marker hashes. Exact-owned save-only API setup adds only the marker; native deployment of the same initial source commit refreshes the build snapshot. One fixture/two child deploys are enumerated and deleted; baseline 6/quota 6/term 0, own local input/configs removed. This completes the original Bex Node control, while normal/omitted/empty PATH, hosted literal Blueprint, current-build create and remaining runtime/surface/descendant checks stay open. Deployed SHA unknown; no QA implementation or ship.

The empty-PATH runtime gate requires Live/HTTP200 with PATH still empty through both native clients, after a native image startup correction. A successful RUN helper, normal-PATH recovery or source-only image-config inference does not close that gate.

## Empty-PATH Node runtime failure — sweep216, 2026-10-13 UTC

[Complete native evidence and cleanup](evidence/live-node-sweep216/README.md) proves the remaining runtime failure through published Bex 0.3.2 and unmodified Render 2.27/a764810a7682. Both accept PATH= and finish building, but the inherited docker-entrypoint.sh cannot start. Render reaches update_failed/HTTP503; Bex is captured stalled with the same startup event, then superseded by the recovery control. Changing only PATH and natively deploying each same original commit reaches Live/HTTP200 and exact marker/file bytes. Fresh checkout omitted-PATH and published normal-PATH controls pass. All four fixtures/six child records are deleted, baseline/quota six restored; deployed SHA remains unknown.

The exact source-pinned Node OCI index declares relative ENTRYPOINT docker-entrypoint.sh on Linux amd64 and arm64 and copies it to /usr/local/bin. nativeDockerfile in lego/operator/internal/build/native.go:234 inherits it while installing an absolute CMD; appContainer in lego/operator/internal/controller/deployment_projection.go:245 keeps the native image entrypoint. The current generated-shell tests pass even for empty PATH because RUN does not exercise ENTRYPOINT. This is an additional native runtime boundary, within the original empty-PATH DoD. Source-pinned metadata is not mislabeled as inspection of the deployed artifact.

Add t007 (30m): make the native image's authored start command independent of a toolchain image's inherited relative entrypoint. Evaluate clearing that ENTRYPOINT for native-generated images, preserve the exact configured PATH and absolute CMD, and audit all six native toolchains/App types. Opaque Docker and buildpack image launch behavior stays separately governed. Reopen t004/t005 for quality and actual image-startup regression coverage; prior helper completion remains historical. No product fix or ship by QA. Hosted literal Blueprint, all hosted runtimes, full surfaces and descendant teardown remain unverified.

## Reopened after independent upstream closure — sweep218

Commit 9f507a2a7 closed the prior build-helper scope using Python/Node narrow-PATH and generated-shell controls, with hosted empty PATH explicitly unexercised. Sweep216 now supplies two native empty-PATH image-startup failures and same-commit PATH-only recovery. Native source still inherits the relative Node entrypoint. The original empty-PATH runtime DoD is unmet, so t007 and t002–t006 remain active; the upstream receipts are preserved as historical text in evidence/upstream-closeout-9f507a2a7. No duplicate milestone or QA source fix is created.
