# w8 · m129 — Render's free-plan "migrate in the build command" recipe works on bex

**Worker:** worker8 **Goal:** the build gets the service's `fromDatabase`/`fromService` variables and can reach its own workspace's datastores, as on Render **Status:** todo（2026-10-29 解除阻塞；原记录：promoted from 158 on 2026-10-21; **BLOCKED — needs the user's sign-off on the threat review in t001: letting tenant build code reach that workspace's own datastores changes the `bex-build` default-deny posture (ADR022:239); ADR018 allows no divergence without it, so the cheaper keep-the-boundary option cannot be chosen unilaterally**)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Threat review and ADR022 addendum: scoped build→own-workspace datastore reach | 30m | — |
| t002 | Project fromDatabase/fromService values into native and Dockerfile builds through the secret-mount path, never as Job-spec literals | 60m | t001 |
| t003 | Generate a per-build egress NetworkPolicy to the owning workspace's datastore endpoints only | 60m | t001 |
| t004 | Operator tests: build env carries DATABASE_URL via a mount; no literal in the Job spec; no reach to another workspace | 40m | t002, t003 |
| t005 | Blueprint plan/validate and docs: ADR018 Pre-Deploy row, ADR060, build-boundary.yaml comments | 25m | t004 |
| t006 | Render parity: compare against render-examples/django and Rails free-plan migrate-in-build across REST/GraphQL/MCP/dashboard | 20m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 30m | t007 |
| t009 | QA on production (needs the release pipeline): django example migrates in the build and goes live; cross-workspace negative control | 45m | t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

`render-examples/django` (`./build.sh` runs `manage.py migrate` against `DATABASE_URL` from `fromDatabase`) goes live on a free workspace; the native and Dockerfile build env carry the value through a secret mount (never in the Job spec); a build in one workspace cannot reach another workspace's database; ADR018/ADR022 record the decision.

## Source + Goal linkage

- **Source:** [w8/158 note, done/158.md](../done/158.md) from `/qa-find-bugs-cli` sweep 1274 (root cause: `operator/internal/build/native.go` `BuildLiterals` skips `ValueFrom`; `deploy/gitops/base/build-boundary.yaml` allows only DNS, registry and public ports).
- **Goal linkage:** Render parity first (ADR008): Render's documented Django and Rails free-plan setup fails its first build on bex.
- **Expected outcome:** Render apps that migrate in the build command import and deploy unchanged.
- **Why now:** major parity gap that fails every such app's first build. Render parity task included (tenant-facing build behavior); the pre-deploy alternative stays available on every plan.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m129
- **结论来源：** 现有规则（AGENTS.md：Render parity 优先；与 parity 冲突的 ADR 或加固决定要修改，不能因旧文档保留差异）。
- **结论：** 采用本 milestone 已写明的限定方案：build 只能访问**所属 workspace 自己的**数据库和服务（每次 build 生成的 egress NetworkPolicy 只放行这些 endpoint），`fromDatabase`/`fromService` 的值只经 secret mount 进入 build，绝不作为 Job spec 字面量。t001 由 worker 写完威胁评审，并作为 ADR022 的增补（修订 ADR022:239 的 build default-deny），不再等用户签字。
- **不变的边界：** 跨 workspace 访问仍然禁止；t004 必须证明一个 workspace 的 build 连不到另一个 workspace 的数据库。威胁评审如果发现限定方案不能保证这一点，worker 应停下，重新回到 blocked 并说明原因。
- **用户门槛：** 无。
- **worker 下一步：** loopx 依次执行 t001–t007；验收以 `render-examples/django` 在 free workspace 首次 build 即迁移成功、且跨 workspace 不可达为准。
