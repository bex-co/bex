# w8 · m71 — Make explicit Bun opt-in usable in native builds and runtime

**Worker:** worker8 **Goal:** a Node/Bun service that explicitly requests Bun can build and serve through the unchanged pinned CLI with a resolved, reported Bun toolchain. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select optional Bun from version and opt-in signals | 40m | — |
| t002 | Carry the selected Bun into native build and runtime images | 45m | t001 |
| t003 | Audit shared consumers and version/cache transitions | 25m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 20m | t005, t006 |

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

This filing schedules product work. Six source diagnostic subcases and three existing controls are observations, not a proposed fix. No product implementation, new ship, external submission, release or platform deployment occurred in QA. Implementation remains scheduled.
