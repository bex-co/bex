# w4 · m598 — A sandbox terminal processes typed-ahead input at once, erases tabs, supports ECHOPRT and `tcflow()`, as Linux does

**Worker:** worker4 **Goal:** a PTY inside a bex sandbox handles canonical typeahead, tab erasure, `ECHOPRT` and `tcflow()` (`TCXONC`) the way Linux's `n_tty` line discipline does, closing the remaining differences left by `w4/m574`. Typing a line ahead while a program runs and then pressing Ctrl-C echoes and interrupts at once instead of waiting for a read; erasing a tab moves the cursor back to where the tab started; `ECHOPRT` echoes `\…/`; and `tcflow()` stops and starts output or sends `VSTOP`/`VSTART`. **Status:** blocked (t001–t005, t007–t011 done; t006 needs the user's DCO sign-off; t012 follows)

## Tasks (in order)

| id   | title                                                                                       | est   | depends_on                   |
| ---- | ------------------------------------------------------------------------------------------- | ----- | ---------------------------- |
| t001 | Canonical typeahead: multi-line read buffer with immediate input processing — **DONE**      | 1h30m | —                            |
| t002 | Erase a tab from the screen as Linux's `ECHO_OP_ERASE_TAB` does — **DONE**                  | 45m   | t001                         |
| t003 | Implement `ECHOPRT` hardcopy erase echo (`\…/`) — **DONE**                                  | 45m   | t001                         |
| t004 | Implement `TCXONC` (`tcflow()`) ioctl — **DONE**                                            | 45m   | t001                         |
| t005 | Extend the PTY probe and runsc CI check to typeahead, tab erase, ECHOPRT and flow — **DONE**| 30m   | t001, t002, t003, t004       |
| t006 | Send the fixes upstream to gVisor                                                           | 30m   | t001, t002, t003, t004       |
| t007 | Build `release-20260622.0-bex.3`, pin it and roll the sandbox pool — **DONE**               | 1h    | t001, t002, t003, t004, t005 |
| t008 | Live check: a production sandbox PTY matches Linux on typeahead and remaining gaps — **DONE** | 30m | t007                         |
| t009 | Render parity: Render Sandboxes and Devin's terminals, and update ADR104 / ADR018 — **DONE**| 20m   | t008                         |
| t010 | Simplify — **DONE**                                                                         | 15m   | t009                         |
| t011 | Test coverage — **DONE**                                                                    | 30m   | t009                         |
| t012 | Closeout                                                                                    | 15m   | t006, t010, t011             |

## Definition of done

- Each of the four fixes (t001–t004) has syscall tests in the fork's `test/syscalls/linux/pty.cc` (plus `devpts` unit tests where helpful). Each test passes on native Linux and on the patched `runsc`, and fails on `runsc` with only that fix reverted.
- On a production bex sandbox placed on a node running the new pin, `scripts/pty-signal-echo-probe.py` (extended by t005) reports 0 cases differing from Linux, including:
  - canonical typeahead: writing `ls\n`, `ab`, `\x03` with no reader echoes `ls\r\n`, `ab`, `^C`, delivers `SIGINT`, and flushes both lines;
  - tab erase, `ECHOPRT`, and `tcflow()` (`TCOOFF`/`TCOON`).
