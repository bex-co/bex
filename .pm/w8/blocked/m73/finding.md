# Native Node builds cannot run the documented pnpm command

**Severity:** major — valid native Node hosting builds fail before application dependency work. **Owner:** operator native image assembly. Source: functional `$qa-find-bugs-cli` w8/sweep315,2026-10-15 UTC. [Complete receipts](evidence/cli-sweep315/README.md).

Published Bex0.3.2 and unchanged Render2.27/a764810a7682 each create an explicit Free Node24 web service in isolated bex-canary / tea-daif693dqjvc73e7as3g. Both unchanged requests return201/exit0 and preserve the requested runtime, plan, environment and exact build/start commands. Both initial builds fail with pnpm: command not found / BuildKit exit127. The command only checks pnpm --version before a standard-library marker program, so no lockfile, dependency compatibility or project installation can cause this failure.

```sh
bex services create --name "$FRESH_OWNED_NAME" --type web_service --runtime node \
  --repo https://github.com/expressjs/serve-static --branch master \
  --build-command 'pnpm --version' \
  --start-command 'node -e "require("node:http").createServer((q,r)=>r.end("ready")).listen(Number(process.env.PORT),"0.0.0.0")"' \
  --env-var NODE_VERSION=24 --plan free --region frankfurt \
  --auto-deploy=false --confirm -o json
```

This shortened reproduction preserves the failing command; full original markers, Node snapshot code, exact argv and raw native requests are in the evidence. Use a fresh owned name and isolated workspace/config. Repeat using the unchanged Render pin against Bex.

Both same-service native Node-only control updates reach Live/HTTP200 with matching build/runtime markers and actual Node v24.21.0. Both phases return pnpm ENOENT while npm11.19.0, Yarn1.22.22 and Corepack0.36.0 work. Thus primary version selection, normal toolchain PATH, command bytes and env delivery pass. The pnpm executable is missing at build and runtime. No client repair, npm/Corepack installer workaround, explicit PATH override or Bun opt-in is used.

[Render's native runtime inventory](https://render.com/docs/native-runtimes) includes pnpm during build and runtime, and [its deploy examples](https://render.com/docs/deploys#build-command) include pnpm install for Node. Target: a supported reported pnpm executable on the normal Node toolchain PATH before an authored build and retained at runtime; npm/Yarn controls continue working. No authenticated Render-hosting measurement is claimed.

## Producer, consumer and concrete correction

- Native create wire is exact: pinned cmd/servicecreate.go → pkg/service/create.go carries ordinary NODE_VERSION and native build/start fields. Backend `apps/rest.go:479,484,567–569` maps them into CreateRequest; GraphQL `apps/graphql.go:720–722`, MCP `apps/mcp.go:270–272,360–362` and Blueprint `apps/deploy.go:2603–2605` feed the same App contract. No new runtime enum/CLI flag or fork is needed.
- `lego/operator/internal/controller/app_controller.go:1482–1496` chooses native for supported languages and static commands. The build options at1036–1047 carry runtime, exact commands, env revision/files and resolver image. `build/native.go:148` uses Node as static's implicit toolchain.
- `native.go:92–96` inventories official Node22/24/26 images. `nativeDockerfile:234–274` copies source/loader then immediately runs the authored build, retaining the base for runtime; `nativeBuildPreparer:321` supplies this generated file. No pnpm installation or shim activation exists. `nativeresolve.go:48–69` selects/replaces only FROM, not the package tool layer. Six real source artifact observations confirm this for Node22/24/26 × web/static.
- Add a consistent reviewed pnpm setup for native Node artifacts before the authored command, retaining its executable/dependencies in the resulting service image. A compatible pinned pnpm installation into the normal Node bin path can cover all reviewed lines without replacing Yarn. Reuse the existing image/selection assembly; do not depend on users manually invoking corepack enable. Keep static extraction behavior and no runtime installer prerequisite.
- If using Corepack instead, explicitly provide and activate a reviewed compatible Corepack/pnpm combination for every supported line. [Corepack's primary README](https://github.com/nodejs/corepack#default-installs) says bundled Corepack stops before Node25 and activation creates the pnpm/Yarn launchers. Live Node24 has Corepack but no pnpm launcher; source-selected Node26 cannot be assumed to bundle it. [pnpm's installation guide](https://pnpm.io/installation) describes compatible installation options. The final implementation should select one consistent approach and execute it in tests.
- Preserve m51 primary-version choice/narration, m61 exact commands/default/narrow/empty PATH and ENTRYPOINT behavior, env/file values, non-Node generators and existing npm/Yarn command resolution. pnpm artifact identity must participate in existing generated/cache inputs; test stable/change transitions rather than inventing a new hash. Do not generalize optional companion-tool policy or packageManager-version semantics without separate evidence.

## Shared scope, dedupe and limits

All native compute families share the generator (web, private, worker, cron) and static uses its Node default. Node24 web is the only live package-manager case here. Node22/26/static are source-generated observations only, not hosted runtime proof. Render's all-native utility inventory is broader than this Node finding; other native families' pnpm availability is unexercised and must not be claimed fixed by changing Node alone.

Searched all open/blocked/done board items, open milestone READMEs, DO_NOT_DO, compatibility ledger and targeted native source history. No pnpm availability issue exists. Related [m71 Bun](../../done/m71/README.md) provisions a separately requested JavaScript engine with explicit opt-in/version selection; [m72 Poetry](../m72/README.md) concerns Python tooling. Neither supplies the ordinary pnpm command, which is a distinct Node package-manager gap without any opt-in setting. Coordinate shared image changes and reuse materialization where appropriate; no duplicate Bun/Poetry task is filed. Closed m51 primary-version and m61 PATH/entrypoint behavior pass controls;052 correctly reports127. This is an uncovered omission, not a claim those fixes regressed or that all their original live DoDs were replayed. Build-cache package download caches are deliberately deferred by ADR060 and are outside this fix.

Estimate155m/six tasks: pnpm setup55, shared/cache audit20, parity15, simplify10, meaningful execution coverage35, live closeout20. No paid fixtures or cron are authorized in this hunt. Other runtime live coverage, package install/lockfile behavior, packageManager version dispatch, exact serving SHA, fixed hosted build, full operator suite and frontend write journeys remain unverified. Both owned parents/four deployment children are fully cleaned with baseline/quota preserved. This schedules the repair; product code is unchanged.
