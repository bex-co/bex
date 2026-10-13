# w8 · m61 — Honor application PATH during native builds

**Worker:** worker8 **Goal:** native build helpers finish their work before handing the application its exact configured PATH. **Status:** blocked — source loader/preparer work and tests are complete; sweep183 still reproduces both narrow-PATH Python failures, with both same-commit broadened-PATH controls passing. Complete deployed acceptance remains required.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve application PATH across both native preparation stages — **DONE 2026-10-12** | 45m | — |
| t002 | Audit native consumers and verify hosted recovery — **BLOCKED: sweep183 narrow-PATH cases still fail; recovery controls pass** | 30m | t001 |
| t003 | Render parity — **BLOCKED: native Python evidence refreshed; complete surface/Node/control acceptance remains** | 20m | t002 |
| t004 | Simplify — **DONE 2026-10-12** | 15m | t003 |
| t005 | Test coverage — **DONE 2026-10-12** | 35m | t003 |
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
- **Blocked:** t002/t003 and t006 need the fix deployed to production, then fresh owned Free Python/Node fixtures through released Bex and pinned Render (PATH plus later variable, PATH alone, omitted/empty/full controls), surface checks and cleanup per the DoD.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs-cli` in w8, sweep163, 2026-10-12 UTC. [Finding](finding.md) and [sanitized evidence](../../evidence/m61/README.md).
- **Goal linkage:** ADR008 core hosting, ADR004 source builds and ADR018 Render compatibility. Accepted application configuration must not stop the platform before the user command runs.
- **Expected outcome:** a custom application PATH works for an absolute build command, independently of the names or order of other configured variables.
- **Why now:** two fresh native clients produce `build_failed`; changing only PATH and redeploying the same commit recovers both. Actual generated-loader and generated-preparer executions identify two affected stages.
- **Render parity included:** environment configuration is exposed through REST, GraphQL, MCP and the dashboard, although the correction belongs to the operator. The current official documentation supports build/runtime availability; this specific experiment was not run against Render production.
- **Sizing:** 165 minutes across six tasks, including both preparation stages, shared consumers and closing checks. This is filing only; no product fix or ship was performed by the hunt.
