# w4 · m598 — A sandbox terminal processes typed-ahead input at once, erases tabs, supports ECHOPRT and `tcflow()`, as Linux does

**Worker:** worker4 **Goal:** a PTY inside a bex sandbox handles canonical typeahead, tab erasure, `ECHOPRT` and `tcflow()` (`TCXONC`) the way Linux's `n_tty` line discipline does, closing the remaining differences left by `w4/m574`. Typing a line ahead while a program runs and then pressing Ctrl-C echoes and interrupts at once instead of waiting for a read; erasing a tab moves the cursor back to where the tab started; `ECHOPRT` echoes `\…/`; and `tcflow()` stops and starts output or sends `VSTOP`/`VSTART`. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est   | depends_on               |
| ---- | ---------------------------------------------------------------------------------- | ----- | ------------------------ |
| t001 | Canonical typeahead: multi-line read buffer with immediate input processing        | 1h30m | —                        |
| t002 | Erase a tab from the screen as Linux's `ECHO_OP_ERASE_TAB` does                    | 45m   | t001                     |
| t003 | Implement `ECHOPRT` hardcopy erase echo (`\…/`)                                    | 45m   | t001                     |
| t004 | Implement `TCXONC` (`tcflow()`) ioctl                                              | 45m   | t001                     |
| t005 | Extend the PTY probe and runsc CI check to typeahead, tab erase, ECHOPRT and flow  | 30m   | t001, t002, t003, t004   |
| t006 | Send the fixes upstream to gVisor                                                  | 30m   | t001, t002, t003, t004   |
| t007 | Build `release-20260622.0-bex.3`, pin it and roll the sandbox pool                 | 1h    | t001, t002, t003, t004, t005 |
| t008 | Live check: a production sandbox PTY matches Linux on typeahead and remaining gaps | 30m   | t007                     |
| t009 | Render parity: Render Sandboxes and Devin's terminals, and update ADR104 / ADR018  | 20m   | t008                     |
| t010 | Simplify                                                                           | 15m   | t009                     |
| t011 | Test coverage                                                                      | 30m   | t009                     |
| t012 | Closeout                                                                           | 15m   | t006, t010, t011         |

## Definition of done

- Each of the four fixes (t001–t004) has syscall tests in the fork's `test/syscalls/linux/pty.cc` (plus `devpts` unit tests where helpful). Each test passes on native Linux and on the patched `runsc`, and fails on `runsc` with only that fix reverted.
- On a production bex sandbox placed on a node running the new pin, `scripts/pty-signal-echo-probe.py` (extended by t005) reports 0 cases differing from Linux, including:
  - canonical typeahead: writing `ls\n`, `ab`, `\x03` with no reader echoes `ls\r\n`, `ab`, `^C`, delivers `SIGINT`, and flushes both lines;
  - tab erase, `ECHOPRT`, and `tcflow()` (`TCOOFF`/`TCOON`).
- `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` pins a `bex-co/gvisor` `release-20260622.0-bex.<n>` (or a stock release carrying every fix) by sha512, with binaries outside git, and every sandbox node runs it with no CAPI template drift.
- `.github/workflows/sandbox-runsc-pty.yml` fails against the `release-20260622.0-bex.2` pin and passes against the new one.
- Each fix has an upstream `gvisor/gvisor` pull request linked here (t006).
- [ADR104 § Terminals in a sandbox](../../../docs/ADR104-sandbox-render-parity.md#terminals-in-a-sandbox-w4m545-w4m574) and [ADR018](../../../docs/ADR018-render-parity.md)'s sandbox terminal line discipline row drop every fixed difference.

## Source + Goal linkage

- **Source:** inbox note w4/723 ([`source-723.md`](source-723.md)), filed by `w4/m574` on 2026-11-22 after its 71-case line-discipline harness found 3 remaining differences against native Linux (typeahead, tab erase, `ECHOPRT`) plus missing `tcflow()` (`TCXONC`). Promoted by `/loopx w4` on 2026-11-23: four gVisor fixes with syscall tests, an upstream DCO step, a fork release build, a surge-first production pool roll, a live check, and ADR updates exceed an inbox note's ~1h budget across multiple tasks (same shape as `w4/m574`).
- **Premise checked 2026-11-23** in `bex-co/gvisor@bex/line-discipline` (`b66db2b09`, current release `release-20260622.0-bex.2` pinned in `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml:274`):
  - **No typeahead.** `pkg/sentry/fsimpl/devpts/line_discipline.go:650` still returns early (`if l.termios.LEnabled(linux.ICANON) && q.readable { return 0, false }`), and `queue.go:49,68,108,155-165` holds a single `readBuf` + `readable` boolean.
  - **No tab erase.** `line_discipline.go:795-796` has `else if toErase == '\t' { // Not implemented }`.
  - **No `ECHOPRT`.** `line_discipline.go:789-790` has `if l.termios.LEnabled(linux.ECHOPRT) { // Not implemented }`.
  - **No `tcflow()` (`TCXONC`).** Neither `pkg/sentry/fsimpl/devpts/master.go` (`:147-315`) nor `replica.go` (`:137-209`) handles `linux.TCXONC`.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)) and sandboxes ([ADR104](../../../docs/ADR104-sandbox-render-parity.md)) run behind gVisor ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md)). Devin runs a real Linux kernel, so every difference in `devpts` shows up as sandbox-only terminal bugs or failures in tenant tests and interactive shells.
- **Expected outcome:** typing ahead and hitting Ctrl-C interrupts a running program immediately and echoes as Linux does; erasing tabs, `ECHOPRT`, and `tcflow()` match Linux; and ADR018's terminal row no longer carries unsigned-off divergences.
- **Why now:** `w4/m574` just shipped `release-20260622.0-bex.2`, so the fork branches, local test harness, reproducible go-branch build recipe, and surge-first pool roll runbook are fresh. Bundling tab erase, `ECHOPRT`, and `TCXONC` with typeahead closes all remaining recorded line-discipline gaps in one pool roll and one upstream review pass.
- **Render parity:** included (t009). Sandbox and agent terminals face tenants; Devin (the coding-agent reference) runs Linux `n_tty`, and Render's `sandboxes exec` has no TTY (`w4/m545` t005).