- `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` pins a `bex-co/gvisor` `release-20260622.0-bex.<n>` (or a stock release carrying every fix) by sha512, with binaries outside git, and every sandbox node runs it with no CAPI template drift.
- `.github/workflows/sandbox-runsc-pty.yml` fails against the `release-20260622.0-bex.2` pin and passes against the new one.
- Each fix has an upstream `gvisor/gvisor` pull request linked here (t006).
- [ADR104 § Terminals in a sandbox](../../../../docs/ADR104-sandbox-render-parity.md#terminals-in-a-sandbox-w4m545-w4m574-w4m598) and [ADR018](../../../../docs/ADR018-render-parity.md)'s sandbox terminal line discipline row drop every fixed difference.

## Source + Goal linkage

- **Source:** inbox note w4/723 ([`source-723.md`](source-723.md)), filed by `w4/m574` on 2026-11-22 after its 71-case line-discipline harness found 3 remaining differences against native Linux (typeahead, tab erase, `ECHOPRT`) plus missing `tcflow()` (`TCXONC`). Promoted by `/loopx w4` on 2026-11-23: four gVisor fixes with syscall tests, an upstream DCO step, a fork release build, a surge-first production pool roll, a live check, and ADR updates exceed an inbox note's ~1h budget across multiple tasks (same shape as `w4/m574`).
- **Premise checked 2026-11-23** in `bex-co/gvisor@bex/line-discipline` (`b66db2b09`, current release `release-20260622.0-bex.2` pinned in `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml:274`):
  - **No typeahead.** `pkg/sentry/fsimpl/devpts/line_discipline.go:650` still returns early (`if l.termios.LEnabled(linux.ICANON) && q.readable { return 0, false }`), and `queue.go:49,68,108,155-165` holds a single `readBuf` + `readable` boolean.
  - **No tab erase.** `line_discipline.go:795-796` has `else if toErase == '\t' { // Not implemented }`.
  - **No `ECHOPRT`.** `line_discipline.go:789-790` has `if l.termios.LEnabled(linux.ECHOPRT) { // Not implemented }`.
  - **No `tcflow()` (`TCXONC`).** Neither `pkg/sentry/fsimpl/devpts/master.go` (`:147-315`) nor `replica.go` (`:137-209`) handles `linux.TCXONC`.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)) and sandboxes ([ADR104](../../../../docs/ADR104-sandbox-render-parity.md)) run behind gVisor ([ADR042](../../../../docs/ADR042-sandbox-cluster-substrate.md)). Devin runs a real Linux kernel, so every difference in `devpts` shows up as sandbox-only terminal bugs or failures in tenant tests and interactive shells.
- **Expected outcome:** typing ahead and hitting Ctrl-C interrupts a running program immediately and echoes as Linux does; erasing tabs, `ECHOPRT`, and `tcflow()` match Linux; and ADR018's terminal row no longer carries unsigned-off divergences.
- **Why now:** `w4/m574` just shipped `release-20260622.0-bex.2`, so the fork branches, local test harness, reproducible go-branch build recipe, and surge-first pool roll runbook are fresh. Bundling tab erase, `ECHOPRT`, and `TCXONC` with typeahead closes all remaining recorded line-discipline gaps in one pool roll and one upstream review pass.
- **Render parity:** included (t009). Sandbox and agent terminals face tenants; Devin (the coding-agent reference) runs Linux `n_tty`, and Render's `sandboxes exec` has no TTY (`w4/m545` t005).

## Record 2026-11-23 (/loopx w4)

