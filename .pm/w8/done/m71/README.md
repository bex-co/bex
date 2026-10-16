# w8 · m71 — Make explicit Bun opt-in usable in native builds and runtime

**Worker:** worker8 **Goal:** a Node/Bun service that explicitly requests Bun can build and serve through the unchanged pinned CLI with a resolved, reported Bun toolchain. **Status:** done — implementation760502dad and sweep327 two-client first-Bun-Live acceptance, fresh omission control, current source/execution checks and complete cleanup verified

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select optional Bun from version and opt-in signals — **DONE** | 40m | — |
| t002 | Carry the selected Bun into native build and runtime images — **DONE** | 45m | t001 |
| t003 | Audit shared consumers and version/cache transitions — **DONE** | 25m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 10m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | [Closeout](done/t007.md) — **DONE** | 20m | t005, t006 |

## Definition of done

- Fresh published Bex and unchanged pinned Render clients create Free native Node services with the finding's explicit Bun request and authored commands. Both reach Live, actual build and HTTP output identify the supported resolved Bun version/marker, and no user-installed workaround is required. Preserve a normal Node/no-opt-in control.
- Maintained execution regressions fail before the fix and pass after for version/file/lock opt-ins, omission, supported/unsupported requests, primary language coexistence, shared callers and version/cache transitions. Exercise generated build/runtime artifacts and preserve m51/m61 guarantees; text-only Dockerfile assertions do not close this gate.
- Existing REST/GraphQL/MCP/Blueprint/dashboard env settings and readback lead to the same operator behavior; exact upstream CLI/node wire stays unchanged. Relevant module and reviewed-toolchain inventory checks pass. Record untested live variants honestly.
- Both owned parents, enumerated deployments and any new fixture dependencies are cleaned, immutable baseline/quota retained. Complete every task and actual live acceptance before moving the milestone to done.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep306,2026-10-15 UTC. [Finding](finding.md) · [complete receipts](evidence/cli-sweep306/README.md).
- **Goal linkage:** ADR008 dependable Render-alternative hosting; ADR018 native-runtime compatibility and ADR060 D7's reviewed toolchain selection.
- **Expected outcome:** the existing BUN_VERSION opt-in supplies Bun for both native build and the resulting service image.
- **Why now:** two valid native creates preserve the setting yet fail with missing Bun/exit127; ordinary Node controls prove env delivery and show ENOENT at build and runtime. The current selector and generated images have no Bun setup.
- **Sizing:**200m/seven tasks. Optional version selection, actual toolchain materialization, shared/cache behavior and hosted acceptance exceed a sub-hour note.
- **Render parity included:** this changes native execution behind existing public configuration surfaces. Official Bun opt-in is the target; no authenticated Render run or complete utility parity is claimed.

The original filing scheduled the missing-toolchain fix. Implementation760502dad is now hosted-accepted in [sweep327](evidence/cli-sweep327/README.md): both native first Bun deployments and a separate no-opt-in Node control pass, current package/generated-image checks pass, and all three fixtures are cleaned. QA added evidence and board closeout only; no additional product implementation or ship is included.
