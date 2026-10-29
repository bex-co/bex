# w8 · m129 — Render's free-plan "migrate in the build command" recipe works on bex

**Worker:** worker8 **Goal:** the build gets the service's `fromDatabase`/`fromService` variables and can reach its own workspace's datastores, as on Render **Status:** in progress — t001–t008 done 2026-10-29 (code + docs + tests, see Progress below); t009 needs the shipped operator on production (2026-10-29 解除阻塞；原记录：promoted from 158 on 2026-10-21; **BLOCKED — needs the user's sign-off on the threat review in t001: letting tenant build code reach that workspace's own datastores changes the `bex-build` default-deny posture (ADR022:239); ADR018 allows no divergence without it, so the cheaper keep-the-boundary option cannot be chosen unilaterally**)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Threat review and ADR022 addendum: scoped build→own-workspace datastore reach — **DONE** | 30m | — |
| [t002](done/t002.md) | Project fromDatabase/fromService values into native and Dockerfile builds through the secret-mount path, never as Job-spec literals — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Generate a per-build egress NetworkPolicy to the owning workspace's datastore endpoints only — **DONE** | 60m | t001 |
| [t004](done/t004.md) | Operator tests: build env carries DATABASE_URL via a mount; no literal in the Job spec; no reach to another workspace — **DONE** | 40m | t002, t003 |
| [t005](done/t005.md) | Blueprint plan/validate and docs: ADR018 Pre-Deploy row, ADR060, build-boundary.yaml comments — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Render parity: compare against render-examples/django and Rails free-plan migrate-in-build across REST/GraphQL/MCP/dashboard — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
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

## Progress (2026-10-29, worker8): t001–t008 shipped in code, t009 needs production

**What shipped (operator only; no backend, API or dashboard change):**

- **t001:** threat review and amendment in [ADR022 § Build reach to the owning workspace's datastores](../../../docs/ADR022-tenant-isolation.md#build-reach-to-the-owning-workspaces-datastores-w8m129-2026-10-29). It cites this unblock record and amends the build default-deny sentence. The review found that the scoped design keeps cross-workspace reach closed by construction, so there was no reason to stop.
- **t002:** `projectNativeBuildEnv` resolves every `spec.env` `SecretKeyRef` (`fromDatabase`, Key Value `fromService`) from the App namespace. It copies only the referenced key into the App-owned merged env Secret (`<app>-native-env`) that native and Dockerfile builds already mount (`readBuildEnvRefs`, `lego/operator/internal/controller/build_datastore_reach.go`). The Job spec names only the Secret. Protected Secrets are refused. A build waits as `BuildQueued` while the connection Secret is missing or the linked Database is `Pending`/`Provisioning` (`buildDatabasePending`). kpack is unchanged (w1/121).
- **t003:** `reconcileBuildDatastoreReach` converges two NetworkPolicies at dispatch and deletes them at the build's terminal observation:
  - `bex-build/bld-<app>-datastore-egress`: the pod selector is `app.bex.co/build`, `app-uid`, `component=build` and `workspace`, all operator-set. Egress goes only to `kubernetes.io/metadata.name=<App ns>` on TCP 5432/6379/6380.
  - `<ws>/<app>-build-ingress`: owned by the App. It admits exactly those pods from `bex-build` on the same ports.
  - Both use the isolation-aware target selector (w8/m136). Only canonical namespaces (`ns == workspace label`) are eligible. The finalizer reclaims the build-namespace half (component `execution-network-policy`).
- **t004/t008:** tests.
  - Unit tests in `build_datastore_reach_test.go` cover four things. DATABASE_URL arrives in the merged Secret for native and Dockerfile builds, and the marshalled Job has no URI or password. The ref read contract (pending / protected / missing key / later duplicate wins / PORT dropped) and the database-phase hold both hold. A NetworkPolicy evaluator over the generated objects proves A→A allowed and A→B, B→A, an isolated environment, a non-datastore port, platform namespaces, forged UIDs and publish pods all denied. Revocation is ownership-gated, the egress policy fails closed on a foreign same-named policy, and ineligible namespaces get no grant.
  - Envtest `build_datastore_reach_envtest_test.go` runs the reconciler end to end: hold without the Secret, dispatch after it, DATABASE_URL only via the mount, the policies select the real Job's pods, both are removed after the build completes.
- **t005:** [ADR018](../../../docs/ADR018-render-parity.md) Pre-Deploy row, an [ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md) addendum, `deploy/gitops/base/build-boundary.yaml` comments, and the `BEX_BUILD_NAMESPACE` row in `lego/operator/AGENTS.md`. Blueprint plan/validate needs no change: no warning ever told users that builds cannot reach the database, and the same `envVars`/`buildCommand` now simply work.
- **t006:** parity. REST, GraphQL, MCP and dashboard shapes are unchanged: the build env and network are platform runtime behavior, and no field was added or removed. Render's django `build.sh` (`pip install`, `collectstatic`, `migrate`) and the Rails free-plan `bundle exec rake db:migrate` in the build command both need exactly DATABASE_URL plus reach, which they now get.

**Open gaps (not signed off as divergences; follow-ups to file):**

1. A build cannot reach private web/private services (`fromService` `host`/`hostport`); only datastore ports are open.
2. kpack (`builder: buildpack`) builds still get no Secret-backed variables (w1/121).
3. Cancel leaves the policy pair in place until the App's next build ends or the App is deleted. It is inert, since it selects only that App lifetime's build pods.

**t009 must check on production after the operator ships:**

- Import `render-examples/django` as a Blueprint on a **free** workspace (with `PYTHON_VERSION=3.12.11`; w8/159 is done). The first build waits for `mysitedb` and then runs `manage.py migrate` successfully. The service goes `live`, and `/admin/login/` returns 200.
- `kubectl -n bex-build get job <bld-…> -o yaml` shows no connection string. `kubectl -n bex-build get secret <srv>-native-env` holds DATABASE_URL.
- While the build runs, `bld-<srv>-datastore-egress` (bex-build) and `<srv>-build-ingress` (workspace ns) exist. Both are gone after the build ends.
- Negative control: a second workspace's build with a `buildCommand` such as `pg_isready -h <dpg-of-ws1>-rw.<ws1>.svc -p 5432 || nc -zvw3 …` must fail to connect, both by the internal DNS name and by pod IP. In the same run, the workspace's own database must succeed.
- Hubble/cilium monitor shows the denied cross-workspace flow as a policy drop.
- Delete all fixtures by id with 404 proofs.