**Fixes (t001–t004).** Fork [bex-co/gvisor](https://github.com/bex-co/gvisor), four commits stacked on w4/m574's `bex/line-discipline` (`b66db2b09`), each with native-and-runsc syscall tests in `test/syscalls/linux/pty.cc` and Go unit tests in `pkg/sentry/fsimpl/devpts/devpts_test.go`:

| Fix | Upstream branch `bex/line-discipline-typeahead` | Release branch `bex/release-20260622.0-echo-signal-chars` | Per-fix branch | New syscall tests |
| --- | --- | --- | --- | --- |
| `devpts: Process canonical typeahead while unread lines wait` (t001) | `98933f332` | `d55192d` | `bex/line-discipline-6-typeahead` | `Typeahead*` (7) |
| `devpts: Erase tabs from the screen as Linux's ECHO_OP_ERASE_TAB does` (t002) | `dc017a7c9` | `b19d125` | `bex/line-discipline-7-tab-erase` | `EraseTab*` (3) |
| `devpts: Implement ECHOPRT hardcopy erase echo` (t003) | `f47b1fdcb` | `86cfa61` | `bex/line-discipline-8-echoprt` | `EchoPrt*` (3) |
| `devpts: Implement TCXONC (tcflow) ioctl` (t004) | `72ca9395b` | `e90ac89` | `bex/line-discipline-9-tcxonc` | `TcFlow*` (3) |

What each fix does:

- **Canonical typeahead (t001).** `queue.lineEnds` records completed canonical line end offsets (including zero-length EOF lines) so `inputQueueTransformer.transform()` continues processing echo, signals, IXON flow control, and line editing while completed lines wait unread. Canonical `queue.read()` returns at most up to `lineEnds[0]` (popping an empty EOF as a 0-byte read), `FIONREAD` reports `lineEnds[last]` (`inq_canon()`), `readReadiness()` reports `ReadableEvents` for empty EOF lines, `VERASE`/`VWERASE`/`VKILL`/`VREPRINT` stay within `readBuf[canonHead:]`, and `ICANON` toggles preserve buffered bytes while `flushForSignal` and `TCFLSH` clear `lineEnds`.
- **Tab erase (t002).** `lineDiscipline.canonColumn` snapshots the cursor column (from `outQueue`'s `l.column` plus pending `echoBuf`) when a canonical line begins or is reprinted; `eraseTab` walks `readBuf[canonHead:]` before the erased tab (handling nested tabs, control chars under `ECHOCTL`/`-ECHOCTL`, and `IUTF8` continuation bytes) and emits `8 - col%8` backspaces. `outputQueueTransformer` also resets `l.column` on `ONLCR` newlines, skips advancing `l.column` on control characters, and expands `XTABS` to the actual column distance.
- **`ECHOPRT` (t003).** `lineDiscipline.erasing` tracks an open `\…/` hardcopy erase sequence: the first erase emits `\`, each erased character (including control chars, tabs, and multi-byte UTF-8 runes) is echoed via `echoChar`, and `finishErasing()` emits `/` on typing a non-terminating character, `VLNEXT`, `VREPRINT`, or non-`ECHOKE` `VKILL`. Signal flush, `TCIFLUSH`, and leaving `ICANON` clear `erasing` silently, while terminating characters leave `erasing` open until the next non-terminating character, matching Linux `n_tty`.
- **`TCXONC` / `tcflow()` (t004).** Added `TCXONC`, `TCOOFF`, `TCOON`, `TCIOFF`, `TCION` to `pkg/abi/linux/ioctl.go` and wired `tcFlow` on both `replica.go` and `master.go`. Replica `TCOOFF`/`TCOON` use a dedicated `tcoStopped` flag (so `VSTART`, `IXANY`, signals, and clearing `IXON` never override `TCOOFF`, and `TCOON` resumes only when `TCOOFF` was active), emit `TIOCPKT_STOP`/`TIOCPKT_START`, and wake blocked writers regardless of `IXON`. Master `TCOOFF`/`TCOON` stop/start master writes (`masterStopped`, no `TIOCPKT`). `TCIOFF`/`TCION` send `VSTOP`/`VSTART` (raw on replica output, or into the input queue from master) unless disabled or stopped. Invalid actions return `EINVAL`.

**Local verification (t001–t004, t011).** Actions remain disabled on `bex-co/gvisor`, so every check ran locally in Docker (`m574-cxx:latest`, Linux 7.0.12, arm64, `runsc --platform=systrap`, `golang:1.26.10`):

- **Native Linux 7.0.12:** all 16 new tests (`*Typeahead*:*EraseTab*:*EchoPrt*:*TcFlow*`) passed.
- **Patched `runsc` (`e90ac895dca8`):** full release `pty_test` passed, **160 of 160** (1 skipped: `SetTTYDifferentSession` under `CAP_SYS_ADMIN`).
- **Revert-and-fail checks on `runsc`:**
  - Without t001 (`release-20260622.0-bex.2`): **7 of 7** `*Typeahead*` tests failed (`0 passed`).
  - Without t002 (`t001` only): **3 of 3** `*EraseTab*` tests failed (`0 passed`).
  - Without t003 (`t001+t002`): **3 of 3** `*EchoPrt*` tests failed (`0 passed`).
  - Without t004 (`t001+t002+t003`): **3 of 3** `*TcFlow*` tests failed (`0 passed`).
- **Go unit tests (`pkg/sentry/fsimpl/devpts`):** all passed (`TestSimpleMasterToReplica`, `TestEchoDeadlock`, `TestEndOfFileHandling`, `TestSignalFlushesOutputMasterHasNoRoomFor`, `TestEcho` including 10 new tab-erase and `ECHOPRT` cases, `TestFlowControl`, `TestCanonicalTypeahead`, `TestTCFlow`).
- **`checklocks`:** 0 lock errors on `bex/line-discipline-typeahead`'s `pkg/sentry/fsimpl/devpts` (only the advisory annotations upstream master already prints).

**Probe extension (t005).** `scripts/pty-signal-echo-probe.py` gained four cases (`tab erase`, `echoprt`, `canonical typeahead ctrl-c`, `tcflow TCOOFF/TCOON`; 12 total) in `33d658b497`:

- Native Linux 7.0.12: `0 of 12 cases differ from Linux`.
- Patched `runsc` (`bex.3` arm64): `0 of 12 cases differ from Linux`.
- Previous `runsc` (`bex.2` arm64): `4 of 12 cases differ from Linux` (all 4 new cases failed: `tab erase`, `echoprt`, `canonical typeahead ctrl-c`, `tcflow TCOOFF/TCOON`).
- macOS 25.5.0 note: `tab erase` and `canonical typeahead ctrl-c` match Linux; `echoprt` (BSD skips `\…/` when `ECHOE` is set) and `tcflow` (BSD buffers writes rather than returning `EAGAIN`) differ from Linux.
- `scripts/ci_test_inputs.py`, `scripts/clusterapi-validate.sh`, and `scripts/capi-sandbox-bootstrap.test.sh` all passed.

**Build, pin, and surge-first pool roll (t007).** Published [release-20260622.0-bex.3](https://github.com/bex-co/gvisor/releases/tag/release-20260622.0-bex.3) from `google/gvisor` `go` branch `d10071d635665b840936420353a489ca5f9f250d` + `bex-go-tree.patch` with Go 1.26.10 (`CGO_ENABLED=0 GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags '-X gvisor.dev/gvisor/runsc/version.version=e90ac895dca8' -o runsc ./runsc`). Two independent builds (warm cache and clean checkout + empty cache) produced the exact same SHA-512:

- `runsc`: `1dcb247c44c2ac2e43ef0b0690906b4e0f1809693beb80fed87bd8a9562b62af0e4eacc3c7ac2ef0a8c1b3d906beadd0ecdbbde6a939a5fe4a8deab9e0381a2f` (`runsc --version`: `e90ac895dca8`).
- `containerd-shim-runsc-v1`: `6670b23959486010b727143678d6c3d915b00c83efa4bd5642600efcbc071072747e4bd2d6bd8dedce279c871be4233f427d7f855d1c94f3770616cc92d4049a` (`bex.1` binary, unchanged).
- Pinned in `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` in commit `33d658b497`. `.github/workflows/sandbox-runsc-pty.yml` run [45249972483](https://github.com/bex-co/bex/actions/runs/45249972483) passed on `ubuntu-latest` (x86_64) against `release-20260622.0-bex.3`.
- Surge-first production roll (2026-11-23 UTC):
  1. Applied `KubeadmConfigTemplate/bex-sandbox` (`kubectl diff` changed only `GVURL` and `RUNSC_SHA512`).
  2. Scaled `MachineDeployment/bex-sandbox` to 2; `bex-sandbox-mzp4j-7mcj4` (`2.31.42.36`) reached `Ready` in 78 s; verified both `sha512sum` digests and `runsc version e90ac895dca8` on the node.
  3. Cordoned `bex-sandbox-mzp4j-tssmk` (0 running tenant sandbox pods on it, so no session was interrupted).
  4. Annotated `machine/bex-sandbox-mzp4j-tssmk` with `cluster.x-k8s.io/delete-machine=yes` and scaled back to 1. Only `bex-sandbox-mzp4j-7mcj4` remains, and `scripts/capi-template-drift.sh` reports `no drift`.

**Live check (t008).** Signed in as QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with `bex` CLI v0.9.0; created sandbox `sbx-dc29cr6dfl0s73a7egi0` (image `qa-20261121-m545-py`, pod `b9f1a813-2f54-4c7b-b543-4b92b2f3560f-0`), placed on `bex-sandbox-mzp4j-7mcj4`, kernel `4.19.0-gvisor`:

- `scripts/pty-signal-echo-probe.py`: `4.19.0-gvisor: 0 of 12 cases differ from Linux`.
- 29-case line-discipline harness (typeahead `c6a`/multi-line/EOF/partial-read/bounded-edit/`VSTOP`, tab erase across columns/prompt/ctrl/`VKILL`/`IUTF8`, `ECHOPRT` across `VERASE`/`VWERASE`/`VKILL`/`VREPRINT`/`VLNEXT`/newline/signal-flush/`NOFLSH`, `tcflow()` across replica/master/`TIOCPKT`/`TCIOFF`/`TCION`/`EINVAL`): **0 of 29 cases differed** from native Linux 7.0.12.
- Cleanup: terminated `sbx-dc29cr6dfl0s73a7egi0`, verified `tea-daif693dqjvc73e7as3g-sandbox` holds 0 pods, ran `bex logout`, and revoked both QA Kratos sessions.

**Render parity and docs (t009).** Replayed canonical typeahead + Ctrl-C directly through `pods/exec` with a TTY (`kubectl exec -it`) on `b9f1a813-2f54-4c7b-b543-4b92b2f3560f-0` (a child blocking `SIGINT` without reading stdin): typing `ls\n` echoed `ls\r\n`, `ab` echoed `ab`, and `\x03` echoed `^C` and delivered `SIGINT` immediately with empty stdin (`GOT SIGINT \r\n`, exit 0). Updated [ADR104 § Terminals in a sandbox](../../../../docs/ADR104-sandbox-render-parity.md#terminals-in-a-sandbox-w4m545-w4m574-w4m598) and [ADR018](../../../../docs/ADR018-render-parity.md) (`Terminal line discipline in a sandbox` now `✅`).

**Simplify (t010).** Reviewed both the `bex-co/bex` diff (`scripts/pty-signal-echo-probe.py`, `sandbox-pool.yaml`) and the `bex-co/gvisor` diff (`queue.go`, `line_discipline.go`, `master.go`, `replica.go`, `ioctl.go`). Caught `termios.error` cleanly in `tcflow_child()` so unpatched runsc prints `ERR 25` instead of a traceback; kept `advanceColumn` and `outputQueueTransformer` separate to avoid interleaving `sizeBudget` checks.

## Parked 2026-11-23

What remains is t006 (upstream `gvisor/gvisor` PRs requiring human DCO sign-off) and t012 (closeout after t006). gVisor's [CONTRIBUTING.md](https://github.com/gvisor/gvisor/blob/master/CONTRIBUTING.md) requires a human DCO `Signed-off-by` on every commit and forbids AI tools from adding it. Steps for the user (stacked on `w4/m574`'s `b66db2b09`):

1. Review `bex-co/gvisor@bex/line-discipline-typeahead` (`72ca9395b`): four commits on `b66db2b09`, each message written as a standalone PR description with no AI attribution or `Signed-off-by`.
2. Sign them off (`git rebase --signoff b66db2b09`, or rebase from `eb263e1f4` to sign m545 + m574 + m598 in one pass) and push.
3. Open one pull request per fix to `gvisor/gvisor`, in order: `bex/line-discipline-6-typeahead` (`98933f332`), `bex/line-discipline-7-tab-erase` (`dc017a7c9`), `bex/line-discipline-8-echoprt` (`f47b1fdcb`), and `bex/line-discipline-9-tcxonc` (`72ca9395b`).
4. Then a worker records the PR links here (t006) and closes the milestone (t012). The entire production half of the DoD already holds on `release-20260622.0-bex.3`.
